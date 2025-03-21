package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/pranesh-j/subplexity/internal/models"
)

type RedditService struct {
	ClientID     string
	ClientSecret string
	UserAgent    string
	httpClient   *http.Client
}

func NewRedditService(clientID, clientSecret string) *RedditService {
	return &RedditService{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		UserAgent:    "Subplexity/1.0",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SearchReddit performs a search on Reddit based on the provided query and search mode
func (s *RedditService) SearchReddit(query string, searchMode string, limit int) ([]models.SearchResult, error) {
	if limit <= 0 {
		limit = 25 // Default limit
	}

	// Define the search URL based on the search mode
	searchType := "sr,link,user"
	switch searchMode {
	case "Posts":
		searchType = "link"
	case "Comments":
		searchType = "comment"
	case "Communities":
		searchType = "sr"
	}

	// Build the search URL
	searchURL := fmt.Sprintf("https://www.reddit.com/search.json?q=%s&type=%s&limit=%d&sort=relevance",
		url.QueryEscape(query), searchType, limit)

	// Create the request
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Set the User-Agent header
	req.Header.Set("User-Agent", s.UserAgent)

	// Execute the request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing request: %w", err)
	}
	defer resp.Body.Close()

	// Check the response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error response from Reddit API: %s", resp.Status)
	}

	// Read the entire response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	// Try to parse the response as a JSON object first
	var redditResponse map[string]interface{}
	err = json.Unmarshal(body, &redditResponse)
	if err != nil {
		// If that fails, try parsing as a JSON array
		var redditArray []interface{}
		err = json.Unmarshal(body, &redditArray)
		if err != nil {
			return nil, fmt.Errorf("error decoding response: %w", err)
		}
		
		// Handle array response
		var results []models.SearchResult
		for _, item := range redditArray {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			
			kind, _ := itemMap["kind"].(string)
			data, ok := itemMap["data"].(map[string]interface{})
			if !ok {
				continue
			}
			
			result := models.SearchResult{}
			
			// Extract common fields
			if id, ok := data["id"].(string); ok {
				result.ID = id
			}
			if subreddit, ok := data["subreddit"].(string); ok {
				result.Subreddit = subreddit
			}
			if author, ok := data["author"].(string); ok {
				result.Author = author
			}
			if url, ok := data["url"].(string); ok {
				result.URL = url
			}
			if createdUTC, ok := data["created_utc"].(float64); ok {
				result.CreatedUTC = int64(createdUTC)
			}
			if score, ok := data["score"].(float64); ok {
				result.Score = int(score)
			}
			
			// Handle different types
			switch kind {
			case "t1": // Comment
				result.Type = "comment"
				if title, ok := data["link_title"].(string); ok {
					result.Title = "Comment in r/" + result.Subreddit + ": " + title
				} else {
					result.Title = "Comment in r/" + result.Subreddit
				}
				if body, ok := data["body"].(string); ok {
					result.Content = body
				}
			case "t3": // Post
				result.Type = "post"
				if title, ok := data["title"].(string); ok {
					result.Title = title
				}
				if selfText, ok := data["selftext"].(string); ok {
					result.Content = selfText
				}
				if numComments, ok := data["num_comments"].(float64); ok {
					result.CommentCount = int(numComments)
				}
			case "t5": // Subreddit
				result.Type = "subreddit"
				if displayName, ok := data["display_name"].(string); ok {
					result.Title = "r/" + displayName
				}
				if description, ok := data["public_description"].(string); ok {
					result.Content = description
				}
				if subscribers, ok := data["subscribers"].(float64); ok {
					// Just store it somewhere, could add a field for this if needed
					_ = int(subscribers)
				}
			}
			
			results = append(results, result)
		}
		
		return results, nil
	} else {
		// Handle object response (standard Reddit API format)
		var results []models.SearchResult
		
		data, ok := redditResponse["data"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid response format: missing data field")
		}
		
		children, ok := data["children"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid response format: missing children array")
		}
		
		for _, child := range children {
			childMap, ok := child.(map[string]interface{})
			if !ok {
				continue
			}
			
			kind, _ := childMap["kind"].(string)
			childData, ok := childMap["data"].(map[string]interface{})
			if !ok {
				continue
			}
			
			result := models.SearchResult{}
			
			// Extract common fields
			if id, ok := childData["id"].(string); ok {
				result.ID = id
			}
			if subreddit, ok := childData["subreddit"].(string); ok {
				result.Subreddit = subreddit
			}
			if author, ok := childData["author"].(string); ok {
				result.Author = author
			}
			if url, ok := childData["url"].(string); ok {
				result.URL = url
			}
			if createdUTC, ok := childData["created_utc"].(float64); ok {
				result.CreatedUTC = int64(createdUTC)
			}
			if score, ok := childData["score"].(float64); ok {
				result.Score = int(score)
			}
			
			// Handle different types
			switch kind {
			case "t1": // Comment
				result.Type = "comment"
				if title, ok := childData["link_title"].(string); ok {
					result.Title = "Comment in r/" + result.Subreddit + ": " + title
				} else {
					result.Title = "Comment in r/" + result.Subreddit
				}
				if body, ok := childData["body"].(string); ok {
					result.Content = body
				}
			case "t3": // Post
				result.Type = "post"
				if title, ok := childData["title"].(string); ok {
					result.Title = title
				}
				if selfText, ok := childData["selftext"].(string); ok {
					result.Content = selfText
				}
				if numComments, ok := childData["num_comments"].(float64); ok {
					result.CommentCount = int(numComments)
				}
			case "t5": // Subreddit
				result.Type = "subreddit"
				if displayName, ok := childData["display_name"].(string); ok {
					result.Title = "r/" + displayName
				}
				if description, ok := childData["public_description"].(string); ok {
					result.Content = description
				}
				if subscribers, ok := childData["subscribers"].(float64); ok {
					// Just store it somewhere, could add a field for this if needed
					_ = int(subscribers)
				}
			}
			
			results = append(results, result)
		}
		
		return results, nil
	}
}