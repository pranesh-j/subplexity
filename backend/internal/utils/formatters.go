// File: backend/internal/utils/formatters.go

package utils

import (
	"fmt"
	"strings"
	"time"
)

// FormatTimeAgo formats a time as a human-readable "time ago" string
func FormatTimeAgo(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)
	
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 48*time.Hour:
		return "yesterday"
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%d days ago", days)
	case diff < 30*24*time.Hour:
		weeks := int(diff.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	case diff < 365*24*time.Hour:
		months := int(diff.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(diff.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

// TruncateWithEllipsis truncates a string to the given length and adds ellipsis if needed
func TruncateWithEllipsis(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}
	
	// Find a good breakpoint to avoid cutting words in the middle
	breakPoint := maxLength - 3 // Reserve space for ellipsis
	for i := breakPoint; i > breakPoint-20 && i > 0; i-- {
		if text[i] == ' ' || text[i] == ',' || text[i] == '.' {
			breakPoint = i
			break
		}
	}
	
	return text[:breakPoint] + "..."
}

// SanitizeString removes potentially problematic characters from a string
func SanitizeString(input string) string {
	// Replace problematic characters
	replacer := strings.NewReplacer(
		"<", "&lt;",
		">", "&gt;",
		"&", "&amp;",
		"\"", "&quot;",
		"'", "&#39;",
	)
	
	return replacer.Replace(input)
}

// FormatNumber returns a formatted string representation of a number
// with thousand separators
func FormatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	
	if n < 10000 {
		// For 4-digit numbers, show with comma: 1,234
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	
	if n < 1000000 {
		// For larger numbers, use K format: 12.3K
		if n < 10000 {
			return fmt.Sprintf("%.1fK", float64(n)/1000)
		}
		return fmt.Sprintf("%dK", n/1000)
	}
	
	// For millions or more, use M format: 1.2M
	return fmt.Sprintf("%.1fM", float64(n)/1000000)
}

// GetReadingTime estimates the reading time for a text in minutes
func GetReadingTime(text string) int {
	// Average reading speed is about 200-250 words per minute
	const wordsPerMinute = 225
	
	// Count words (roughly by counting spaces)
	wordCount := len(strings.Fields(text))
	
	// Calculate reading time in minutes, with a minimum of 1 minute
	readingTime := (wordCount + wordsPerMinute - 1) / wordsPerMinute // Ceiling division
	if readingTime < 1 {
		readingTime = 1
	}
	
	return readingTime
}