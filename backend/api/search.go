// api/search.go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/prane/subplex/models"
	"github.com/prane/subplex/services"
	"github.com/vartanbeno/go-reddit/v2/reddit"
)

// SearchHandler handles Reddit search requests
func SearchHandler(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers for the preflight request
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	
	// Set CORS headers for the main request
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	query := r.URL.Query().Get("query")
	if query == "" {
		http.Error(w, "Search query is required", http.StatusBadRequest)
		return
	}
	
	limitStr := r.URL.Query().Get("limit")
	limit := 25 // default limit
	
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}
	
	posts, err := services.SearchPosts(query, limit)
	if err != nil {
		http.Error(w, "Failed to search posts: "+err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Convert Reddit posts to our model
	result := make([]models.Post, 0, len(posts))
	for _, post := range posts {
		result = append(result, convertRedditPost(post))
	}
	
	response := models.SearchResponse{
		Posts:  result,
		Total:  len(result),
		Limit:  limit,
		Offset: 0,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// TrendingHandler handles trending subreddits requests
func TrendingHandler(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers for the preflight request
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	
	// Set CORS headers for the main request
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	subreddits, err := services.GetTrendingSubreddits()
	if err != nil {
		http.Error(w, "Failed to get trending subreddits: "+err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Convert to a simpler format for the frontend
	type TrendingSubreddit struct {
		Name        string `json:"title"`
		PostCount   int    `json:"posts"`
		Type        string `json:"type"`
		Subscribers int    `json:"subscribers"`
		URL         string `json:"url"`
	}
	
	result := make([]TrendingSubreddit, 0, len(subreddits))
	for _, sub := range subreddits {
		// Determine trending type based on stats
		trendingType := "Popular"
		if sub.SubredditType == "public" && sub.ActiveUserCount > 1000 {
			trendingType = "Hot"
		} else if sub.Created.After(time.Now().AddDate(0, -1, 0)) {
			trendingType = "Trending"
		}
		
		result = append(result, TrendingSubreddit{
			Name:        sub.Name,
			PostCount:   sub.ActiveUserCount / 100, // Approximate based on active users
			Type:        trendingType,
			Subscribers: sub.Subscribers,
			URL:         "https://reddit.com" + sub.URL,
		})
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"subreddits": result,
	})
}

// Helper function to convert Reddit post to our model
func convertRedditPost(post *reddit.Post) models.Post {
	return models.Post{
		ID:            post.ID,
		Title:         post.Title,
		Content:       post.Body,
		Subreddit:     post.SubredditName,
		Author:        post.Author,
		Score:         post.Score,
		CommentCount:  post.NumberOfComments,
		URL:           post.URL,
		Created:       float64(post.Created.Unix()),
		Permalink:     "https://reddit.com" + post.Permalink,
		IsSelfPost:    post.IsSelfPost,
		IsVideo:       post.IsVideo,
		ThumbnailURL:  post.Thumbnail,
		FullImageURL:  post.URL, // Only correct if it's an image post
	}
}

// CommentHandler gets comments for a specific post
func CommentHandler(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers for the preflight request
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	
	// Set CORS headers for the main request
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	postID := r.URL.Query().Get("id")
	if postID == "" {
		http.Error(w, "Post ID is required", http.StatusBadRequest)
		return
	}
	
	comments, err := services.GetPostComments(postID)
	if err != nil {
		http.Error(w, "Failed to get comments: "+err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Convert to a simpler format
	type Comment struct {
		ID        string    `json:"id"`
		Author    string    `json:"author"`
		Body      string    `json:"body"`
		Score     int       `json:"score"`
		Created   float64   `json:"created"`
		Permalink string    `json:"permalink"`
		Replies   []Comment `json:"replies,omitempty"`
	}
	
	var convertComment func(*reddit.Comment) Comment
	convertComment = func(c *reddit.Comment) Comment {
		comment := Comment{
			ID:        c.ID,
			Author:    c.Author,
			Body:      c.Body,
			Score:     c.Score,
			Created:   float64(c.Created.Unix()),
			Permalink: "https://reddit.com" + c.Permalink,
		}
		
		// Convert replies recursively
		if len(c.Replies) > 0 {
			comment.Replies = make([]Comment, 0, len(c.Replies))
			for _, reply := range c.Replies {
				comment.Replies = append(comment.Replies, convertComment(reply))
			}
		}
		
		return comment
	}
	
	result := make([]Comment, 0, len(comments))
	for _, comment := range comments {
		result = append(result, convertComment(comment))
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"comments": result,
	})
}