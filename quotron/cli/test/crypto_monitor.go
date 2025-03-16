package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	CryptoQuoteChannel = "quotron:crypto"
)

// StockQuote represents a stock quote (used for crypto quotes too)
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
	fmt.Printf("Redis connection established: %s\n", pong)
	
	// Create a pub/sub
	pubsub := client.Subscribe(ctx, CryptoQuoteChannel)
	defer pubsub.Close()
	
	// Get message channel
	ch := pubsub.Channel()
	
	fmt.Println("Monitoring crypto quotes channel. Press Ctrl+C to exit.")
	for msg := range ch {
		fmt.Printf("\nReceived crypto quote: %s\n", msg.Payload)
		
		var quote StockQuote
		if err := json.Unmarshal([]byte(msg.Payload), &quote); err != nil {
			fmt.Printf("Failed to unmarshal quote: %v\n", err)
			continue
		}
		
		fmt.Printf("[%s] %s @ $%.2f (%.2f%%, Vol: %d) - From %s\n", 
			quote.Timestamp.Format("15:04:05"),
			quote.Symbol, quote.Price, quote.ChangePercent, quote.Volume, quote.Source)
	}
}