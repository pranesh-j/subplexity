// File: internal/services/search_vector.go

package services

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	
	"github.com/pranesh-j/subplexity/internal/models"
)

// SearchVector defines the interface for different search strategies
type SearchVector interface {
    Execute(ctx context.Context, service *RedditService) ([]models.SearchResult, error)
    GetDescription() string
}

// SearchStrategy combines multiple search vectors
type SearchStrategy struct {
    Vectors []SearchVector
    Limit   int
}

// StandardSearchVector implements basic Reddit search
type StandardSearchVector struct {
    Query      string
    SearchMode string
    Limit      int
}

func (v *StandardSearchVector) Execute(ctx context.Context, service *RedditService) ([]models.SearchResult, error) {
    // Build query parameters
    queryParams := url.Values{}
    queryParams.Set("q", v.Query)
    queryParams.Set("limit", fmt.Sprintf("%d", v.Limit))
    
    if v.SearchMode != "All" {
        // Map search mode to search type parameter
        switch v.SearchMode {
        case "Posts":
            queryParams.Set("type", "link")
        case "Comments":
            queryParams.Set("type", "comment")
        case "Communities":
            queryParams.Set("type", "sr")
        }
    }
    
    // Execute the search
    endpoint := fmt.Sprintf("/search.json?%s", queryParams.Encode())
    return service.executeSearchRequest(ctx, endpoint)
}

func (v *StandardSearchVector) GetDescription() string {
    return fmt.Sprintf("Standard search for '%s' with mode '%s'", v.Query, v.SearchMode)
}

// SortedContentVector searches content sorted by hot/new/top
type SortedContentVector struct {
    Keywords  []string
    SortType  string // "hot", "new", "top"
    TimeFrame string
    Limit     int
}

func (v *SortedContentVector) Execute(ctx context.Context, service *RedditService) ([]models.SearchResult, error) {
    // Construct a basic query from keywords
    query := strings.Join(v.Keywords, " ")
    
    // Find relevant subreddits first
    subQuery := url.Values{}
    subQuery.Set("q", query)
    subQuery.Set("limit", "5")
    
    subredditEndpoint := fmt.Sprintf("/subreddits/search.json?%s", subQuery.Encode())
    subreddits, err := service.executeSearchRequest(ctx, subredditEndpoint)
    if err != nil {
        return nil, err
    }
    
    // Default to popular/all if no relevant subreddits found
    targetSubreddits := []string{"popular", "all"}
    if len(subreddits) > 0 {
        targetSubreddits = []string{}
        for _, sr := range subreddits {
            targetSubreddits = append(targetSubreddits, sr.Subreddit)
        }
    }
    
    // Limit to top 3 most relevant subreddits
    if len(targetSubreddits) > 3 {
        targetSubreddits = targetSubreddits[:3]
    }
    
    // Search each subreddit's sorted content
    var allResults []models.SearchResult
    var mu sync.Mutex
    var wg sync.WaitGroup
    
    for _, subreddit := range targetSubreddits {
        wg.Add(1)
        go func(sr string) {
            defer wg.Done()
            
            // Build query for sorted content
            sortQuery := url.Values{}
            sortQuery.Set("limit", fmt.Sprintf("%d", v.Limit/len(targetSubreddits)))
            
            if v.TimeFrame != "" && v.TimeFrame != "all" {
                sortQuery.Set("t", v.TimeFrame)
            }
            
            // Execute the request
            sortEndpoint := fmt.Sprintf("/r/%s/%s.json?%s", sr, v.SortType, sortQuery.Encode())
            results, err := service.executeSearchRequest(ctx, sortEndpoint)
            if err != nil {
                return
            }
            
            // Filter results for keyword relevance
            var filtered []models.SearchResult
            for _, result := range results {
                for _, keyword := range v.Keywords {
                    if strings.Contains(strings.ToLower(result.Title+" "+result.Content), 
                                       strings.ToLower(keyword)) {
                        filtered = append(filtered, result)
                        break
                    }
                }
            }
            
            mu.Lock()
            allResults = append(allResults, filtered...)
            mu.Unlock()
        }(subreddit)
    }
    
    wg.Wait()
    return allResults, nil
}

func (v *SortedContentVector) GetDescription() string {
    return fmt.Sprintf("Content sorted by '%s' within timeframe '%s'", v.SortType, v.TimeFrame)
}

// TrendAnalysisVector specifically targets trending/ranking content
type TrendAnalysisVector struct {
    Keywords  []string
    TimeFrame string
    Limit     int
}

func (v *TrendAnalysisVector) Execute(ctx context.Context, service *RedditService) ([]models.SearchResult, error) {
    // Search for "top" "best" "ranking" type posts
    rankingTerms := []string{"top", "best", "ranking", "ranked", "list", "popular"}
    
    // Combine keywords with ranking terms for better results
    var queries []string
    for _, keyword := range v.Keywords {
        for _, term := range rankingTerms {
            queries = append(queries, fmt.Sprintf("%s %s", keyword, term))
        }
    }
    
    // Execute multiple searches
    var allResults []models.SearchResult
    var mu sync.Mutex
    var wg sync.WaitGroup
    
    for _, q := range queries {
        wg.Add(1)
        go func(query string) {
            defer wg.Done()
            
            // Build standard search query
            queryParams := url.Values{}
            queryParams.Set("q", query)
            queryParams.Set("limit", fmt.Sprintf("%d", v.Limit/len(queries)))
            queryParams.Set("sort", "relevance")
            
            if v.TimeFrame != "" && v.TimeFrame != "all" {
                queryParams.Set("t", v.TimeFrame)
            }
            
            endpoint := fmt.Sprintf("/search.json?%s", queryParams.Encode())
            results, err := service.executeSearchRequest(ctx, endpoint)
            if err != nil {
                return
            }
            
            mu.Lock()
            allResults = append(allResults, results...)
            mu.Unlock()
        }(q)
    }
    
    wg.Wait()
    return allResults, nil
}

func (v *TrendAnalysisVector) GetDescription() string {
    return fmt.Sprintf("Trend analysis for ranking content within timeframe '%s'", v.TimeFrame)
}