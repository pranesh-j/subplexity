// File: backend/internal/services/reddit_parser.go

package services

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/pranesh-j/subplexity/internal/models"
)

// parseRedditResponse parses a Reddit API response into SearchResult objects
func parseRedditResponse(rawResponse []byte) ([]models.SearchResult, error) {
	// First try to parse as listing
	var response struct {
		Kind string `json:"kind"`
		Data struct {
			Children []struct {
				Kind string `json:"kind"`
				Data json.RawMessage `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}

	if err := json.Unmarshal(rawResponse, &response); err != nil {
		return nil, fmt.Errorf("error parsing Reddit JSON response: %w", err)
	}

	// Process results
	var results []models.SearchResult

	for _, child := range response.Data.Children {
		// Get type from kind
		resultType := getTypeFromKind(child.Kind)
		if resultType == "unknown" {
			// Skip unknown types
			continue
		}

		// Parse the item data based on its type
		var result models.SearchResult
		result.Type = resultType

		// Parse the specific data fields
		switch resultType {
		case "post":
			if err := parsePostData(child.Data, &result); err != nil {
				log.Printf("Error parsing post data: %v", err)
				continue
			}
		case "comment":
			if err := parseCommentData(child.Data, &result); err != nil {
				log.Printf("Error parsing comment data: %v", err)
				continue
			}
		case "subreddit":
			if err := parseSubredditData(child.Data, &result); err != nil {
				log.Printf("Error parsing subreddit data: %v", err)
				continue
			}
		default:
			// Skip unknown types
			continue
		}

		// Only add valid results that have at least an ID and title
		if result.ID != "" && result.Title != "" {
			results = append(results, result)
		}
	}

	return results, nil
}

// parsePostData parses a post (t3) item
func parsePostData(data []byte, result *models.SearchResult) error {
	var post struct {
		ID           string  `json:"id"`
		Author       string  `json:"author"`
		Title        string  `json:"title"`
		Selftext     string  `json:"selftext"`
		Subreddit    string  `json:"subreddit"`
		Score        int     `json:"score"`
		NumComments  int     `json:"num_comments"`
		CreatedUTC   float64 `json:"created_utc"`
		Permalink    string  `json:"permalink"`
		URL          string  `json:"url"`
		Distinguished string  `json:"distinguished"`
		Stickied     bool    `json:"stickied"`
	}

	if err := json.Unmarshal(data, &post); err != nil {
		return fmt.Errorf("error parsing post JSON: %w", err)
	}

	result.ID = post.ID
	result.Author = post.Author
	result.Title = post.Title
	result.Content = post.Selftext
	result.Subreddit = post.Subreddit
	result.Score = post.Score
	result.CommentCount = post.NumComments
	result.CreatedUTC = int64(post.CreatedUTC)

	// Set URL (use permalink if available)
	if post.Permalink != "" {
		result.URL = "https://www.reddit.com" + post.Permalink
	} else if post.URL != "" {
		result.URL = post.URL
	} else {
		result.URL = fmt.Sprintf("https://www.reddit.com/r/%s/comments/%s", post.Subreddit, post.ID)
	}

	// Add additional context for special posts
	if post.Distinguished != "" || post.Stickied {
		// Add some metadata to help with relevance ranking
		tags := []string{}
		if post.Stickied {
			tags = append(tags, "stickied")
		}
		if post.Distinguished != "" {
			tags = append(tags, post.Distinguished)
		}
		
		if len(tags) > 0 {
			if result.Content != "" {
				result.Content = fmt.Sprintf("[%s] %s", strings.Join(tags, ", "), result.Content)
			} else {
				result.Content = fmt.Sprintf("[%s]", strings.Join(tags, ", "))
			}
		}
	}

	return nil
}

// parseCommentData parses a comment (t1) item
func parseCommentData(data []byte, result *models.SearchResult) error {
	var comment struct {
		ID           string  `json:"id"`
		Author       string  `json:"author"`
		Body         string  `json:"body"`
		Subreddit    string  `json:"subreddit"`
		Score        int     `json:"score"`
		CreatedUTC   float64 `json:"created_utc"`
		Permalink    string  `json:"permalink"`
		LinkID       string  `json:"link_id"`
		LinkTitle    string  `json:"link_title"`
		Distinguished string  `json:"distinguished"`
	}

	if err := json.Unmarshal(data, &comment); err != nil {
		return fmt.Errorf("error parsing comment JSON: %w", err)
	}

	result.ID = comment.ID
	result.Author = comment.Author
	result.Content = comment.Body
	result.Subreddit = comment.Subreddit
	result.Score = comment.Score
	result.CreatedUTC = int64(comment.CreatedUTC)

	// Set title and URL
	if comment.LinkTitle != "" {
		result.Title = fmt.Sprintf("Comment on: %s", comment.LinkTitle)
	} else {
		result.Title = fmt.Sprintf("Comment in r/%s", comment.Subreddit)
	}

	if comment.Permalink != "" {
		result.URL = "https://www.reddit.com" + comment.Permalink
	} else {
		commentID := strings.TrimPrefix(comment.ID, "t1_")
		parentID := strings.TrimPrefix(comment.LinkID, "t3_")
		result.URL = fmt.Sprintf("https://www.reddit.com/r/%s/comments/%s/_/%s", 
			comment.Subreddit, parentID, commentID)
	}

	return nil
}

// parseSubredditData parses a subreddit (t5) item
func parseSubredditData(data []byte, result *models.SearchResult) error {
	var subreddit struct {
		ID              string  `json:"id"`
		DisplayName     string  `json:"display_name"`
		Title           string  `json:"title"`
		Description     string  `json:"description"`
		PublicDescription string `json:"public_description"`
		Subscribers     int     `json:"subscribers"`
		CreatedUTC      float64 `json:"created_utc"`
		NSFW            bool    `json:"over_18"`
	}

	if err := json.Unmarshal(data, &subreddit); err != nil {
		return fmt.Errorf("error parsing subreddit JSON: %w", err)
	}

	result.ID = subreddit.ID
	result.Title = fmt.Sprintf("r/%s", subreddit.DisplayName)
	result.Subreddit = subreddit.DisplayName
	
	// Use the most informative description available
	if subreddit.PublicDescription != "" {
		result.Content = subreddit.PublicDescription
	} else if subreddit.Description != "" {
		result.Content = subreddit.Description
	} else {
		result.Content = subreddit.Title
	}
	
	result.Score = subreddit.Subscribers
	result.CreatedUTC = int64(subreddit.CreatedUTC)
	result.URL = fmt.Sprintf("https://www.reddit.com/r/%s", subreddit.DisplayName)

	// Add NSFW tag to content if applicable
	if subreddit.NSFW {
		result.Content = fmt.Sprintf("[NSFW] %s", result.Content)
	}

	return nil
}

// getTypeFromKind converts Reddit "kind" prefixes to our content types
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