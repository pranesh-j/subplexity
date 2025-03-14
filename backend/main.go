// main.go
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
	"github.com/prane/subplex/api"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	fmt.Println("Starting Subplexity API server...")
	
	// Create router
	r := mux.NewRouter()
	
	// API routes
	r.HandleFunc("/api/search", api.SearchHandler).Methods("GET")
	r.HandleFunc("/api/trending", api.TrendingHandler).Methods("GET")
	r.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("API is running"))
	}).Methods("GET")
	
	// Setup CORS
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{os.Getenv("CORS_ALLOWED_ORIGINS")},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           300,
	})
	
	// Set up server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port
	}
	
	srv := &http.Server{
		Handler:      corsHandler.Handler(r),
		Addr:         ":" + port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	
	// Start server
	log.Printf("Server running on port %s", port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}