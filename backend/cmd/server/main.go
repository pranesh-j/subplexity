// File: backend/cmd/server/main.go

package main

import (
	"context"
	"log"
	"os"
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

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: No .env file found, using environment variables")
	}

	// Get required environment variables
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	redditClientID := os.Getenv("REDDIT_API_CLIENT_ID")
	redditClientSecret := os.Getenv("REDDIT_API_CLIENT_SECRET")

	if redditClientID == "" || redditClientSecret == "" {
		log.Println("Warning: Reddit API credentials not set. Set REDDIT_API_CLIENT_ID and REDDIT_API_CLIENT_SECRET environment variables")
	}

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
			// Pass the request context to ensure proper cancellation
			searchHandler.HandleSearch(c)
		})
	}

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// Start server with graceful shutdown
	log.Printf("Server starting on port %s\n", port)
	
	// Update search handler to use context for all operations
	go func() {
		if err := searchHandler.Init(ctx); err != nil {
			log.Printf("Failed to initialize search handler: %v", err)
		}
	}()

	// Start the server
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}