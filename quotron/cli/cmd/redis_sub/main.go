package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	StockStream  = "quotron:stocks:stream"  // Stream name for stock quotes
	ConsumerGroup = "quotron:cli"           // Consumer group name
	ConsumerID    = "cli-consumer"          // Consumer ID
)

// StockQuote represents a single stock quote
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
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	
	// Get Redis address from command line
	redisAddr := flag.String("redis", "localhost:6379", "Redis server address")
	flag.Parse()
	
	// Connect to Redis
	client := redis.NewClient(&redis.Options{
		Addr: *redisAddr,
	})
	defer client.Close()
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Test connection
	_, err := client.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis at %s: %v", *redisAddr, err)
	}
	log.Printf("Connected to Redis at %s", *redisAddr)
	
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
	
	// Set up handler for Ctrl+C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		log.Println("Received interrupt signal, shutting down...")
		cancel()
	}()
	
	log.Printf("Listening for messages on Redis Stream %s...", StockStream)
	
	// Process incoming messages in a loop
	for {
		select {
		case <-ctx.Done():
			return
			
		default:
			// Read from stream with 2-second blocking timeout
			streams, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    ConsumerGroup,
				Consumer: ConsumerID,
				Streams:  []string{StockStream, ">"}, // ">" means only undelivered messages
				Count:    10, // Process up to 10 messages at a time
				Block:    2 * time.Second,
			}).Result()
			
			if err != nil {
				if err == redis.Nil || err.Error() == "redis: nil" {
					// No messages available, just continue
					continue
				}
				
				// Real error
				log.Printf("Error reading from stream: %v", err)
				time.Sleep(1 * time.Second) // Avoid tight loops on errors
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
			
			// Brief pause to avoid CPU spinning if no messages
			time.Sleep(100 * time.Millisecond)
		}
	}
}