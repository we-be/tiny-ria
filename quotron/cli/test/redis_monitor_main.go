package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	StockQuotesChannel = "quotron:queue:stock_quotes"
	MarketDataChannel = "quotron:queue:market_indices"
)

func main() {
	// Set logging
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(os.Stdout)
	log.Println("Starting Redis channel monitor...")

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

	// Monitor for 30 seconds
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	timeout := time.After(30 * time.Second)
	
	for {
		select {
		case <-ticker.C:
			// Check channels
			stockSubs, err := client.PubSubNumSub(ctx, StockQuotesChannel).Result()
			if err != nil {
				log.Printf("Error checking stock channel: %v", err)
				continue
			}
			
			marketSubs, err := client.PubSubNumSub(ctx, MarketDataChannel).Result()
			if err != nil {
				log.Printf("Error checking market channel: %v", err)
				continue
			}
			
			// Display results
			fmt.Printf("[%s] Channel subscribers - %s: %d, %s: %d\n", 
				time.Now().Format("15:04:05.000"),
				StockQuotesChannel, stockSubs[StockQuotesChannel],
				MarketDataChannel, marketSubs[MarketDataChannel])
				
		case <-timeout:
			fmt.Println("\nMonitoring finished")
			return
		}
	}
}