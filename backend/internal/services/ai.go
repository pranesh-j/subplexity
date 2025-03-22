// backend/internal/services/ai.go
// Note: This is a partial file update - only the modified functions are shown.
// Keep all other functions unchanged

// Replace the existing buildStructuredPrompt function with this:
func buildStructuredPrompt(query string, results []models.SearchResult) string {
	var content strings.Builder
	
	// Start with clear instructions
	content.WriteString("# Reddit Search Analysis Task\n\n")
	content.WriteString(fmt.Sprintf("User Query: %s\n\n", query))
	
	content.WriteString("## Instructions:\n")
	content.WriteString("1. You will solve this step-by-step using multiple reasoning steps, clearly labeled.\n")
	content.WriteString("2. First, think about what information you need to answer this query effectively.\n")
	content.WriteString("3. For each result, evaluate its relevance, credibility, and key information.\n") 
	content.WriteString("4. Identify conflicts, agreements, and patterns across different sources.\n")
	content.WriteString("5. Draw conclusions based on the most reliable information.\n")
	content.WriteString("6. Format your response with the following structure:\n")
	content.WriteString("   - '## Step 1: Understanding the query'\n")
	content.WriteString("   - '## Step 2: Analyzing key sources'\n") 
	content.WriteString("   - '## Step 3: Evaluating conflicting information' (if applicable)\n")
	content.WriteString("   - '## Step 4: Synthesizing findings'\n")
	content.WriteString("   - '## Answer: [Clear, direct answer to the user's query]'\n")
	content.WriteString("7. Each step should show your detailed thinking process.\n")
	content.WriteString("8. Use [1], [2], etc. to cite specific results.\n\n")
	
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
	content.WriteString("### Step 1: Understanding the query\n")
	content.WriteString("Start by analyzing what the user is really asking for and what kind of information would satisfy their query.\n\n")
	
	content.WriteString("### Step 2: Analyzing key sources\n")
	content.WriteString("Evaluate the most relevant sources and extract key information that helps answer the query.\n\n")
	
	content.WriteString("### Step 3: Evaluating conflicting information\n")
	content.WriteString("Identify and resolve any contradictions or conflicts between different sources.\n\n")
	
	content.WriteString("### Step 4: Synthesizing findings\n")
	content.WriteString("Bring together all the key insights to form a coherent understanding.\n\n")
	
	content.WriteString("### Answer\n")
	content.WriteString("Finally, provide a direct, comprehensive answer to the user's query with citations.\n\n")
	
	return content.String()
}

// Replace the existing extractReasoningAndAnswer function with this:
func extractReasoningAndAnswer(fullText string) (string, string) {
	// Extract steps of reasoning and the final answer
	answerRegex := regexp.MustCompile(`(?i)(?:#+\s*Answer:?|#+\s*Conclusion:?)\s*`)
	answerMatch := answerRegex.FindStringIndex(fullText)
	
	if answerMatch != nil {
		// Everything before the answer is reasoning
		reasoning := strings.TrimSpace(fullText[:answerMatch[0]])
		// Everything after the answer marker is the answer
		answer := strings.TrimSpace(fullText[answerMatch[1]:])
		
		return reasoning, answer
	}
	
	// If we can't find an explicit answer section, try to find the last step
	stepRegex := regexp.MustCompile(`(?i)(?:#+\s*Step\s+\d+:?)\s*`)
	stepMatches := stepRegex.FindAllStringIndex(fullText, -1)
	
	if len(stepMatches) > 0 {
		// The last step might contain the answer
		lastStepIndex := stepMatches[len(stepMatches)-1][0]
		
		// Look for a paragraph break after the last step
		paragraphBreak := -1
		for i := lastStepIndex + 100; i < len(fullText)-1; i++ {
			if fullText[i] == '\n' && fullText[i+1] == '\n' {
				paragraphBreak = i
				break
			}
		}
		
		if paragraphBreak != -1 {
			reasoning := strings.TrimSpace(fullText[:paragraphBreak])
			answer := strings.TrimSpace(fullText[paragraphBreak+2:])
			return reasoning, answer
		} else {
			// If no clear paragraph break, use the last 1/4 of text as the answer
			cutPoint := len(fullText) - (len(fullText) / 4)
			reasoning := strings.TrimSpace(fullText[:cutPoint])
			answer := strings.TrimSpace(fullText[cutPoint:])
			return reasoning, answer
		}
	}
	
	// Last resort: split into first 3/4 (reasoning) and last 1/4 (answer)
	cutPoint := (len(fullText) * 3) / 4
	reasoning := strings.TrimSpace(fullText[:cutPoint])
	answer := strings.TrimSpace(fullText[cutPoint:])
	
	return reasoning, answer
}