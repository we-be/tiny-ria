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
	ChannelName = "quotron:stocks"
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
	
	// Subscribe to channel
	pubsub := client.Subscribe(ctx, ChannelName)
	defer pubsub.Close()
	
	// Wait for confirmation of subscription
	_, err = pubsub.Receive(ctx)
	if err != nil {
		log.Fatalf("Failed to subscribe to channel: %v", err)
	}
	log.Printf("Subscribed to channel: %s", ChannelName)
	
	// Check subscription count
	count, err := client.PubSubNumSub(ctx, ChannelName).Result()
	if err != nil {
		log.Printf("Failed to get subscriber count: %v", err)
	} else {
		log.Printf("Channel %s has %d subscribers", ChannelName, count[ChannelName])
	}
	
	// Set up handler for Ctrl+C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		log.Println("Received interrupt signal, shutting down...")
		cancel()
	}()
	
	// Process incoming messages
	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		
		case msg, ok := <-ch:
			if !ok {
				log.Println("Channel closed")
				return
			}
			
			// Try to parse the message as a stock quote
			var quote StockQuote
			err := json.Unmarshal([]byte(msg.Payload), &quote)
			if err != nil {
				log.Printf("Error parsing message: %v", err)
				log.Printf("Raw message: %s", msg.Payload)
				continue
			}
			
			// Process the stock quote
			fmt.Printf("Received quote for %s: $%.2f (%s)\n", 
				quote.Symbol, quote.Price, quote.Exchange)
		}
	}
}