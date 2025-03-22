// File: backend/internal/services/ai_models.go

package services

import (
	"strings"

	"github.com/pranesh-j/subplexity/internal/models"
)

// AIModelConfig contains configuration for an AI model
type AIModelConfig struct {
	Name               string
	Provider           string // e.g., "OpenAI", "Anthropic"
	PromptTemplate     string // Which prompt template to use
	MaxTokens          int    // Maximum tokens for response
	MaxResultsInPrompt int    // How many search results to include in prompt
	MaxContentLength   int    // Maximum length for each content snippet
	Temperature        float32 // Randomness parameter
	SectionMarkers     map[string]string // Custom section markers for this model
	ResponseFormat     string // Expected response format
	TokenLimit         int    // Total token limit for the model
	QueryWeights       map[string]float32 // Weight different query complexities
}


// This update needs to be applied to the loadModelConfigurations function
// in backend/internal/services/ai_models.go

func loadModelConfigurations() map[string]*AIModelConfig {
	configs := map[string]*AIModelConfig{
		"Claude": {
			Name:               "Claude",
			Provider:           "Anthropic",
			PromptTemplate:     "claude",
			MaxTokens:          2000,  // Reduced from 4000
			MaxResultsInPrompt: 8,     // Reduced from 12
			MaxContentLength:   800,   // Reduced from 1500
			Temperature:        0.7,
			SectionMarkers: map[string]string{
				"reasoning_start": "BEGIN_REASONING",
				"reasoning_end":   "END_REASONING",
				"answer_start":    "BEGIN_ANSWER",
				"answer_end":      "END_ANSWER",
			},
			ResponseFormat: "markdown",
			TokenLimit:     100000,
			QueryWeights: map[string]float32{
				"analytical": 1.2,
				"technical":  1.1,
				"subjective": 0.9,
			},
		},
		"DeepSeek R1": {
			Name:               "DeepSeek R1",
			Provider:           "DeepSeek",
			PromptTemplate:     "deepseek",
			MaxTokens:          2000,
			MaxResultsInPrompt: 5,
			MaxContentLength:   800,
			Temperature:        0.7,
			SectionMarkers: map[string]string{
				"reasoning_start": "BEGIN_REASONING",
				"reasoning_end":   "END_REASONING",
				"answer_start":    "BEGIN_ANSWER",
				"answer_end":      "END_ANSWER",
			},
			ResponseFormat: "markdown",
			TokenLimit:     32768,
			QueryWeights: map[string]float32{
				"analytical": 1.0,
				"technical":  1.3,
				"subjective": 0.8,
			},
		},
		"Google Gemini": {
			Name:               "Google Gemini",
			Provider:           "Google",
			PromptTemplate:     "gemini",
			MaxTokens:          2000,
			MaxResultsInPrompt: 5,
			MaxContentLength:   800,
			Temperature:        0.7,
			SectionMarkers: map[string]string{
				"reasoning_start": "BEGIN_REASONING",
				"reasoning_end":   "END_REASONING",
				"answer_start":    "BEGIN_ANSWER",
				"answer_end":      "END_ANSWER",
			},
			ResponseFormat: "markdown",
			TokenLimit:     128000, // Updated for Gemini 2.0 Flash
			QueryWeights: map[string]float32{
				"analytical": 1.1,
				"technical":  1.0,
				"subjective": 1.0,
			},
		},
	}
	
	// Add a default configuration that will be used if model is not found
	configs["default"] = &AIModelConfig{
		Name:               "Default",
		Provider:           "Anthropic",
		PromptTemplate:     "default",
		MaxTokens:          1500,
		MaxResultsInPrompt: 5,
		MaxContentLength:   500,
		Temperature:        0.7,
		SectionMarkers: map[string]string{
			"reasoning_start": "BEGIN_REASONING",
			"reasoning_end":   "END_REASONING",
			"answer_start":    "BEGIN_ANSWER",
			"answer_end":      "END_ANSWER",
		},
		ResponseFormat: "text",
		TokenLimit:     16384,
		QueryWeights: map[string]float32{
			"analytical": 1.0,
			"technical":  1.0,
			"subjective": 1.0,
		},
	}
	
	return configs
}

// SelectModelForQuery determines the best model based on query and results
func SelectModelForQuery(query string, results []models.SearchResult, availableModels map[string]*AIModelConfig) *AIModelConfig {
	// If only one model available, use it
	if len(availableModels) <= 1 {
		for _, model := range availableModels {
			return model
		}
		return nil // Should never happen
	}
	
	// Analyze query characteristics
	queryType := analyzeQueryType(query)
	complexity := calculateQueryComplexity(query, results)
	
	// Score each model
	type scoredModel struct {
		config *AIModelConfig
		score  float32
	}
	
	var scoredModels []scoredModel
	
	for _, model := range availableModels {
		// Skip the default model
		if model.Name == "Default" {
			continue
		}
		
		// Base score on complexity
		score := float32(model.TokenLimit) / 10000.0 // Higher token limit is better
		
		// Adjust score based on query characteristics
		if weight, ok := model.QueryWeights[queryType]; ok {
			score *= weight
		}
		
		// Adjust for complexity
		if complexity > 7 && model.TokenLimit >= 32000 {
			score *= 1.5 // Strongly prefer models with high token limits for complex queries
		}
		
		scoredModels = append(scoredModels, scoredModel{
			config: model,
			score:  score,
		})
	}
	
	// Find highest scoring model
	var bestModel *AIModelConfig
	var bestScore float32 = -1
	
	for _, scoredModel := range scoredModels {
		if scoredModel.score > bestScore {
			bestScore = scoredModel.score
			bestModel = scoredModel.config
		}
	}
	
	// If no model selected, use default
	if bestModel == nil {
		return availableModels["default"]
	}
	
	return bestModel
}

// analyzeQueryType determines the general category of a query
func analyzeQueryType(query string) string {
	query = strings.ToLower(query)
	
	// Check for analytical queries
	analyticalTerms := []string{
		"analyze", "compare", "difference", "explain", "why", "how does", 
		"implications", "effects", "impact", "research", "study", "findings",
	}
	
	// Check for technical queries
	technicalTerms := []string{
		"code", "algorithm", "programming", "technical", "function", "engineering",
		"implementation", "hardware", "software", "architecture", "framework",
	}
	
	// Check for subjective queries
	subjectiveTerms := []string{
		"best", "favorite", "opinion", "think", "feel", "recommend", "should i",
		"better", "worse", "good", "bad", "like", "dislike", "worth",
	}
	
	// Count matches for each category
	analyticalCount := countTermMatches(query, analyticalTerms)
	technicalCount := countTermMatches(query, technicalTerms)
	subjectiveCount := countTermMatches(query, subjectiveTerms)
	
	// Determine primary type
	if analyticalCount > technicalCount && analyticalCount > subjectiveCount {
		return "analytical"
	} else if technicalCount > analyticalCount && technicalCount > subjectiveCount {
		return "technical"
	} else if subjectiveCount > 0 {
		return "subjective"
	}
	
	// Default to analytical for general queries
	return "analytical"
}

// countTermMatches counts how many terms from the list appear in the text
func countTermMatches(text string, terms []string) int {
	count := 0
	for _, term := range terms {
		if strings.Contains(text, term) {
			count++
		}
	}
	return count
}

// calculateQueryComplexity scores the complexity of a query from 1-10
func calculateQueryComplexity(query string, results []models.SearchResult) int {
	complexity := 0
	
	// Factor 1: Query length (1-3 points)
	words := len(strings.Fields(query))
	if words > 20 {
		complexity += 3
	} else if words > 10 {
		complexity += 2
	} else {
		complexity += 1
	}
	
	// Factor 2: Question marks (0-2 points)
	questionMarks := strings.Count(query, "?")
	complexity += min(questionMarks, 2)
	
	// Factor 3: Advanced query indicators (0-2 points)
	if containsAny(query, []string{"compare", "versus", "difference", "explain", "analyze", "implications"}) {
		complexity += 2
	}
	
	// Factor 4: Result diversity (0-3 points)
	subreddits := make(map[string]bool)
	for _, result := range results {
		subreddits[result.Subreddit] = true
	}
	
	if len(subreddits) > 5 {
		complexity += 3
	} else if len(subreddits) > 3 {
		complexity += 2
	} else if len(subreddits) > 1 {
		complexity += 1
	}
	
	return complexity
}

// Helper function to check if text contains any keywords
func containsAny(text string, keywords []string) bool {
	text = strings.ToLower(text)
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}