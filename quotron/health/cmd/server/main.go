package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/lib/pq"
	"github.com/we-be/tiny-ria/quotron/health"
	"github.com/we-be/tiny-ria/quotron/health/api"
)

func main() {
	// Parse command line flags
	port := flag.Int("port", 8085, "Port to listen on")
	dbHost := flag.String("db-host", "localhost", "Database host")
	dbPort := flag.Int("db-port", 5432, "Database port")
	dbName := flag.String("db-name", "quotron", "Database name")
	dbUser := flag.String("db-user", "quotron", "Database user")
	dbPassword := flag.String("db-password", "quotron", "Database password")
	flag.Parse()

	// Connect to the database
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		*dbHost, *dbPort, *dbUser, *dbPassword, *dbName,
	)
	
	log.Printf("Connecting to database: %s:%d/%s", *dbHost, *dbPort, *dbName)
	
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer db.Close()

	// Test the database connection
	err = db.Ping()
	if err != nil {
		log.Fatalf("Error pinging database: %v", err)
	}
	log.Println("Connected to database successfully")

	// Create the health service
	healthService := health.NewHealthService(db)

	// Create the API
	healthAPI := api.NewHealthAPI(healthService)

	// Start the HTTP server
	addr := fmt.Sprintf(":%d", *port)
	server := &http.Server{
		Addr:    addr,
		Handler: healthAPI,
	}

	// Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Starting health server on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error starting server: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-stop
	log.Println("Shutting down server...")
	
	// Close the database connection
	db.Close()
	log.Println("Server shutdown complete")
}