package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	// Parse command-line flags
	symbol := flag.String("symbol", "BTC-USD", "Cryptocurrency symbol to fetch")
	redisOnly := flag.Bool("redis-only", true, "Use only Redis for data flow (no direct DB write)")
	flag.Parse()
	
	// Set up logging
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(os.Stdout)
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	fmt.Printf("Testing crypto quote job with ETL flow for symbol: %s\n", *symbol)
	
	// Create job with our fixed implementation that doesn't write to DB
	job := NewCryptoQuoteJob("", *redisOnly)
	
	// Set up job parameters
	params := map[string]string{
		"symbols": *symbol,
	}
	
	// Execute job - this will fetch the data and publish to Redis only
	fmt.Println("Executing crypto quote job (Redis publish only)...")
	err := job.Execute(ctx, params)
	if err != nil {
		log.Fatalf("Failed to execute crypto quote job: %v", err)
	}
	
	fmt.Println("Crypto quote job completed successfully")
	fmt.Println("Data published to Redis. The ETL service should now process this data.")
	fmt.Println("")
	fmt.Println("To verify:")
	fmt.Println("1. Ensure ETL service is running: go run /home/hunter/Desktop/tiny-ria/quotron/cli/cmd/etl/main.go")  
	fmt.Println("2. Check Redis messages: go run redis_monitor_main.go")
	fmt.Println("3. Check database for new entries from ETL service")
}