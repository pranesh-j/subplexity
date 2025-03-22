// File: backend/cmd/server/main.go

package main

import (
	"context"
	"log"
	"net/http"  // Add this import
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/pranesh-j/subplexity/api/handlers"
	"github.com/pranesh-j/subplexity/internal/services"
)

func main() {
	// Set up context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Capture OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutdown signal received")
		cancel()
	}()

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: No .env file found, using environment variables")
	}

	// Get required environment variables
	port := getEnvWithDefault("PORT", "8080")
	redditClientID := os.Getenv("REDDIT_API_CLIENT_ID")
	redditClientSecret := os.Getenv("REDDIT_API_CLIENT_SECRET")

	// Check for API credentials
	if redditClientID == "" || redditClientSecret == "" {
		log.Println("Warning: Reddit API credentials not set. Set REDDIT_API_CLIENT_ID and REDDIT_API_CLIENT_SECRET environment variables")
	}

	// Check for AI model API keys
	checkAICredentials()

	// Initialize services
	redditService := services.NewRedditService(redditClientID, redditClientSecret)
	aiService := services.NewAIService()

	// Initialize handlers
	searchHandler := handlers.NewSearchHandler(redditService, aiService)

	// Set up router - using production mode
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	// Configure CORS for development and production
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "https://subplexity.vercel.app"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Content-Length", "Accept-Encoding", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// API routes
	api := r.Group("/api")
	{
		api.POST("/search", func(c *gin.Context) {
			// Create a request-specific context with timeout
			reqCtx, reqCancel := context.WithTimeout(ctx, 30*time.Second)
			defer reqCancel()
			
			// Fixed: Pass the gin.Context directly instead of trying to modify its request
			// Just set a value in the context that can be retrieved later
			c.Set("requestContext", reqCtx)
			searchHandler.HandleSearch(c)
		})
		
		// Add health check endpoint
		api.GET("/health", func(c *gin.Context) {
			// Fixed: Using a simple static response instead of calling a non-existent method
			c.JSON(200, gin.H{
				"status":      "ok",
				"time":        time.Now().Format(time.RFC3339),
				"reddit_auth": "initialized", // Simplified status
			})
		})
	}

	// Start server with graceful shutdown
	log.Printf("Server starting on port %s\n", port)
	
	// Update search handler to use context for all operations
	go func() {
		if err := searchHandler.Init(ctx); err != nil {
			log.Printf("Failed to initialize search handler: %v", err)
		}
	}()

	// Create and start the server with graceful shutdown
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}
	
	// Start server in a goroutine so it doesn't block shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()
	
	// Wait for context cancellation (from signal or other shutdown trigger)
	<-ctx.Done()
	
	// Create a timeout context for shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	
	// Attempt graceful shutdown
	log.Println("Shutting down server...")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}
	
	log.Println("Server gracefully stopped")
}

// getEnvWithDefault returns the value of an environment variable or a default value
func getEnvWithDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// checkAICredentials checks if AI model API keys are set and logs warnings
func checkAICredentials() {
	// Check for Anthropic API key
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		log.Println("Warning: ANTHROPIC_API_KEY not set. Claude model will use mock responses.")
	}
	
	// Check for OpenAI API key
	if os.Getenv("OPENAI_API_KEY") == "" {
		log.Println("Warning: OPENAI_API_KEY not set. GPT models will use mock responses.")
	}
	
	// Check for Google AI API key
	if os.Getenv("GOOGLE_API_KEY") == "" {
		log.Println("Warning: GOOGLE_API_KEY not set. Gemini models will use mock responses.")
	}
	
	// Check for DeepSeek API key
	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		log.Println("Warning: DEEPSEEK_API_KEY not set. DeepSeek models will use mock responses.")
	}
}