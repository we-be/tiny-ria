package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	StockStream = "quotron:stocks:stream"  // Stock quotes stream
	StreamMaxLen = 1000                   // Maximum number of messages to keep in the stream
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

var symbols = []string{"AAPL", "MSFT", "GOOG", "AMZN", "META", "TSLA", "NVDA"}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	
	// Command line flags
	redisAddr := flag.String("redis", "localhost:6379", "Redis server address")
	symbol := flag.String("symbol", "", "Stock symbol")
	price := flag.Float64("price", 0, "Stock price")
	count := flag.Int("count", 1, "Number of messages to send")
	delay := flag.Int("delay", 1000, "Delay between messages (ms)")
	flag.Parse()
	
	// Connect to Redis
	client := redis.NewClient(&redis.Options{
		Addr: *redisAddr,
	})
	defer client.Close()
	
	ctx := context.Background()
	
	// Test connection
	_, err := client.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis at %s: %v", *redisAddr, err)
	}
	log.Printf("Connected to Redis at %s", *redisAddr)
	
	// Check stream info
	streamInfo, err := client.XInfoStream(ctx, StockStream).Result()
	if err != nil {
		if err == redis.Nil {
			log.Printf("Stream %s does not exist yet, it will be created", StockStream)
		} else {
			log.Printf("Warning: could not get stream info: %v", err)
		}
	} else {
		log.Printf("Stream %s has %d messages", StockStream, streamInfo.Length)
	}
	
	// Random price changes
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	
	// Send quotes
	for i := 0; i < *count; i++ {
		// Create a quote
		var quote StockQuote
		
		if *symbol != "" {
			// Use provided symbol and price
			quote.Symbol = *symbol
			if *price > 0 {
				quote.Price = *price
			} else {
				quote.Price = 100.0 + r.Float64()*200.0
			}
		} else {
			// Generate random data
			quote.Symbol = symbols[r.Intn(len(symbols))]
			quote.Price = 100.0 + r.Float64()*200.0
		}
		
		// Add additional data
		quote.Change = r.Float64()*4.0 - 2.0
		quote.ChangePercent = quote.Change / quote.Price * 100.0
		quote.Volume = int64(1000000 + r.Intn(10000000))
		quote.Timestamp = time.Now()
		quote.Exchange = "NASDAQ"
		quote.Source = "TEST"
		
		// Convert to JSON
		data, err := json.Marshal(quote)
		if err != nil {
			log.Printf("Error marshaling quote: %v", err)
			continue
		}
		
		// Publish to Redis Stream
		log.Printf("Publishing quote for %s: $%.2f to stream", quote.Symbol, quote.Price)
		
		// Create values map for XAdd
		values := map[string]interface{}{
			"data": string(data),
		}
		
		// Add to stream
		result, err := client.XAdd(ctx, &redis.XAddArgs{
			Stream: StockStream,
			ID:     "*", // Auto-generate ID
			Values: values,
			MaxLen: StreamMaxLen,
		}).Result()
		
		if err != nil {
			log.Printf("Error publishing to stream: %v", err)
			continue
		}
		
		log.Printf("Published message with ID: %s", result)
		
		// Wait before sending the next message
		if i < *count-1 && *delay > 0 {
			time.Sleep(time.Duration(*delay) * time.Millisecond)
		}
	}
	
	log.Printf("Published %d messages to stream", *count)
}