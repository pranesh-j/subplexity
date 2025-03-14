package main

import (
	"fmt"
	"github.com/vartanbeno/go-reddit/v2/reddit"
	"reflect"
)

func main() {
	// Check the structure of ListPostSearchOptions
	fmt.Println("ListPostSearchOptions Structure:")
	searchOptions := reflect.TypeOf(reddit.ListPostSearchOptions{})
	for i := 0; i < searchOptions.NumField(); i++ {
		fmt.Printf("  %s: %s\n", searchOptions.Field(i).Name, searchOptions.Field(i).Type)
	}

	// Check ListSubredditOptions too
	fmt.Println("\nListSubredditOptions Structure:")
	subredditOptions := reflect.TypeOf(reddit.ListSubredditOptions{})
	for i := 0; i < subredditOptions.NumField(); i++ {
		fmt.Printf("  %s: %s\n", subredditOptions.Field(i).Name, subredditOptions.Field(i).Type)
	}
}