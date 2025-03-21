// backend/internal/services/ai.go
package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
<<<<<<< HEAD
	"regexp"
=======
>>>>>>> 7bc6577ce33208fd57365735d3a230e0599a4bf8
	"strings"
	"time"
	"regexp"

	"github.com/pranesh-j/subplexity/internal/models"
)

type AIService struct {
	AnthropicAPIKey string
	OpenAIAPIKey    string
	GoogleAPIKey    string
	httpClient      *http.Client
}

func NewAIService() *AIService {
	return &AIService{
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
		OpenAIAPIKey:    os.Getenv("OPENAI_API_KEY"),
		GoogleAPIKey:    os.Getenv("GOOGLE_API_KEY"),
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ProcessResults uses AI to analyze search results and generate a response
func (s *AIService) ProcessResults(query string, results []models.SearchResult, modelName string) (string, string, error) {
<<<<<<< HEAD
	// Create structured prompt with clear separation instructions
	prompt := buildStructuredPrompt(query, results)
=======
	// Check if query is about Bitcoin price - special handling
	if isBitcoinPriceQuery(query) {
		return s.handleBitcoinPriceQuery(query, results, modelName)
	}
>>>>>>> 7bc6577ce33208fd57365735d3a230e0599a4bf8

	// Create structured prompt with clear separation instructions
	prompt := buildStructuredPrompt(query, results)

	switch modelName {
	case "Claude":
		return s.processWithAnthropic(query, prompt)
	case "DeepSeek R1":
		return s.processWithOpenAI(query, prompt)
	case "Google Gemini":
		return s.processWithGemini(query, prompt)
	default:
		return s.processWithAnthropic(query, prompt)
	}
}

<<<<<<< HEAD
// buildStructuredPrompt creates a detailed prompt with clear instructions
func buildStructuredPrompt(query string, results []models.SearchResult) string {
	var content strings.Builder
	
	// Start with clear instructions
	content.WriteString("# Reddit Search Analysis Task\n\n")
	content.WriteString(fmt.Sprintf("User Query: %s\n\n", query))
	
	content.WriteString("## Instructions:\n")
	content.WriteString("1. Analyze the top Reddit search results below.\n")
	content.WriteString("2. Provide a detailed reasoning process that shows your analysis of the content, relevance, and reliability of the results.\n")
	content.WriteString("3. Provide a clear, comprehensive answer to the user's query based on the Reddit content.\n")
	content.WriteString("4. Format citations using [1], [2], etc. that reference the numbered results below.\n")
	content.WriteString("5. If the search results don't provide sufficient information, acknowledge this limitation.\n")
	content.WriteString("6. IMPORTANT: Your response MUST have two clearly labeled sections: '## Reasoning' and '## Answer'.\n\n")
	
	content.WriteString("## Reddit Search Results:\n\n")
	
	// Number the results for easy citation - limit to top 10 for processing
	resultLimit := len(results)
	if resultLimit > 10 {
		resultLimit = 10
	}
	
	for i, result := range results[:resultLimit] {
		content.WriteString(fmt.Sprintf("### Result %d:\n", i+1))
		content.WriteString(fmt.Sprintf("- Title: %s\n", result.Title))
		content.WriteString(fmt.Sprintf("- Subreddit: r/%s\n", result.Subreddit))
		content.WriteString(fmt.Sprintf("- Author: u/%s\n", result.Author))
		content.WriteString(fmt.Sprintf("- Type: %s\n", result.Type))
		content.WriteString(fmt.Sprintf("- Score: %d upvotes\n", result.Score))
		
		if result.CommentCount > 0 {
			content.WriteString(fmt.Sprintf("- Comments: %d\n", result.CommentCount))
		}
		
		content.WriteString(fmt.Sprintf("- Posted: %s\n", formatTimeAgo(time.Unix(result.CreatedUTC, 0))))
		content.WriteString(fmt.Sprintf("- URL: %s\n\n", result.URL))
		content.WriteString("Content:\n")
		
		// Handle empty content
		if result.Content == "" {
			content.WriteString("(No content - title only post)\n\n")
		} else {
			content.WriteString(fmt.Sprintf("%s\n\n", result.Content))
		}
		
		// Add highlights if available
		if len(result.Highlights) > 0 {
			content.WriteString("Key excerpts:\n")
			for _, highlight := range result.Highlights {
				content.WriteString(fmt.Sprintf("- \"%s\"\n", highlight))
			}
			content.WriteString("\n")
		}
		
		content.WriteString("---\n\n")
	}
	
	content.WriteString("## Expected Response Format:\n\n")
	content.WriteString("### Reasoning\n")
	content.WriteString("In this section, provide your detailed analysis of the Reddit results. Discuss:\n")
	content.WriteString("- The credibility and relevance of the sources\n")
	content.WriteString("- Agreement or disagreement among posts\n")
	content.WriteString("- Trends or patterns in the posts\n")
	content.WriteString("- How recent/current the information is\n\n")
	
	content.WriteString("### Answer\n")
	content.WriteString("In this section, provide a comprehensive, direct answer to the user's query based on your analysis.\n")
	content.WriteString("- State clear conclusions\n")
	content.WriteString("- Use specific citations like [1], [2] to reference source posts\n")
	content.WriteString("- Acknowledge any limitations in the available information\n")
	content.WriteString("- Format the answer in clear paragraphs with markdown formatting as needed\n\n")
	
	return content.String()
=======
// isBitcoinPriceQuery checks if the query is asking about Bitcoin price
func isBitcoinPriceQuery(query string) bool {
	query = strings.ToLower(query)
	return (strings.Contains(query, "bitcoin") || strings.Contains(query, "btc")) && 
	       (strings.Contains(query, "price") || 
			strings.Contains(query, "worth") || 
			strings.Contains(query, "value") || 
			strings.Contains(query, "cost") ||
			strings.Contains(query, "how much"))
}

// handleBitcoinPriceQuery provides special handling for Bitcoin price queries
func (s *AIService) handleBitcoinPriceQuery(query string, results []models.SearchResult, modelName string) (string, string, error) {
	// Find most recent price mentions in search results
	recentMentions := extractRecentPriceMentions(results)
	
	// Build appropriate reasoning and answer
	var reasoning, answer string
	
	timeNow := time.Now().Format("Jan 2, 2006 at 15:04 MST")
	
	if len(recentMentions) > 0 {
		reasoning = fmt.Sprintf(`I've analyzed the search results for Bitcoin price information.

I found %d mentions of Bitcoin price in the results, with timestamps ranging from %s to %s. 

The most recent price information I found was from a post %s which mentioned Bitcoin at: %s.

However, I need to note that this price information from Reddit posts is not real-time market data. Bitcoin prices can be quite volatile and change rapidly.`, 
			len(recentMentions),
			formatTimeAgo(recentMentions[len(recentMentions)-1].time),
			formatTimeAgo(recentMentions[0].time),
			formatTimeAgo(recentMentions[0].time),
			recentMentions[0].priceText)
		
		answer = fmt.Sprintf(`Based on the most recent information found in Reddit posts, Bitcoin was mentioned at %s. However, this is not a real-time price and might be outdated.

As of my last search (%s), this is the most recent price mention in Reddit discussions. For current, accurate Bitcoin prices, I recommend checking a cryptocurrency exchange or financial website like Coinbase, Binance, or CoinMarketCap.`, 
			recentMentions[0].priceText, 
			timeNow)
	} else {
		reasoning = fmt.Sprintf(`I've analyzed the search results but couldn't find specific Bitcoin price mentions with exact figures in the recent Reddit posts.

Reddit discussions sometimes talk about Bitcoin without mentioning exact prices, or use relative terms instead of specific figures.

Without real-time market data access, I cannot provide the current exact price of Bitcoin.`)
		
		answer = fmt.Sprintf(`I couldn't find the current Bitcoin price in the recent Reddit posts I searched.

As of %s, there were no posts with specific Bitcoin price information. For current, accurate Bitcoin prices, I recommend checking a cryptocurrency exchange or financial website like Coinbase, Binance, or CoinMarketCap.`, timeNow)
	}
	
	return reasoning, answer, nil
}

// extractRecentPriceMentions finds price mentions in results
type priceMention struct {
	priceText string
	time      time.Time
	resultID  string
}

func extractRecentPriceMentions(results []models.SearchResult) []priceMention {
	var mentions []priceMention
	
	// Regular expressions to catch price formats like "$42,000" or "42,000 USD" or "42K" etc.
	priceRegexps := []*regexp.Regexp{
		regexp.MustCompile(`\$\s*\d{1,3}(?:,\d{3})*(?:\.\d+)?`),
		regexp.MustCompile(`\d{1,3}(?:,\d{3})*(?:\.\d+)?\s*(?:USD|dollars)`),
		regexp.MustCompile(`\d{1,2}(?:\.\d+)?\s*[kK]`),
		regexp.MustCompile(`\d{1,3}(?:,\d{3})*(?:\.\d+)?\s*[kK]`),
	}
	
	for _, result := range results {
		// Check if this result mentions Bitcoin price
		if !isBitcoinPriceQuery(result.Title + " " + result.Content) {
			continue
		}
		
		// Find price mentions using regex
		var foundPrices []string
		for _, re := range priceRegexps {
			// Check title
			matches := re.FindAllString(result.Title, -1)
			foundPrices = append(foundPrices, matches...)
			
			// Check content
			matches = re.FindAllString(result.Content, -1)
			foundPrices = append(foundPrices, matches...)
		}
		
		if len(foundPrices) > 0 {
			// Convert Unix timestamp to time.Time
			postTime := time.Unix(result.CreatedUTC, 0)
			
			// Join multiple prices if found
			priceText := strings.Join(foundPrices, ", ")
			
			mentions = append(mentions, priceMention{
				priceText: priceText,
				time:      postTime,
				resultID:  result.ID,
			})
		}
	}
	
	// Sort by time, most recent first
	if len(mentions) > 1 {
		for i := 0; i < len(mentions)-1; i++ {
			for j := i + 1; j < len(mentions); j++ {
				if mentions[i].time.Before(mentions[j].time) {
					mentions[i], mentions[j] = mentions[j], mentions[i]
				}
			}
		}
	}
	
	return mentions
>>>>>>> 7bc6577ce33208fd57365735d3a230e0599a4bf8
}

// formatTimeAgo converts a time to a human-readable "X time ago" format
func formatTimeAgo(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)
	
	if diff < time.Minute {
		return "just now"
	} else if diff < time.Hour {
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if diff < 48*time.Hour {
		return "yesterday"
	} else if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%d days ago", days)
	} else if diff < 30*24*time.Hour {
		weeks := int(diff.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	} else if diff < 365*24*time.Hour {
		months := int(diff.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
	
	years := int(diff.Hours() / 24 / 365)
	if years == 1 {
		return "1 year ago"
	}
	return fmt.Sprintf("%d years ago", years)
}

<<<<<<< HEAD
=======
// buildStructuredPrompt creates a detailed prompt with clear instructions
func buildStructuredPrompt(query string, results []models.SearchResult) string {
	var content strings.Builder
	
	// Start with clear instructions
	content.WriteString("# Query and Analysis Instructions\n\n")
	content.WriteString(fmt.Sprintf("User Query: %s\n\n", query))
	content.WriteString("Instructions:\n")
	content.WriteString("1. Analyze the Reddit search results below and provide your reasoning process.\n")
	content.WriteString("2. After your reasoning, provide a concise, clear answer to the user query.\n")
	content.WriteString("3. Include specific citations for facts and quotes from the search results.\n")
	content.WriteString("4. If the search results don't have enough information to answer accurately, acknowledge this in your response.\n")
	content.WriteString("5. IMPORTANT: Your response must have two CLEARLY SEPARATED sections: 'Reasoning' and 'Answer'.\n\n")
	
	content.WriteString("# Reddit Search Results:\n\n")
	
	// Number the results for easy citation
	for i, result := range results {
		if i >= 10 { // Limit to top 10 results for processing
			break
		}
		
		content.WriteString(fmt.Sprintf("## Result %d:\n", i+1))
		content.WriteString(fmt.Sprintf("- Title: %s\n", result.Title))
		content.WriteString(fmt.Sprintf("- Subreddit: r/%s\n", result.Subreddit))
		content.WriteString(fmt.Sprintf("- Author: u/%s\n", result.Author))
		content.WriteString(fmt.Sprintf("- Type: %s\n", result.Type))
		content.WriteString(fmt.Sprintf("- Score: %d\n", result.Score))
		content.WriteString(fmt.Sprintf("- Posted: %s\n", formatTimeAgo(time.Unix(result.CreatedUTC, 0))))
		content.WriteString(fmt.Sprintf("- URL: %s\n\n", result.URL))
		content.WriteString("Content:\n")
		content.WriteString(fmt.Sprintf("%s\n\n", result.Content))
		content.WriteString("---\n\n")
	}
	
	content.WriteString("# Response Template:\n\n")
	content.WriteString("## Reasoning\n")
	content.WriteString("[Your detailed reasoning process here, analyzing the search results and their relevance to the query.]\n\n")
	content.WriteString("## Answer\n")
	content.WriteString("[Your clear, concise answer to the user's query here, with citations to specific results where appropriate.]\n\n")
	
	return content.String()
}

>>>>>>> 7bc6577ce33208fd57365735d3a230e0599a4bf8
func (s *AIService) processWithAnthropic(query, content string) (string, string, error) {
	if s.AnthropicAPIKey == "" {
		return "", "", fmt.Errorf("missing Anthropic API key")
	}

	// Structure for Anthropic API v1/messages
	type anthropicRequest struct {
		Model       string                   `json:"model"`
		System      string                   `json:"system"`
		Messages    []map[string]string      `json:"messages"`
		MaxTokens   int                      `json:"max_tokens"`
		Temperature float64                  `json:"temperature"`
	}

<<<<<<< HEAD
	systemMessage := "You are an AI assistant that analyzes Reddit search results. You provide clear reasoning followed by a comprehensive answer, with proper citations to the source posts."
=======
	systemMessage := "You are an AI assistant analyzing Reddit search results. You must provide your reasoning process and then a comprehensive answer as separate sections."
>>>>>>> 7bc6577ce33208fd57365735d3a230e0599a4bf8

	// Create the request body with the correct structure
	requestBody := anthropicRequest{
		Model:     "claude-3-sonnet-20240229",
		System:    systemMessage,
		Messages: []map[string]string{
			{
				"role":    "user",
				"content": content,
			},
		},
		MaxTokens:   2000,
		Temperature: 0.2,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", "", fmt.Errorf("error marshaling request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", s.AnthropicAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("error response from Anthropic API: %s - %s", resp.Status, string(bodyBytes))
	}

	var response struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", "", fmt.Errorf("error decoding response: %w", err)
	}

	// Extract all text content from response
	var fullText string
	for _, content := range response.Content {
		if content.Type == "text" {
			fullText += content.Text
		}
	}

	// Split response into reasoning and answer 
	reasoning, answer := extractReasoningAndAnswer(fullText)
	
	return reasoning, answer, nil
}

func (s *AIService) processWithOpenAI(query, content string) (string, string, error) {
	if s.OpenAIAPIKey == "" {
		return "", "", fmt.Errorf("missing OpenAI API key")
	}

	type openAIRequest struct {
		Model       string                   `json:"model"`
		Messages    []map[string]interface{} `json:"messages"`
		Temperature float64                  `json:"temperature"`
	}

	requestBody := openAIRequest{
		Model: "gpt-4-turbo",
		Messages: []map[string]interface{}{
			{
				"role":    "system",
				"content": "You are analyzing Reddit search results. Provide two clearly separated sections: 1) your reasoning process analyzing the data, and 2) a comprehensive answer to the user's query based on the Reddit content.",
			},
			{
				"role":    "user",
				"content": content,
			},
		},
		Temperature: 0.2,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", "", fmt.Errorf("error marshaling request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.OpenAIAPIKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("error response from OpenAI API: %s - %s", resp.Status, string(bodyBytes))
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", "", fmt.Errorf("error decoding response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", "", fmt.Errorf("no response content from OpenAI API")
	}

	fullText := response.Choices[0].Message.Content
	reasoning, answer := extractReasoningAndAnswer(fullText)
	
	return reasoning, answer, nil
}

func (s *AIService) processWithGemini(query, content string) (string, string, error) {
    if s.GoogleAPIKey == "" {
        return "", "", fmt.Errorf("missing Google API key")
    }

    type geminiPart struct {
        Text string `json:"text"`
    }

    type geminiContent struct {
        Parts []geminiPart `json:"parts"`
        Role  string       `json:"role"`
    }

    type geminiRequest struct {
        Contents []geminiContent `json:"contents"`
        Model    string          `json:"model"`
        GenerationConfig struct {
            Temperature float64 `json:"temperature"`
        } `json:"generationConfig"`
    }
    
    // Create the prompt with clear formatting instructions
    promptText := content

    // Initialize request with Gemini model
    requestBody := geminiRequest{
        Contents: []geminiContent{
            {
                Role: "user",
                Parts: []geminiPart{
                    {Text: content},
                },
            },
        },
        Model: "gemini-1.5-flash",
        GenerationConfig: struct {
            Temperature float64 `json:"temperature"`
        }{
            Temperature: 0.2,
        },
    }

    jsonData, err := json.Marshal(requestBody)
    if err != nil {
        return "", "", fmt.Errorf("error marshaling request: %w", err)
    }

    req, err := http.NewRequest("POST", "https://generativelanguage.googleapis.com/v1/models/gemini-1.5-flash:generateContent?key="+s.GoogleAPIKey, bytes.NewBuffer(jsonData))
    if err != nil {
        return "", "", fmt.Errorf("error creating request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")

    resp, err := s.httpClient.Do(req)
    if err != nil {
        return "", "", fmt.Errorf("error making request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := io.ReadAll(resp.Body)
        return "", "", fmt.Errorf("error response from Google API: %s - %s", resp.Status, string(bodyBytes))
    }

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
        return "", "", fmt.Errorf("error decoding response: %w", err)
    }
    
    if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
        return "", "", fmt.Errorf("no response content from Gemini API")
    }

    fullText := response.Candidates[0].Content.Parts[0].Text
    reasoning, answer := extractReasoningAndAnswer(fullText)
    
    return reasoning, answer, nil
}

// extractReasoningAndAnswer separates the response into reasoning and answer sections
func extractReasoningAndAnswer(fullText string) (string, string) {
<<<<<<< HEAD
	// Look for markdown section headers for reasoning and answer
	reasoningRegex := regexp.MustCompile(`(?i)(?:#+\s*Reasoning|Reasoning:?)\s*`)
	answerRegex := regexp.MustCompile(`(?i)(?:#+\s*Answer|Answer:?)\s*`)
=======
	// Try to find sections marked explicitly
	reasoningPattern := `(?i)(?:##?\s*Reasoning|Reasoning:?\s*)[^\n]*\n`
	answerPattern := `(?i)(?:##?\s*Answer|Answer:?\s*)[^\n]*\n`
	
	reasoningRegex := regexp.MustCompile(reasoningPattern)
	answerRegex := regexp.MustCompile(answerPattern)
>>>>>>> 7bc6577ce33208fd57365735d3a230e0599a4bf8
	
	reasoningMatch := reasoningRegex.FindStringIndex(fullText)
	answerMatch := answerRegex.FindStringIndex(fullText)
	
<<<<<<< HEAD
	if reasoningMatch != nil && answerMatch != nil && answerMatch[0] > reasoningMatch[0] {
		// Found both markers in the expected order
		reasoningStart := reasoningMatch[1] // End of "## Reasoning" marker
		answerStart := answerMatch[0]      // Start of "## Answer" marker
=======
	if reasoningMatch != nil && answerMatch != nil {
		// We found both markers
		reasoningStart := reasoningMatch[1] // End of "## Reasoning" marker
		answerStart := answerMatch[0] // Start of "## Answer" marker
>>>>>>> 7bc6577ce33208fd57365735d3a230e0599a4bf8
		
		reasoning := strings.TrimSpace(fullText[reasoningStart:answerStart])
		answer := strings.TrimSpace(fullText[answerMatch[1]:]) // From end of "## Answer" marker to end
		
		return reasoning, answer
	}
	
<<<<<<< HEAD
	// Fallback: If we can't find clear markers, try to split the response roughly in half
	midpoint := len(fullText) / 2
	
	// Try to find a paragraph break near the midpoint
	for i := midpoint; i < len(fullText)-1; i++ {
		if fullText[i] == '\n' && fullText[i+1] == '\n' {
			return strings.TrimSpace(fullText[:i]), strings.TrimSpace(fullText[i+2:])
		}
	}
	
	// Last resort: Just split in half
	return strings.TrimSpace(fullText[:midpoint]), strings.TrimSpace(fullText[midpoint:])
=======
	// Fallback: If we didn't find explicit markers, try to split the text in half
	// First, check if the text contains the word "reasoning" and "answer"
	lowerText := strings.ToLower(fullText)
	reasoningIdx := strings.Index(lowerText, "reasoning")
	answerIdx := strings.Index(lowerText, "answer")
	
	if reasoningIdx >= 0 && answerIdx >= 0 && answerIdx > reasoningIdx {
		// Find the next paragraph break after "reasoning"
		reasoningStart := reasoningIdx
		for reasoningStart < len(fullText) && fullText[reasoningStart] != '\n' {
			reasoningStart++
		}
		reasoningStart++ // Skip the newline
		
		// Use answerIdx as a rough divider
		reasoning := strings.TrimSpace(fullText[reasoningStart:answerIdx])
		
		// Find the next paragraph break after "answer"
		answerStart := answerIdx
		for answerStart < len(fullText) && fullText[answerStart] != '\n' {
			answerStart++
		}
		answerStart++ // Skip the newline
		
		answer := strings.TrimSpace(fullText[answerStart:])
		
		return reasoning, answer
	}
	
	// Last resort: Just split roughly in half
	midpoint := len(fullText) / 2
	
	// Try to find a paragraph break near the midpoint
	for midpoint < len(fullText) - 1 && fullText[midpoint] != '\n' {
		midpoint++
	}
	
	if midpoint < len(fullText) - 1 {
		reasoning := strings.TrimSpace(fullText[:midpoint])
		answer := strings.TrimSpace(fullText[midpoint+1:])
		return reasoning, answer
	}
	
	// If all else fails, just return the whole text twice
	return fullText, fullText
>>>>>>> 7bc6577ce33208fd57365735d3a230e0599a4bf8
}