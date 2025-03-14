// models/search.go
package models

// SearchRequest represents a search request
type SearchRequest struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

// SearchResponse represents a search response
type SearchResponse struct {
	Posts  []Post `json:"posts"`
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// Post represents a simplified Reddit post
type Post struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Content       string  `json:"content"`
	Subreddit     string  `json:"subreddit"`
	Author        string  `json:"author"`
	Score         int     `json:"score"`
	CommentCount  int     `json:"commentCount"`
	URL           string  `json:"url"`
	Created       float64 `json:"created"`
	Permalink     string  `json:"permalink"`
	IsSelfPost    bool    `json:"isSelfPost"`
	IsVideo       bool    `json:"isVideo"`
	ThumbnailURL  string  `json:"thumbnailURL,omitempty"`
	FullImageURL  string  `json:"fullImageURL,omitempty"`
}