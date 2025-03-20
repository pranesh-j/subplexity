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
	ID           string `json:"id"`
	Title        string `json:"title"`
	Subreddit    string `json:"subreddit"`
	Author       string `json:"author"`
	Content      string `json:"content"`
	URL          string `json:"url"`
	CreatedUTC   int64  `json:"createdUtc"`
	Score        int    `json:"score"`
	CommentCount int    `json:"commentCount,omitempty"`
	Type         string `json:"type"` // "post", "comment", or "subreddit"
}

// SearchResponse represents the search response
type SearchResponse struct {
	Results     []SearchResult `json:"results"`
	TotalCount  int            `json:"totalCount"`
	Reasoning   string         `json:"reasoning,omitempty"`
	Answer      string         `json:"answer,omitempty"`
	ElapsedTime float64        `json:"elapsedTime"`
}