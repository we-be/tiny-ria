package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"log"
	"os"
	"time"
)

const (
	// Stream configuration 
	StockStream   = "quotron:stocks:stream"  // Stock quotes stream
	ConsumerGroup = "quotron:cli"           // Consumer group name
	ConsumerName  = "cli-consumer"          // Consumer name
)

// StockQuote represents a stock quote
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
	// Setup logging
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(os.Stdout)
	log.Printf("Starting Redis Stream consumer for %s", StockStream)
	
	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	
	// Create context
	ctx := context.Background()
	
	// Ping Redis
	_, err := client.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Redis ping failed: %v", err)
	}
	log.Printf("Redis ping successful")
	
	// Create consumer group if it doesn't exist
	_, err = client.XGroupCreateMkStream(ctx, StockStream, ConsumerGroup, "0").Result()
	if err != nil {
		// If group already exists, this is fine
		if !redis.HasErrorPrefix(err, "BUSYGROUP") {
			log.Printf("Failed to create consumer group: %v", err)
		} else {
			log.Printf("Using existing consumer group %s for stream %s", ConsumerGroup, StockStream)
		}
	} else {
		log.Printf("Created consumer group %s for stream %s", ConsumerGroup, StockStream)
	}
	
	// Get stream info
	streamInfo, err := client.XInfoStream(ctx, StockStream).Result()
	if err != nil {
		log.Printf("Error getting stream info: %v", err)
	} else {
		log.Printf("Stream %s length: %d", StockStream, streamInfo.Length)
	}
	
	// Keep running for 60 seconds
	log.Printf("Stream consumer active, waiting for messages...")
	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Read from stream with a short blocking timeout
			streams, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    ConsumerGroup,
				Consumer: ConsumerName,
				Streams:  []string{StockStream, ">"}, // ">" means only undelivered messages
				Count:    5, // Process up to 5 messages at a time
				Block:    500 * time.Millisecond,
			}).Result()
			
			if err != nil {
				if err == redis.Nil || err.Error() == "redis: nil" {
					// No messages available, just continue
					continue
				}
				log.Printf("Error reading from stream: %v", err)
				continue
			}
			
			// Process messages
			for _, stream := range streams {
				for _, message := range stream.Messages {
					// Extract payload
					data, ok := message.Values["data"].(string)
					if !ok {
						log.Printf("Invalid message format, no data field: %v", message.Values)
						// Acknowledge message even if invalid
						client.XAck(ctx, StockStream, ConsumerGroup, message.ID)
						continue
					}
					
					// Try to parse the message as a stock quote
					var quote StockQuote
					err := json.Unmarshal([]byte(data), &quote)
					if err != nil {
						log.Printf("Error parsing message: %v", err)
						log.Printf("Raw message: %s", data)
						// Acknowledge message
						client.XAck(ctx, StockStream, ConsumerGroup, message.ID)
						continue
					}
					
					// Process the stock quote
					fmt.Printf("Received quote for %s: $%.2f (%s) @ %s\n", 
						quote.Symbol, quote.Price, quote.Exchange, quote.Timestamp.Format(time.RFC3339))
					
					// Acknowledge message after successful processing
					client.XAck(ctx, StockStream, ConsumerGroup, message.ID)
				}
			}
			
		case <-timeout:
			log.Printf("Test completed after 60 seconds")
			return
		}
	}
}