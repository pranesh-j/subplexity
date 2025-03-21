// backend/api/handlers/search.go
package handlers

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
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

	// Check if this is a special query that needs real-time data
	needsRealTimeData := isRealTimeDataQuery(req.Query)

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

	// Generate citations from the answer
	citations := extractCitations(answer, results)

	// For real-time data queries, add disclaimer if needed
	if needsRealTimeData {
		if !strings.Contains(answer, "real-time") && !strings.Contains(answer, "up-to-date") {
			answer += "\n\nNote: This information is based on Reddit posts which may not reflect real-time data. For current pricing or real-time information, please check specialized sources."
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
		Citations:   citations,
		ElapsedTime: elapsedTime,
		LastUpdated: time.Now().Unix(),
		RequestParams: models.RequestParams{
			Query:      req.Query,
			SearchMode: req.SearchMode,
			ModelName:  req.ModelName,
			Limit:      req.Limit,
		},
	}

	c.JSON(http.StatusOK, response)
}

// isRealTimeDataQuery identifies queries that need real-time data
func isRealTimeDataQuery(query string) bool {
	query = strings.ToLower(query)
	
	// Price-related queries
	pricePatterns := []string{
		"price", "worth", "value", "cost", "how much", "market cap",
		"trading at", "current", "today", "now", "latest",
	}
	
	// Assets that typically need real-time data
	assetPatterns := []string{
		"bitcoin", "btc", "crypto", "stock", "share", "market", 
		"currency", "forex", "exchange rate", "interest rate",
	}
	
	// Check for combinations of price and asset patterns
	for _, price := range pricePatterns {
		if strings.Contains(query, price) {
			for _, asset := range assetPatterns {
				if strings.Contains(query, asset) {
					return true
				}
			}
		}
	}
	
	// Check for specific time-sensitive patterns
	timePatterns := []string{
		"right now", "currently", "latest news", "breaking news",
		"live", "real time", "real-time", "today's", "happening now",
	}
	
	for _, pattern := range timePatterns {
		if strings.Contains(query, pattern) {
			return true
		}
	}
	
	return false
}

// extractCitations identifies and formats citations from the answer text
func extractCitations(answer string, results []models.SearchResult) []models.Citation {
	var citations []models.Citation
	
	// Look for explicit citation patterns like [1], [2], etc.
	re := regexp.MustCompile(`\[(\d+)\]`)
	matches := re.FindAllStringSubmatch(answer, -1)
	
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		
		// Parse the citation number
		index := 0
		_, err := fmt.Sscanf(match[1], "%d", &index)
		if err != nil || index <= 0 || index > len(results) {
			continue
		}
		
		// Adjust for 0-based indexing
		resultIndex := index - 1
		
		// Add citation
		citation := models.Citation{
			Index:     index,
			Text:      getCitationContext(answer, match[0]),
			URL:       results[resultIndex].URL,
			Title:     results[resultIndex].Title,
			Type:      results[resultIndex].Type,
			Subreddit: results[resultIndex].Subreddit,
		}
		
		// Check if this citation already exists (avoid duplicates)
		isDuplicate := false
		for _, existing := range citations {
			if existing.Index == citation.Index {
				isDuplicate = true
				break
			}
		}
		
		if !isDuplicate {
			citations = append(citations, citation)
		}
	}
	
	// If no explicit citations found, try to find "Result X mentions..." patterns
	if len(citations) == 0 {
		resultRe := regexp.MustCompile(`(?i)(?:Result|Post|Comment)\s+(\d+)`)
		matches := resultRe.FindAllStringSubmatch(answer, -1)
		
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			
			// Parse the result number
			index := 0
			_, err := fmt.Sscanf(match[1], "%d", &index)
			if err != nil || index <= 0 || index > len(results) {
				continue
			}
			
			// Adjust for 0-based indexing
			resultIndex := index - 1
			
			// Add citation
			citation := models.Citation{
				Index:     index,
				Text:      getCitationContext(answer, match[0]),
				URL:       results[resultIndex].URL,
				Title:     results[resultIndex].Title,
				Type:      results[resultIndex].Type,
				Subreddit: results[resultIndex].Subreddit,
			}
			
			// Check for duplicates
			isDuplicate := false
			for _, existing := range citations {
				if existing.Index == citation.Index {
					isDuplicate = true
					break
				}
			}
			
			if !isDuplicate {
				citations = append(citations, citation)
			}
		}
	}
	
	// If still no citations found but we have specific subreddit mentions, use those
	if len(citations) == 0 {
		subredditRe := regexp.MustCompile(`r/([a-zA-Z0-9_]+)`)
		matches := subredditRe.FindAllStringSubmatch(answer, -1)
		
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			
			subredditName := match[1]
			
			// Find matching results
			for i, result := range results {
				if strings.EqualFold(result.Subreddit, subredditName) {
					// Add citation
					citation := models.Citation{
						Index:     i + 1,
						Text:      getCitationContext(answer, match[0]),
						URL:       result.URL,
						Title:     result.Title,
						Type:      result.Type,
						Subreddit: result.Subreddit,
					}
					
					// Check for duplicates
					isDuplicate := false
					for _, existing := range citations {
						if existing.Index == citation.Index {
							isDuplicate = true
							break
						}
					}
					
					if !isDuplicate {
						citations = append(citations, citation)
						break // Just add the first match for each subreddit
					}
				}
			}
		}
	}
	
	return citations
}

// getCitationContext returns the sentence or context around a citation
func getCitationContext(text, marker string) string {
	// Find the sentence containing the citation
	sentences := splitIntoSentences(text)
	
	for _, sentence := range sentences {
		if strings.Contains(sentence, marker) {
			// Clean up the sentence
			clean := strings.TrimSpace(sentence)
			
			// If it's too long, truncate it
			if len(clean) > 150 {
				clean = clean[:147] + "..."
			}
			
			return clean
		}
	}
	
	// If we can't find the exact sentence, return a placeholder
	return "Referenced content"
}

// splitIntoSentences breaks text into sentences
func splitIntoSentences(text string) []string {
	// Regex for sentence splitting that handles abbreviations and special cases
	re := regexp.MustCompile(`[.!?]\s+[A-Z]`)
	
	// Find all sentence boundaries
	boundaries := re.FindAllStringIndex(text, -1)
	
	// If no boundaries found, return the whole text as one sentence
	if len(boundaries) == 0 {
		return []string{text}
	}
	
	// Build sentences
	var sentences []string
	prevEnd := 0
	
	for _, boundary := range boundaries {
		// End of sentence is position of the punctuation mark
		endPos := boundary[0] + 1
		sentences = append(sentences, text[prevEnd:endPos])
		
		// Start of next sentence is after the space
		prevEnd = boundary[0] + 2
	}
	
	// Add the last sentence if there's text left
	if prevEnd < len(text) {
		sentences = append(sentences, text[prevEnd:])
	}
	
	return sentences
}