package main

import (
	"fmt"
	"reflect"

	"github.com/vartanbeno/go-reddit/v2/reddit"
)

func main() {
	// Check the structure of reddit.Subreddit
	fmt.Println("Subreddit Structure:")
	subredditType := reflect.TypeOf(reddit.Subreddit{})
	for i := 0; i < subredditType.NumField(); i++ {
		fmt.Printf("  %s: %s\n", subredditType.Field(i).Name, subredditType.Field(i).Type)
	}

	// Check the structure of reddit.Post
	fmt.Println("\nPost Structure:")
	postType := reflect.TypeOf(reddit.Post{})
	for i := 0; i < postType.NumField(); i++ {
		fmt.Printf("  %s: %s\n", postType.Field(i).Name, postType.Field(i).Type)
	}

	// Check the structure of reddit.Comment
	fmt.Println("\nComment Structure:")
	commentType := reflect.TypeOf(reddit.Comment{})
	for i := 0; i < commentType.NumField(); i++ {
		fmt.Printf("  %s: %s\n", commentType.Field(i).Name, commentType.Field(i).Type)
	}
}