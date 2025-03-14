package main

import (
	"fmt"
	"github.com/vartanbeno/go-reddit/v2/reddit"
	"reflect"
)

func main() {
	// Check the structure of different types
	fmt.Println("ListOptions Structure:")
	listOptions := reflect.TypeOf(reddit.ListOptions{})
	for i := 0; i < listOptions.NumField(); i++ {
		fmt.Printf("  %s: %s\n", listOptions.Field(i).Name, listOptions.Field(i).Type)
	}

	// Check the methods of the Subreddit service
	subredditType := reflect.TypeOf(&reddit.SubredditService{})
	fmt.Println("\nSubredditService Methods:")
	for i := 0; i < subredditType.NumMethod(); i++ {
		method := subredditType.Method(i)
		fmt.Printf("  %s: %s\n", method.Name, method.Type)
	}

	// Try to find the SearchPosts method specifically
	fmt.Println("\nSearching for SearchPosts method:")
	searchPostsMethod, found := subredditType.MethodByName("SearchPosts")
	if found {
		fmt.Printf("  SearchPosts: %s\n", searchPostsMethod.Type)
		
		// Print parameter types
		fmt.Println("  Parameter types:")
		for i := 1; i < searchPostsMethod.Type.NumIn(); i++ {
			fmt.Printf("    Param %d: %s\n", i, searchPostsMethod.Type.In(i))
		}
	} else {
		fmt.Println("  SearchPosts method not found")
	}
}