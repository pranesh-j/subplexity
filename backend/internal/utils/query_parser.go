// File: internal/utils/query_parser.go

package utils

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// StopWords is a set of common words to filter out
var StopWords = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "or": true, "but": true,
	"is": true, "are": true, "was": true, "were": true, "be": true, "being": true, 
	"in": true, "on": true, "at": true, "to": true, "for": true, "with": true,
	"about": true, "what": true, "when": true, "where": true, "who": true, "why": true,
	"how": true, "of": true, "from": true, "by": true, "as": true, "this": true,
	"that": true, "these": true, "those": true, "which": true, "whose": true,
	"than": true, "then": true, "if": true, "else": true, "so": true, "just": true,
	"get": true, "can": true, "will": true, "should": true, "would": true, "could": true,
	"i": true, "me": true, "my": true, "mine": true, "you": true, "your": true,
	"it": true, "its": true, "their": true, "they": true, "them": true, "he": true,
	"she": true, "his": true, "her": true, "am": true, "has": true, "have": true,
	"had": true, "do": true, "does": true, "did": true, "done": true, "not": true,
}

// QueryIntent represents the detected intent of a search query
type QueryIntent int

const (
	GeneralIntent QueryIntent = iota
	SubredditIntent
	PostIntent
	CommentIntent
	UserIntent
	TimeBasedIntent
	TrendingIntent
	RankingIntent    // New intent for ranking/listing queries
	ComparisonIntent // New intent for comparing things
)

// Enhance QueryParams structure to be more flexible
type QueryParams struct {
	Intent           QueryIntent
	Keywords         []string
	Subreddits       []string
	Authors          []string
	ExcludeTerms     []string
	TimeFrame        string
	SortBy           string
	FilteredKeywords []string
	OriginalQuery    string
	// New fields
	IsTimeSensitive   bool                 // Indicates if query has temporal aspects
	RelevanceFactors  map[string]float64   // Dynamic relevance weights
	HasRankingAspect  bool                 // Queries about "top", "best", etc.
	QueryCategories   []string             // Detected general categories (e.g., "entertainment", "technology")
	QuantityRequested int                  // If query requests a specific number of results (e.g., "top 5")
}

// Enhanced ParseQuery function to better handle ranking and time-sensitive queries
func ParseQuery(query string) QueryParams {
	params := QueryParams{
		Intent:           GeneralIntent,
		OriginalQuery:    query,
		TimeFrame:        "all",      // Default timeframe
		SortBy:           "relevance", // Default sort
		RelevanceFactors: make(map[string]float64),
		QuantityRequested: 0,         // Default to no specific quantity
	}
	
	// Convert to lowercase for easier parsing
	queryLower := strings.ToLower(query)
	
	// Extract subreddits mentioned with r/ prefix
	subredditRegex := regexp.MustCompile(`\br/([a-zA-Z0-9_]+)`)
	subredditMatches := subredditRegex.FindAllStringSubmatch(queryLower, -1)
	for _, match := range subredditMatches {
		if len(match) > 1 {
			params.Subreddits = append(params.Subreddits, match[1])
		}
	}
	
	// Extract users mentioned with u/ prefix
	userRegex := regexp.MustCompile(`\bu/([a-zA-Z0-9_-]+)`)
	userMatches := userRegex.FindAllStringSubmatch(queryLower, -1)
	for _, match := range userMatches {
		if len(match) > 1 {
			params.Authors = append(params.Authors, match[1])
		}
	}
	
	// Detect negation terms (exclude)
	excludeRegex := regexp.MustCompile(`-([a-zA-Z0-9_]+)`)
	excludeMatches := excludeRegex.FindAllStringSubmatch(queryLower, -1)
	for _, match := range excludeMatches {
		if len(match) > 1 {
			params.ExcludeTerms = append(params.ExcludeTerms, match[1])
		}
	}
	
	// Enhanced temporal terms detection - more comprehensive
	temporalTerms := []string{
		"now", "current", "currently", "today", "tonight", 
		"this week", "this month", "this year", "recent",
		"latest", "right now", "presently", "at the moment",
		"nowadays", "trending", "present", "2024", "2025",
	}
	
	// Check for time sensitivity with improved precision
	for _, term := range temporalTerms {
		if strings.Contains(queryLower, term) {
			params.IsTimeSensitive = true
			
			// Set appropriate TimeFrame based on term
			if strings.Contains(term, "year") || term == "2024" || term == "2025" {
				params.TimeFrame = "year"
			} else if strings.Contains(term, "month") {
				params.TimeFrame = "month"
			} else if strings.Contains(term, "week") {
				params.TimeFrame = "week"
			} else {
				params.TimeFrame = "day" // Default for "now", "today", etc.
			}
			
			// Add recency as a relevance factor
			params.RelevanceFactors["recency"] = 2.0
			// Update intent
			params.Intent = TimeBasedIntent
			break
		}
	}
	
	// Enhanced ranking term detection
	rankingTerms := []string{
		"top", "best", "greatest", "worst", "highest", "lowest", 
		"most", "least", "ranking", "ranked", "popular", "favorite",
		"essential", "must-see", "must-watch", "must-play", "must-read",
	}
	
	// More sophisticated ranking detection that looks for full terms
	for _, term := range rankingTerms {
		// Look for the term as a whole word, not just a substring
		termPattern := fmt.Sprintf(`\b%s\b`, term)
		if regexp.MustCompile(termPattern).MatchString(queryLower) {
			params.HasRankingAspect = true
			params.RelevanceFactors["engagement"] = 1.5 // Prioritize highly-engaged content
			
			// Check for ranking intent specifically
			if strings.Contains(queryLower, "top") || strings.Contains(queryLower, "best") || 
			   strings.Contains(queryLower, "greatest") || strings.Contains(queryLower, "ranking") {
				params.Intent = RankingIntent
			}
			
			// Set sort order based on ranking term
			if strings.Contains(queryLower, "top") {
				params.SortBy = "top"
			} else if strings.Contains(queryLower, "best") {
				params.SortBy = "relevance" // "best" in Reddit is a comment sort
			}
			break
		}
	}
	
	// Improved quantity detection for ranking queries
	quantityRegex := regexp.MustCompile(`\b(top|best|worst)\s+(\d+)\b`)
	quantityMatch := quantityRegex.FindStringSubmatch(queryLower)
	if len(quantityMatch) > 2 {
		// Parse the number (ignoring errors, defaulting to 0)
		quantity, _ := strconv.Atoi(quantityMatch[2])
		params.QuantityRequested = quantity
	}
	
	// Also look for number words - fixed duplicate keys
	numberWords := map[string]int{
		"five": 5, 
		"ten": 10, 
		"fifteen": 15, 
		"twenty": 20,
		"three": 3, 
		"seven": 7,
	}
	
	for word, num := range numberWords {
		wordPattern := fmt.Sprintf(`\b(top|best)\s+%s\b`, word)
		if regexp.MustCompile(wordPattern).MatchString(queryLower) {
			params.QuantityRequested = num
			break
		}
	}
	
	// Check for trending intent with more signals
	trendingTerms := []string{"trending", "popular", "hot", "viral", "blowing up", "going viral"}
	for _, term := range trendingTerms {
		if strings.Contains(queryLower, term) {
			params.Intent = TrendingIntent
			if term == "hot" {
				params.SortBy = "hot"
			} else if term == "popular" {
				params.SortBy = "top" 
			}
			break
		}
	}
	
	// Enhanced comparison intent detection
	comparisonTerms := []string{
		"vs", "versus", "compared to", "difference between", "better than",
		"or", "alternative to", "similar to", "which is better",
	}
	for _, term := range comparisonTerms {
		if strings.Contains(queryLower, term) {
			params.Intent = ComparisonIntent
			// For comparison queries, both recency and engagement matter
			params.RelevanceFactors["balanced"] = 1.0
			break
		}
	}
	
	// Detect specific content type intent with more signals
	if len(params.Subreddits) > 0 {
		params.Intent = SubredditIntent
	} else if regexp.MustCompile(`\bcomment(s|ing)?\b`).MatchString(queryLower) {
		params.Intent = CommentIntent
	} else if regexp.MustCompile(`\b(post|thread|submission)(s|ed|ing)?\b`).MatchString(queryLower) {
		params.Intent = PostIntent
	} else if len(params.Authors) > 0 {
		params.Intent = UserIntent
	}
	
	// Extract keywords (all remaining terms)
	// First, clean query by removing special syntax
	cleanQuery := queryLower
	cleanQuery = subredditRegex.ReplaceAllString(cleanQuery, " ")
	cleanQuery = userRegex.ReplaceAllString(cleanQuery, " ")
	cleanQuery = excludeRegex.ReplaceAllString(cleanQuery, " ")
	
	// Split into words and filter out stop words
	words := strings.Fields(cleanQuery)
	for _, word := range words {
		if len(word) > 2 && !StopWords[word] {
			params.Keywords = append(params.Keywords, word)
		}
	}
	
	// Create filtered keywords (important terms only)
	params.FilteredKeywords = FilterKeywords(params.Keywords)
	
	// Detect general categories from keywords
	params.QueryCategories = DetectCategories(params.FilteredKeywords)
	
	return params
}

// FilterKeywords removes stop words and keeps only meaningful terms
func FilterKeywords(keywords []string) []string {
	var filtered []string
	
	for _, word := range keywords {
		if len(word) > 2 && !StopWords[word] {
			filtered = append(filtered, word)
		}
	}
	
	return filtered
}

// DetectCategories identifies general topic categories from keywords without hardcoding specific responses
func DetectCategories(keywords []string) []string {
	// Topic category maps (expandable, but domain-agnostic)
	categoryKeywords := map[string][]string{
		"technology": {"tech", "software", "hardware", "app", "computer", "digital", "device", "code", "program", "programming", "javascript", "python", "golang", "algorithm", "data", "cloud", "server", "api", "mobile", "web", "developer"},
		
		"entertainment": {"show", "movie", "film", "tv", "television", "series", "episode", "watch", "stream", "actor", "actress", "director", "character", "plot", "scene", "season", "netflix", "hulu", "disney", "hbo", "amazon", "comedy", "drama"},
		
		"gaming": {"game", "gaming", "play", "player", "console", "ps5", "xbox", "nintendo", "steam", "fps", "rpg", "mmorpg", "strategy", "puzzle", "minecraft", "fortnite", "character", "level", "quest", "achievement", "multiplayer", "esports"},
		
		"sports": {"team", "player", "match", "sport", "league", "championship", "tournament", "game", "score", "coach", "athlete", "basketball", "football", "soccer", "baseball", "hockey", "tennis", "golf", "olympics", "nfl", "nba", "mlb"},
		
		"science": {"science", "scientific", "research", "study", "experiment", "theory", "discovery", "biology", "chemistry", "physics", "astronomy", "space", "earth", "environment", "climate", "laboratory", "scientist", "hypothesis", "data"},
		
		"finance": {"money", "stock", "invest", "finance", "financial", "market", "trade", "crypto", "bitcoin", "ethereum", "dividend", "retirement", "saving", "budget", "loan", "mortgage", "credit", "debt", "bank", "portfolio", "fund"},
		
		"health": {"health", "medical", "doctor", "medicine", "symptom", "treatment", "diet", "exercise", "fitness", "wellness", "mental", "anxiety", "depression", "therapy", "workout", "nutrition", "vitamin", "protein", "disease", "condition"},
		
		"food": {"food", "recipe", "cook", "cooking", "restaurant", "meal", "dish", "ingredient", "kitchen", "chef", "bake", "dessert", "dinner", "lunch", "breakfast", "flavor", "cuisine", "taste", "delicious", "pizza", "burger", "vegan"},
		
		"travel": {"travel", "trip", "vacation", "destination", "hotel", "flight", "visit", "tour", "country", "city", "beach", "mountain", "hiking", "backpacking", "resort", "cruise", "passport", "tourism", "sight", "landmark", "adventure"},
		
		"education": {"learn", "school", "college", "university", "course", "study", "education", "academic", "student", "teacher", "professor", "degree", "major", "class", "lecture", "exam", "homework", "textbook", "curriculum", "grade"},
		
		"politics": {"politics", "government", "election", "vote", "president", "congress", "senate", "democrat", "republican", "liberal", "conservative", "policy", "law", "regulation", "campaign", "political", "candidate", "ballot", "debate"},
		
		"relationships": {"relationship", "dating", "marriage", "partner", "boyfriend", "girlfriend", "husband", "wife", "divorce", "breakup", "family", "friend", "romantic", "love", "couple", "wedding", "engagement", "conflict", "communication"},
	}
	
	// Detect categories from keywords
	categoryMatches := make(map[string]int)
	for _, keyword := range keywords {
		for category, categoryWords := range categoryKeywords {
			for _, categoryWord := range categoryWords {
				if strings.Contains(keyword, categoryWord) || strings.Contains(categoryWord, keyword) {
					categoryMatches[category]++
					break
				}
			}
		}
	}
	
	// Return categories that have matches, ordered by match count
	var categories []string
	for category, count := range categoryMatches {
		if count > 0 {
			categories = append(categories, category)
		}
	}
	
	// Sort categories by match count (highest first)
	sort.Slice(categories, func(i, j int) bool {
		return categoryMatches[categories[i]] > categoryMatches[categories[j]]
	})
	
	return categories
}