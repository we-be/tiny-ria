package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/we-be/tiny-ria/quotron/cli/pkg/etl"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Define command-line flags
	redisAddr := flag.String("redis", "localhost:6379", "Redis server address")
	dbHost := flag.String("dbhost", "localhost", "Database host")
	dbPort := flag.Int("dbport", 5432, "Database port")
	dbName := flag.String("dbname", "quotron", "Database name")
	dbUser := flag.String("dbuser", "quotron", "Database user")
	dbPass := flag.String("dbpass", "quotron", "Database password")
	workers := flag.Int("workers", 2, "Number of worker threads")

	// Define command flags
	startCmd := flag.Bool("start", false, "Start the ETL service")
	stopCmd := flag.Bool("stop", false, "Stop the ETL service")
	statusCmd := flag.Bool("status", false, "Check ETL service status")
	
	flag.Parse()

	// Construct database connection string
	dbConnStr := fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=disable",
		*dbHost, *dbPort, *dbName, *dbUser, *dbPass,
	)

	// Create ETL service
	service := etl.NewService(*redisAddr, dbConnStr, *workers)

	// Handle commands
	if *startCmd {
		fmt.Println("Starting ETL service...")
		if err := service.Start(); err != nil {
			log.Fatalf("Failed to start ETL service: %v", err)
		}
		fmt.Printf("ETL service started with %d workers\n", *workers)
		fmt.Printf("Using Redis at %s\n", *redisAddr)
		fmt.Printf("Using database at %s:%d\n", *dbHost, *dbPort)
		
		// Set up signal handler to keep the service running
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		
		// Wait for termination signal
		<-sigCh
		fmt.Println("Received termination signal, shutting down...")
		service.Stop()
		
	} else if *stopCmd {
		fmt.Println("Stopping ETL service...")
		service.Stop()
		fmt.Println("ETL service stopped")
		
	} else if *statusCmd {
		if service.IsRunning() {
			fmt.Println("ETL service is running")
		} else {
			fmt.Println("ETL service is not running")
		}
		
	} else {
		// If no specific command is given, print usage
		fmt.Println("ETL Service Management Tool")
		fmt.Println("---------------------------")
		fmt.Println("Usage:")
		fmt.Printf("  %s -start   # Start the ETL service\n", os.Args[0])
		fmt.Printf("  %s -stop    # Stop the ETL service\n", os.Args[0])
		fmt.Printf("  %s -status  # Check service status\n", os.Args[0])
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
	}
}