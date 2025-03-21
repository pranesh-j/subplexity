package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pranesh-j/subplexity/internal/models"
	"github.com/pranesh-j/subplexity/internal/services"
)

type SearchHandler struct {
	RedditService *services.RedditService
	AIService     *services.AIService
}

func NewSearchHandler(redditService *services.RedditService, aiService *services.AIService) *SearchHandler {
	return &SearchHandler{
		RedditService: redditService,
		AIService:     aiService,
	}
}

func (h *SearchHandler) HandleSearch(c *gin.Context) {
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
	results, err := h.RedditService.SearchReddit(req.Query, req.SearchMode, req.Limit)
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
		})
		return
	}

	// Process results with AI (with error handling)
	var reasoning, answer string
	aiErr := error(nil)
	
	// Only attempt AI processing if we have at least one result
	if len(results) > 0 {
		reasoning, answer, aiErr = h.AIService.ProcessResults(req.Query, results, req.ModelName)
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
		Results:     results,
		TotalCount:  len(results),
		Reasoning:   reasoning,
		Answer:      answer,
		ElapsedTime: elapsedTime,
	}

	c.JSON(http.StatusOK, response)
}