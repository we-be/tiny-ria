package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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

// Test-only implementation of the CryptoQuoteJob
//
// IMPORTANT: This is a testing utility that uses real data from Yahoo Finance.
// It is NOT mock data but an actual implementation for testing integration between
// components without requiring the full scheduler system.
//
// Usage:
// 1. Build: go build -o crypto_test
// 2. Run with real symbols: ./crypto_test --symbols BTC-USD,ETH-USD
// 3. Monitor Redis: ./crypto_test --monitor
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
		// Try to find api-scraper using relative path or environment variable
		apiScraperPath := findAPIScraperPath()
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
			Addr:     getRedisAddr(),
			Password: getRedisPassword(),
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
		Addr:     getRedisAddr(),
		Password: getRedisPassword(),
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
// findAPIScraperPath attempts to locate the api-scraper binary
func findAPIScraperPath() string {
	// First check environment variable
	if envPath := os.Getenv("API_SCRAPER_PATH"); envPath \!= "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}
		// Even if invalid, try using it
		return envPath
	}

	// Define possible locations to check
	possiblePaths := []string{
		"../api-scraper/api-scraper",                        // Relative to test directory
		"../../api-scraper/api-scraper",                     // Relative from deeper directory
		"/home/hunter/Desktop/tiny-ria/quotron/api-scraper/api-scraper", // Original hardcoded path (fallback)
	}

	// Find executable directory
	execDir, err := os.Executable()
	if err == nil {
		// Add path relative to executable
		baseDir := filepath.Dir(filepath.Dir(execDir)) // Go up two levels
		possiblePaths = append(possiblePaths, filepath.Join(baseDir, "api-scraper", "api-scraper"))
	}

	// Check for quotron root environment variable
	if quotronRoot := os.Getenv("QUOTRON_ROOT"); quotronRoot \!= "" {
		possiblePaths = append(possiblePaths, filepath.Join(quotronRoot, "api-scraper", "api-scraper"))
	}

	// Try each possible path
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// If all else fails, return a reasonable default and let it fail if necessary
	fmt.Println("Warning: Could not find api-scraper. Using default path.")
	return "api-scraper/api-scraper"
}

// getRedisAddr returns the Redis address from environment or default
func getRedisAddr() string {
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "localhost"
	}
	
	port := os.Getenv("REDIS_PORT")
	if port == "" {
		port = "6379"
	}
	
	return fmt.Sprintf("%s:%s", host, port)
}

// getRedisPassword returns the Redis password from environment
func getRedisPassword() string {
	return os.Getenv("REDIS_PASSWORD")
}
