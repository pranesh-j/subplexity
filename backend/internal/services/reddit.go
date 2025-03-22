// File: backend/internal/services/reddit.go

package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"strings"
	"time"

	"github.com/pranesh-j/subplexity/internal/cache"
	"github.com/pranesh-j/subplexity/internal/models"
	"github.com/pranesh-j/subplexity/internal/utils"
)

// Constants for the Reddit service
const (
	redditOAuthBaseURL   = "https://oauth.reddit.com"
	redditPublicBaseURL  = "https://www.reddit.com"
	redditUserAgent      = "golang:com.subplexity.api:v1.0.0 (by /u/Pran_J)"
	defaultRequestLimit  = 25
	maxRequestLimit      = 100
	maxConcurrentQueries = 5
	initialRetryDelay    = 1 * time.Second
	maxRetryDelay        = 30 * time.Second
	maxRetries           = 3
)

// RedditServiceConfig contains configuration options for the Reddit service
type RedditServiceConfig struct {
	ClientID     string
	ClientSecret string
	UserAgent    string
	HttpClient   *http.Client
	CacheConfig  cache.Config
}

// RedditService handles interactions with the Reddit API
type RedditService struct {
	config       RedditServiceConfig
	auth         *RedditAuth
	resultCache  *cache.Cache
	rateLimiter  chan struct{}
	httpClient   *http.Client
}

// NewRedditService creates a new Reddit service instance
func NewRedditService(clientID, clientSecret string) *RedditService {
	// Default HTTP client with sensible timeouts
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:       20,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: false,
			MaxConnsPerHost:    10,
		},
	}

	// Default configuration
	config := RedditServiceConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		UserAgent:    redditUserAgent,
		HttpClient:   httpClient,
		CacheConfig:  cache.DefaultConfig(),
	}

	// Create auth manager
	auth := NewRedditAuth(clientID, clientSecret, redditUserAgent, httpClient)

	// Create result cache
	resultCache := cache.NewCache(config.CacheConfig)

	return &RedditService{
		config:       config,
		auth:         auth,
		resultCache:  resultCache,
		rateLimiter:  make(chan struct{}, maxConcurrentQueries),
		httpClient:   httpClient,
	}
}

// GetAuthStatus returns the current authentication status
func (s *RedditService) GetAuthStatus() map[string]interface{} {
	return s.auth.GetAuthStatus()
}

// SearchReddit performs a search on Reddit based on the provided query and search mode
func (s *RedditService) SearchReddit(ctx context.Context, query string, searchMode string, limit int) ([]models.SearchResult, error) {
	// Validate and normalize parameters
	if query == "" {
		return nil, errors.New("search query cannot be empty")
	}

	if limit <= 0 {
		limit = defaultRequestLimit
	} else if limit > maxRequestLimit {
		limit = maxRequestLimit
	}

	// Log the search request
	log.Printf("Starting Reddit search for query: '%s', mode: '%s', limit: %d", query, searchMode, limit)

	// Check cache first
	cacheKey := fmt.Sprintf("search:%s:%s:%d", query, searchMode, limit)
	if cachedResults, found := s.resultCache.Get(cacheKey); found {
		log.Printf("Cache hit for query: '%s'", query)
		return cachedResults.([]models.SearchResult), nil
	}

	// Parse query to extract intent and parameters
	params := utils.ParseQuery(query)

	// Determine search strategy based on intent and mode
	var results []models.SearchResult
	var err error

	// Convert searchMode to search type if specified
	searchType := ""
	switch searchMode {
	case "Posts":
		searchType = "link"
	case "Comments":
		searchType = "comment"
	case "Communities":
		searchType = "sr"
	}

	// Override params based on search mode
	if searchType != "" {
		params.SortBy = "relevance" // Default sort for specific content types
	}

	// Execute the appropriate search strategy
	switch {
	case searchType == "sr" || params.Intent == utils.SubredditIntent:
		// Search for subreddits
		results, err = s.searchSubreddits(ctx, params, limit)
	case searchType == "comment" || params.Intent == utils.CommentIntent:
		// Search for comments
		results, err = s.searchComments(ctx, params, limit)
	case params.Intent == utils.UserIntent:
		// Search for user content
		results, err = s.searchUserContent(ctx, params, limit)
	case params.Intent == utils.TrendingIntent:
		// Search for trending content
		results, err = s.searchTrending(ctx, params, limit)
	default:
		// General search - try multiple strategies in parallel
		results, err = s.parallelSearch(ctx, params, searchType, limit)
	}

	if err != nil {
		log.Printf("Search error: %v", err)
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Process and score results
	processedResults := s.processSearchResults(params, results, limit)

	// Cache the processed results if we have any
	if len(processedResults) > 0 {
		s.resultCache.Set(cacheKey, processedResults)
	}

	log.Printf("Search completed, returning %d results", len(processedResults))
	return processedResults, nil
}

// parallelSearch executes multiple search strategies in parallel
func (s *RedditService) parallelSearch(ctx context.Context, params utils.QueryParams, searchType string, limit int) ([]models.SearchResult, error) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Create channels for results and errors
	resultChan := make(chan []models.SearchResult, 3)
	errorChan := make(chan error, 3)
	searchCount := 0

	// Launch search functions based on query intent
	// Always search posts, as they're most common
	searchCount++
	go func() {
		results, err := s.searchPosts(ctx, params, searchType, limit)
		if err != nil {
			errorChan <- err
		} else {
			resultChan <- results
		}
	}()

	// Add comment search if relevant to the query
	if searchType == "" || searchType == "comment" {
		searchCount++
		go func() {
			results, err := s.searchComments(ctx, params, limit/2) // Lower limit for comments
			if err != nil {
				errorChan <- err
			} else {
				resultChan <- results
			}
		}()
	}

	// Add subreddit search if relevant
	if searchType == "" || searchType == "sr" {
		searchCount++
		go func() {
			results, err := s.searchSubreddits(ctx, params, limit/2) // Lower limit for subreddits
			if err != nil {
				errorChan <- err
			} else {
				resultChan <- results
			}
		}()
	}

	// Collect results from all searches
	var allResults []models.SearchResult
	var lastErr error
	resultsReceived := 0
	for resultsReceived < searchCount {
		select {
		case <-ctx.Done():
			// Context timeout or cancellation
			if lastErr == nil {
				lastErr = ctx.Err()
			}
			resultsReceived = searchCount // Exit the loop
		case results := <-resultChan:
			allResults = append(allResults, results...)
			resultsReceived++
		case err := <-errorChan:
			if lastErr == nil {
				lastErr = err
			}
			resultsReceived++
		}
	}

	// Return results even if there were some errors
	if len(allResults) > 0 {
		return allResults, nil
	}

	// Only return error if we have no results
	if lastErr != nil {
		return nil, lastErr
	}

	return allResults, nil
}

// searchPosts searches for posts that match the query
func (s *RedditService) searchPosts(ctx context.Context, params utils.QueryParams, overrideType string, limit int) ([]models.SearchResult, error) {
	// Build search URL
	searchType := "link"
	if overrideType != "" {
		searchType = overrideType
	}

	// Build search query
	q := params.OriginalQuery
	if len(params.Subreddits) > 0 {
		// Add subreddit restriction
		q = fmt.Sprintf("%s subreddit:%s", q, params.Subreddits[0])
	}

	// Build query parameters
	queryParams := url.Values{}
	queryParams.Set("q", q)
	queryParams.Set("type", searchType)
	queryParams.Set("limit", fmt.Sprintf("%d", limit))
	queryParams.Set("sort", params.SortBy)
	queryParams.Set("t", params.TimeFrame)

	// Make request - using path only
	endpoint := fmt.Sprintf("/search.json?%s", queryParams.Encode())
	return s.executeSearchRequest(ctx, endpoint)
}

// searchComments searches for comments that match the query
func (s *RedditService) searchComments(ctx context.Context, params utils.QueryParams, limit int) ([]models.SearchResult, error) {
	// Build search query
	q := params.OriginalQuery
	if len(params.Subreddits) > 0 {
		// Add subreddit restriction
		q = fmt.Sprintf("%s subreddit:%s", q, params.Subreddits[0])
	}

	// Build query parameters
	queryParams := url.Values{}
	queryParams.Set("q", q)
	queryParams.Set("type", "comment")
	queryParams.Set("limit", fmt.Sprintf("%d", limit))
	queryParams.Set("sort", params.SortBy)
	queryParams.Set("t", params.TimeFrame)

	// Make request - using path only
	endpoint := fmt.Sprintf("/search.json?%s", queryParams.Encode())
	return s.executeSearchRequest(ctx, endpoint)
}

// searchSubreddits searches for subreddits that match the query
func (s *RedditService) searchSubreddits(ctx context.Context, params utils.QueryParams, limit int) ([]models.SearchResult, error) {
	// Build search query
	q := params.OriginalQuery
	for _, sr := range params.Subreddits {
		// If subreddit explicitly mentioned, search for it directly
		q = sr
		break
	}

	// Build query parameters
	queryParams := url.Values{}
	queryParams.Set("q", q)
	queryParams.Set("limit", fmt.Sprintf("%d", limit))

	// Make request - using path only
	endpoint := fmt.Sprintf("/subreddits/search.json?%s", queryParams.Encode())
	return s.executeSearchRequest(ctx, endpoint)
}

// searchUserContent searches for content from specific users
func (s *RedditService) searchUserContent(ctx context.Context, params utils.QueryParams, limit int) ([]models.SearchResult, error) {
	if len(params.Authors) == 0 {
		return nil, errors.New("no authors specified for user search")
	}

	// Get content from the first mentioned author
	author := params.Authors[0]
	
	// Build query parameters
	queryParams := url.Values{}
	queryParams.Set("limit", fmt.Sprintf("%d", limit))
	queryParams.Set("sort", params.SortBy)
	queryParams.Set("t", params.TimeFrame)

	// Make request - using path only
	endpoint := fmt.Sprintf("/user/%s.json?%s", author, queryParams.Encode())
	return s.executeSearchRequest(ctx, endpoint)
}

// searchTrending searches for trending content
func (s *RedditService) searchTrending(ctx context.Context, params utils.QueryParams, limit int) ([]models.SearchResult, error) {
	// Determine relevant subreddits based on keywords
	var subreddits []string
	if len(params.Subreddits) > 0 {
		subreddits = params.Subreddits
	} else {
		// Find subreddits via search first
		srParams := params
		srResults, err := s.searchSubreddits(ctx, srParams, 3)
		if err == nil && len(srResults) > 0 {
			for _, result := range srResults {
				if result.Type == "subreddit" {
					subreddits = append(subreddits, result.Subreddit)
				}
			}
		}

		// If no subreddits found, use default popular ones
		if len(subreddits) == 0 {
			subreddits = []string{"popular", "all"}
		}
	}

	// Limit to top 3 subreddits for efficiency
	if len(subreddits) > 3 {
		subreddits = subreddits[:3]
	}

	// Determine sort method
	sort := "hot" // Default for trending
	if params.SortBy != "relevance" {
		sort = params.SortBy
	}

	// Create wait group and mutex for concurrent fetching
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allResults []models.SearchResult
	
	// Fetch from each subreddit
	for _, sr := range subreddits {
		wg.Add(1)
		go func(subreddit string) {
			defer wg.Done()

			// Build query parameters
			queryParams := url.Values{}
			queryParams.Set("limit", fmt.Sprintf("%d", limit/len(subreddits)))
			queryParams.Set("t", params.TimeFrame)

			// Make request - using path only
			endpoint := fmt.Sprintf("/r/%s/%s.json?%s", subreddit, sort, queryParams.Encode())
			results, err := s.executeSearchRequest(ctx, endpoint)
			if err != nil {
				log.Printf("Error fetching trending from r/%s: %v", subreddit, err)
				return
			}

			// Filter results to match query keywords if needed
			var filtered []models.SearchResult
			for _, result := range results {
				// Only keep results that match query keywords
				if s.resultMatchesKeywords(result, params.FilteredKeywords) {
					filtered = append(filtered, result)
				}
			}

			// Add to combined results
			if len(filtered) > 0 {
				mu.Lock()
				allResults = append(allResults, filtered...)
				mu.Unlock()
			}
		}(sr)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	return allResults, nil
}

// executeSearchRequest performs the actual HTTP request to the Reddit API
func (s *RedditService) executeSearchRequest(ctx context.Context, endpoint string) ([]models.SearchResult, error) {
	// Acquire rate limiter slot
	select {
	case s.rateLimiter <- struct{}{}:
		// Got the slot, continue
		defer func() { <-s.rateLimiter }()
	case <-ctx.Done():
		// Context cancelled while waiting for slot
		return nil, ctx.Err()
	}

	// Get access token (authenticated requests are preferred)
	var token string
	token, err := s.auth.GetAccessToken(ctx)
	if err != nil {
		log.Printf("Warning: Failed to get access token, proceeding without authentication: %v", err)
		// Continue without token - Reddit allows anonymous access with rate limits
	}

	// Determine which base URL to use and construct the full URL
	var fullEndpoint string
	if strings.HasPrefix(endpoint, "http") {
		// If the endpoint is already a full URL, use it as is
		fullEndpoint = endpoint
	} else {
		// Otherwise, add the base URL
		if token != "" {
			// Use OAuth API for authenticated requests
			fullEndpoint = redditOAuthBaseURL + endpoint
			log.Printf("Using OAuth endpoint: %s", fullEndpoint)
		} else {
			// Use public API for unauthenticated requests
			fullEndpoint = redditPublicBaseURL + endpoint
			log.Printf("Using public endpoint: %s", fullEndpoint)
		}
	}
	
	// Create the request
	req, err := http.NewRequestWithContext(ctx, "GET", fullEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Set required headers
	log.Printf("Setting request headers - User-Agent: %s", s.config.UserAgent)
	req.Header.Set("User-Agent", s.config.UserAgent)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		log.Printf("Added Authorization header with token length: %d", len(token))
	}

	// Additional debug headers that help Reddit identify your app
	req.Header.Set("Accept", "application/json")

	// Log request details for debugging
	log.Printf("Making Reddit API request to: %s", fullEndpoint)

	// Verify headers are set
	if req.Header.Get("User-Agent") == "" {
		log.Printf("Warning: User-Agent header is empty, this will cause 403 errors")
		req.Header.Set("User-Agent", redditUserAgent) // Fallback
	}

	// Execute request with retries
	var resp *http.Response
	var retryDelay time.Duration = initialRetryDelay

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Add jitter to retry delay (±10%)
			jitter := time.Duration(float64(retryDelay) * (0.9 + 0.2*float64(time.Now().Nanosecond())/1e9))
			select {
			case <-time.After(jitter):
				// Wait completed
			case <-ctx.Done():
				// Context cancelled while waiting
				return nil, ctx.Err()
			}
			log.Printf("Retry attempt %d for request to %s", attempt, fullEndpoint)
		}

		// Clone request to ensure body is available for retry
		reqClone := req.Clone(ctx)

		// Make the request
		var reqErr error
		resp, reqErr = s.httpClient.Do(reqClone)
		
		// Check for context cancellation
		if reqErr != nil {
			if errors.Is(reqErr, context.Canceled) || errors.Is(reqErr, context.DeadlineExceeded) {
				return nil, ctx.Err()
			}
			
			if attempt == maxRetries {
				return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries, reqErr)
			}
			
			// Exponential backoff for next retry
			retryDelay = time.Duration(float64(retryDelay) * 1.5)
			if retryDelay > maxRetryDelay {
				retryDelay = maxRetryDelay
			}
			continue
		}

		// Check for HTTP error status
		if resp.StatusCode != http.StatusOK {
			// Read error body for better diagnostics
			bodyBytes, _ := io.ReadAll(resp.Body)
			errorDetails := string(bodyBytes)
			
			// Log detailed error info
			log.Printf("Reddit API error (%d): %s", resp.StatusCode, errorDetails)
			log.Printf("Request URL: %s", fullEndpoint)
			log.Printf("User-Agent: %s", req.Header.Get("User-Agent"))
			log.Printf("Has Auth Token: %v", token != "")
			
			resp.Body.Close()
			
			if resp.StatusCode == http.StatusUnauthorized {
				// Clear invalid token
				s.auth.Clear()
				
				if attempt == maxRetries {
					return nil, fmt.Errorf("received unauthorized status after %d attempts", maxRetries)
				}
				
				// Try to get new token for next attempt
				token, _ = s.auth.GetAccessToken(ctx)
				if token != "" {
					req.Header.Set("Authorization", "Bearer "+token)
				}
				
				retryDelay = initialRetryDelay // Reset delay for auth retry
				continue
			}
			
			if resp.StatusCode == http.StatusForbidden {
				if attempt == maxRetries {
					return nil, fmt.Errorf("access forbidden (403): %s", errorDetails)
				}
				
				// For 403, try different approach - modify URL for public API
				if token != "" && attempt == 1 {
					log.Printf("Switching to public API after 403 with authenticated request")
					// Try public API instead of OAuth API
					token = ""
					req.Header.Del("Authorization")
					
					// Reconstruct URL with public base
					if !strings.HasPrefix(endpoint, "http") {
						fullEndpoint = redditPublicBaseURL + endpoint
						req, _ = http.NewRequestWithContext(ctx, "GET", fullEndpoint, nil)
						req.Header.Set("User-Agent", s.config.UserAgent)
						req.Header.Set("Accept", "application/json")
					}
				}
				
				continue
			}
			
			// Handle rate limiting
			if resp.StatusCode == http.StatusTooManyRequests {
				if attempt == maxRetries {
					return nil, errors.New("rate limited by Reddit API")
				}
				
				// Use rate limit headers if available
				resetHeader := resp.Header.Get("X-Ratelimit-Reset")
				if resetHeader != "" {
					var seconds int
					if _, err := fmt.Sscanf(resetHeader, "%d", &seconds); err == nil && seconds > 0 {
						retryDelay = time.Duration(seconds) * time.Second
					}
				}
				
				continue
			}
			
			if attempt == maxRetries {
				return nil, fmt.Errorf("HTTP error: %d - %s", resp.StatusCode, errorDetails)
			}
			
			// Exponential backoff for next retry
			retryDelay = time.Duration(float64(retryDelay) * 1.5)
			if retryDelay > maxRetryDelay {
				retryDelay = maxRetryDelay
			}
			continue
		}

		// Success, break retry loop
		break
	}

	// Process response
	defer resp.Body.Close()
	
	// Read response body with max size limit to prevent abuse
	const maxResponseSize = 10 * 1024 * 1024 // 10MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	// Parse response into search results
	results, err := parseRedditResponse(body)
	if err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return results, nil
}

// Helper method to check if a result matches query keywords
func (s *RedditService) resultMatchesKeywords(result models.SearchResult, keywords []string) bool {
	if len(keywords) == 0 {
		return true // No keywords to match
	}

	// Check in title and content
	titleAndContent := result.Title + " " + result.Content
	titleAndContent = strings.ToLower(titleAndContent)

	for _, keyword := range keywords {
		if strings.Contains(titleAndContent, strings.ToLower(keyword)) {
			return true
		}
	}

	return false
}