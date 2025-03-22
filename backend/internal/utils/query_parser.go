// File: backend/internal/utils/query_parser.go

package utils

import (
	"regexp"
	"strings"
)

// QueryIntent represents the detected intent of a search query
type QueryIntent int

const (
	GeneralIntent QueryIntent = iota
	SubredditIntent
	PostIntent
	CommentIntent
	UserIntent
	TimeBasedIntent
	TrendingIntent
)

// QueryParams contains the extracted parameters from a query
type QueryParams struct {
	Intent           QueryIntent
	Keywords         []string
	Subreddits       []string
	Authors          []string
	ExcludeTerms     []string
	TimeFrame        string
	SortBy           string
	FilteredKeywords []string
	OriginalQuery    string
}

// StopWords is a set of common words to filter out
var StopWords = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "or": true, "but": true,
	"is": true, "are": true, "was": true, "were": true, "be": true, "being": true, 
	"in": true, "on": true, "at": true, "to": true, "for": true, "with": true,
	"about": true, "what": true, "when": true, "where": true, "who": true, "why": true,
	"how": true, "of": true, "from": true, "by": true, "as": true, "this": true,
	"that": true, "these": true, "those": true, "which": true, "whose": true,
	"than": true, "then": true, "if": true, "else": true, "so": true, "just": true,
	"get": true, "can": true, "will": true, "should": true, "would": true, "could": true,
}

// ParseQuery analyzes a search query string and extracts structured parameters
func ParseQuery(query string) QueryParams {
	params := QueryParams{
		Intent:        GeneralIntent,
		OriginalQuery: query,
		TimeFrame:     "all", // Default timeframe
		SortBy:        "relevance", // Default sort
	}
	
	// Convert to lowercase for easier parsing
	queryLower := strings.ToLower(query)
	
	// Extract subreddits mentioned with r/ prefix
	subredditRegex := regexp.MustCompile(`\br/([a-zA-Z0-9_]+)`)
	subredditMatches := subredditRegex.FindAllStringSubmatch(queryLower, -1)
	for _, match := range subredditMatches {
		if len(match) > 1 {
			params.Subreddits = append(params.Subreddits, match[1])
		}
	}
	
	// Extract users mentioned with u/ prefix
	userRegex := regexp.MustCompile(`\bu/([a-zA-Z0-9_-]+)`)
	userMatches := userRegex.FindAllStringSubmatch(queryLower, -1)
	for _, match := range userMatches {
		if len(match) > 1 {
			params.Authors = append(params.Authors, match[1])
		}
	}
	
	// Detect negation terms (exclude)
	excludeRegex := regexp.MustCompile(`-([a-zA-Z0-9_]+)`)
	excludeMatches := excludeRegex.FindAllStringSubmatch(queryLower, -1)
	for _, match := range excludeMatches {
		if len(match) > 1 {
			params.ExcludeTerms = append(params.ExcludeTerms, match[1])
		}
	}
	
	// Detect time-related terms
	timeTerms := map[string]string{
		"today":       "day",
		"yesterday":   "day",
		"this week":   "week",
		"this month":  "month",
		"this year":   "year",
		"last week":   "week",
		"last month":  "month",
		"last year":   "year",
		"recent":      "week",
		"latest":      "day",
		"new":         "day",
	}
	
	for term, timeframe := range timeTerms {
		if strings.Contains(queryLower, term) {
			params.TimeFrame = timeframe
			params.Intent = TimeBasedIntent
			break
		}
	}
	
	// Detect trending intent
	trendingTerms := []string{"trending", "popular", "hot", "top", "best"}
	for _, term := range trendingTerms {
		if strings.Contains(queryLower, term) {
			params.Intent = TrendingIntent
			if term == "top" {
				params.SortBy = "top"
			} else if term == "hot" {
				params.SortBy = "hot"
			} else if term == "new" {
				params.SortBy = "new" 
			} else if term == "best" {
				params.SortBy = "best"
			}
			break
		}
	}
	
	// Detect specific content type intent
	if len(params.Subreddits) > 0 {
		params.Intent = SubredditIntent
	} else if strings.Contains(queryLower, "comment") {
		params.Intent = CommentIntent
	} else if strings.Contains(queryLower, "post") || strings.Contains(queryLower, "thread") {
		params.Intent = PostIntent
	} else if len(params.Authors) > 0 {
		params.Intent = UserIntent
	}
	
	// Extract keywords (all remaining terms)
	// First, clean query by removing special syntax
	cleanQuery := queryLower
	cleanQuery = subredditRegex.ReplaceAllString(cleanQuery, " ")
	cleanQuery = userRegex.ReplaceAllString(cleanQuery, " ")
	cleanQuery = excludeRegex.ReplaceAllString(cleanQuery, " ")
	
	// Split into words and filter out stop words
	words := strings.Fields(cleanQuery)
	for _, word := range words {
		if len(word) > 2 && !StopWords[word] {
			params.Keywords = append(params.Keywords, word)
		}
	}
	
	// Create filtered keywords (important terms only)
	params.FilteredKeywords = FilterKeywords(params.Keywords)
	
	return params
}

// FilterKeywords removes stop words and keeps only meaningful terms
func FilterKeywords(keywords []string) []string {
	var filtered []string
	
	for _, word := range keywords {
		if len(word) > 2 && !StopWords[word] {
			filtered = append(filtered, word)
		}
	}
	
	return filtered
}