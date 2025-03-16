package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	// Use the same channel name as the ETL service - MUST match pkg/etl/service.go
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

// We only need StockQuote, removing MarketData

func main() {
	// Set logging
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(os.Stdout)
	log.Println("Starting ETL worker test...")

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

	// Subscribe to the stock channel
	pubsub := client.Subscribe(ctx, StockChannel)
	defer pubsub.Close()

	// Start subscriber in goroutine
	go func() {
		log.Println("ETL worker is waiting for messages...")
		ch := pubsub.Channel()

		for msg := range ch {
			log.Printf("Received message on channel %s: %s", msg.Channel, msg.Payload)

			// Process stock quote message
			var quote StockQuote
			if err := json.Unmarshal([]byte(msg.Payload), &quote); err != nil {
				log.Printf("Error unmarshal stock quote: %v", err)
				continue
			}
			log.Printf("Processed stock quote: %s at $%.2f (%s)", 
				quote.Symbol, quote.Price, quote.Exchange)
		}
	}()

	// Wait for subscription to be established
	time.Sleep(1 * time.Second)

	// Publish a test stock quote
	sampleQuote := StockQuote{
		Symbol:        "AAPL",
		Price:         189.84,
		Change:        2.36,
		ChangePercent: 1.26,
		Volume:        42768321,
		Timestamp:     time.Now(),
		Exchange:      "NASDAQ",
		Source:        "TEST",
	}

	// Publish sample stock quote
	quoteData, _ := json.MarshalIndent(sampleQuote, "", "  ")
	log.Printf("Publishing stock quote: %s", string(quoteData))
	err = client.Publish(ctx, StockChannel, string(quoteData)).Err()
	if err != nil {
		log.Fatalf("Failed to publish stock quote: %v", err)
	}

	// Wait for messages to be processed
	log.Println("Waiting for message processing...")
	time.Sleep(10 * time.Second)
	log.Println("ETL worker test completed")
}