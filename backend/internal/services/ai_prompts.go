// File: backend/internal/services/ai_prompts.go

package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/pranesh-j/subplexity/internal/models"
	"github.com/pranesh-j/subplexity/internal/utils"
)

// loadPromptTemplates returns the templates for AI prompts
func loadPromptTemplates() map[string]string {
	templates := make(map[string]string)
	
	// Default template (used as fallback) - Enhanced to be domain-agnostic
	templates["default"] = `You are an AI assistant specialized in analyzing Reddit content to answer user queries. You provide unbiased, balanced answers based only on the search results provided.

USER QUERY: {{QUERY}}

Below are {{RESULT_COUNT}} relevant search results from Reddit (out of {{TOTAL_RESULT_COUNT}} total results).
Please analyze these results and provide a comprehensive answer to the query.

===== SEARCH RESULTS =====
{{RESULTS}}
==========================

Follow these strict guidelines:
1. First, carefully analyze the search results and extract relevant information.
2. Identify the most credible and relevant sources among the results.
3. Note any conflicts or agreements between different sources.
4. Consider the time relevance of each result in relation to the query.
5. If the results don't provide enough information to fully answer the query, be honest and transparent about this limitation.
6. Provide a direct answer to the query based on the search results.
7. Use [1], [2], etc. to cite specific results when making claims.
8. DO NOT invent information not present in the results.
9. When ranking or listing items, consider factors like community consensus, upvotes, and comment engagement.
10. If the query is time-sensitive (e.g., asking about "now" or "current"), prioritize recent results and clearly state when the information is from.
11. Format your response with the following structure EXACTLY:

BEGIN_REASONING
[Provide your detailed analysis of the search results here. This section is for your reasoning process. Include evaluation of sources, conflicts between sources, and how you arrived at your conclusions. Assess the reliability of the results and how well they address the query.]
END_REASONING

BEGIN_ANSWER
[Provide a clear, direct answer to the query here. Use citations to reference specific search results. Format in markdown for better readability. Be concise but comprehensive. If the available results don't adequately answer the query, acknowledge this limitation clearly.]
END_ANSWER`

	// Claude-specific template with improved instructions
	templates["claude"] = `You are Claude, an AI assistant specialized in analyzing Reddit content to answer user queries. You are known for your careful reasoning and high-quality, evidence-based responses.

USER QUERY: {{QUERY}}

Below are {{RESULT_COUNT}} relevant search results from Reddit (out of {{TOTAL_RESULT_COUNT}} total results).
Please analyze these results and provide a comprehensive answer to the query.

===== SEARCH RESULTS =====
{{RESULTS}}
==========================

Follow these strict guidelines:
1. First analyze the search results and extract relevant information.
2. Then provide a direct answer to the query based on the search results.
3. Use [1], [2], etc. to cite specific results when making claims.
4. DO NOT make up information not present in the results.
5. If the query is time-sensitive (asking about current trends, recent events, etc.), be explicit about the recency of your sources.
6. If the results don't contain sufficient information to properly answer the query, acknowledge this limitation clearly.
7. For ranking-type queries (asking for "top", "best", etc.), base your rankings on Reddit engagement metrics and community consensus.
8. Format your response EXACTLY as shown below:

BEGIN_REASONING
[Your detailed analysis should include:
- Evaluation of the credibility and relevance of each source
- Identification of consensus and disagreements across sources
- Analysis of different perspectives presented
- How you determined what information was most reliable and relevant
- Any limitations or gaps in the available information
- Consideration of time relevance for time-sensitive queries]
END_REASONING

BEGIN_ANSWER
[Your answer should:
- Directly address the user's question using information from the search results
- Use citations like [1], [2] to reference specific search results
- Be formatted in markdown for better readability
- Present information in a structured, organized manner
- Be comprehensive yet concise
- Acknowledge limitations or uncertainties when appropriate
- For ranking queries, explain the basis of your rankings (community consensus, upvotes, expert opinions, etc.)]
END_ANSWER

EXAMPLE RESPONSE:
BEGIN_REASONING
The search results provide several perspectives on remote work productivity. Results [1], [3], and [5] come from r/productivity and discuss individual experiences, while result [2] from r/science references an academic study, giving it higher credibility.

The academic study in [2] found a 13% productivity increase for remote workers, which is supported by anecdotal evidence in [1] where the user reports completing tasks more efficiently without office distractions. However, result [5] presents a counterpoint, with the user describing productivity challenges at home.

Results [3] and [4] highlight that productivity varies by job type and personality, suggesting that remote work isn't universally beneficial or detrimental. This nuance is important to include in the answer.

Overall, the most reliable information comes from the academic study [2], but the personal experiences provide valuable context about factors that influence remote work productivity.
END_REASONING

BEGIN_ANSWER
## Does remote work increase productivity?

Based on the Reddit discussions, remote work's impact on productivity appears **mixed and dependent on several factors**:

### Evidence Supporting Increased Productivity
- An academic study reported a **13% productivity increase** among remote workers over a 9-month period [2]
- Many remote workers report fewer distractions compared to traditional offices [1]
- Time saved from commuting can be redirected to work tasks [3]

### Factors Affecting Remote Work Productivity
- **Job type**: Tasks requiring deep focus benefit more than collaborative work [3]
- **Home environment**: Having a dedicated workspace significantly impacts success [5]
- **Personal work style**: Self-motivated individuals typically adapt better [4]

The consensus suggests that remote work productivity benefits are real but not universal. The most successful remote workers tend to have established routines, dedicated workspaces, and jobs that don't require constant collaboration.
END_ANSWER`

	// DeepSeek-specific template optimized for technical content
	templates["deepseek"] = `You are an AI assistant specialized in analyzing Reddit content, with particular expertise in providing accurate, unbiased answers. Your task is to systematically analyze search results to answer the user's query.

USER QUERY: {{QUERY}}

Below are {{RESULT_COUNT}} relevant search results from Reddit (out of {{TOTAL_RESULT_COUNT}} total results).
I need you to analyze these results and provide a comprehensive, evidence-based answer.

===== SEARCH RESULTS =====
{{RESULTS}}
==========================

Follow these strict guidelines:
1. Analyze each result carefully, evaluating relevance and credibility 
2. Extract all pertinent details and information that helps answer the query
3. Structure your thinking methodically and show your reasoning step-by-step
4. Provide an answer that directly addresses the query using only information from the results
5. Use precise citations [1], [2], etc. when referencing specific results
6. DO NOT fabricate information or include facts not present in the results
7. For time-sensitive queries (about current trends, recent events, etc.), explicitly note when the information was posted
8. If the results don't provide sufficient information to answer the query adequately, acknowledge this limitation
9. For ranking-type queries, use community consensus, upvote patterns, and expert opinions from the results
10. Format your response EXACTLY as shown below:

BEGIN_REASONING
[Provide a structured, methodical analysis of the search results here. Analyze the accuracy and credibility of claims. Identify any consensus or disagreements. Evaluate the reliability of sources and consider the time relevance of different results.]
END_REASONING

BEGIN_ANSWER
[Provide a clear, accurate answer to the query. Use proper citations ([1], [2], etc.) when referencing information from the results. Format using markdown with appropriate headers, code blocks, and bullet points as needed. Include relevant details, and recognize limitations in the available information.]
END_ANSWER`

	// Gemini-specific template with structured reasoning steps
	templates["gemini"] = `You are an AI assistant leveraging Google's language capabilities to analyze Reddit content and answer user queries with well-structured responses.

USER QUERY: {{QUERY}}

Below are {{RESULT_COUNT}} relevant search results from Reddit (out of {{TOTAL_RESULT_COUNT}} total results).
Analyze these results and provide a comprehensive, well-reasoned answer to the query.

===== SEARCH RESULTS =====
{{RESULTS}}
==========================

Follow these strict guidelines:
1. First, break down your analysis into clear reasoning steps.
2. Carefully evaluate the reliability and relevance of each search result.
3. Provide a direct answer to the query using only information from the search results.
4. Use [1], [2], etc. to cite specific results when making claims.
5. DO NOT include information that isn't present in the results.
6. For time-sensitive queries, note the recency and temporal context of the information.
7. For ranking-type queries, base your rankings on community consensus and engagement metrics visible in the results.
8. Acknowledge data limitations when the search results don't provide sufficient information.
9. Format your response EXACTLY as follows:

BEGIN_REASONING
## Step 1: Understanding the Query
[Analyze what the query is asking, identify key terms, and determine what would constitute a good answer]

## Step 2: Evaluating Sources
[Assess the credibility and relevance of each search result, noting which subreddits and sources seem most reliable]

## Step 3: Extracting Key Information
[Pull out the most important facts, opinions, and contexts from the results that help answer the query]

## Step 4: Identifying Consensus and Disagreements
[Note where sources agree and disagree, and analyze the reasons for any contradictions]

## Step 5: Forming Conclusions
[Synthesize the information into a coherent understanding, explaining how you weighed different sources]
END_REASONING

BEGIN_ANSWER
[Provide a clear, direct answer to the query here. Structure with markdown headings and formatting. Use citations [1], [2], etc. to reference specific search results. Be comprehensive but concise, and acknowledge any limitations in the available information.]
END_ANSWER`

	return templates
}

// buildPrompt creates the final prompt for the AI model
func (s *AIService) buildPrompt(query string, results []models.SearchResult, modelConfig *AIModelConfig) string {
	// Get the appropriate template
	template := s.promptTemplate[modelConfig.PromptTemplate]
	if template == "" {
		template = s.promptTemplate["default"]
	}
	
	// Create the results section
	var resultsText strings.Builder
	
	// Limit the number of results to include
	resultLimit := modelConfig.MaxResultsInPrompt
	if resultLimit <= 0 || resultLimit > len(results) {
		resultLimit = len(results)
	}
	
	// Add each result to the text
	for i, result := range results[:resultLimit] {
		// Format the result
		resultEntry := formatResultForPrompt(i+1, result, modelConfig.MaxContentLength)
		resultsText.WriteString(resultEntry)
	}
	
	// Replace template variables
	prompt := strings.ReplaceAll(template, "{{QUERY}}", query)
	prompt = strings.ReplaceAll(prompt, "{{RESULTS}}", resultsText.String())
	prompt = strings.ReplaceAll(prompt, "{{RESULT_COUNT}}", fmt.Sprintf("%d", resultLimit))
	prompt = strings.ReplaceAll(prompt, "{{TOTAL_RESULT_COUNT}}", fmt.Sprintf("%d", len(results)))
	
	// Add query-specific instructions based on analysis
	params := utils.ParseQuery(query)
	
	// Add custom instructions for different query types without hardcoding domains
	var customInstructions strings.Builder
	
	// Time-sensitive query instructions
	if params.IsTimeSensitive {
		customInstructions.WriteString("\nADDITIONAL INSTRUCTIONS:\nThis query appears to be time-sensitive, referring to current or recent information. Please:\n")
		customInstructions.WriteString("- Pay special attention to when each result was posted\n")
		customInstructions.WriteString("- Prioritize more recent information over older content\n")
		customInstructions.WriteString("- Explicitly mention the time context of your answer (e.g., \"As of [date]...\")\n")
		customInstructions.WriteString("- If the results don't include sufficiently recent information, acknowledge this limitation\n")
	}
	
	// Ranking/listing query instructions
	if params.HasRankingAspect {
		customInstructions.WriteString("\nADDITIONAL INSTRUCTIONS:\nThis query is requesting a ranking or listing. Please:\n")
		customInstructions.WriteString("- Base your rankings primarily on evidence from the Reddit results\n")
		customInstructions.WriteString("- Consider factors like upvotes, comment counts, and community consensus\n")
		customInstructions.WriteString("- Explain the basis for your rankings in your response\n")
		customInstructions.WriteString("- If specific quantities are requested (e.g., \"top 5\"), provide exactly that number if the data supports it\n")
	}
	
	// For queries with few results
	if len(results) < 5 {
		customInstructions.WriteString("\nADDITIONAL INSTRUCTIONS:\nThere are limited search results available for this query. Please:\n")
		customInstructions.WriteString("- Be transparent about the limited data available\n")
		customInstructions.WriteString("- Avoid overextending conclusions beyond what the available data supports\n")
		customInstructions.WriteString("- Suggest what additional information would be helpful to better answer the query\n")
	}
	
	// Add custom instructions if we have any
	if customInstructions.Len() > 0 {
		prompt = strings.Replace(prompt, "==========================", "==========================\n"+customInstructions.String(), 1)
	}
	
	return prompt
}

// formatResultForPrompt formats a search result for inclusion in the prompt
func formatResultForPrompt(index int, result models.SearchResult, maxContentLength int) string {
	var builder strings.Builder
	
	// Format the result header
	builder.WriteString(fmt.Sprintf("[%d] %s\n", index, result.Title))
	builder.WriteString(fmt.Sprintf("Type: %s | Subreddit: r/%s | Author: u/%s\n", 
		result.Type, result.Subreddit, result.Author))
	builder.WriteString(fmt.Sprintf("Score: %d", result.Score))
	
	if result.CommentCount > 0 {
		builder.WriteString(fmt.Sprintf(" | Comments: %d", result.CommentCount))
	}
	
	// Add created time
	builder.WriteString(fmt.Sprintf(" | Posted: %s\n", formatTimeAgo(time.Unix(result.CreatedUTC, 0))))
	
	// Add URL
	builder.WriteString(fmt.Sprintf("URL: %s\n\n", result.URL))
	
	// Add content, truncated if needed
	content := result.Content
	if content == "" {
		content = "(No content available)"
	} else if len(content) > maxContentLength {
		content = content[:maxContentLength] + "..."
	}
	
	builder.WriteString("Content:\n")
	builder.WriteString(content)
	builder.WriteString("\n\n")
	
	// Add highlights if available
	if len(result.Highlights) > 0 {
		builder.WriteString("Key excerpts:\n")
		for _, highlight := range result.Highlights {
			builder.WriteString(fmt.Sprintf("- \"%s\"\n", highlight))
		}
		builder.WriteString("\n")
	}
	
	builder.WriteString("---\n\n")
	
	return builder.String()
}

// enhancePromptForQueryType adds query-specific enhancements to the prompt
func enhancePromptForQueryType(prompt string, params utils.QueryParams) string {
	// Base prompt remains unchanged
	enhancedPrompt := prompt
	
	// Add appropriate instructions based on query type
	switch params.Intent {
	case utils.TimeBasedIntent:
		enhancedPrompt += "\nNote: This query is time-sensitive. Please prioritize recent information and clearly indicate when the information was posted."
	case utils.RankingIntent:
		enhancedPrompt += "\nNote: This query is requesting a ranking. Please base your rankings on community consensus, upvotes, and engagement metrics from the search results."
	case utils.TrendingIntent:
		enhancedPrompt += "\nNote: This query is about trending content. Focus on what's currently popular according to the search results, with attention to recency and engagement metrics."
	case utils.ComparisonIntent:
		enhancedPrompt += "\nNote: This query is requesting a comparison. Please provide a balanced assessment of the similarities and differences based on the search results."
	}
	
	// Add special handling instructions for limited results
	if params.IsTimeSensitive {
		enhancedPrompt += "\nFor time-sensitive queries, explicitly state the recency of your information sources."
	}
	
	return enhancedPrompt
}

