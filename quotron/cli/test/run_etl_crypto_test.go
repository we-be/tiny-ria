package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/we-be/tiny-ria/quotron/cli/pkg/etl"
)

func main() {
	// Parse command-line flags
	redisAddr := flag.String("redis", "localhost:6379", "Redis address")
	dbConnStr := flag.String("db", "postgres://postgres:postgres@localhost:5432/quotron?sslmode=disable", "Database connection string")
	workers := flag.Int("workers", 2, "Number of worker threads")
	duration := flag.Int("duration", 120, "How long to run the service (seconds)")
	flag.Parse()

	// Set up logging
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(os.Stdout)

	// Create and start ETL service
	service := etl.NewService(*redisAddr, *dbConnStr, *workers)
	
	err := service.Start()
	if err != nil {
		log.Fatalf("Failed to start ETL service: %v", err)
	}
	
	fmt.Println("ETL service started successfully")
	fmt.Println("Service is now running and will consume messages from:")
	fmt.Println("- Redis Pub/Sub channels: quotron:stocks and quotron:crypto")
	fmt.Println("- Redis Streams: quotron:stocks:stream and quotron:crypto:stream")
	fmt.Println("Press Ctrl+C to stop")
	
	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	
	// Wait for either timeout or signal
	select {
	case <-sigCh:
		fmt.Println("\nReceived signal, stopping...")
	case <-time.After(time.Duration(*duration) * time.Second):
		fmt.Printf("\nTimeout of %d seconds reached, stopping...\n", *duration)
	}
	
	// Stop the service
	service.Stop()
	fmt.Println("ETL service stopped")
}