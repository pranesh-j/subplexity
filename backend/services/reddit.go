// services/reddit.go
package services

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/vartanbeno/go-reddit/v2/reddit"
	"golang.org/x/oauth2"
)

var (
	redditClient     *reddit.Client
	redditClientOnce sync.Once
)

// GetRedditClient returns a singleton Reddit client
func GetRedditClient() *reddit.Client {
	redditClientOnce.Do(func() {
		credentials := &reddit.Credentials{
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
	
	posts, _, err := client.Subreddit.SearchPosts(ctx, query, &reddit.ListOptions{
		Limit: limit,
	})
	
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
	
	subreddits, _, err := client.Subreddit.Popular(ctx, &reddit.ListOptions{
		Limit: 10,
	})
	
	if err != nil {
		return nil, err
	}
	
	return subreddits, nil
}