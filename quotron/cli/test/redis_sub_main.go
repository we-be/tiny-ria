package main

import (
	"context"
	"github.com/go-redis/redis/v8"
	"log"
	"os"
	"time"
)

func main() {
	// Channel must match the ETL service
	channel := "quotron:queue:stock_quotes"
	
	// Setup logging
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(os.Stdout)
	log.Printf("Starting Redis subscription test on channel %s", channel)
	
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
	
	// Subscribe to channel
	log.Printf("Subscribing to channel %s", channel)
	pubsub := client.Subscribe(ctx, channel)
	defer pubsub.Close()
	
	// Confirm subscription
	_, err = pubsub.Receive(ctx)
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}
	log.Printf("Subscription confirmed")
	
	// Check number of subscribers
	subs, err := client.PubSubNumSub(ctx, channel).Result()
	if err != nil {
		log.Printf("Error checking subscribers: %v", err)
	} else {
		log.Printf("Channel %s has %d subscribers", channel, subs[channel])
	}
	
	// Start listening in a separate goroutine
	go func() {
		ch := pubsub.Channel()
		log.Printf("Listening for messages...")
		
		for msg := range ch {
			log.Printf("Received message: %s", msg.Payload)
		}
	}()
	
	// Check subscribers again after a moment
	time.Sleep(1 * time.Second)
	subs, err = client.PubSubNumSub(ctx, channel).Result()
	if err != nil {
		log.Printf("Error checking subscribers: %v", err)
	} else {
		log.Printf("Channel %s has %d subscribers", channel, subs[channel])
	}
	
	// Keep running for 60 seconds
	log.Printf("Subscription active, waiting for messages...")
	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Check number of subscribers
			subs, err := client.PubSubNumSub(ctx, channel).Result()
			if err != nil {
				log.Printf("Error checking subscribers: %v", err)
			} else {
				log.Printf("Channel %s has %d subscribers", channel, subs[channel])
			}
			
		case <-timeout:
			log.Printf("Test completed after 60 seconds")
			return
		}
	}
}