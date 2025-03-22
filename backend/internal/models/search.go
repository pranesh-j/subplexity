// backend/internal/models/search.go
package models

// SearchRequest represents the incoming search request
type SearchRequest struct {
	Query      string `json:"query"`
	SearchMode string `json:"searchMode"`
	ModelName  string `json:"modelName"`
	Limit      int    `json:"limit,omitempty"`
}

// SearchResult represents a single result from Reddit
type SearchResult struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Subreddit    string   `json:"subreddit"`
	Author       string   `json:"author"`
	Content      string   `json:"content"`
	URL          string   `json:"url"`
	CreatedUTC   int64    `json:"createdUtc"`
	Score        int      `json:"score"`
	CommentCount int      `json:"commentCount,omitempty"`
	Type         string   `json:"type"` // "post", "comment", or "subreddit"
	Highlights   []string `json:"highlights,omitempty"` // Key excerpts to highlight
}

// Citation represents a reference to a source in the results
type Citation struct {
	Index     int    `json:"index"`
	Text      string `json:"text"`
	URL       string `json:"url"`
	Title     string `json:"title"`
	Type      string `json:"type"` // "post", "comment", "subreddit"
	Subreddit string `json:"subreddit"`
}

// ReasoningStep represents a single step in the AI's reasoning process
type ReasoningStep struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// SearchResponse represents the search response with enhanced RAG information
type SearchResponse struct {
	Results        []SearchResult  `json:"results"`
	TotalCount     int             `json:"totalCount"`
	Reasoning      string          `json:"reasoning,omitempty"`
	ReasoningSteps []ReasoningStep `json:"reasoningSteps,omitempty"`
	Answer         string          `json:"answer,omitempty"`
	Citations      []Citation      `json:"citations,omitempty"`
	ElapsedTime    float64         `json:"elapsedTime"`
	LastUpdated    int64           `json:"lastUpdated"` // Unix timestamp of data freshness
	RequestParams  RequestParams   `json:"requestParams,omitempty"`
}

// RequestParams captures the original request parameters for reference
type RequestParams struct {
	Query      string `json:"query"`
	SearchMode string `json:"searchMode"`
	ModelName  string `json:"modelName"`
	Limit      int    `json:"limit"`
}