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
	// Redis streams
	StockStream    = "quotron:stocks:stream"
	CryptoStream   = "quotron:crypto:stream"
	IndexStream    = "quotron:indices:stream"
	RedisAddr      = "localhost:6379"
)

func main() {
	log.Println("Starting Redis Monitor for Quotron Streams...")

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
	
	// Counter for messages
	messageCount := map[string]int{
		StockStream:   0,
		CryptoStream:  0,
		IndexStream:   0,
	}

	// Start a goroutine to periodically display counts and monitor streams
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Get stream counts and latest messages
				for _, stream := range []string{StockStream, CryptoStream, IndexStream} {
					// Get stream info
					info, err := client.XInfoStream(ctx, stream).Result()
					if err == nil {
						messageCount[stream] = int(info.Length)
						
						// Get latest messages if verbose mode is enabled
						if showMessageContent {
							messages, err := client.XRevRange(ctx, stream, "+", "-", 1).Result()
							if err == nil && len(messages) > 0 {
								log.Printf("Latest message on %s: %v", stream, messages[0].Values)
							}
						}
					} else {
						log.Printf("Error getting stream info for %s: %v", stream, err)
					}
				}

				// Display counts
				fmt.Println("\n--- Message Counts ---")
				fmt.Printf("Streams:\n")
				fmt.Printf("  %s: %d messages\n", StockStream, messageCount[StockStream])
				fmt.Printf("  %s: %d messages\n", CryptoStream, messageCount[CryptoStream])
				fmt.Printf("  %s: %d messages\n", IndexStream, messageCount[IndexStream])
				fmt.Println("-----------------------")
			}
		}
	}()

	// Main loop just keeps the program running
	log.Printf("Monitoring Redis streams. Press Ctrl+C to exit...")
	<-ctx.Done()
	log.Println("Shutting down...")
}