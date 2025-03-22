// File: backend/api/handlers/search.go

package handlers

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pranesh-j/subplexity/internal/models"
	"github.com/pranesh-j/subplexity/internal/services"
)

type SearchHandler struct {
	RedditService *services.RedditService
	AIService     *services.AIService
	initialized   bool
}

func NewSearchHandler(redditService *services.RedditService, aiService *services.AIService) *SearchHandler {
	return &SearchHandler{
		RedditService: redditService,
		AIService:     aiService,
	}
}

// Init initializes the handler (like warming up connections)
func (h *SearchHandler) Init(ctx context.Context) error {
	if h.initialized {
		return nil
	}
	
	// Nothing to initialize yet
	h.initialized = true
	return nil
}

func (h *SearchHandler) HandleSearch(c *gin.Context) {
	// Create a context with timeout for the request
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	var req models.SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Invalid request payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request payload", 
			"details": err.Error(),
		})
		return
	}

	// Validate request
	if req.Query == "" {
		log.Println("Search query cannot be empty")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query cannot be empty"})
		return
	}

	// Log the incoming request
	log.Printf("Search request: Query='%s', Mode='%s', Model='%s', Limit=%d", 
		req.Query, req.SearchMode, req.ModelName, req.Limit)

	// Set default limit and search mode if not provided
	if req.Limit <= 0 {
		req.Limit = 25
	}
	if req.Limit > 100 {
		req.Limit = 100 // Cap the maximum limit
	}
	if req.SearchMode == "" {
		req.SearchMode = "All" // Default search mode
	}
	if req.ModelName == "" {
		req.ModelName = "Claude" // Default AI model
	}

	// Measure execution time
	startTime := time.Now()

	// Search Reddit with timeout
	results, err := h.RedditService.SearchReddit(ctx, req.Query, req.SearchMode, req.Limit)
	if err != nil {
		log.Printf("Failed to search Reddit: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to search Reddit", 
			"details": err.Error(),
		})
		return
	}

	// If no results were found, return an empty response with explanation
	if len(results) == 0 {
		log.Println("No search results found")
		c.JSON(http.StatusOK, models.SearchResponse{
			Results:     []models.SearchResult{},
			TotalCount:  0,
			Reasoning:   "No results found for your query.",
			Answer:      "There are no Reddit results matching your search criteria. Please try a different query or search mode.",
			ElapsedTime: time.Since(startTime).Seconds(),
			LastUpdated: time.Now().Unix(),
			RequestParams: models.RequestParams{
				Query:      req.Query,
				SearchMode: req.SearchMode,
				ModelName:  req.ModelName,
				Limit:      req.Limit,
			},
		})
		return
	}

	// Filter out completely irrelevant results based on the query terms
	var relevantResults []models.SearchResult
	
	// Extract main keywords from the query
	queryWords := strings.Fields(strings.ToLower(req.Query))
	
	// Filter out common stop words
	var queryKeywords []string
	for _, word := range queryWords {
		// Only keep meaningful words (longer than 2 chars)
		if len(word) > 2 {
			queryKeywords = append(queryKeywords, word)
		}
	}
	
	// Only perform filtering if we have meaningful keywords
	if len(queryKeywords) > 0 {
		for _, result := range results {
			resultText := strings.ToLower(result.Title + " " + result.Content)
			
			// Check if any main query terms are present
			for _, term := range queryKeywords {
				if strings.Contains(resultText, term) {
					relevantResults = append(relevantResults, result)
					break
				}
			}
		}
		
		// Only use filtered results if we found some
		if len(relevantResults) > 0 {
			log.Printf("Filtered results from %d to %d relevant items based on query keywords", 
				len(results), len(relevantResults))
			results = relevantResults
		}
	}

	// Process results with AI (with error handling)
	var reasoning, answer string
	var reasoningSteps []models.ReasoningStep
	var citations []models.Citation
	aiErr := error(nil)
	
	// Only attempt AI processing if we have at least one result
	if len(results) > 0 {
		reasoning, answer, reasoningSteps, citations, aiErr = h.AIService.ProcessResults(ctx, req.Query, results, req.ModelName)
		if aiErr != nil {
			log.Printf("AI processing error: %v", aiErr)
			// Still continue - we'll return the raw results
			reasoning = "AI processing failed: " + aiErr.Error()
			answer = "The search found results, but AI analysis couldn't be completed. The raw results are still available."
		}
	}

	elapsedTime := time.Since(startTime).Seconds()
	log.Printf("Search completed in %.2f seconds, found %d results", elapsedTime, len(results))

	// Prepare response
	response := models.SearchResponse{
		Results:        results,
		TotalCount:     len(results),
		Reasoning:      reasoning,
		ReasoningSteps: reasoningSteps,
		Answer:         answer,
		Citations:      citations,
		ElapsedTime:    elapsedTime,
		LastUpdated:    time.Now().Unix(),
		RequestParams: models.RequestParams{
			Query:      req.Query,
			SearchMode: req.SearchMode,
			ModelName:  req.ModelName,
			Limit:      req.Limit,
		},
	}

	c.JSON(http.StatusOK, response)
}