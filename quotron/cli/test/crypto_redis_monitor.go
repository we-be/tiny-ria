package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	StockChannel  = "quotron:stocks"
	CryptoChannel = "quotron:crypto"
	StockStream   = "quotron:stocks:stream"
	CryptoStream  = "quotron:crypto:stream"
)

func main() {
	// Parse command-line flags
	redisAddr := flag.String("redis", "localhost:6379", "Redis address")
	duration := flag.Int("duration", 60, "Monitoring duration in seconds")
	flag.Parse()

	// Set logging
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(os.Stdout)
	log.Println("Starting Redis channel monitor...")

	// Connect to Redis
	client := redis.NewClient(&redis.Options{
		Addr:     *redisAddr,
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

	// Monitor for specified duration
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	timeout := time.After(time.Duration(*duration) * time.Second)
	
	for {
		select {
		case <-ticker.C:
			// Check channels
			stockSubs, err := client.PubSubNumSub(ctx, StockChannel).Result()
			if err != nil {
				log.Printf("Error checking stock channel: %v", err)
				continue
			}
			
			cryptoSubs, err := client.PubSubNumSub(ctx, CryptoChannel).Result()
			if err != nil {
				log.Printf("Error checking crypto channel: %v", err)
				continue
			}
			
			// Check streams
			var stockStreamLen, cryptoStreamLen int64
			stockStreamInfo, err := client.XInfoStream(ctx, StockStream).Result()
			if err == nil {
				stockStreamLen = stockStreamInfo.Length
			}
			
			cryptoStreamInfo, err := client.XInfoStream(ctx, CryptoStream).Result()
			if err == nil {
				cryptoStreamLen = cryptoStreamInfo.Length
			}
			
			// Display results
			fmt.Printf("[%s] Status:\n", time.Now().Format("15:04:05.000"))
			fmt.Printf("  PubSub: %s (%d subs), %s (%d subs)\n", 
				StockChannel, stockSubs[StockChannel],
				CryptoChannel, cryptoSubs[CryptoChannel])
			fmt.Printf("  Streams: %s (%d msgs), %s (%d msgs)\n", 
				StockStream, stockStreamLen,
				CryptoStream, cryptoStreamLen)
			fmt.Println()
				
		case <-timeout:
			fmt.Println("\nMonitoring finished")
			return
		}
	}
}