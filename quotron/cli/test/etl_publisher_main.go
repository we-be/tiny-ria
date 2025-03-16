package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	// Channel names from etl_service.go - MUST match /pkg/etl/service.go
	StockChannel = "quotron:stocks"
)

// StockQuote represents a single stock quote (matching ETL service struct)
type StockQuote struct {
	Symbol        string    `json:"symbol"`
	Price         float64   `json:"price"`
	Change        float64   `json:"change"`
	ChangePercent float64   `json:"changePercent"`
	Volume        int64     `json:"volume"`
	Timestamp     time.Time `json:"timestamp"`
	Exchange      string    `json:"exchange"`
	Source        string    `json:"source"`
}

func main() {
	// Set logging
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(os.Stdout)
	log.Println("Starting ETL publisher test...")

	// Get symbol from command line args or use default
	symbol := "AAPL"
	if len(os.Args) > 1 {
		symbol = os.Args[1]
	}

	// Get price from command line args or use default
	price := 189.84
	if len(os.Args) > 2 {
		if p, err := strconv.ParseFloat(os.Args[2], 64); err == nil {
			price = p
		}
	}

	// Connect to Redis
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", 
		Password: "",
		DB:       0,
	})

	ctx := context.Background()

	// Test Redis connection
	pong, err := client.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Printf("Redis connection established: %s", pong)

	// Create a sample stock quote
	sampleQuote := StockQuote{
		Symbol:        symbol,
		Price:         price,
		Change:        2.36,
		ChangePercent: 1.26,
		Volume:        42768321,
		Timestamp:     time.Now(),
		Exchange:      "NASDAQ",
		Source:        "TEST",
	}

	// Publish to Redis
	// Publish in a loop
	count := 5
	for i := 0; i < count; i++ {
		// Update timestamp for each iteration
		sampleQuote.Timestamp = time.Now()
		
		quoteData, err := json.MarshalIndent(sampleQuote, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal quote: %v", err)
		}
		
		log.Printf("Publishing stock quote (%d/%d): %s", i+1, count, string(quoteData))
		
		// Publish to Redis
		result := client.Publish(ctx, StockChannel, string(quoteData))
		if err := result.Err(); err != nil {
			log.Fatalf("Failed to publish message: %v", err)
		}
		
		// Get the number of clients that received the message
		receivers, err := result.Result()
		if err != nil {
			fmt.Printf("Failed to get publish result: %v\n", err)
		} else {
			fmt.Printf("Message published to %d clients\n", receivers)
		}
		
		// Wait before sending next message
		if i < count-1 {
			time.Sleep(2 * time.Second)
		}
	}
	
	log.Println("Publication complete")
}