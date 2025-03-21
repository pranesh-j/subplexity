package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	// Prepare the content for the AI
	var content string
	content += fmt.Sprintf("Query: %s\n\n", query)
	content += "Reddit Search Results:\n"

	for i, result := range results {
		if i >= 10 { // Limit to top 10 results for AI processing
			break
		}

		content += fmt.Sprintf("---\nResult %d:\n", i+1)
		content += fmt.Sprintf("Title: %s\n", result.Title)
		content += fmt.Sprintf("Subreddit: r/%s\n", result.Subreddit)
		content += fmt.Sprintf("Author: u/%s\n", result.Author)
		content += fmt.Sprintf("Type: %s\n", result.Type)
		content += fmt.Sprintf("Score: %d\n", result.Score)
		content += fmt.Sprintf("Content: %s\n", result.Content)
		content += fmt.Sprintf("URL: %s\n", result.URL)
		content += "---\n\n"
	}

	switch modelName {
	case "Claude":
		return s.processWithAnthropic(query, content)
	case "DeepSeek R1":
		return s.processWithOpenAI(query, content) // DeepSeek integration would be here
	case "Google Gemini":
		return s.processWithGemini(query, content)
	default:
		return s.processWithAnthropic(query, content)
	}
}

func (s *AIService) processWithAnthropic(query, content string) (string, string, error) {
	if s.AnthropicAPIKey == "" {
		return "", "", fmt.Errorf("missing Anthropic API key")
	}

	// Fix: Correct structure for Anthropic API v1/messages
	type anthropicRequest struct {
		Model       string      `json:"model"`
		System      string      `json:"system"`
		Messages    []map[string]string `json:"messages"`
		MaxTokens   int         `json:"max_tokens"`
		Temperature float64     `json:"temperature"`
	}

	systemMessage := "You are an AI assistant analyzing Reddit search results. First, provide your reasoning process, then provide a comprehensive answer."

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
		MaxTokens:   1000,
		Temperature: 0.3,
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

	// Split into reasoning and answer
	// This is a simplified approach - in production, you might want a more sophisticated algorithm
	reasoningEndIdx := len(fullText) / 2 
	
	reasoning := fullText[:reasoningEndIdx]
	answer := fullText[reasoningEndIdx:]

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
		Model: "gpt-4",
		Messages: []map[string]interface{}{
			{
				"role":    "system",
				"content": "You are analyzing Reddit search results. First, provide your reasoning process about the data, then provide a comprehensive answer to the user's query based on the Reddit content.",
			},
			{
				"role":    "user",
				"content": content,
			},
		},
		Temperature: 0.3,
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
	reasoningEndIdx := len(fullText) / 2 // Simplified split
	
	reasoning := fullText[:reasoningEndIdx]
	answer := fullText[reasoningEndIdx:]

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
    
    // Create the prompt with instructions about analyzing Reddit results
    promptText := "You are analyzing Reddit search results. First, provide your reasoning process about the data, then provide a comprehensive answer to the user's query based on the Reddit content.\n\n" + content

    // Initialize request with Gemini 2.0 Flash model
    requestBody := geminiRequest{
        Contents: []geminiContent{
            {
                Role: "user",
                Parts: []geminiPart{
                    {Text: promptText},
                },
            },
        },
        Model: "gemini-1.5-flash",
        GenerationConfig: struct {
            Temperature float64 `json:"temperature"`
        }{
            Temperature: 0.3,
        },
    }

    jsonData, err := json.Marshal(requestBody)
    if err != nil {
        return "", "", fmt.Errorf("error marshaling request: %w", err)
    }

    // Use the correct API endpoint for Gemini 2.0
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
    reasoningEndIdx := len(fullText) / 2 
    
    reasoning := fullText[:reasoningEndIdx]
    answer := fullText[reasoningEndIdx:]

    return reasoning, answer, nil
}