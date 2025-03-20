package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/yourname/subplexity/internal/models"
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

	type anthropicMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type anthropicRequest struct {
		Model     string             `json:"model"`
		Messages  []anthropicMessage `json:"messages"`
		MaxTokens int                `json:"max_tokens"`
	}

	systemMessage := "You are an AI assistant analyzing Reddit search results. First, provide your reasoning process, then provide a comprehensive answer."
	
	// Format messages for Anthropic
	messages := []anthropicMessage{
		{Role: "system", Content: systemMessage},
		{Role: "user", Content: content},
	}

	requestBody := anthropicRequest{
		Model:     "claude-3-sonnet-20240229",
		Messages:  messages,
		MaxTokens: 1000,
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
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return "", "", fmt.Errorf("error response from Anthropic API: %s - %s", resp.Status, string(bodyBytes))
	}

	var response struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", "", fmt.Errorf("error decoding response: %w", err)
	}

	// Extract reasoning and answer from the response
	// This is a simplification - you'd want more robust parsing
	fullText := response.Content[0].Text
	reasoningEndIdx := len(fullText) / 2 // Simplified split
	
	reasoning := fullText[:reasoningEndIdx]
	answer := fullText[reasoningEndIdx:]

	return reasoning, answer, nil
}

func (s *AIService) processWithOpenAI(query, content string) (string, string, error) {
	if s.OpenAIAPIKey == "" {
		return "", "", fmt.Errorf("missing OpenAI API key")
	}

	type openAIMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type openAIRequest struct {
		Model       string          `json:"model"`
		Messages    []openAIMessage `json:"messages"`
		Temperature float64         `json:"temperature"`
	}

	systemPrompt := "You are analyzing Reddit search results. First, provide your reasoning process about the data, then provide a comprehensive answer to the user's query based on the Reddit content."

	messages := []openAIMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: content},
	}

	requestBody := openAIRequest{
		Model:       "gpt-4",
		Messages:    messages,
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
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
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

	// Extract reasoning and answer from the response
	// This is a simplification - you'd want more robust parsing
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
		Contents    []geminiContent `json:"contents"`
		Model       string          `json:"model"`
		Temperature float64         `json:"temperature"`
	}

	systemPrompt := "You are analyzing Reddit search results. First, provide your reasoning process about the data, then provide a comprehensive answer to the user's query based on the Reddit content."

	// Format content for Gemini
	requestBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{{Text: systemPrompt}},
				Role:  "system",
			},
			{
				Parts: []geminiPart{{Text: content}},
				Role:  "user",
			},
		},
		Model:       "Gemini 1.5 Flash",
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", "", fmt.Errorf("error marshaling request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent?key="+s.GoogleAPIKey, bytes.NewBuffer(jsonData))
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
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
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

	// Extract reasoning and answer from the response
	// This is a simplification - you'd want more robust parsing
	fullText := response.Candidates[0].Content.Parts[0].Text
	reasoningEndIdx := len(fullText) / 2 // Simplified split
	
	reasoning := fullText[:reasoningEndIdx]
	answer := fullText[reasoningEndIdx:]

	return reasoning, answer, nil
}