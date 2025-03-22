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

// Completely revamp the calculateRelevanceScore function to be domain-agnostic
func calculateRelevanceScore(result models.SearchResult, params utils.QueryParams) float64 {
    // Base score starts at 100
    score := 100.0
    
    // 1. Content type relevance - weight based on intent, not hardcoded domain
    switch result.Type {
    case "post":
        if params.Intent == utils.PostIntent {
            score += 100
        }
    case "comment":
        if params.Intent == utils.CommentIntent {
            score += 100
        }
    case "subreddit":
        if params.Intent == utils.SubredditIntent {
            score += 100
        }
    }
    
    // 2. Keyword matching - completely query-dependent
    titleMatches := 0
    contentMatches := 0
    
    // Count matches in title
    titleLower := strings.ToLower(result.Title)
    for _, keyword := range params.FilteredKeywords {
        if strings.Contains(titleLower, strings.ToLower(keyword)) {
            titleMatches++
            score += 30 // High value for title matches
        }
    }
    
    // Count matches in content
    contentLower := strings.ToLower(result.Content)
    for _, keyword := range params.FilteredKeywords {
        if strings.Contains(contentLower, strings.ToLower(keyword)) {
            contentMatches++
            score += 20 // Medium value for content matches
        }
    }
    
    // Bonus for matching most/all keywords
    if len(params.FilteredKeywords) > 0 {
        keywordCoverage := float64(titleMatches+contentMatches) / float64(len(params.FilteredKeywords))
        score += keywordCoverage * 100 // Up to 100 points for complete coverage
    }
    
    // 3. Temporal relevance - based on query time sensitivity
    if params.IsTimeSensitive {
        // Calculate age of the content
        ageInSeconds := time.Now().Unix() - result.CreatedUTC
        ageInDays := ageInSeconds / (60 * 60 * 24)
        
        // Apply temporal scoring based on timeframe
        switch params.TimeFrame {
        case "day":
            if ageInDays < 1 {
                score += 300 // Very recent
            } else if ageInDays < 3 {
                score += 150 // Recent
            } else if ageInDays < 7 {
                score += 50 // Somewhat recent
            }
        case "week":
            if ageInDays < 7 {
                score += 200 // Within a week
            } else if ageInDays < 14 {
                score += 100 // Within two weeks
            } else if ageInDays < 30 {
                score += 50 // Within a month
            }
        case "month":
            if ageInDays < 30 {
                score += 150 // Within a month
            } else if ageInDays < 60 {
                score += 75 // Within two months
            }
        case "year":
            if ageInDays < 365 {
                score += 100 // Within a year
            }
        }
    }
    
    // 4. Engagement metrics - universal signals of content quality
    // Use logarithmic scaling to prevent very popular content from dominating
    if result.Score > 0 {
        score += math.Log10(float64(result.Score)+10) * 20
    }
    
    if result.CommentCount > 0 {
        score += math.Log10(float64(result.CommentCount)+10) * 15
    }
    
    // 5. Apply custom relevance factors from query analysis
    for factor, weight := range params.RelevanceFactors {
        switch factor {
        case "recency":
            // Already handled above, but could apply multiplier here
            ageScore := calculateAgeScore(result.CreatedUTC)
            score += ageScore * weight
        case "engagement":
            engagementScore := calculateEngagementScore(result.Score, result.CommentCount)
            score += engagementScore * weight
        }
    }
    
    return score
}

// Helper functions
func calculateAgeScore(createdUTC int64) float64 {
    ageInSeconds := time.Now().Unix() - createdUTC
    ageInDays := ageInSeconds / (60 * 60 * 24)
    
    // Inverse logarithmic decay - newer content scores higher
    if ageInDays == 0 {
        return 100 // Today
    }
    return 100 / (1 + math.Log10(float64(ageInDays)))
}

func calculateEngagementScore(score int, commentCount int) float64 {
    // Combined engagement metric
    return (math.Log10(float64(score)+10) * 2) + (math.Log10(float64(commentCount)+10) * 3)
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