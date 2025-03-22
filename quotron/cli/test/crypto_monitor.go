package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	CryptoStream = "quotron:crypto:stream"
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
		Addr:     getRedisAddr(),
		Password: getRedisPassword(),
		DB:       0,
	})
	
	ctx := context.Background()
	
	// Test Redis connection
	pong, err := client.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	fmt.Printf("Redis connection established: %s\n", pong)
	
	// Create consumer group if it doesn't exist
	// Use a unique consumer group for this monitor
	consumerGroup := "crypto-monitor"
	streamName := CryptoStream
	
	// Create the consumer group
	_, err = client.XGroupCreateMkStream(ctx, streamName, consumerGroup, "0").Result()
	if err != nil && !redis.HasErrorPrefix(err, "BUSYGROUP") {
		log.Fatalf("Failed to create consumer group: %v", err)
	}
	
	fmt.Println("Monitoring crypto quotes stream. Press Ctrl+C to exit.")
	
	// Monitor stream
	for {
		// Read from stream with 1-second blocking timeout
		streams, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    consumerGroup,
			Consumer: "monitor-1",
			Streams:  []string{streamName, ">"},
			Count:    10,
			Block:    time.Second,
		}).Result()
		
		if err != nil {
			if err == redis.Nil || err.Error() == "redis: nil" {
				// No messages, just continue
				time.Sleep(500 * time.Millisecond)
				continue
			}
			
			log.Printf("Error reading from stream: %v", err)
			time.Sleep(time.Second)
			continue
		}
		
		// Process messages
		for _, stream := range streams {
			for _, message := range stream.Messages {
				// Extract payload
				data, ok := message.Values["data"].(string)
				if !ok {
					log.Printf("Invalid message format, no data field")
					continue
				}
				
				fmt.Printf("\nReceived crypto quote: %s\n", data)
				
				var quote StockQuote
				if err := json.Unmarshal([]byte(data), &quote); err != nil {
					fmt.Printf("Failed to unmarshal quote: %v\n", err)
					continue
				}
				
				fmt.Printf("[%s] %s @ $%.2f (%.2f%%, Vol: %d) - From %s\n", 
					quote.Timestamp.Format("15:04:05"),
					quote.Symbol, quote.Price, quote.ChangePercent, quote.Volume, quote.Source)
				
				// Acknowledge message
				client.XAck(ctx, streamName, consumerGroup, message.ID)
			}
		}
	}
}

// getRedisAddr returns the Redis address from environment or default
func getRedisAddr() string {
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "localhost"
	}
	
	port := os.Getenv("REDIS_PORT")
	if port == "" {
		port = "6379"
	}
	
	return fmt.Sprintf("%s:%s", host, port)
}

// getRedisPassword returns the Redis password from environment
func getRedisPassword() string {
	return os.Getenv("REDIS_PASSWORD")
}