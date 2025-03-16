package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	CryptoQuoteChannel = "quotron:crypto"
)

// StockQuote represents a stock/crypto quote
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

// Mock implementation of the CryptoQuoteJob for testing
func main() {
	// Parse command-line flags
	symbolsFlag := flag.String("symbols", "BTC-USD,ETH-USD,SOL-USD", "Comma-separated list of crypto symbols")
	monitorFlag := flag.Bool("monitor", false, "Monitor Redis for crypto quotes")
	flag.Parse()

	if *monitorFlag {
		monitorRedis()
		return
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Split symbols and process each one
	symbols := strings.Split(*symbolsFlag, ",")
	fmt.Printf("Processing %d cryptocurrency symbols: %s\n", len(symbols), *symbolsFlag)

	for i, symbol := range symbols {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			continue
		}

		// Format crypto symbol if needed (ensure BTC-USD format)
		if !strings.Contains(symbol, "-") {
			symbol = symbol + "-USD"
			fmt.Printf("Formatted crypto symbol to %s\n", symbol)
		}

		// Add a delay between requests to avoid rate limiting
		if i > 0 {
			// Wait 5 seconds between requests to avoid rate limiting
			fmt.Printf("Waiting 5 seconds before next request (API rate limiting)...\n")
			select {
			case <-ctx.Done():
				log.Fatalf("Context cancelled: %v", ctx.Err())
			case <-time.After(5 * time.Second):
				// Continue after delay
			}
		}

		fmt.Printf("Fetching crypto quote for %s\n", symbol)
		
		// Call Yahoo Finance API through the api-scraper
		apiScraperPath := "/home/hunter/Desktop/tiny-ria/quotron/api-scraper/api-scraper"
		args := []string{"--yahoo", "--symbol", symbol, "--json"}
		
		fmt.Printf("Executing: %s %v\n", apiScraperPath, args)
		cmd := exec.CommandContext(ctx, apiScraperPath, args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("Failed to execute API scraper with Yahoo Finance: %v, output: %s\n", err, output)
			continue
		}

		// Parse the JSON
		var quoteData map[string]interface{}
		if err := json.Unmarshal(output, &quoteData); err != nil {
			fmt.Printf("Failed to parse JSON: %v\n", err)
			continue
		}

		// Create StockQuote from JSON data
		quote := &StockQuote{
			Symbol:        symbol,
			Price:         0.0,  // Will be populated from JSON
			Change:        0.0,  // Will be populated from JSON
			ChangePercent: 0.0,  // Will be populated from JSON
			Volume:        0,    // Will be populated from JSON
			Timestamp:     time.Now(),
			Exchange:      "CRYPTO",
			Source:        "Yahoo Finance",
		}
		
		// Extract values from JSON
		if price, ok := quoteData["price"].(float64); ok {
			quote.Price = price
		}
		if change, ok := quoteData["change"].(float64); ok {
			quote.Change = change
		}
		if changePercent, ok := quoteData["changePercent"].(float64); ok {
			quote.ChangePercent = changePercent
		}
		if volume, ok := quoteData["volume"].(float64); ok {
			quote.Volume = int64(volume)
		}
		
		// Print quote information
		fmt.Printf("Crypto Quote: %s @ $%.2f (%.2f%%, Vol: %d)\n", 
			quote.Symbol, quote.Price, quote.ChangePercent, quote.Volume)
			
		// Publish to Redis
		redisClient := redis.NewClient(&redis.Options{
			Addr:     "localhost:6379",
			Password: "",
			DB:       0,
		})
		defer redisClient.Close()
		
		// Convert to JSON
		data, err := json.Marshal(quote)
		if err != nil {
			fmt.Printf("Failed to marshal crypto quote: %v\n", err)
			continue
		}
		
		// Publish to Redis
		result := redisClient.Publish(ctx, CryptoQuoteChannel, string(data))
		if err := result.Err(); err != nil {
			fmt.Printf("Failed to publish to Redis: %v\n", err)
			continue
		}
		
		receivers, err := result.Result()
		if err != nil {
			fmt.Printf("Failed to get publish result: %v\n", err)
		} else {
			fmt.Printf("Published crypto quote to %d subscribers\n", receivers)
		}
		
		// Save output to file
		outputDir := "data"
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			fmt.Printf("Couldn't create data directory: %v\n", err)
		} else {
			timestamp := time.Now().Format("20060102-150405")
			filename := fmt.Sprintf("%s/%s-yahoo-%s.json", outputDir, symbol, timestamp)
			if err := os.WriteFile(filename, output, 0644); err != nil {
				fmt.Printf("Couldn't save output to %s: %v\n", filename, err)
			} else {
				fmt.Printf("Saved output to %s\n", filename)
			}
		}
	}

	fmt.Println("\nAll cryptocurrency quotes processed successfully")
	fmt.Println("To view Redis messages, run with the --monitor flag")
}

// monitorRedis listens for crypto quotes on the Redis channel
func monitorRedis() {
	ctx := context.Background()
	
	// Connect to Redis
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	defer client.Close()
	
	// Test Redis connection
	pong, err := client.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	fmt.Printf("Redis connection established: %s\n", pong)
	
	// Create a pub/sub
	pubsub := client.Subscribe(ctx, CryptoQuoteChannel)
	defer pubsub.Close()
	
	// Get message channel
	ch := pubsub.Channel()
	
	fmt.Println("Monitoring crypto quotes channel. Press Ctrl+C to exit.")
	for msg := range ch {
		fmt.Printf("Received raw message: %s\n", msg.Payload)
		
		var quote StockQuote
		if err := json.Unmarshal([]byte(msg.Payload), &quote); err != nil {
			fmt.Printf("Failed to unmarshal quote: %v\n", err)
			continue
		}
		
		fmt.Printf("[%s] %s @ $%.2f (%.2f%%, Vol: %d) - From %s\n", 
			quote.Timestamp.Format("15:04:05"),
			quote.Symbol, quote.Price, quote.ChangePercent, quote.Volume, quote.Source)
	}
}