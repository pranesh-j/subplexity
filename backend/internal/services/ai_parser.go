// File: backend/internal/services/ai_parser.go

package services

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pranesh-j/subplexity/internal/models"
)

// extractReasoningAndAnswer extracts the reasoning and answer sections from the AI response
func (s *AIService) extractReasoningAndAnswer(response string, modelConfig *AIModelConfig) (string, string, error) {
	// Get the section markers for the model
	reasoningStart := modelConfig.SectionMarkers["reasoning_start"]
	reasoningEnd := modelConfig.SectionMarkers["reasoning_end"]
	answerStart := modelConfig.SectionMarkers["answer_start"]
	answerEnd := modelConfig.SectionMarkers["answer_end"]
	
	// Create pattern for extracting reasoning section
	reasoningPattern := fmt.Sprintf(`(?s)%s\s*(.*?)\s*%s`, 
		regexp.QuoteMeta(reasoningStart), 
		regexp.QuoteMeta(reasoningEnd))
	
	reasoningRegex := regexp.MustCompile(reasoningPattern)
	reasoningMatches := reasoningRegex.FindStringSubmatch(response)
	
	// Create pattern for extracting answer section
	answerPattern := fmt.Sprintf(`(?s)%s\s*(.*?)\s*%s`, 
		regexp.QuoteMeta(answerStart), 
		regexp.QuoteMeta(answerEnd))
	
	answerRegex := regexp.MustCompile(answerPattern)
	answerMatches := answerRegex.FindStringSubmatch(response)
	
	// Extract sections
	var reasoning, answer string
	
	if len(reasoningMatches) > 1 {
		reasoning = strings.TrimSpace(reasoningMatches[1])
	}
	
	if len(answerMatches) > 1 {
		answer = strings.TrimSpace(answerMatches[1])
	}
	
	// Validate extraction
	if reasoning == "" && answer == "" {
		// Try alternative extraction methods
		return "", "", fmt.Errorf("failed to extract reasoning and answer with primary method")
	}
	
	// Clean up sections
	reasoning = cleanupSection(reasoning)
	answer = cleanupSection(answer)
	
	return reasoning, answer, nil
}

// cleanupSection performs post-processing on extracted sections
func cleanupSection(text string) string {
	// Clean up any remaining section markers
	text = regexp.MustCompile(`(?i)BEGIN_(?:REASONING|ANSWER)`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?i)END_(?:REASONING|ANSWER)`).ReplaceAllString(text, "")
	
	// Remove extra whitespace
	text = strings.TrimSpace(text)
	
	// Remove excess newlines (more than 2 consecutive)
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")
	
	return text
}

// extractReasoningSteps extracts structured reasoning steps from the reasoning text
func (s *AIService) extractReasoningSteps(reasoning string) []models.ReasoningStep {
	if reasoning == "" {
		return nil
	}
	
	var steps []models.ReasoningStep
	
	// Try to find explicit step headers first (Step 1: ..., Step 2: ..., etc.)
	stepPattern := `(?im)^(?:#+\s*)?(?:Step\s+)?([0-9]+)(?:[:\.]\s+|\s*-\s*)(.+?)$`
	stepRegex := regexp.MustCompile(stepPattern)
	stepMatches := stepRegex.FindAllStringSubmatchIndex(reasoning, -1)
	
	if len(stepMatches) > 0 {
		// Found explicit steps
		for i, match := range stepMatches {
			// Extract step title
			titleStart := match[4]
			titleEnd := match[5]
			title := reasoning[titleStart:titleEnd]
			
			// Extract content (from end of this match to start of next match or end of text)
			contentStart := match[1] // End of the entire match
			contentEnd := len(reasoning)
			if i < len(stepMatches)-1 {
				contentEnd = stepMatches[i+1][0] // Start of next match
			}
			
			content := reasoning[contentStart:contentEnd]
			
			// Clean up
			title = strings.TrimSpace(title)
			content = strings.TrimSpace(content)
			
			// Add step
			steps = append(steps, models.ReasoningStep{
				Title:   title,
				Content: content,
			})
		}
		
		return steps
	}
	
	// If no explicit steps found, try to find markdown headers
	headerPattern := `(?m)^(#+)\s+(.+?)$`
	headerRegex := regexp.MustCompile(headerPattern)
	headerMatches := headerRegex.FindAllStringSubmatchIndex(reasoning, -1)
	
	if len(headerMatches) > 0 {
		// Found markdown headers
		for i, match := range headerMatches {
			// Extract header text
			headerStart := match[4]
			headerEnd := match[5]
			header := reasoning[headerStart:headerEnd]
			
			// Extract content
			contentStart := match[1] // End of the header line
			contentEnd := len(reasoning)
			if i < len(headerMatches)-1 {
				contentEnd = headerMatches[i+1][0] // Start of next header
			}
			
			content := reasoning[contentStart:contentEnd]
			
			// Clean up
			header = strings.TrimSpace(header)
			content = strings.TrimSpace(content)
			
			// Skip if this seems to be the start of an answer section
			if strings.Contains(strings.ToLower(header), "answer") ||
			   strings.Contains(strings.ToLower(header), "conclusion") {
				continue
			}
			
			// Add step
			steps = append(steps, models.ReasoningStep{
				Title:   header,
				Content: content,
			})
		}
		
		return steps
	}
	
	// If no structure found, split by paragraphs and group into logical sections
	paragraphs := strings.Split(reasoning, "\n\n")
	if len(paragraphs) >= 3 {
		// Create logical groupings based on paragraph count
		const maxSteps = 4
		stepSize := len(paragraphs) / maxSteps
		if stepSize < 1 {
			stepSize = 1
		}
		
		for i := 0; i < len(paragraphs); i += stepSize {
			end := i + stepSize
			if end > len(paragraphs) {
				end = len(paragraphs)
			}
			
			// Create a step from this group of paragraphs
			content := strings.Join(paragraphs[i:end], "\n\n")
			title := generateStepTitle(content, i/stepSize+1)
			
			steps = append(steps, models.ReasoningStep{
				Title:   title,
				Content: content,
			})
			
			if len(steps) >= maxSteps {
				break
			}
		}
		
		return steps
	}
	
	// Fallback: just create a single step with all reasoning
	steps = append(steps, models.ReasoningStep{
		Title:   "Analysis of search results",
		Content: reasoning,
	})
	
	return steps
}

// generateStepTitle creates a title for a step based on its content
func generateStepTitle(content string, stepNumber int) string {
	// Try to find a representative sentence in the first paragraph
	firstParagraph := strings.Split(content, "\n")[0]
	sentences := strings.Split(firstParagraph, ". ")
	
	if len(sentences) > 0 {
		firstSentence := sentences[0]
		
		// Trim to a reasonable length
		if len(firstSentence) > 50 {
			firstSentence = firstSentence[:47] + "..."
		}
		
		// Add step number
		return fmt.Sprintf("Step %d: %s", stepNumber, firstSentence)
	}
	
	// Fallback to generic title
	return fmt.Sprintf("Step %d: Analysis", stepNumber)
}

// extractCitations extracts citation information from the answer text
func (s *AIService) extractCitations(answer string, results []models.SearchResult) []models.Citation {
	if answer == "" || len(results) == 0 {
		return nil
	}
	
	// Pattern to match citations like [1], [2], etc.
	citationPattern := `\[([0-9]+)\]`
	citationRegex := regexp.MustCompile(citationPattern)
	
	// Find all citation matches
	matches := citationRegex.FindAllStringSubmatch(answer, -1)
	
	// Track processed citations to avoid duplicates
	processedCitations := make(map[int]bool)
	var citations []models.Citation
	
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		
		// Parse citation index
		var index int
		_, err := fmt.Sscanf(match[1], "%d", &index)
		if err != nil {
			continue
		}
		
		// Skip if already processed
		if processedCitations[index] {
			continue
		}
		
		// Mark as processed
		processedCitations[index] = true
		
		// Skip if index is out of range
		if index < 1 || index > len(results) {
			continue
		}
		
		// Get the corresponding result
		result := results[index-1]
		
		// Extract context around citation
		context := extractCitationContext(answer, index)
		
		// Create citation object
		citation := models.Citation{
			Index:     index,
			Text:      context,
			URL:       result.URL,
			Title:     result.Title,
			Type:      result.Type,
			Subreddit: result.Subreddit,
		}
		
		citations = append(citations, citation)
	}
	
	return citations
}

// extractCitationContext extracts the text context around a citation
func extractCitationContext(text string, index int) string {
	// Create a pattern to find the specific citation
	pattern := fmt.Sprintf(`\[%d\]`, index)
	
	// Find the citation location
	re := regexp.MustCompile(pattern)
	match := re.FindStringIndex(text)
	if match == nil {
		return ""
	}
	
	// Find the surrounding sentence
	startPos := match[0]
	endPos := match[1]
	
	// Look backward for sentence start (period followed by space, or start of text)
	sentenceStart := 0
	for i := startPos - 1; i >= 0; i-- {
		if i > 0 && text[i-1] == '.' && text[i] == ' ' {
			sentenceStart = i + 1
			break
		}
		if text[i] == '\n' && (i == 0 || text[i-1] == '\n') {
			sentenceStart = i + 1
			break
		}
	}
	
	// Look forward for sentence end (period, exclamation, question mark)
	sentenceEnd := len(text)
	for i := endPos; i < len(text); i++ {
		if i < len(text)-1 && (text[i] == '.' || text[i] == '!' || text[i] == '?') && text[i+1] == ' ' {
			sentenceEnd = i + 1
			break
		}
		if i < len(text)-1 && text[i] == '\n' && text[i+1] == '\n' {
			sentenceEnd = i
			break
		}
	}
	
	// Extract sentence
	sentence := text[sentenceStart:sentenceEnd]
	
	// Clean up
	sentence = strings.TrimSpace(sentence)
	
	// Truncate if too long
	if len(sentence) > 200 {
		// Try to find a clean breakpoint
		breakPoint := 197
		for i := 197; i >= 170; i-- {
			if sentence[i] == ' ' || sentence[i] == ',' {
				breakPoint = i
				break
			}
		}
		sentence = sentence[:breakPoint] + "..."
	}
	
	return sentence
}