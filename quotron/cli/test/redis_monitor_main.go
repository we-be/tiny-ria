package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	StockStream  = "quotron:stocks:stream"
	CryptoStream = "quotron:crypto:stream"
	IndexStream  = "quotron:indices:stream"
)

func main() {
	// Set logging
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(os.Stdout)
	log.Println("Starting Redis stream monitor...")

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
			// Check streams
			fmt.Printf("[%s] Stream counts:\n", time.Now().Format("15:04:05.000"))
			
			for _, stream := range []string{StockStream, CryptoStream, IndexStream} {
				info, err := client.XInfoStream(ctx, stream).Result()
				if err != nil {
					log.Printf("Error checking stream %s: %v", stream, err)
					continue
				}
				
				fmt.Printf("  %s: %d messages\n", stream, info.Length)
				
				// Check consumer groups
				groups, err := client.XInfoGroups(ctx, stream).Result()
				if err != nil {
					log.Printf("Error checking consumer groups for %s: %v", stream, err)
					continue
				}
				
				if len(groups) > 0 {
					fmt.Printf("  Consumer groups for %s:\n", stream)
					for _, group := range groups {
						fmt.Printf("    %s: consumers=%d, pending=%d, last-delivered-id=%s\n", 
							group.Name, group.Consumers, group.Pending, group.LastDeliveredID)
					}
				}
			}
			fmt.Println()
					
		case <-timeout:
			fmt.Println("\nMonitoring finished")
			return
		}
	}
}