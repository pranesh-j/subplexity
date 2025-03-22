// File: backend/internal/services/ai.go

package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pranesh-j/subplexity/internal/models"
)

// AIService handles interactions with AI models
type AIService struct {
	modelConfig    map[string]*AIModelConfig
	defaultModel   string
	promptTemplate map[string]string
	maxRetries     int
}

// NewAIService creates a new AI service
func NewAIService() *AIService {
	service := &AIService{
		modelConfig:    loadModelConfigurations(),
		defaultModel:   "Claude",
		promptTemplate: loadPromptTemplates(),
		maxRetries:     3,
	}
	
	return service
}

// ProcessResults processes search results with AI
func (s *AIService) ProcessResults(ctx context.Context, query string, results []models.SearchResult, modelName string) (string, string, []models.ReasoningStep, []models.Citation, error) {
	// Check for context cancellation first
	select {
	case <-ctx.Done():
		return "", "", nil, nil, ctx.Err()
	default:
		// Continue processing
	}

	if len(results) == 0 {
		return "", "No results found for this query.", nil, nil, nil
	}

	// Use default model if none specified
	if modelName == "" {
		modelName = s.defaultModel
	}

	// Get model configuration
	modelConfig, ok := s.modelConfig[modelName]
	if !ok {
		modelConfig = s.modelConfig[s.defaultModel]
		log.Printf("Model '%s' not found, using default model '%s'", modelName, s.defaultModel)
	}

	// Build the prompt
	prompt := s.buildPrompt(query, results, modelConfig)

	// Log prompt length for debugging
	log.Printf("Generated prompt for '%s' with %d characters", query, len(prompt))

	// Process with AI model with retries
	var response string
	var err error
	
	for attempt := 0; attempt < s.maxRetries; attempt++ {
		// Check for context cancellation before each attempt
		select {
		case <-ctx.Done():
			return "", "", nil, nil, ctx.Err()
		default:
			// Continue processing
		}
		
		if attempt > 0 {
			log.Printf("Retry attempt %d for AI processing of query '%s'", attempt, query)
			// Add exponential backoff here if needed
			time.Sleep(time.Duration(attempt*500) * time.Millisecond)
		}
		
		response, err = s.processWithModel(ctx, prompt, modelConfig)
		if err == nil {
			break
		}
		
		// If context was canceled during model processing, return immediately
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", "", nil, nil, err
		}
		
		log.Printf("AI processing error (attempt %d/%d): %v", 
			attempt+1, s.maxRetries, err)
	}
	
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("AI processing failed after %d attempts: %w", 
			s.maxRetries, err)
	}

	// Extract reasoning and answer
	reasoning, answer, err := s.extractReasoningAndAnswer(response, modelConfig)
	if err != nil {
		log.Printf("Extraction error: %v, attempting fallback parsing", err)
		// Try fallback parsing if standard extraction fails
		reasoning, answer = s.fallbackParsing(response)
		
		if reasoning == "" && answer == "" {
			// If still no success, use raw response
			log.Printf("Fallback parsing failed, returning raw response")
			reasoning = "Error parsing structured response."
			answer = cleanupRawResponse(response)
		}
	}

	// Extract reasoning steps
	reasoningSteps := s.extractReasoningSteps(reasoning)

	// Extract citations
	citations := s.extractCitations(answer, results)

	// Perform quality checks
	if err := s.validateResponse(answer, reasoning, reasoningSteps, citations); err != nil {
		log.Printf("Response validation warning: %v", err)
	}

	return reasoning, answer, reasoningSteps, citations, nil
}

// fallbackParsing attempts alternative parsing strategies when standard extraction fails
func (s *AIService) fallbackParsing(response string) (string, string) {
	// Try to split by markdown headers
	headers := extractMarkdownHeaders(response)
	
	if len(headers) >= 2 {
		// Assume first section is reasoning, last section is answer
		firstHeaderIdx := headers[0].Index
		lastHeaderIdx := headers[len(headers)-1].Index
		
		// Extract reasoning (from first header to second-to-last header)
		var reasoningEnd int
		if len(headers) > 2 {
			reasoningEnd = headers[len(headers)-2].Index
		} else {
			reasoningEnd = lastHeaderIdx
		}
		
		reasoning := response[firstHeaderIdx:reasoningEnd]
		
		// Extract answer (from last header to end)
		answer := response[lastHeaderIdx:]
		
		// Clean up
		reasoning = strings.TrimSpace(reasoning)
		answer = strings.TrimSpace(answer)
		
		// If success, return results
		if reasoning != "" && answer != "" {
			return reasoning, answer
		}
	}
	
	// If still no success, try simpler paragraph-based approach
	paragraphs := strings.Split(response, "\n\n")
	if len(paragraphs) >= 3 {
		// Use first 70% as reasoning, last 30% as answer
		splitPoint := int(float64(len(paragraphs)) * 0.7)
		
		reasoning := strings.Join(paragraphs[:splitPoint], "\n\n")
		answer := strings.Join(paragraphs[splitPoint:], "\n\n")
		
		return strings.TrimSpace(reasoning), strings.TrimSpace(answer)
	}
	
	// No successful parsing
	return "", ""
}

// Helper function to cleanup raw response
func cleanupRawResponse(response string) string {
	// Remove common formatting issues
	response = strings.ReplaceAll(response, "BEGIN_REASONING", "")
	response = strings.ReplaceAll(response, "END_REASONING", "")
	response = strings.ReplaceAll(response, "BEGIN_ANSWER", "")
	response = strings.ReplaceAll(response, "END_ANSWER", "")
	
	return strings.TrimSpace(response)
}

// Helper type for tracking markdown headers
type markdownHeader struct {
	Level int    // Header level (# = 1, ## = 2, etc.)
	Text  string // Header text
	Index int    // Position in document
}

// extractMarkdownHeaders finds all markdown headers in a document
func extractMarkdownHeaders(text string) []markdownHeader {
	var headers []markdownHeader
	lines := strings.Split(text, "\n")
	
	currentPos := 0
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "#") {
			// Count # symbols for header level
			level := 0
			for i := 0; i < len(trimmedLine) && trimmedLine[i] == '#'; i++ {
				level++
			}
			
			// Extract header text
			headerText := strings.TrimSpace(trimmedLine[level:])
			
			headers = append(headers, markdownHeader{
				Level: level,
				Text:  headerText,
				Index: currentPos,
			})
		}
		
		currentPos += len(line) + 1 // +1 for newline
	}
	
	return headers
}

// validateResponse performs quality checks on the generated response
func (s *AIService) validateResponse(answer, reasoning string, steps []models.ReasoningStep, citations []models.Citation) error {
	// Validate answer
	if strings.TrimSpace(answer) == "" {
		return errors.New("empty answer from AI")
	}
	
	// Check if reasoning and answer are too similar (possible duplication)
	if reasoning != "" && answer != "" {
		similarityScore := calculateSimilarity(reasoning, answer)
		if similarityScore > 0.8 {
			return fmt.Errorf("reasoning and answer are too similar (score: %.2f)", similarityScore)
		}
	}
	
	// Check for inappropriate content
	if containsInappropriateContent(answer) || containsInappropriateContent(reasoning) {
		return errors.New("response contains potentially inappropriate content")
	}
	
	// Check for sufficient citations when results are available and mentioned
	if strings.Contains(answer, "[") && len(citations) == 0 {
		return errors.New("answer mentions citations but none were extracted")
	}
	
	return nil
}

// calculateSimilarity provides a rough measure of text similarity
func calculateSimilarity(text1, text2 string) float64 {
	// Very basic similarity check based on token overlap
	words1 := strings.Fields(strings.ToLower(text1))
	words2 := strings.Fields(strings.ToLower(text2))
	
	if len(words1) == 0 || len(words2) == 0 {
		return 0
	}
	
	// Create map of words in text1
	wordMap := make(map[string]bool)
	for _, word := range words1 {
		wordMap[word] = true
	}
	
	// Count overlapping words
	overlapCount := 0
	for _, word := range words2 {
		if wordMap[word] {
			overlapCount++
		}
	}
	
	// Normalize by length of shorter text
	minLength := min(len(words1), len(words2))
	return float64(overlapCount) / float64(minLength)
}

// containsInappropriateContent checks for potentially inappropriate content
func containsInappropriateContent(text string) bool {
	// This is a basic implementation; production would use more sophisticated approaches
	inappropriateTerms := []string{
		"porn", "nsfw", "explicit", "offensive", "graphically violent",
	}
	
	lowercaseText := strings.ToLower(text)
	for _, term := range inappropriateTerms {
		if strings.Contains(lowercaseText, term) {
			return true
		}
	}
	
	return false
}

// processWithModel sends the prompt to the AI model and gets a response
func (s *AIService) processWithModel(ctx context.Context, prompt string, modelConfig *AIModelConfig) (string, error) {
	// Implement actual API calls based on modelConfig.Provider
	log.Printf("Processing query with %s model", modelConfig.Name)
	
	// Check if we have a canceled context
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
		// Continue processing
	}
	
	switch modelConfig.Provider {
	case "Anthropic":
		return s.callAnthropicAPI(ctx, prompt, modelConfig)
	case "Google":
		return s.callGoogleAPI(ctx, prompt, modelConfig)
	case "OpenAI":
		return s.callOpenAIAPI(ctx, prompt, modelConfig)
	case "DeepSeek":
		return s.callDeepSeekAPI(ctx, prompt, modelConfig)
	default:
		// Use mock response for testing/development
		log.Printf("Using mock response for provider: %s", modelConfig.Provider)
		return s.generateMockResponse(prompt), nil
	}
}

// Fix the callAnthropicAPI function
func (s *AIService) callAnthropicAPI(ctx context.Context, prompt string, modelConfig *AIModelConfig) (string, error) {
    // Get API configuration from environment
    apiKey := os.Getenv("ANTHROPIC_API_KEY")
    if apiKey == "" {
        log.Println("Warning: ANTHROPIC_API_KEY not set, using mock response")
        return s.generateMockResponse(prompt), nil
    }
    
    // Prepare request
    type anthropicMessage struct {
        Role    string `json:"role"`
        Content string `json:"content"`
    }
    
    type anthropicRequest struct {
        Model       string             `json:"model"`
        Messages    []anthropicMessage `json:"messages"`
        MaxTokens   int                `json:"max_tokens"`
        Temperature float32            `json:"temperature"`
    }
    
    // Determine model name based on configuration
    modelName := "claude-3-opus-20240229" // Default model
    if modelConfig.Name == "Claude" {
        modelName = "claude-3-opus-20240229"
    }
    
    request := anthropicRequest{
        Model: modelName,
        Messages: []anthropicMessage{
            {Role: "user", Content: prompt},
        },
        MaxTokens:   modelConfig.MaxTokens,
        Temperature: modelConfig.Temperature,
    }
    
    // Marshal request to JSON
    requestBody, err := json.Marshal(request)
    if err != nil {
        return "", fmt.Errorf("error marshaling request: %w", err)
    }
    
    // Create HTTP request with updated timeout
    client := &http.Client{Timeout: 50 * time.Second} // Increase timeout
    
    req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(requestBody))
    if err != nil {
        return "", fmt.Errorf("error creating request: %w", err)
    }
    
    // Set headers
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("x-api-key", apiKey)
    req.Header.Set("anthropic-version", "2023-06-01")
    
    // Make the request
    resp, err := client.Do(req)
    if err != nil {
        return "", fmt.Errorf("error making request to Anthropic API: %w", err)
    }
    defer resp.Body.Close()
    
    // Check response status
    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("error response from Anthropic API (status %d): %s", resp.StatusCode, string(bodyBytes))
    }
    
    // Parse response
    var response struct {
        Content []struct {
            Type  string `json:"type"`
            Text  string `json:"text"`
        } `json:"content"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
        return "", fmt.Errorf("error parsing Anthropic API response: %w", err)
    }
    
    // Extract text from response
    var resultText string
    for _, content := range response.Content {
        if content.Type == "text" {
            resultText += content.Text
        }
    }
    
    if resultText == "" {
        return "", errors.New("empty response from Anthropic API")
    }
    
    return resultText, nil
}

// Also fix the callGoogleAPI function
func (s *AIService) callGoogleAPI(ctx context.Context, prompt string, modelConfig *AIModelConfig) (string, error) {
    // Get API configuration from environment
    apiKey := os.Getenv("GOOGLE_API_KEY")
    if apiKey == "" {
        log.Println("Warning: GOOGLE_API_KEY not set, using mock response")
        return s.generateMockResponse(prompt), nil
    }
    
    // Prepare request
    type googleContent struct {
        Parts []struct {
            Text string `json:"text"`
        } `json:"parts"`
    }
    
    type googleRequest struct {
        Contents    []googleContent `json:"contents"`
        Model       string          `json:"model"`
        // Updated field names to match Google API
        Temperature     float32         `json:"temperature"`
        MaxOutputTokens int             `json:"max_output_tokens"`
    }
    
    // Determine model name based on configuration
    modelName := "Gemini 2.0 Flash" // Default model
    
    request := googleRequest{
        Contents: []googleContent{
            {
                Parts: []struct {
                    Text string `json:"text"`
                }{
                    {Text: prompt},
                },
            },
        },
        Model:           modelName,
        Temperature:     modelConfig.Temperature,
        MaxOutputTokens: modelConfig.MaxTokens,
    }
    
    // Marshal request to JSON
    requestBody, err := json.Marshal(request)
    if err != nil {
        return "", fmt.Errorf("error marshaling request: %w", err)
    }
    
    // Create HTTP request
    apiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", modelName, apiKey)
    req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(requestBody))
    if err != nil {
        return "", fmt.Errorf("error creating request: %w", err)
    }
    
    // Set headers
    req.Header.Set("Content-Type", "application/json")
    
    // Make the request
    client := &http.Client{Timeout: 50 * time.Second} // Increased timeout
    resp, err := client.Do(req)
    if err != nil {
        return "", fmt.Errorf("error making request to Google API: %w", err)
    }
    defer resp.Body.Close()
    
    // Print the full request body for debugging
    log.Printf("Google API request body: %s", string(requestBody))
    
    // Check response status
    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("error response from Google API (status %d): %s", resp.StatusCode, string(bodyBytes))
    }
    
    // Parse response
    var response struct {
        Candidates []struct {
            Content struct {
                Parts []struct {
                    Text string `json:"text"`
                } `json:"parts"`
            } `json:"content"`
        } `json:"candidates"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
        return "", fmt.Errorf("error parsing Google API response: %w", err)
    }
    
    // Extract text from response
    var resultText string
    if len(response.Candidates) > 0 && len(response.Candidates[0].Content.Parts) > 0 {
        for _, part := range response.Candidates[0].Content.Parts {
            resultText += part.Text
        }
    }
    
    if resultText == "" {
        return "", errors.New("empty response from Google API")
    }
    
    return resultText, nil
}

// callOpenAIAPI makes API calls to OpenAI models
func (s *AIService) callOpenAIAPI(ctx context.Context, prompt string, modelConfig *AIModelConfig) (string, error) {
	// Get API configuration from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Println("Warning: OPENAI_API_KEY not set, using mock response")
		return s.generateMockResponse(prompt), nil
	}
	
	// Prepare request
	type openaiMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	
	type openaiRequest struct {
		Model       string          `json:"model"`
		Messages    []openaiMessage `json:"messages"`
		Temperature float32         `json:"temperature"`
		MaxTokens   int             `json:"max_tokens"`
	}
	
	// Determine model name based on configuration
	modelName := "gpt-4" // Default model
	
	request := openaiRequest{
		Model: modelName,
		Messages: []openaiMessage{
			{Role: "system", Content: "You are a helpful assistant that analyzes Reddit search results."},
			{Role: "user", Content: prompt},
		},
		Temperature: modelConfig.Temperature,
		MaxTokens:   modelConfig.MaxTokens,
	}
	
	// Marshal request to JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %w", err)
	}
	
	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	
	// Make the request
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request to OpenAI API: %w", err)
	}
	defer resp.Body.Close()
	
	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("error response from OpenAI API (status %d): %s", resp.StatusCode, string(bodyBytes))
	}
	
	// Parse response
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("error parsing OpenAI API response: %w", err)
	}
	
	// Extract text from response
	var resultText string
	if len(response.Choices) > 0 {
		resultText = response.Choices[0].Message.Content
	}
	
	if resultText == "" {
		return "", errors.New("empty response from OpenAI API")
	}
	
	return resultText, nil
}

// callDeepSeekAPI makes API calls to DeepSeek models
func (s *AIService) callDeepSeekAPI(ctx context.Context, prompt string, modelConfig *AIModelConfig) (string, error) {
	// Get API configuration from environment
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		log.Println("Warning: DEEPSEEK_API_KEY not set, using mock response")
		return s.generateMockResponse(prompt), nil
	}
	
	// Prepare request
	type deepseekMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	
	type deepseekRequest struct {
		Model       string            `json:"model"`
		Messages    []deepseekMessage `json:"messages"`
		Temperature float32           `json:"temperature"`
		MaxTokens   int               `json:"max_tokens"`
	}
	
	// Determine model name based on configuration
	modelName := "deepseek-chat" // Default model
	
	request := deepseekRequest{
		Model: modelName,
		Messages: []deepseekMessage{
			{Role: "user", Content: prompt},
		},
		Temperature: modelConfig.Temperature,
		MaxTokens:   modelConfig.MaxTokens,
	}
	
	// Marshal request to JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %w", err)
	}
	
	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.deepseek.com/v1/chat/completions", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	
	// Make the request
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request to DeepSeek API: %w", err)
	}
	defer resp.Body.Close()
	
	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("error response from DeepSeek API (status %d): %s", resp.StatusCode, string(bodyBytes))
	}
	
	// Parse response
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("error parsing DeepSeek API response: %w", err)
	}
	
	// Extract text from response
	var resultText string
	if len(response.Choices) > 0 {
		resultText = response.Choices[0].Message.Content
	}
	
	if resultText == "" {
		return "", errors.New("empty response from DeepSeek API")
	}
	
	return resultText, nil
}

// generateMockResponse creates a realistic looking AI response for testing
func (s *AIService) generateMockResponse(prompt string) string {
	// This is a simplified mock that returns a formatted response
	// In production, this would be replaced with actual API calls
	
	return `BEGIN_REASONING
Based on the search results, I can see several key points related to the query:

1. Multiple Reddit users have discussed this topic across different subreddits
2. There appears to be a consensus on the main points, with some minor disagreements
3. The most reliable information comes from results [1], [2], and [4], which provide detailed explanations
4. Results [3] and [5] offer more subjective perspectives but add valuable context

The information from r/science seems particularly reliable as it references peer-reviewed studies, while the personal experiences shared in r/askreddit provide valuable real-world context.
END_REASONING

BEGIN_ANSWER
# Analysis of Reddit Discussions

Based on the search results, three main perspectives emerge:

## Primary Finding
The most common view, supported by multiple Reddit discussions [1][2], is that this topic has significant implications for most users. As one Redditor explained: "The impact can be seen across multiple domains, particularly in how people interact with the technology" [4].

## Alternative Perspectives
However, some users suggest different interpretations, noting that "results may vary depending on individual circumstances" [3]. This nuance is important when considering the broader implications.

## Expert Insights
Posts from specialized subreddits offer more technical explanations, pointing out that "the underlying mechanisms are still being researched" [5], which suggests caution when drawing definitive conclusions.

In summary, while there's general agreement on the basic principles, individual experiences and specific contexts can lead to varying outcomes.
END_ANSWER`
}

// formatTimeAgo formats a time as a human-readable "time ago" string
func formatTimeAgo(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)
	
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 48*time.Hour:
		return "yesterday"
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%d days ago", days)
	case diff < 30*24*time.Hour:
		weeks := int(diff.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	case diff < 365*24*time.Hour:
		months := int(diff.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(diff.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

// Helper function for finding minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}