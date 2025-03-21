package main

import (
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
		api.POST("/search", searchHandler.HandleSearch)
	}

	// Start server
	log.Printf("Server starting on port %s\n", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}