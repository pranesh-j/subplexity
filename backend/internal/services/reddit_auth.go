// File: backend/internal/services/reddit_auth.go

package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	redditTokenURL    = "https://www.reddit.com/api/v1/access_token"
	tokenExpiryBuffer = 5 * time.Minute
)

// RedditAuth manages Reddit API authentication
type RedditAuth struct {
	clientID     string
	clientSecret string
	userAgent    string
	httpClient   *http.Client
	
	accessToken    string
	tokenExpiry    time.Time
	tokenLock      sync.RWMutex
	
	// For monitoring and metrics
	lastTokenRefresh time.Time
	tokenRefreshes   int
	tokenErrors      int
}

// NewRedditAuth creates a new Reddit authentication manager
func NewRedditAuth(clientID, clientSecret, userAgent string, httpClient *http.Client) *RedditAuth {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}
	
	return &RedditAuth{
		clientID:     clientID,
		clientSecret: clientSecret,
		userAgent:    userAgent,
		httpClient:   httpClient,
	}
}

// GetAccessToken obtains or refreshes a Reddit access token
func (r *RedditAuth) GetAccessToken(ctx context.Context) (string, error) {
	// First check if we have a valid token (read lock)
	r.tokenLock.RLock()
	if r.accessToken != "" && time.Now().Add(tokenExpiryBuffer).Before(r.tokenExpiry) {
		token := r.accessToken
		r.tokenLock.RUnlock()
		return token, nil
	}
	r.tokenLock.RUnlock()
	
	// Need to refresh token (write lock)
	r.tokenLock.Lock()
	defer r.tokenLock.Unlock()
	
	// Double-check we still need refresh after acquiring write lock
	if r.accessToken != "" && time.Now().Add(tokenExpiryBuffer).Before(r.tokenExpiry) {
		return r.accessToken, nil
	}
	
	// Get new token
	log.Println("Obtaining new Reddit access token")
	
	// Prepare the token request
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	
	req, err := http.NewRequestWithContext(ctx, "POST", redditTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		r.tokenErrors++
		return "", fmt.Errorf("error creating token request: %w", err)
	}
	
	// Set required headers with more detailed logging
	log.Printf("Setting auth headers - User-Agent: %s", r.userAgent)
	req.Header.Set("User-Agent", r.userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	// Add authorization header
	authStr := fmt.Sprintf("%s:%s", r.clientID, r.clientSecret)
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(authStr))
	req.Header.Set("Authorization", "Basic "+encodedAuth)
	
	// Log full request details before sending (redacting the auth token for security)
	log.Printf("Sending token request to %s with User-Agent: %s", redditTokenURL, req.Header.Get("User-Agent"))
	
	// Make the request
	resp, err := r.httpClient.Do(req)
	if err != nil {
		r.tokenErrors++
		return "", fmt.Errorf("error making token request: %w", err)
	}
	defer resp.Body.Close()
	
	// Check for errors
	if resp.StatusCode != http.StatusOK {
		r.tokenErrors++
		// Try to read error message from response
		var errorResponse map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errorResponse); err == nil {
			if error, ok := errorResponse["error"]; ok {
				return "", fmt.Errorf("reddit auth error: %v", error)
			}
		}
		return "", fmt.Errorf("token request failed with status: %d", resp.StatusCode)
	}
	
	// Parse the response
	var tokenResponse struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
		Scope       string `json:"scope"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		r.tokenErrors++
		return "", fmt.Errorf("error parsing token response: %w", err)
	}
	
	if tokenResponse.AccessToken == "" {
		r.tokenErrors++
		return "", fmt.Errorf("received empty access token")
	}
	
	// Update the token and expiry
	r.accessToken = tokenResponse.AccessToken
	r.tokenExpiry = time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)
	r.lastTokenRefresh = time.Now()
	r.tokenRefreshes++
	
	log.Printf("Successfully obtained Reddit access token, expires in %d seconds", tokenResponse.ExpiresIn)
	return r.accessToken, nil
}

// GetAuthStatus gets authentication metrics
func (r *RedditAuth) GetAuthStatus() map[string]interface{} {
	r.tokenLock.RLock()
	defer r.tokenLock.RUnlock()
	
	return map[string]interface{}{
		"has_token":         r.accessToken != "",
		"token_expires_in":  time.Until(r.tokenExpiry).Seconds(),
		"refresh_count":     r.tokenRefreshes,
		"error_count":       r.tokenErrors,
		"last_refresh_ago":  time.Since(r.lastTokenRefresh).Seconds(),
	}
}

// Clear invalidates the current token
func (r *RedditAuth) Clear() {
	r.tokenLock.Lock()
	defer r.tokenLock.Unlock()
	
	r.accessToken = ""
	r.tokenExpiry = time.Time{}
}