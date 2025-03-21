package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/pranesh-j/subplexity/internal/models"
)

// RedditService handles interactions with the Reddit API
type RedditService struct {
	ClientID      string
	ClientSecret  string
	UserAgent     string
	httpClient    *http.Client
	accessToken   string
	tokenExpiry   time.Time
	tokenLock     sync.Mutex
	lastErrorTime time.Time
	retryLimit    int
}

// NewRedditService creates a new Reddit service instance
func NewRedditService(clientID, clientSecret string) *RedditService {
	// Log credentials for debugging (first few chars only)
	if clientID != "" && clientSecret != "" {
		idPreview := clientID
		secretPreview := clientSecret
		if len(idPreview) > 5 {
			idPreview = idPreview[:5] + "..."
		}
		if len(secretPreview) > 5 {
			secretPreview = secretPreview[:5] + "..."
		}
		log.Printf("Initializing Reddit service with ID: %s, Secret: %s", idPreview, secretPreview)
	} else {
		log.Printf("WARNING: Initializing Reddit service with missing credentials!")
	}
	
	return &RedditService{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		UserAgent:    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		retryLimit: 3,
	}
}

// SearchReddit performs a search on Reddit based on the provided query and search mode
func (s *RedditService) SearchReddit(query string, searchMode string, limit int) ([]models.SearchResult, error) {
	log.Printf("Starting Reddit search for query: '%s', mode: '%s', limit: %d", query, searchMode, limit)
	
	if limit <= 0 {
		limit = 25 // Default limit
	}

	// Try multiple search methods
	results, err := s.searchWithDirectUrl(query, searchMode, limit)
	if err != nil || len(results) == 0 {
		log.Printf("First search method failed or returned no results, trying alternative method")
		return s.searchWithTrendingEndpoint(query, searchMode, limit)
	}
	
	return results, nil
}

// searchWithDirectUrl searches directly using Reddit's API endpoints
func (s *RedditService) searchWithDirectUrl(query string, searchMode string, limit int) ([]models.SearchResult, error) {
	// First, try the more reliable trending endpoint if the query implies trending
	if strings.Contains(strings.ToLower(query), "trend") || 
	   strings.Contains(strings.ToLower(query), "popular") ||
	   strings.Contains(strings.ToLower(query), "what's hot") {
		results, err := s.searchWithTrendingEndpoint(query, searchMode, limit)
		if err == nil && len(results) > 0 {
			return results, nil
		}
	}

	var searchURL string
	
	// Use search mode to modify search parameters
	switch searchMode {
	case "Posts":
		searchURL = fmt.Sprintf("https://www.reddit.com/search.json?q=%s&type=link&limit=%d&sort=relevance&t=day",
			url.QueryEscape(query), limit)
	case "Comments":
		searchURL = fmt.Sprintf("https://www.reddit.com/search.json?q=%s&type=comment&limit=%d&sort=relevance&t=day",
			url.QueryEscape(query), limit)
	case "Communities":
		searchURL = fmt.Sprintf("https://www.reddit.com/search.json?q=%s&type=sr&limit=%d&sort=relevance",
			url.QueryEscape(query), limit)
	default:
		// "All" search mode - try to be more specific
		if strings.Contains(strings.ToLower(query), "trend") || 
		   strings.Contains(strings.ToLower(query), "popular") {
			// For trending queries, use more relevant timing
			searchURL = fmt.Sprintf("https://www.reddit.com/search.json?q=%s&sort=hot&t=day&limit=%d",
				url.QueryEscape(query), limit)
		} else {
			searchURL = fmt.Sprintf("https://www.reddit.com/search.json?q=%s&limit=%d&sort=relevance",
				url.QueryEscape(query), limit)
		}
	}
	
	log.Printf("Search URL: %s", searchURL)

	// Create the request
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		log.Printf("Error creating search request: %v", err)
		return nil, fmt.Errorf("error creating search request: %w", err)
	}

	// Set the User-Agent header - VERY IMPORTANT FOR REDDIT
	req.Header.Set("User-Agent", s.UserAgent)
	
	// Execute the request
	log.Println("Sending search request to Reddit")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("Error executing search request: %v", err)
		return nil, fmt.Errorf("error executing search request: %w", err)
	}
	defer resp.Body.Close()

	// Check the response status
	log.Printf("Search response status: %s", resp.Status)
	
	// Read full response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return nil, fmt.Errorf("error reading response body: %w", err)
	}
	
	// Check for error responses
	if resp.StatusCode != http.StatusOK {
		log.Printf("Error from Reddit API (%d): %s", resp.StatusCode, string(bodyBytes))
		return nil, fmt.Errorf("error response from Reddit API: %s", resp.Status)
	}

	// Log a preview of the response
	preview := string(bodyBytes)
	if len(preview) > 500 {
		preview = preview[:500]
	}
	log.Printf("Response body preview: %s", preview)

	// Try to parse the response
	var redditResponse struct {
		Data struct {
			Children []struct {
				Kind string `json:"kind"`
				Data json.RawMessage `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	
	err = json.Unmarshal(bodyBytes, &redditResponse)
	if err != nil {
		log.Printf("Error parsing Reddit response: %v", err)
		return nil, fmt.Errorf("error parsing Reddit response: %w", err)
	}
	
	// Process the results
	var results []models.SearchResult
	
	for _, child := range redditResponse.Data.Children {
		// Use a generic structure for any Reddit item
		var genericData map[string]interface{}
		
		if err := json.Unmarshal(child.Data, &genericData); err != nil {
			log.Printf("Error parsing child data: %v", err)
			continue
		}
		
		// Create a result based on the type
		result := models.SearchResult{
			Type: getTypeFromKind(child.Kind),
			ID:   getString(genericData, "id"),
		}
		
		// Fill in common fields
		switch child.Kind {
		case "t1": // Comment
			result.Author = getString(genericData, "author")
			result.Content = getString(genericData, "body")
			result.Score = getInt(genericData, "score")
			result.Subreddit = getString(genericData, "subreddit")
			result.Title = "Comment in r/" + result.Subreddit
			result.CreatedUTC = getInt64(genericData, "created_utc")
			
			permalink := getString(genericData, "permalink")
			if permalink != "" {
				result.URL = "https://www.reddit.com" + permalink
			} else {
				result.URL = "https://www.reddit.com/r/" + result.Subreddit
			}
			
		case "t3": // Post
			result.Author = getString(genericData, "author")
			result.Title = getString(genericData, "title")
			result.Content = getString(genericData, "selftext")
			result.Score = getInt(genericData, "score")
			result.Subreddit = getString(genericData, "subreddit")
			result.CreatedUTC = getInt64(genericData, "created_utc")
			result.CommentCount = getInt(genericData, "num_comments")
			
			permalink := getString(genericData, "permalink")
			if permalink != "" {
				result.URL = "https://www.reddit.com" + permalink
			} else {
				result.URL = getString(genericData, "url")
			}
			
		case "t5": // Subreddit
			result.Title = "r/" + getString(genericData, "display_name")
			result.Subreddit = getString(genericData, "display_name")
			result.Content = getString(genericData, "public_description")
			result.Score = getInt(genericData, "subscribers")
			result.CreatedUTC = getInt64(genericData, "created_utc")
			result.URL = "https://www.reddit.com/r/" + getString(genericData, "display_name")
		}
		
		// Only add if we have at least a title
		if result.Title != "" {
			results = append(results, result)
		}
	}
	
	log.Printf("Found %d valid results", len(results))
	return results, nil
}

// searchWithTrendingEndpoint directly accesses Reddit's trending content
func (s *RedditService) searchWithTrendingEndpoint(query string, searchMode string, limit int) ([]models.SearchResult, error) {
    var subredditSources []string
    
    // Popular subreddits to check for trending content
    popularSubreddits := []string{
        "popular", "all", "news", "worldnews", "AskReddit", "technology", 
        "science", "gaming", "movies", "politics", "todayilearned",
    }
    
    // Select which subreddits to check based on query
    if strings.Contains(strings.ToLower(query), "tech") || 
       strings.Contains(strings.ToLower(query), "technology") {
        subredditSources = []string{"technology", "gadgets", "programming"}
    } else if strings.Contains(strings.ToLower(query), "gaming") || 
              strings.Contains(strings.ToLower(query), "game") {
        subredditSources = []string{"gaming", "games", "pcgaming"}
    } else if strings.Contains(strings.ToLower(query), "news") || 
              strings.Contains(strings.ToLower(query), "world") {
        subredditSources = []string{"news", "worldnews", "politics"}
    } else if strings.Contains(strings.ToLower(query), "science") {
        subredditSources = []string{"science", "askscience", "space"}
    } else {
        // Default to these for general trending
        subredditSources = []string{"popular", "all"}
        
        // Add some from the popular list
        for _, sr := range popularSubreddits {
            if strings.Contains(strings.ToLower(query), strings.ToLower(sr)) {
                subredditSources = []string{sr}
                break
            }
        }
    }
    
    var allResults []models.SearchResult
    
    // Search each subreddit in parallel
    var wg sync.WaitGroup
    var mu sync.Mutex
    
    for _, subreddit := range subredditSources {
        wg.Add(1)
        
        go func(sr string) {
            defer wg.Done()
            
            // Use different sorts for different queries
            sort := "hot" // Default to hot for trending content
            if strings.Contains(strings.ToLower(query), "new") {
                sort = "new"
            } else if strings.Contains(strings.ToLower(query), "top") {
                sort = "top"
            } else if strings.Contains(strings.ToLower(query), "best") {
                sort = "best"
            }
            
            // Use a smaller limit per subreddit
            srLimit := limit / len(subredditSources)
            if srLimit < 5 {
                srLimit = 5
            }
            
            timeframe := "day"  // Default to day
            if strings.Contains(strings.ToLower(query), "week") {
                timeframe = "week"
            } else if strings.Contains(strings.ToLower(query), "month") {
                timeframe = "month"
            } else if strings.Contains(strings.ToLower(query), "year") {
                timeframe = "year"
            } else if strings.Contains(strings.ToLower(query), "all time") {
                timeframe = "all"
            }
            
            endpoint := fmt.Sprintf("https://www.reddit.com/r/%s/%s.json?limit=%d&t=%s", 
                          sr, sort, srLimit, timeframe)
                          
            log.Printf("Fetching trending from: %s", endpoint)
            
            req, err := http.NewRequest("GET", endpoint, nil)
            if err != nil {
                log.Printf("Error creating trending request: %v", err)
                return
            }
            
            req.Header.Set("User-Agent", s.UserAgent)
            
            resp, err := s.httpClient.Do(req)
            if err != nil {
                log.Printf("Error executing trending request: %v", err)
                return
            }
            defer resp.Body.Close()
            
            if resp.StatusCode != http.StatusOK {
                log.Printf("Error from Reddit trending API: %s", resp.Status)
                return
            }
            
            bodyBytes, err := io.ReadAll(resp.Body)
            if err != nil {
                log.Printf("Error reading trending response: %v", err)
                return
            }
            
            var trendingResponse struct {
                Data struct {
                    Children []struct {
                        Kind string `json:"kind"`
                        Data json.RawMessage `json:"data"`
                    } `json:"children"`
                } `json:"data"`
            }
            
            err = json.Unmarshal(bodyBytes, &trendingResponse)
            if err != nil {
                log.Printf("Error parsing trending response: %v", err)
                return
            }
            
            var subredditResults []models.SearchResult
            
            for _, child := range trendingResponse.Data.Children {
                // Skip if not a post (for trending we want posts)
                if child.Kind != "t3" && searchMode != "Communities" && searchMode != "Comments" {
                    continue
                }
                
                var genericData map[string]interface{}
                
                if err := json.Unmarshal(child.Data, &genericData); err != nil {
                    continue
                }
                
                result := models.SearchResult{
                    Type: getTypeFromKind(child.Kind),
                    ID:   getString(genericData, "id"),
                }
                
                // Fill in fields based on type
                switch child.Kind {
                case "t1": // Comment
                    if searchMode == "Posts" {
                        continue // Skip comments in post mode
                    }
                    result.Author = getString(genericData, "author")
                    result.Content = getString(genericData, "body")
                    result.Score = getInt(genericData, "score")
                    result.Subreddit = getString(genericData, "subreddit")
                    result.Title = "Comment in r/" + result.Subreddit
                    result.CreatedUTC = getInt64(genericData, "created_utc")
                    
                    permalink := getString(genericData, "permalink")
                    if permalink != "" {
                        result.URL = "https://www.reddit.com" + permalink
                    } else {
                        result.URL = "https://www.reddit.com/r/" + result.Subreddit
                    }
                    
                case "t3": // Post
                    if searchMode == "Comments" || searchMode == "Communities" {
                        continue // Skip posts in comment or community mode
                    }
                    result.Author = getString(genericData, "author")
                    result.Title = getString(genericData, "title")
                    result.Content = getString(genericData, "selftext")
                    result.Score = getInt(genericData, "score")
                    result.Subreddit = getString(genericData, "subreddit")
                    result.CreatedUTC = getInt64(genericData, "created_utc")
                    result.CommentCount = getInt(genericData, "num_comments")
                    
                    permalink := getString(genericData, "permalink")
                    if permalink != "" {
                        result.URL = "https://www.reddit.com" + permalink
                    } else {
                        result.URL = getString(genericData, "url")
                    }
                    
                case "t5": // Subreddit
                    if searchMode == "Posts" || searchMode == "Comments" {
                        continue // Skip subreddits in post or comment mode
                    }
                    result.Title = "r/" + getString(genericData, "display_name")
                    result.Subreddit = getString(genericData, "display_name")
                    result.Content = getString(genericData, "public_description")
                    result.Score = getInt(genericData, "subscribers")
                    result.CreatedUTC = getInt64(genericData, "created_utc")
                    result.URL = "https://www.reddit.com/r/" + getString(genericData, "display_name")
                }
                
                // Only add if we have at least a title
                if result.Title != "" {
                    subredditResults = append(subredditResults, result)
                }
            }
            
            // Add these results to the overall results
            if len(subredditResults) > 0 {
                mu.Lock()
                allResults = append(allResults, subredditResults...)
                mu.Unlock()
            }
            
        }(subreddit)
    }
    
    // Wait for all goroutines to complete
    wg.Wait()
    
    log.Printf("Found %d trending results across %d subreddits", len(allResults), len(subredditSources))
    
    // If we have too many, truncate to the limit
    if len(allResults) > limit {
        allResults = allResults[:limit]
    }
    
    return allResults, nil
}

// Helper function to get string value from map with type safety
func getString(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if strVal, ok := val.(string); ok {
			return strVal
		}
	}
	return ""
}

// Helper function to get int value from map with type safety
func getInt(data map[string]interface{}, key string) int {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			// Try to parse string to int if needed
		}
	}
	return 0
}

// Helper function to get int64 value from map with type safety
func getInt64(data map[string]interface{}, key string) int64 {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case int64:
			return v
		case float64:
			return int64(v)
		case int:
			return int64(v)
		}
	}
	return 0
}

// Helper function to convert Reddit "kind" to our type
func getTypeFromKind(kind string) string {
	switch kind {
	case "t1":
		return "comment"
	case "t3":
		return "post"
	case "t5":
		return "subreddit"
	default:
		return "unknown"
	}
}