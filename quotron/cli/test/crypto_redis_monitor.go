package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Redis channels and streams
	StockChannel   = "quotron:stocks"
	StockStream    = "quotron:stocks:stream"
	CryptoChannel  = "quotron:crypto"
	CryptoStream   = "quotron:crypto:stream"
	IndexChannel   = "quotron:indices"
	IndexStream    = "quotron:indices:stream"
	RedisAddr      = "localhost:6379"
)

func main() {
	log.Println("Starting Redis Monitor for Quotron Channels and Streams...")

	// Parse command-line arguments
	var showMessageContent bool
	for _, arg := range os.Args[1:] {
		if arg == "-v" || arg == "--verbose" {
			showMessageContent = true
		}
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handler
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalCh
		fmt.Println("\nReceived termination signal, shutting down...")
		cancel()
	}()

	// Connect to Redis
	client := redis.NewClient(&redis.Options{
		Addr: RedisAddr,
	})
	defer client.Close()

	// Verify connection
	_, err := client.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Printf("Connected to Redis at %s", RedisAddr)

	// Subscribe to all PubSub channels
	pubsub := client.Subscribe(ctx, StockChannel, CryptoChannel, IndexChannel)
	defer pubsub.Close()

	// Verify subscription
	_, err = pubsub.Receive(ctx)
	if err != nil {
		log.Fatalf("Failed to subscribe to channels: %v", err)
	}
	log.Printf("Subscribed to channels: %s, %s, %s", StockChannel, CryptoChannel, IndexChannel)

	// Channel for PubSub messages
	msgCh := pubsub.Channel()

	// Counter for messages
	messageCount := map[string]int{
		StockChannel:  0,
		CryptoChannel: 0,
		IndexChannel:  0,
		StockStream:   0,
		CryptoStream:  0,
		IndexStream:   0,
	}

	// Start a goroutine to periodically display counts
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Get stream counts
				for _, stream := range []string{StockStream, CryptoStream, IndexStream} {
					info, err := client.XInfoStream(ctx, stream).Result()
					if err == nil {
						messageCount[stream] = int(info.Length)
					}
				}

				// Display counts
				fmt.Println("\n--- Message Counts ---")
				fmt.Printf("PubSub Channels:\n")
				fmt.Printf("  %s: %d messages\n", StockChannel, messageCount[StockChannel])
				fmt.Printf("  %s: %d messages\n", CryptoChannel, messageCount[CryptoChannel])
				fmt.Printf("  %s: %d messages\n", IndexChannel, messageCount[IndexChannel])
				
				fmt.Printf("\nStreams:\n")
				fmt.Printf("  %s: %d messages\n", StockStream, messageCount[StockStream])
				fmt.Printf("  %s: %d messages\n", CryptoStream, messageCount[CryptoStream])
				fmt.Printf("  %s: %d messages\n", IndexStream, messageCount[IndexStream])
				fmt.Println("-----------------------")
			}
		}
	}()

	// Main loop for processing PubSub messages
	log.Printf("Waiting for messages...")
	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down...")
			return

		case msg, ok := <-msgCh:
			if !ok {
				log.Println("PubSub channel closed")
				return
			}

			// Increment counter
			messageCount[msg.Channel]++
			
			// Log the message
			if showMessageContent {
				log.Printf("Channel: %s, Message: %s", msg.Channel, msg.Payload)
			} else {
				log.Printf("Received message on channel: %s", msg.Channel)
			}
		}
	}
}