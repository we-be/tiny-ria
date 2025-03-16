package main

import (
	"context"
	"encoding/json"
	"flag"
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
	// Parse command line arguments
	symbol := flag.String("symbol", "BTC-USD", "Cryptocurrency symbol (e.g., BTC-USD)")
	price := flag.Float64("price", 67500.0, "Current price")
	change := flag.Float64("change", 1200.0, "Price change")
	changePercent := flag.Float64("change-percent", 1.75, "Price change percentage")
	volume := flag.Int64("volume", 15000000, "Trading volume")
	monitor := flag.Bool("monitor", false, "Monitor mode - listen for messages")
	flag.Parse()

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
	
	if *monitor {
		// Create a pub/sub
		pubsub := client.Subscribe(ctx, CryptoQuoteChannel)
		defer pubsub.Close()
		
		// In a separate goroutine, receive messages
		ch := pubsub.Channel()
		
		fmt.Println("Monitoring crypto quotes channel...")
		for msg := range ch {
			fmt.Printf("Received crypto quote: %s\n", msg.Payload)
			
			var quote StockQuote
			if err := json.Unmarshal([]byte(msg.Payload), &quote); err != nil {
				fmt.Printf("Failed to unmarshal quote: %v\n", err)
				continue
			}
			
			fmt.Printf("Crypto Quote: %s @ $%.2f (%.2f%%, Vol: %d) - From %s\n", 
				quote.Symbol, quote.Price, quote.ChangePercent, quote.Volume, quote.Source)
		}
	} else {
		// Publish a crypto quote
		quote := StockQuote{
			Symbol:        *symbol,
			Price:         *price,
			Change:        *change,
			ChangePercent: *changePercent,
			Volume:        *volume,
			Timestamp:     time.Now(),
			Exchange:      "CRYPTO",
			Source:        "Test Publisher",
		}
		
		data, err := json.Marshal(quote)
		if err != nil {
			log.Fatalf("Failed to marshal crypto quote: %v", err)
		}
		
		fmt.Printf("Publishing crypto quote for %s @ $%.2f\n", quote.Symbol, quote.Price)
		
		result := client.Publish(ctx, CryptoQuoteChannel, string(data))
		if err := result.Err(); err != nil {
			log.Fatalf("Failed to publish crypto quote: %v", err)
		}
		
		receivers, err := result.Result()
		if err != nil {
			fmt.Printf("Failed to get publish result: %v\n", err)
		} else {
			fmt.Printf("Crypto quote published to %d clients\n", receivers)
		}

		// Also publish directly to the Redis stream if requested via flag
		fmt.Println("Crypto quote published successfully")
	}
}