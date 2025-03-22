// File: backend/internal/services/reddit_relevance.go

package services

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/pranesh-j/subplexity/internal/models"
	"github.com/pranesh-j/subplexity/internal/utils"
)

// Define scoredResult type for use in relevance ranking
type scoredResult struct {
	result models.SearchResult
	score  float64
}

// extractHighlights extracts key excerpts from content that match keywords
func extractHighlights(content string, keywords []string) []string {
	if content == "" || len(keywords) == 0 {
		return nil
	}
	
	// Split content into sentences
	sentences := splitIntoSentences(content)
	if len(sentences) == 0 {
		return nil
	}
	
	// Score each sentence based on keyword matches
	type scoredSentence struct {
		text  string
		score int
	}
	
	var scoredSentences []scoredSentence
	
	for _, sentence := range sentences {
		if len(strings.TrimSpace(sentence)) < 10 {
			continue // Skip very short sentences
		}
		
		score := 0
		lowerSentence := strings.ToLower(sentence)
		
		for _, keyword := range keywords {
			if len(keyword) > 2 && strings.Contains(lowerSentence, strings.ToLower(keyword)) {
				score += 1
				
				// Bonus for containing multiple keywords
				if strings.Count(lowerSentence, strings.ToLower(keyword)) > 1 {
					score += 1
				}
			}
		}
		
		// Only consider sentences with at least one keyword match
		if score > 0 {
			scoredSentences = append(scoredSentences, scoredSentence{
				text:  sentence,
				score: score,
			})
		}
	}
	
	// Sort sentences by score (highest first)
	sort.Slice(scoredSentences, func(i, j int) bool {
		return scoredSentences[i].score > scoredSentences[j].score
	})
	
	// Take up to 3 highest scoring sentences as highlights
	maxHighlights := 3
	if len(scoredSentences) < maxHighlights {
		maxHighlights = len(scoredSentences)
	}
	
	highlights := make([]string, 0, maxHighlights)
	
	for i := 0; i < maxHighlights; i++ {
		highlight := strings.TrimSpace(scoredSentences[i].text)
		
		// Truncate if too long
		const maxHighlightLength = 200
		if len(highlight) > maxHighlightLength {
			// Find a good breakpoint
			breakPoint := maxHighlightLength
			for j := maxHighlightLength - 1; j >= maxHighlightLength-30; j-- {
				if j < len(highlight) && (highlight[j] == ' ' || highlight[j] == ',' || highlight[j] == '.') {
					breakPoint = j
					break
				}
			}
			highlight = highlight[:breakPoint] + "..."
		}
		
		highlights = append(highlights, highlight)
	}
	
	return highlights
}

// splitIntoSentences breaks text into sentences
func splitIntoSentences(text string) []string {
	// Common abbreviations to handle specially
	abbreviations := []string{
		"Mr.", "Mrs.", "Ms.", "Dr.", "Prof.",
		"Inc.", "Ltd.", "Co.", "Corp.",
		"vs.", "etc.", "i.e.", "e.g.",
		"U.S.", "U.K.", "E.U.",
		"Jan.", "Feb.", "Aug.", "Sept.", "Oct.", "Nov.", "Dec.",
	}
	
	// Replace periods in abbreviations temporarily
	for _, abbr := range abbreviations {
		text = strings.ReplaceAll(text, abbr, strings.ReplaceAll(abbr, ".", "##PD##"))
	}
	
	// Use regex-free approach for better performance
	var sentences []string
	var currentSentence strings.Builder
	
	for i := 0; i < len(text); i++ {
		currentSentence.WriteByte(text[i])
		
		// Check for sentence-ending punctuation
		if text[i] == '.' || text[i] == '!' || text[i] == '?' {
			// Look ahead to see if this is really the end of a sentence
			isEndOfSentence := false
			
			// Check if we're at the end of text
			if i == len(text)-1 {
				isEndOfSentence = true
			} else {
				// Check if followed by space and capital letter
				for j := i + 1; j < len(text); j++ {
					if text[j] == ' ' || text[j] == '\t' || text[j] == '\n' {
						continue
					}
					
					// If next non-whitespace char is capital, it's a new sentence
					if j < len(text) && text[j] >= 'A' && text[j] <= 'Z' {
						isEndOfSentence = true
					}
					
					break
				}
				
				// Also end sentence if followed by multiple newlines
				if i+2 < len(text) && text[i+1] == '\n' && text[i+2] == '\n' {
					isEndOfSentence = true
				}
			}
			
			if isEndOfSentence {
				sentences = append(sentences, currentSentence.String())
				currentSentence.Reset()
			}
		}
	}
	
	// Add any remaining content as the last sentence
	if currentSentence.Len() > 0 {
		sentences = append(sentences, currentSentence.String())
	}
	
	// Restore periods in abbreviations
	for i := range sentences {
		sentences[i] = strings.ReplaceAll(sentences[i], "##PD##", ".")
	}
	
	return sentences
}

// processSearchResults processes and ranks search results
func (s *RedditService) processSearchResults(params utils.QueryParams, results []models.SearchResult, limit int) []models.SearchResult {
	if len(results) == 0 {
		return results
	}
	
	// Score results based on relevance to query
	var scoredResults []scoredResult
	
	// Process each result
	for _, result := range results {
		// Calculate relevance score
		score := calculateRelevanceScore(result, params)
		
		// Add to scored results
		scoredResults = append(scoredResults, scoredResult{
			result: result,
			score:  score,
		})
	}
	
	// Sort by relevance score (highest first)
	sort.Slice(scoredResults, func(i, j int) bool {
		return scoredResults[i].score > scoredResults[j].score
	})
	
	// Apply result diversity to avoid all results being the same type
	diversifiedResults := diversifyResults(scoredResults)
	
	// Limit to requested number of results
	if len(diversifiedResults) > limit {
		diversifiedResults = diversifiedResults[:limit]
	}
	
	// Extract highlights for final results
	finalResults := make([]models.SearchResult, len(diversifiedResults))
	for i, sr := range diversifiedResults {
		// Extract highlights for each result
		sr.result.Highlights = extractHighlights(sr.result.Content, params.Keywords)
		
		// Add to final results
		finalResults[i] = sr.result
	}
	
	return finalResults
}

// calculateRelevanceScore computes a relevance score for a search result
func calculateRelevanceScore(result models.SearchResult, params utils.QueryParams) float64 {
	// Base scores by content type (these can be tuned)
	typeBaseScores := map[string]float64{
		"post":      100.0,
		"comment":   80.0,
		"subreddit": 90.0,
	}
	
	// Start with type-based score
	score := typeBaseScores[result.Type]
	if score == 0 {
		score = 70.0 // Default for unknown types
	}
	
	// Check if this is completely unrelated content
	isRelevant := false
	
	// Extract main keywords from query
	mainKeywords := extractMainKeywords(params.OriginalQuery)
	
	// Check if title contains any main keywords
	titleLower := strings.ToLower(result.Title)
	contentLower := strings.ToLower(result.Content)
	
	for _, keyword := range mainKeywords {
		if strings.Contains(titleLower, keyword) || strings.Contains(contentLower, keyword) {
			isRelevant = true
			score += 50.0
		}
	}
	
	// If no main keywords are found, severely penalize the score
	if !isRelevant {
		score -= 500.0
	}
	
	// Adjust based on query intent and result type match
	switch params.Intent {
	case utils.SubredditIntent:
		if result.Type == "subreddit" {
			score += 200.0
		}
	case utils.PostIntent:
		if result.Type == "post" {
			score += 200.0
		}
	case utils.CommentIntent:
		if result.Type == "comment" {
			score += 200.0
		}
	case utils.UserIntent:
		// Check if result author matches any requested authors
		for _, author := range params.Authors {
			if strings.EqualFold(result.Author, author) {
				score += 200.0
				break
			}
		}
	}
	
	// Keyword matching in title (most important)
	for _, keyword := range params.Keywords {
		if strings.Contains(strings.ToLower(result.Title), strings.ToLower(keyword)) {
			score += 50.0
			
			// Extra points for exact matches
			if strings.Contains(strings.ToLower(result.Title), strings.ToLower(" "+keyword+" ")) {
				score += 25.0
			}
		}
	}
	
	// Keyword matching in content
	contentMatchCount := 0
	for _, keyword := range params.Keywords {
		if strings.Contains(strings.ToLower(result.Content), strings.ToLower(keyword)) {
			contentMatchCount++
			score += 30.0
			
			// Count occurrences for frequency bonus
			keywordCount := countOccurrences(result.Content, keyword)
			if keywordCount > 1 {
				// Logarithmic bonus for multiple occurrences
				score += math.Log2(float64(keywordCount)) * 15.0
			}
		}
	}
	
	// Bonus for matching all keywords (very relevant content)
	if contentMatchCount == len(params.Keywords) && len(params.Keywords) > 0 {
		score += 100.0
	}
	
	// Subreddit matching
	for _, subreddit := range params.Subreddits {
		if strings.EqualFold(result.Subreddit, subreddit) {
			score += 150.0
			break
		}
	}
	
	// Popularity factors - use logarithmic scale to prevent very popular content from dominating
	
	// Votes matter (more upvotes = community validation)
	if result.Score > 0 {
		// Logarithmic scale with diminishing returns
		upvoteScore := math.Log2(float64(result.Score) + 10.0) * 15.0
		score += upvoteScore
	}
	
	// Comments indicate engagement
	if result.CommentCount > 0 {
		commentScore := math.Log2(float64(result.CommentCount) + 10.0) * 10.0
		score += commentScore
	}
	
	// Recency is important (newer = more relevant, with decay)
	ageInDays := (time.Now().Unix() - result.CreatedUTC) / (60 * 60 * 24)
	
	// Different decay rates based on content type and age
	var recencyScore float64
	
	switch {
	case ageInDays < 1: // Last 24 hours - very fresh
		recencyScore = 200.0
	case ageInDays < 7: // Last week - fresh
		recencyScore = 150.0 - (float64(ageInDays) * 10.0)
	case ageInDays < 30: // Last month - relevant
		recencyScore = 100.0 - (float64(ageInDays) * 2.0)
	case ageInDays < 90: // Last 3 months - somewhat relevant
		recencyScore = 40.0 - (float64(ageInDays-30) * 0.3)
	case ageInDays < 365: // Last year - less relevant
		recencyScore = 10.0 - (float64(ageInDays-90) * 0.02)
	default: // Older - historical
		recencyScore = 0.0
	}
	
	// For time-based queries, recency is even more important
	if params.Intent == utils.TimeBasedIntent {
		recencyScore *= 2.0
	}
	
	score += recencyScore
	
	// Content quality factors
	
	// Content length factor - more detailed content may be more valuable
	// But prevent long walls of text from dominating just due to length
	if len(result.Content) > 0 {
		// Logarithmic scale with cap
		contentLengthFactor := math.Min(25.0, math.Log10(float64(len(result.Content)))*5.0)
		score += contentLengthFactor
	}
	
	// Penalize excluded terms if present
	for _, excludeTerm := range params.ExcludeTerms {
		titleAndContent := result.Title + " " + result.Content
		if strings.Contains(strings.ToLower(titleAndContent), strings.ToLower(excludeTerm)) {
			score -= 200.0 // Significant penalty
		}
	}
	
	return score
}

// Extract main identifying keywords from the query
func extractMainKeywords(query string) []string {
	query = strings.ToLower(query)
	words := strings.Fields(query)
	var keywords []string
	
	// Filter out common stop words and keep meaningful terms
	for _, word := range words {
		// Only keep words with 3+ characters as significant keywords
		if len(word) > 2 {
			// Skip common words that don't add much meaning
			if !isStopWord(word) {
				keywords = append(keywords, word)
			}
		}
	}
	
	return keywords
}

// Helper to identify common stop words
func isStopWord(word string) bool {
	stopWords := map[string]bool{
		"the": true, "and": true, "for": true, "are": true, "but": true,
		"not": true, "you": true, "all": true, "any": true, "can": true,
		"had": true, "has": true, "may": true, "was": true, "who": true,
		"why": true, "will": true, "with": true, "from": true,
	}
	return stopWords[word]
}

// countOccurrences counts how many times a keyword appears in text
func countOccurrences(text, keyword string) int {
	if keyword == "" {
		return 0
	}
	
	text = strings.ToLower(text)
	keyword = strings.ToLower(keyword)
	
	return strings.Count(text, keyword)
}

// diversifyResults ensures variety in the top results
func diversifyResults(scoredResults []scoredResult) []scoredResult {
	if len(scoredResults) <= 5 {
		return scoredResults
	}
	
	// Filter out results with negative scores (irrelevant)
	var filteredResults []scoredResult
	for _, result := range scoredResults {
		if result.score > 0 {
			filteredResults = append(filteredResults, result)
		}
	}
	
	// If we filtered everything, use original list
	if len(filteredResults) == 0 {
		filteredResults = scoredResults
	}
	
	// Initialize with top result
	var diversified []scoredResult
	diversified = append(diversified, filteredResults[0])
	
	// Track types we've added and how many of each
	typeCounts := map[string]int{
		filteredResults[0].result.Type: 1,
	}
	
	// Add one of each type first (if available and in top half)
	for _, typ := range []string{"post", "comment", "subreddit"} {
		if typeCounts[typ] > 0 {
			continue // Already have this type
		}
		
		// Find highest scoring result of this type within top half
		topHalfCutoff := len(filteredResults) / 2
		for i, sr := range filteredResults[1:topHalfCutoff] {
			if sr.result.Type == typ {
				diversified = append(diversified, sr)
				typeCounts[typ]++
				
				// Remove it from original slice to avoid duplicates
				copy(filteredResults[i+1:], filteredResults[i+2:])
				filteredResults = filteredResults[:len(filteredResults)-1]
				
				break
			}
		}
	}
	
	// Ensure we don't oversample any type (max 60% of results)
	maxPerType := int(math.Ceil(float64(len(filteredResults)) * 0.6))
	
	// Fill remaining slots with highest scores, but with type diversity
	for _, sr := range filteredResults {
		// Skip results we've already added
		alreadyAdded := false
		for _, added := range diversified {
			if added.result.ID == sr.result.ID {
				alreadyAdded = true
				break
			}
		}
		
		if alreadyAdded {
			continue
		}
		
		// Check if we're at capacity for this type
		if typeCounts[sr.result.Type] >= maxPerType {
			continue
		}
		
		// Add this result
		diversified = append(diversified, sr)
		typeCounts[sr.result.Type]++
	}
	
	return diversified
}