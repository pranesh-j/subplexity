package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourname/subplexity/internal/models"
	"github.com/yourname/subplexity/internal/services"
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	// Set default limit
	if req.Limit <= 0 {
		req.Limit = 25
	}

	// Measure execution time
	startTime := time.Now()

	// Search Reddit
	results, err := h.RedditService.SearchReddit(req.Query, req.SearchMode, req.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search Reddit: " + err.Error()})
		return
	}

	// Process results with AI
	reasoning, answer, err := h.AIService.ProcessResults(req.Query, results, req.ModelName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process results: " + err.Error()})
		return
	}

	elapsedTime := time.Since(startTime).Seconds()

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