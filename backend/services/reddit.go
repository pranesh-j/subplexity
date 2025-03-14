// services/reddit.go
package services

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/vartanbeno/go-reddit/v2/reddit"
)

var (
	redditClient     *reddit.Client
	redditClientOnce sync.Once
)

// GetRedditClient returns a singleton Reddit client
func GetRedditClient() *reddit.Client {
	redditClientOnce.Do(func() {
		credentials := reddit.Credentials{
			ID:       os.Getenv("REDDIT_CLIENT_ID"),
			Secret:   os.Getenv("REDDIT_CLIENT_SECRET"),
			Username: os.Getenv("REDDIT_USERNAME"),
			Password: os.Getenv("REDDIT_PASSWORD"),
		}

		client, err := reddit.NewClient(credentials)
		if err != nil {
			log.Fatalf("Failed to create Reddit client: %v", err)
		}
		redditClient = client
	})

	return redditClient
}

// SearchPosts searches for posts matching the query
func SearchPosts(query string, limit int) ([]*reddit.Post, error) {
	client := GetRedditClient()
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Create a new instance of ListPostSearchOptions
	searchOpts := new(reddit.ListPostSearchOptions)
	
	// Set the Limit directly - assuming it has a Limit field
	searchOpts.Limit = limit
	
	// Empty string "" means search across all subreddits
	posts, _, err := client.Subreddit.SearchPosts(ctx, "", query, searchOpts)
	
	if err != nil {
		return nil, err
	}
	
	return posts, nil
}

// GetTrendingSubreddits gets trending subreddits
func GetTrendingSubreddits() ([]*reddit.Subreddit, error) {
	client := GetRedditClient()
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Create a new instance of ListSubredditOptions
	opts := new(reddit.ListSubredditOptions)
	
	// Set the Limit directly - assuming it has a Limit field
	opts.Limit = 10
	
	subreddits, _, err := client.Subreddit.Popular(ctx, opts)
	
	if err != nil {
		return nil, err
	}
	
	return subreddits, nil
}

// GetPostComments retrieves comments for a specific post
func GetPostComments(postID string) ([]*reddit.Comment, error) {
	client := GetRedditClient()
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Reddit's API expects the post ID without the "t3_" prefix
	// But we'll handle both cases for flexibility
	if len(postID) > 3 && postID[:3] == "t3_" {
		postID = postID[3:]
	}
	
	// The correct method to get comments
	postAndComments, _, err := client.Post.Get(ctx, postID)
	if err != nil {
		return nil, err
	}
	
	// The post.Get method returns both the post and its comments
	return postAndComments.Comments, nil
}