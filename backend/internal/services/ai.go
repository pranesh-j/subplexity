// backend/internal/services/ai.go
package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

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

	systemMessage := "You are an AI assistant that analyzes Reddit search results. You provide clear reasoning followed by a comprehensive answer, with proper citations to the source posts."

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
				"content": "You are an AI assistant that analyzes Reddit search results. You provide clear reasoning followed by a comprehensive answer, with proper citations to the source posts.",
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
	// Look for markdown section headers for reasoning and answer
	reasoningRegex := regexp.MustCompile(`(?i)(?:#+\s*Reasoning|Reasoning:?)\s*`)
	answerRegex := regexp.MustCompile(`(?i)(?:#+\s*Answer|Answer:?)\s*`)
	
	reasoningMatch := reasoningRegex.FindStringIndex(fullText)
	answerMatch := answerRegex.FindStringIndex(fullText)
	
	if reasoningMatch != nil && answerMatch != nil && answerMatch[0] > reasoningMatch[0] {
		// Found both markers in the expected order
		reasoningStart := reasoningMatch[1] // End of "## Reasoning" marker
		answerStart := answerMatch[0]      // Start of "## Answer" marker
		
		reasoning := strings.TrimSpace(fullText[reasoningStart:answerStart])
		answer := strings.TrimSpace(fullText[answerMatch[1]:]) // From end of "## Answer" marker to end
		
		return reasoning, answer
	}
	
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
}