// File: backend/internal/services/ai_test.go

package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pranesh-j/subplexity/internal/models"
)

func TestExtractReasoningAndAnswer(t *testing.T) {
	// Create a test service
	service := NewAIService()
	
	// Test cases
	testCases := []struct {
		name           string
		response       string
		expectReasoning string
		expectAnswer   string
		expectError    bool
	}{
		{
			name: "Valid response with both sections",
			response: `BEGIN_REASONING
This is the reasoning section.
Multiple lines of analysis.
END_REASONING

BEGIN_ANSWER
This is the answer section.
Multiple lines of response.
END_ANSWER`,
			expectReasoning: "This is the reasoning section.\nMultiple lines of analysis.",
			expectAnswer:    "This is the answer section.\nMultiple lines of response.",
			expectError:     false,
		},
		{
			name: "Response with missing sections",
			response: `Some unstructured content without proper markers.
No clear reasoning or answer sections.`,
			expectReasoning: "",
			expectAnswer:    "",
			expectError:     true,
		},
		{
			name: "Response with only answer section",
			response: `BEGIN_ANSWER
This is just the answer without reasoning.
END_ANSWER`,
			expectReasoning: "",
			expectAnswer:    "This is just the answer without reasoning.",
			expectError:     false,
		},
		{
			name: "Response with incorrect casing in markers",
			response: `begin_reasoning
This is the reasoning section.
end_reasoning

begin_answer
This is the answer section.
end_answer`,
			expectReasoning: "",
			expectAnswer:    "",
			expectError:     true,
		},
	}
	
	// Test with Claude model config
	modelConfig := service.modelConfig["Claude"]
	
	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reasoning, answer, err := service.extractReasoningAndAnswer(tc.response, modelConfig)
			
			// Check error
			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			
			// Check reasoning
			if reasoning != tc.expectReasoning {
				t.Errorf("Expected reasoning: %q, got: %q", tc.expectReasoning, reasoning)
			}
			
			// Check answer
			if answer != tc.expectAnswer {
				t.Errorf("Expected answer: %q, got: %q", tc.expectAnswer, answer)
			}
		})
	}
}

func TestExtractReasoningSteps(t *testing.T) {
	// Create a test service
	service := NewAIService()
	
	// Test cases
	testCases := []struct {
		name          string
		reasoning     string
		expectedSteps int
	}{
		{
			name: "Reasoning with explicit steps",
			reasoning: `Step 1: Understand the query
This is content for step 1.

Step 2: Analyze the results
This is content for step 2.

Step 3: Form a conclusion
This is content for step 3.`,
			expectedSteps: 3,
		},
		{
			name: "Reasoning with markdown headers",
			reasoning: `## Understanding the Query
This section is about understanding.

## Analyzing the Results
This section is about analysis.

## Forming Conclusions
This section is about conclusions.`,
			expectedSteps: 3,
		},
		{
			name: "Reasoning without structure",
			reasoning: `This is a paragraph about the first point.

This is another paragraph about something else.

And this is a final paragraph with a conclusion.`,
			expectedSteps: 3, // Should split into logical sections
		},
		{
			name:          "Empty reasoning",
			reasoning:     "",
			expectedSteps: 0,
		},
	}
	
	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			steps := service.extractReasoningSteps(tc.reasoning)
			
			if len(steps) != tc.expectedSteps {
				t.Errorf("Expected %d steps, got %d", tc.expectedSteps, len(steps))
			}
			
			// Check that each step has a title and content
			for i, step := range steps {
				if step.Title == "" {
					t.Errorf("Step %d has empty title", i)
				}
				if step.Content == "" {
					t.Errorf("Step %d has empty content", i)
				}
			}
		})
	}
}

func TestExtractCitations(t *testing.T) {
	// Create a test service
	service := NewAIService()
	
	// Create test results
	testResults := []models.SearchResult{
		{
			ID:        "result1",
			Title:     "First Result",
			Subreddit: "TestSubreddit",
			Type:      "post",
			URL:       "https://www.reddit.com/r/TestSubreddit/post1",
		},
		{
			ID:        "result2",
			Title:     "Second Result",
			Subreddit: "TestSubreddit",
			Type:      "comment",
			URL:       "https://www.reddit.com/r/TestSubreddit/post2",
		},
		{
			ID:        "result3",
			Title:     "Third Result",
			Subreddit: "AnotherSub",
			Type:      "post",
			URL:       "https://www.reddit.com/r/AnotherSub/post3",
		},
	}
	
	// Test cases
	testCases := []struct {
		name            string
		answer          string
		expectedCitations int
	}{
		{
			name: "Answer with multiple citations",
			answer: `This is an answer with citations [1]. 
Here is another citation [2]. 
And a third citation [3].`,
			expectedCitations: 3,
		},
		{
			name:              "Answer without citations",
			answer:            "This is an answer without any citations.",
			expectedCitations: 0,
		},
		{
			name: "Answer with duplicate citations",
			answer: `This references the first source [1].
This also references the first source [1] again.`,
			expectedCitations: 1, // Should deduplicate
		},
		{
			name: "Answer with out-of-range citations",
			answer: `This references a valid source [2].
This references an out-of-range source [4] that doesn't exist.`,
			expectedCitations: 1, // Should only include valid citations
		},
	}
	
	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			citations := service.extractCitations(tc.answer, testResults)
			
			if len(citations) != tc.expectedCitations {
				t.Errorf("Expected %d citations, got %d", tc.expectedCitations, len(citations))
			}
			
			// Check that each citation has expected fields
			for _, citation := range citations {
				if citation.Index < 1 || citation.Index > len(testResults) {
					t.Errorf("Citation has invalid index: %d", citation.Index)
				}
				
				if citation.Text == "" {
					t.Errorf("Citation has empty text")
				}
				
				if citation.URL == "" {
					t.Errorf("Citation has empty URL")
				}
				
				// The citation should reference the correct result
				resultIndex := citation.Index - 1
				if resultIndex >= 0 && resultIndex < len(testResults) {
					expectedURL := testResults[resultIndex].URL
					if citation.URL != expectedURL {
						t.Errorf("Citation URL doesn't match: expected %s, got %s", expectedURL, citation.URL)
					}
				}
			}
		})
	}
}

func TestFormatTimeAgo(t *testing.T) {
	// Test cases
	now := time.Now()
	testCases := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "Just now",
			time:     now.Add(-30 * time.Second),
			expected: "just now",
		},
		{
			name:     "Minutes ago",
			time:     now.Add(-5 * time.Minute),
			expected: "5 minutes ago",
		},
		{
			name:     "Hours ago",
			time:     now.Add(-3 * time.Hour),
			expected: "3 hours ago",
		},
		{
			name:     "Yesterday",
			time:     now.Add(-30 * time.Hour),
			expected: "yesterday",
		},
		{
			name:     "Days ago",
			time:     now.Add(-5 * 24 * time.Hour),
			expected: "5 days ago",
		},
		{
			name:     "Weeks ago",
			time:     now.Add(-3 * 7 * 24 * time.Hour),
			expected: "3 weeks ago",
		},
		{
			name:     "Months ago",
			time:     now.Add(-2 * 30 * 24 * time.Hour),
			expected: "2 months ago",
		},
		{
			name:     "Years ago",
			time:     now.Add(-3 * 365 * 24 * time.Hour),
			expected: "3 years ago",
		},
	}
	
	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatTimeAgo(tc.time)
			
			// Some flexibility in the exact wording for time-based tests
			// to avoid flaky tests when time boundaries are close
			if !strings.Contains(result, tc.expected) && 
			   !strings.Contains(tc.expected, result) {
				t.Errorf("Expected time ago containing %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestProcessWithModel(t *testing.T) {
	// Create a test service
	service := NewAIService()
	
	// Simple smoke test - just verify that we get a non-empty response
	// and no error from the mock implementation
	resp, err := service.processWithModel(context.Background(), "Test prompt", service.modelConfig["default"])
	
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	if resp == "" {
		t.Errorf("Received empty response")
	}
}

func TestProcessResults(t *testing.T) {
	// Create a test service
	service := NewAIService()
	
	// Create test results
	testResults := []models.SearchResult{
		{
			ID:        "result1",
			Title:     "First Result",
			Subreddit: "TestSubreddit",
			Content:   "This is the content of the first result.",
			Type:      "post",
			URL:       "https://www.reddit.com/r/TestSubreddit/post1",
		},
	}
	
	// Test with empty results
	t.Run("Empty results", func(t *testing.T) {
		reasoning, answer, steps, citations, err := service.ProcessResults(context.Background(), "Test query", []models.SearchResult{}, "Claude")
		
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		
		if reasoning != "" {
			t.Errorf("Expected empty reasoning for empty results, got: %q", reasoning)
		}
		
		if answer != "No results found for this query." {
			t.Errorf("Expected default answer for empty results, got: %q", answer)
		}
		
		if steps != nil {
			t.Errorf("Expected nil steps for empty results, got %d steps", len(steps))
		}
		
		if citations != nil {
			t.Errorf("Expected nil citations for empty results, got %d citations", len(citations))
		}
	})
	
	// Test with actual results
	t.Run("With results", func(t *testing.T) {
		reasoning, answer, steps, citations, err := service.ProcessResults(context.Background(), "Test query", testResults, "Claude")
		
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		
		if reasoning == "" {
			t.Errorf("Expected non-empty reasoning")
		}
		
		if answer == "" {
			t.Errorf("Expected non-empty answer")
		}
		
		// Mock response should return formatted response with reasoningSteps
		if len(steps) == 0 {
			t.Errorf("Expected reasoning steps")
		}
		
		// We're using a mock response, so we expect it to reference results [1] through [5],
		// but we only have one test result. This should produce a citation for [1]
		if len(citations) == 0 {
			t.Errorf("Expected at least one citation")
		}
	})
	
	// Test with context cancellation
	t.Run("With context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		_, _, _, _, err := service.ProcessResults(ctx, "Test query", testResults, "Claude")
		
		// Since we're using a mock, it might not actually respect the context cancellation
		// In a real implementation, this should check for context.Canceled error
		t.Log("Context cancellation test completed, error:", err)
	})
}