// File: backend/internal/services/reddit.go

package services

// Update the import section at the top of backend/internal/services/reddit.go

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"  // Add this for math functions
	"net/http"
	"net/url"
	"regexp" // Add this for regexp functions
	"sort"   // Add this for sorting functions
	"strconv" // Add this for string conversion
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


// Updated SearchReddit function in backend/internal/services/reddit.go
// This changes how it handles ranking queries like "top 5 TV shows right now"

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

    // Parse query to extract intent and parameters
    params := utils.ParseQuery(query)

    // Log the search request
    log.Printf("Starting Reddit search for query: '%s', mode: '%s', limit: %d", query, searchMode, limit)

    // Check cache with time sensitivity awareness
    cacheKey := fmt.Sprintf("search:%s:%s:%d", query, searchMode, limit)
    if !params.IsTimeSensitive {
        // Use normal cache for non-time-sensitive queries
        if cachedResults, found := s.resultCache.Get(cacheKey); found {
            log.Printf("Cache hit for query: '%s'", query)
            return cachedResults.([]models.SearchResult), nil
        }
    } else {
        // Use short TTL cache for time-sensitive queries
        if cachedResults, found := s.resultCache.GetWithTTL(cacheKey, 5*time.Minute); found {
            log.Printf("Short TTL cache hit for time-sensitive query: '%s'", query)
            return cachedResults.([]models.SearchResult), nil
        }
    }
    
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

    // Determine search strategy based on query characteristics
    var results []models.SearchResult
    var err error

    // Extract quantity from query if not already detected
    if params.QuantityRequested <= 0 {
        quantityRegex := regexp.MustCompile(`\b(top|best|worst)\s+(\d+)\b`)
        match := quantityRegex.FindStringSubmatch(strings.ToLower(query))
        if len(match) > 2 {
            quantity, _ := strconv.Atoi(match[2])
            params.QuantityRequested = quantity
            log.Printf("Detected quantity %d in query: '%s'", quantity, query)
        }
    }

    // Check for ranking/listing queries - Special handling for "top X" type queries
    if params.HasRankingAspect && params.QuantityRequested > 0 {
        log.Printf("Using enhanced ranking search for query: '%s' (quantity: %d)", query, params.QuantityRequested)
        results, err = s.enhanceSearchForRankingQueries(ctx, params, limit)
    } else if params.IsTimeSensitive {
        // For time-sensitive queries, we should look at hot/new content and recent posts
        results, err = s.timeAwareSearch(ctx, params, searchType, limit)
    } else if params.HasRankingAspect {
        // For ranking queries without specific quantity, focus on engagement metrics
        results, err = s.rankingFocusedSearch(ctx, params, searchType, limit)
    } else {
        // Execute the standard search strategy based on intent
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
    }

    if err != nil {
        log.Printf("Search error: %v", err)
        return nil, fmt.Errorf("search failed: %w", err)
    }

    // Process and score results
    processedResults := s.processSearchResults(params, results, limit)

    // Cache the processed results with appropriate TTL
    if len(processedResults) > 0 {
        if params.IsTimeSensitive {
            // Use shorter TTL for time-sensitive queries
            s.resultCache.SetWithTTL(cacheKey, processedResults, 15*time.Minute)
        } else {
            // Use default TTL for normal queries
            s.resultCache.Set(cacheKey, processedResults)
        }
    }

    log.Printf("Search completed, returning %d results", len(processedResults))
    return processedResults, nil
}


// timeAwareSearch handles time-sensitive queries by focusing on recent content
func (s *RedditService) timeAwareSearch(ctx context.Context, params utils.QueryParams, searchType string, limit int) ([]models.SearchResult, error) {
    // Create a context with timeout
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    // Create channels for results and errors
    resultChan := make(chan []models.SearchResult, 3)
    errorChan := make(chan error, 3)
    searchCount := 0

    // First strategy: Search posts with time-focused sorting
    searchCount++
    go func() {
        // Use "new" or "hot" sorting for time-sensitive queries
        timeParams := params
        if searchType == "" || searchType == "link" {
            timeParams.SortBy = "new"
            timeParams.TimeFrame = "day" // Focus on last 24 hours
            results, err := s.searchPosts(ctx, timeParams, searchType, limit)
            if err != nil {
                errorChan <- err
            } else {
                resultChan <- results
            }
        } else {
            resultChan <- nil // No results for this strategy
        }
    }()

    // Second strategy: Search subreddit "hot" sections
    searchCount++
    go func() {
        // Identify relevant subreddits for the query
        relevantSubreddits := []string{}
        
        // Try to find relevant subreddits first
        srParams := params
        srResults, err := s.searchSubreddits(ctx, srParams, 3)
        if err == nil && len(srResults) > 0 {
            for _, result := range srResults {
                if result.Type == "subreddit" {
                    relevantSubreddits = append(relevantSubreddits, result.Subreddit)
                }
            }
        }
        
        // If no specific subreddits found, use general ones for the query domain
        if len(relevantSubreddits) == 0 {
            // Add general subreddits based on query categories
            if len(params.QueryCategories) > 0 {
                for _, category := range params.QueryCategories {
                    switch category {
                    case "entertainment":
                        relevantSubreddits = append(relevantSubreddits, "television", "movies", "entertainment")
                    case "technology":
                        relevantSubreddits = append(relevantSubreddits, "technology", "gadgets", "programming")
                    case "gaming":
                        relevantSubreddits = append(relevantSubreddits, "gaming", "games", "pcgaming")
                    // Add more categories as needed
                    default:
                        // Use popular subreddits as fallback
                        if len(relevantSubreddits) == 0 {
                            relevantSubreddits = append(relevantSubreddits, "popular", "all")
                        }
                    }
                }
            } else {
                // Default to popular subreddits if no categories identified
                relevantSubreddits = append(relevantSubreddits, "popular", "all")
            }
        }
        
        // Limit to a reasonable number of subreddits
        if len(relevantSubreddits) > 3 {
            relevantSubreddits = relevantSubreddits[:3]
        }
        
        // Search each subreddit's hot section
        var combinedResults []models.SearchResult
        var mu sync.Mutex
        var wg sync.WaitGroup
        
        for _, sr := range relevantSubreddits {
            wg.Add(1)
            go func(subreddit string) {
                defer wg.Done()
                
                // Search hot section with appropriate time frame
                queryParams := url.Values{}
                queryParams.Set("limit", fmt.Sprintf("%d", limit/len(relevantSubreddits)))
                queryParams.Set("t", params.TimeFrame)
                
                endpoint := fmt.Sprintf("/r/%s/hot.json?%s", subreddit, queryParams.Encode())
                hotResults, err := s.executeSearchRequest(ctx, endpoint)
                if err != nil {
                    return
                }
                
                // Filter results to match query keywords
                var filtered []models.SearchResult
                for _, result := range hotResults {
                    // Only keep results that match query keywords
                    if s.resultMatchesKeywords(result, params.FilteredKeywords) {
                        filtered = append(filtered, result)
                    }
                }
                
                mu.Lock()
                combinedResults = append(combinedResults, filtered...)
                mu.Unlock()
            }(sr)
        }
        
        wg.Wait()
        resultChan <- combinedResults
    }()

    // Third strategy: Standard search with time parameters
    searchCount++
    go func() {
        standardParams := params
        standardParams.TimeFrame = "week" // Focus on last week at most
        results, err := s.parallelSearch(ctx, standardParams, searchType, limit)
        if err != nil {
            errorChan <- err
        } else {
            resultChan <- results
        }
    }()

    // Collect results
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
            if results != nil {
                allResults = append(allResults, results...)
            }
            resultsReceived++
        case err := <-errorChan:
            if lastErr == nil {
                lastErr = err
            }
            resultsReceived++
        }
    }

    // Return results even if some strategies failed
    if len(allResults) > 0 {
        return allResults, nil
    }

    // Only return error if we have no results
    if lastErr != nil {
        return nil, lastErr
    }

    return allResults, nil
}

// rankingFocusedSearch handles queries focusing on rankings (top, best, etc.)
func (s *RedditService) rankingFocusedSearch(ctx context.Context, params utils.QueryParams, searchType string, limit int) ([]models.SearchResult, error) {
    // Create a context with timeout
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    // Create channels for results and errors
    resultChan := make(chan []models.SearchResult, 3)
    errorChan := make(chan error, 3)
    searchCount := 0

    // First strategy: Search posts with explicit ranking terms
    searchCount++
    go func() {
        // Add ranking terms to the query if not already present
        rankingTerms := []string{"top", "best", "popular", "ranking", "ranked"}
        expandedQueries := []string{params.OriginalQuery}
        
        // Create expanded queries with ranking terms
        for _, term := range rankingTerms {
            if !strings.Contains(strings.ToLower(params.OriginalQuery), term) {
                expandedQueries = append(expandedQueries, 
                    fmt.Sprintf("%s %s", term, params.OriginalQuery))
            }
        }
        
        // Limit to a reasonable number of queries
        if len(expandedQueries) > 3 {
            expandedQueries = expandedQueries[:3]
        }
        
        // Execute each query
        var combinedResults []models.SearchResult
        var mu sync.Mutex
        var wg sync.WaitGroup
        
        for _, q := range expandedQueries {
            wg.Add(1)
            go func(query string) {
                defer wg.Done()
                
                // Create params for this specific query
                queryParams := utils.ParseQuery(query)
                queryParams.SortBy = "top"
                queryParams.TimeFrame = "all" // Start with all-time for rankings
                
                results, err := s.searchPosts(ctx, queryParams, searchType, limit/len(expandedQueries))
                if err != nil {
                    return
                }
                
                mu.Lock()
                combinedResults = append(combinedResults, results...)
                mu.Unlock()
            }(q)
        }
        
        wg.Wait()
        resultChan <- combinedResults
    }()

    // Second strategy: Look for high-engagement content in relevant subreddits
    searchCount++
    go func() {
        // Identify relevant subreddits based on query categories
        var relevantSubreddits []string
        
        // First try to find via search
        srResults, err := s.searchSubreddits(ctx, params, 3)
        if err == nil && len(srResults) > 0 {
            for _, result := range srResults {
                relevantSubreddits = append(relevantSubreddits, result.Subreddit)
            }
        }
        
        // If no results, use some popular general subreddits
        if len(relevantSubreddits) == 0 {
            relevantSubreddits = []string{"popular", "all"}
        }
        
        // Limit to a reasonable number
        if len(relevantSubreddits) > 3 {
            relevantSubreddits = relevantSubreddits[:3]
        }
        
        // Search top content in each subreddit
        var combinedResults []models.SearchResult
        var mu sync.Mutex
        var wg sync.WaitGroup
        
        for _, sr := range relevantSubreddits {
            wg.Add(1)
            go func(subreddit string) {
                defer wg.Done()
                
                // Get top posts from this subreddit
                queryParams := url.Values{}
                queryParams.Set("limit", fmt.Sprintf("%d", limit/len(relevantSubreddits)))
                queryParams.Set("t", "all") // All time for rankings
                
                endpoint := fmt.Sprintf("/r/%s/top.json?%s", subreddit, queryParams.Encode())
                topResults, err := s.executeSearchRequest(ctx, endpoint)
                if err != nil {
                    return
                }
                
                // Filter for relevance to query
                var filtered []models.SearchResult
                for _, result := range topResults {
                    if s.resultMatchesKeywords(result, params.FilteredKeywords) {
                        filtered = append(filtered, result)
                    }
                }
                
                mu.Lock()
                combinedResults = append(combinedResults, filtered...)
                mu.Unlock()
            }(sr)
        }
        
        wg.Wait()
        resultChan <- combinedResults
    }()

    // Third strategy: Standard search with top sorting
    searchCount++
    go func() {
        rankParams := params
        rankParams.SortBy = "top"
        
        // Determine appropriate time frame based on query
        if params.TimeFrame == "all" {
            // If no time specified, use a reasonable default (year)
            rankParams.TimeFrame = "year"
        }
        
        results, err := s.parallelSearch(ctx, rankParams, searchType, limit)
        if err != nil {
            errorChan <- err
        } else {
            resultChan <- results
        }
    }()

    // Collect results
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
            if results != nil {
                allResults = append(allResults, results...)
            }
            resultsReceived++
        case err := <-errorChan:
            if lastErr == nil {
                lastErr = err
            }
            resultsReceived++
        }
    }

    // Return results even if some strategies failed
    if len(allResults) > 0 {
        return allResults, nil
    }

    // Only return error if we have no results
    if lastErr != nil {
        return nil, lastErr
    }

    return allResults, nil
}

// enhanceSearchForRankingQueries specially handles ranking-based queries like "top 5 shows"
func (s *RedditService) enhanceSearchForRankingQueries(ctx context.Context, params utils.QueryParams, limit int) ([]models.SearchResult, error) {
    // Create a context with timeout
    ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
    defer cancel()

    log.Printf("Performing enhanced search for ranking query: '%s'", params.OriginalQuery)
    
    // Determine relevant categories based on query to find targeted subreddits
    relevantSubreddits := []string{}
    if len(params.QueryCategories) > 0 {
        // Map general categories to specific subreddits
        categorySubreddits := map[string][]string{
            "entertainment": {"television", "TVshows", "netflix", "hulu", "streaming", "BestOfStreamingVideo"},
            "gaming": {"gaming", "Games", "pcgaming", "PS5", "XboxSeriesX", "NintendoSwitch"},
            "technology": {"technology", "gadgets", "hardware", "software", "programming"},
            "finance": {"investing", "stocks", "personalfinance", "wallstreetbets"},
            "sports": {"sports", "nba", "nfl", "soccer", "formula1", "MMA"},
            "food": {"food", "Cooking", "recipes", "AskCulinary"},
            "travel": {"travel", "TravelHacks", "backpacking", "solotravel"},
        }
        
        // Add relevant subreddits based on categories
        for _, category := range params.QueryCategories {
            if subreddits, ok := categorySubreddits[category]; ok {
                relevantSubreddits = append(relevantSubreddits, subreddits...)
            }
        }
    }
    
    // If no categories matched or not enough subreddits, add general ones
    if len(relevantSubreddits) < 3 {
        // Search for relevant subreddits
        srParams := params
        srResults, err := s.searchSubreddits(ctx, srParams, 5)
        if err == nil && len(srResults) > 0 {
            for _, result := range srResults {
                relevantSubreddits = append(relevantSubreddits, result.Subreddit)
            }
        }
        
        // Still add some general subreddits if we didn't find enough
        if len(relevantSubreddits) < 3 {
            relevantSubreddits = append(relevantSubreddits, "AskReddit", "popular", "all")
        }
    }
    
    // Keep only unique subreddits and limit to a reasonable number
    relevantSubreddits = getUniqueItems(relevantSubreddits)
    if len(relevantSubreddits) > 5 {
        relevantSubreddits = relevantSubreddits[:5]
    }
    
    log.Printf("Targeting subreddits for ranking query: %v", relevantSubreddits)
    
    // Create multiple search strategies in parallel
    searchResults := make(chan []models.SearchResult, 5)
    errorResults := make(chan error, 5)
    searchCount := 0
    
    // Strategy 1: Search with explicit ranking terms in top-rated content
    searchCount++
    go func() {
        // For each target subreddit, search their top content
        var allResults []models.SearchResult
        var mu sync.Mutex
        var wg sync.WaitGroup
        
        for _, sr := range relevantSubreddits {
            wg.Add(1)
            go func(subreddit string) {
                defer wg.Done()
                
                // Build query parameters for top content in this subreddit
                queryParams := url.Values{}
                queryParams.Set("limit", fmt.Sprintf("%d", limit/len(relevantSubreddits)))
                
                // If time sensitive, use appropriate timeframe
                if params.IsTimeSensitive {
                    queryParams.Set("t", params.TimeFrame)
                } else {
                    queryParams.Set("t", "month") // Default to last month for ranking queries
                }
                
                // Search for top content in this subreddit
                endpoint := fmt.Sprintf("/r/%s/top.json?%s", subreddit, queryParams.Encode())
                results, err := s.executeSearchRequest(ctx, endpoint)
                if err != nil {
                    log.Printf("Error searching top content in r/%s: %v", subreddit, err)
                    return
                }
                
                // Filter results for relevance
                var filtered []models.SearchResult
                for _, result := range results {
                    // Enhance the relevance checking for this specific query type
                    isRelevant := s.isRelevantRankingResult(result, params.FilteredKeywords, params.QuantityRequested)
                    if isRelevant {
                        filtered = append(filtered, result)
                    }
                }
                
                mu.Lock()
                allResults = append(allResults, filtered...)
                mu.Unlock()
            }(sr)
        }
        
        wg.Wait()
        
        if len(allResults) > 0 {
            searchResults <- allResults
        } else {
            errorResults <- errors.New("no relevant results found in targeted subreddits")
        }
    }()
    
    // Strategy 2: Standard search with optimized query
    searchCount++
    go func() {
        // Add ranking terms to the query to improve results
        enhancedQuery := params.OriginalQuery
        if !strings.Contains(strings.ToLower(enhancedQuery), "top") && 
           !strings.Contains(strings.ToLower(enhancedQuery), "best") {
            enhancedQuery = "top " + enhancedQuery
        }
        
        // Build query parameters
        queryParams := url.Values{}
        queryParams.Set("q", enhancedQuery)
        queryParams.Set("limit", fmt.Sprintf("%d", limit))
        queryParams.Set("sort", "relevance")
        
        if params.IsTimeSensitive {
            queryParams.Set("t", params.TimeFrame)
        } else {
            queryParams.Set("t", "month")
        }
        
        // Execute the search
        endpoint := fmt.Sprintf("/search.json?%s", queryParams.Encode())
        results, err := s.executeSearchRequest(ctx, endpoint)
        if err != nil {
            errorResults <- err
            return
        }
        
        if len(results) > 0 {
            searchResults <- results
        } else {
            errorResults <- errors.New("no results found with standard search")
        }
    }()
    
    // Strategy 3: Search for recent discussions with ranking terms
    searchCount++
    go func() {
        var combinedQuery string
        
        // Add time reference if query is time-sensitive
        if params.IsTimeSensitive {
            timeTerms := []string{"current", "recent", "latest", "now"}
            hasTimeRef := false
            
            for _, term := range timeTerms {
                if strings.Contains(strings.ToLower(params.OriginalQuery), term) {
                    hasTimeRef = true
                    break
                }
            }
            
            if !hasTimeRef {
                combinedQuery = params.OriginalQuery + " current"
            } else {
                combinedQuery = params.OriginalQuery
            }
        } else {
            combinedQuery = params.OriginalQuery
        }
        
        // Build query parameters for recent content
        queryParams := url.Values{}
        queryParams.Set("q", combinedQuery)
        queryParams.Set("limit", fmt.Sprintf("%d", limit))
        queryParams.Set("sort", "new")
        queryParams.Set("t", "week") // Focus on very recent content
        
        // Execute the search
        endpoint := fmt.Sprintf("/search.json?%s", queryParams.Encode())
        results, err := s.executeSearchRequest(ctx, endpoint)
        if err != nil {
            errorResults <- err
            return
        }
        
        if len(results) > 0 {
            searchResults <- results
        } else {
            errorResults <- errors.New("no recent results found")
        }
    }()
    
    // Collect results from all strategies
    var allResults []models.SearchResult
    var lastError error
    resultsReceived := 0
    
    for resultsReceived < searchCount {
        select {
        case <-ctx.Done():
            // Context canceled or timed out
            if lastError == nil {
                lastError = ctx.Err()
            }
            resultsReceived = searchCount // Exit the loop
        case results := <-searchResults:
            allResults = append(allResults, results...)
            resultsReceived++
        case err := <-errorResults:
            if lastError == nil {
                lastError = err
            }
            resultsReceived++
        }
    }
    
    // Return results even if some strategies failed
    if len(allResults) > 0 {
        // Sort by relevance and recency
        sortedResults := rankResultsForRankingQuery(allResults, params)
        return sortedResults, nil
    }
    
    // Only return error if we have no results
    if lastError != nil {
        return nil, lastError
    }
    
    return allResults, nil
}

// Helper function to check if a result is relevant for a ranking query
func (s *RedditService) isRelevantRankingResult(result models.SearchResult, keywords []string, quantity int) bool {
    // Check if the content is a ranking/list type post
    contentLower := strings.ToLower(result.Title + " " + result.Content)
    
    // Check for ranking indicators
    hasRankingIndicator := strings.Contains(contentLower, "top") || 
                           strings.Contains(contentLower, "best") || 
                           strings.Contains(contentLower, "favorite") ||
                           strings.Contains(contentLower, "greatest") ||
                           strings.Contains(contentLower, "worst") ||
                           strings.Contains(contentLower, "ranked")
    
    // Check for list format indicators
    hasListIndicator := strings.Contains(contentLower, "list") ||
                        regexp.MustCompile(`\d+\.`).MatchString(contentLower) ||
                        regexp.MustCompile(`\[\d+\]`).MatchString(contentLower) ||
                        regexp.MustCompile(`#\d+`).MatchString(contentLower)
    
    // Check if it contains specific quantity reference (e.g., "top 5", "10 best")
    hasQuantityMatch := false
    if quantity > 0 {
        quantityRegex := regexp.MustCompile(fmt.Sprintf(`\b(\d+|five|ten|twenty)\b`))
        quantityMatches := quantityRegex.FindAllString(contentLower, -1)
        
        for _, match := range quantityMatches {
            var num int
            
            // Convert word numbers to digits
            switch match {
            case "five":
                num = 5
            case "ten":
                num = 10
            case "twenty":
                num = 20
            default:
                num, _ = strconv.Atoi(match)
            }
            
            // Check if this number matches our quantity
            if num == quantity {
                hasQuantityMatch = true
                break
            }
        }
    }
    
    // Check for keyword matches
    keywordMatch := false
    if len(keywords) > 0 {
        for _, keyword := range keywords {
            if strings.Contains(contentLower, strings.ToLower(keyword)) {
                keywordMatch = true
                break
            }
        }
    } else {
        // If no keywords (unusual), we'll be more lenient
        keywordMatch = true
    }
    
    // Final relevance decision
    // Most important: keyword match and either ranking indicator or list format
    if keywordMatch && (hasRankingIndicator || hasListIndicator) {
        return true
    }
    
    // If it matches the specific quantity requested, that's a strong signal
    if keywordMatch && hasQuantityMatch {
        return true
    }
    
    // For regular posts that match keywords and have high engagement
    if keywordMatch && result.Score > 100 {
        return true
    }
    
    return false
}

// Helper function to get unique items in a slice
func getUniqueItems(items []string) []string {
    seen := make(map[string]bool)
    unique := []string{}
    
    for _, item := range items {
        if _, exists := seen[item]; !exists {
            seen[item] = true
            unique = append(unique, item)
        }
    }
    
    return unique
}

// rankResultsForRankingQuery sorts results specifically for ranking queries
func rankResultsForRankingQuery(results []models.SearchResult, params utils.QueryParams) []models.SearchResult {
    type scoredResult struct {
        result models.SearchResult
        score  float64
    }
    
    // Score each result based on ranking-specific factors
    var scoredResults []scoredResult
    for _, result := range results {
        // Base score starts at 100
        score := 100.0
        
        // 1. Content type relevance (posts are generally more valuable for rankings)
        if result.Type == "post" {
            score += 50
        }
        
        // 2. Title relevance (rankings often appear in titles)
        titleLower := strings.ToLower(result.Title)
        
        // Check for ranking terms in title
        if strings.Contains(titleLower, "top") || 
           strings.Contains(titleLower, "best") ||
           strings.Contains(titleLower, "greatest") {
            score += 100
        }
        
        // Check for quantity match in title
        if params.QuantityRequested > 0 {
            quantityStr := fmt.Sprintf("%d", params.QuantityRequested)
            if strings.Contains(titleLower, quantityStr) {
                score += 150
            }
        }
        
        // Check for list indicators in content
        contentLower := strings.ToLower(result.Content)
        if regexp.MustCompile(`\d+\.\s`).MatchString(contentLower) ||
           regexp.MustCompile(`\[\d+\]`).MatchString(contentLower) ||
           regexp.MustCompile(`#\d+`).MatchString(contentLower) {
            score += 80
        }
        
        // 3. Keyword matching
        for _, keyword := range params.FilteredKeywords {
            if strings.Contains(titleLower, strings.ToLower(keyword)) {
                score += 30 // Higher value for title matches
            }
            if strings.Contains(contentLower, strings.ToLower(keyword)) {
                score += 20 // Medium value for content matches
            }
        }
        
        // 4. Recency for time-sensitive queries
        if params.IsTimeSensitive {
            ageInSeconds := time.Now().Unix() - result.CreatedUTC
            ageInDays := ageInSeconds / (60 * 60 * 24)
            
            // Heavily prioritize recent content for time-sensitive queries
            if ageInDays < 7 {
                score += 200.0 - (float64(ageInDays) * 25.0)  // Decreases from 200 to 25 over a week
            }
        }
        
        // 5. Engagement metrics with logarithmic scaling
        if result.Score > 0 {
            score += math.Log10(float64(result.Score)+10) * 20
        }
        
        if result.CommentCount > 0 {
            score += math.Log10(float64(result.CommentCount)+10) * 15
        }
        
        scoredResults = append(scoredResults, scoredResult{
            result: result,
            score:  score,
        })
    }
    
    // Sort by score (highest first)
    sort.Slice(scoredResults, func(i, j int) bool {
        return scoredResults[i].score > scoredResults[j].score
    })
    
    // Extract just the results
    sortedResults := make([]models.SearchResult, len(scoredResults))
    for i, sr := range scoredResults {
        sortedResults[i] = sr.result
    }
    
    return sortedResults
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