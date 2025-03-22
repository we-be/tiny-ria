package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	YahooAPIScraperPath = "/home/hunter/Desktop/tiny-ria/quotron/api-scraper/api-scraper"
	CryptoStream        = "quotron:crypto:stream" // Redis stream name
	StreamMaxLen        = 1000                   // Maximum number of messages to keep in the stream
)

// CryptoQuote represents a cryptocurrency quote
type CryptoQuote struct {
	Symbol        string    `json:"symbol"`
	Price         float64   `json:"price"`
	Change        float64   `json:"change"`
	ChangePercent float64   `json:"changePercent"`
	Volume        int64     `json:"volume"`
	Timestamp     time.Time `json:"timestamp"`
	Exchange      string    `json:"exchange"` // Always "CRYPTO" for cryptocurrencies
	Source        string    `json:"source"`   // Source of the data (e.g., "yahoo-finance")
}

// CryptoQuoteJob is a job that fetches cryptocurrency quotes
type CryptoQuoteJob struct {
	apiScraperPath string
	useAPIService  bool
	apiHost        string
	apiPort        int
}

// NewCryptoQuoteJob creates a new CryptoQuoteJob
func NewCryptoQuoteJob(apiScraperPath string, useRedisOnly bool) *CryptoQuoteJob {
	// Use provided path or fall back to env var or default
	if apiScraperPath == "" {
		apiScraperPath = os.Getenv("QUOTRON_API_SCRAPER_PATH")
		if apiScraperPath == "" {
			apiScraperPath = YahooAPIScraperPath
		}
	}

	return &CryptoQuoteJob{
		apiScraperPath: apiScraperPath,
		useAPIService:  false,
	}
}

// WithAPIService configures the job to use the API service instead of direct API
func (j *CryptoQuoteJob) WithAPIService(host string, port int) *CryptoQuoteJob {
	j.useAPIService = true
	j.apiHost = host
	j.apiPort = port
	return j
}

// Execute runs the job with the given parameters
func (j *CryptoQuoteJob) Execute(ctx context.Context, params map[string]string) error {
	// Get symbols from parameters
	symbolsParam, ok := params["symbols"]
	if !ok || symbolsParam == "" {
		return fmt.Errorf("symbols parameter is required")
	}

	// Split multiple symbols
	symbols := strings.Split(symbolsParam, ",")
	
	// Process each symbol
	log.Printf("Fetching quotes for %d symbols: %v", len(symbols), symbols)
	
	for _, symbol := range symbols {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			continue
		}
		
		// Make API call for this symbol
		var err error
		if j.useAPIService {
			err = j.fetchQuoteFromAPI(ctx, symbol)
		} else {
			err = j.fetchQuoteYahoo(ctx, symbol)
		}
		
		if err != nil {
			log.Printf("Error fetching quote for %s: %v", symbol, err)
			continue
		}
		
		log.Printf("Successfully fetched and published quote for %s", symbol)
	}
	
	return nil
}

// fetchQuoteFromAPI fetches a quote from the API service
func (j *CryptoQuoteJob) fetchQuoteFromAPI(ctx context.Context, symbol string) error {
	log.Printf("Fetching quote for %s from API service at %s:%d", symbol, j.apiHost, j.apiPort)
	
	// Generate temporary file for output
	tempDir := os.TempDir()
	timestamp := time.Now().Format("20060102-150405")
	filename := filepath.Join(tempDir, fmt.Sprintf("%s-api-%s.json", symbol, timestamp))
	
	// Create API service command
	apiCmd := fmt.Sprintf("http://%s:%d/api/v1/quotes/crypto?symbol=%s", 
		j.apiHost, j.apiPort, symbol)
	
	cmd := exec.CommandContext(ctx, "curl", "-s", apiCmd, "-o", filename)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("API service error: %v, output: %s", err, output)
	}
	
	// Read the result file
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("error reading API result: %v", err)
	}
	
	// Parse JSON
	var quote CryptoQuote
	err = json.Unmarshal(data, &quote)
	if err != nil {
		return fmt.Errorf("error parsing API result: %v", err)
	}
	
	// Set source and exchange
	quote.Source = "api-service"
	quote.Exchange = "CRYPTO"
	
	// Publish to Redis Stream
	err = j.publishToRedisStream(ctx, &quote)
	if err != nil {
		return fmt.Errorf("error publishing to Redis Stream: %v", err)
	}
	
	// Cleanup
	os.Remove(filename)
	return nil
}

// fetchQuoteYahoo fetches a quote directly from Yahoo Finance
func (j *CryptoQuoteJob) fetchQuoteYahoo(ctx context.Context, symbol string) error {
	log.Printf("Fetching quote for %s directly from Yahoo Finance", symbol)
	
	// Generate temporary file for output
	tempDir := os.TempDir()
	timestamp := time.Now().Format("20060102-150405")
	filename := filepath.Join(tempDir, fmt.Sprintf("%s-yahoo-%s.json", symbol, timestamp))
	
	// Call API scraper with bash to handle output redirection
	cmd := exec.CommandContext(ctx, "bash", "-c", 
		fmt.Sprintf("%s -yahoo -symbol=%s -json > %s", 
			j.apiScraperPath, symbol, filename))
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("API scraper error: %v, output: %s", err, output)
	}
	
	// Read the result file
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("error reading API result: %v", err)
	}
	
	// Parse JSON
	var quote CryptoQuote
	err = json.Unmarshal(data, &quote)
	if err != nil {
		return fmt.Errorf("error parsing API result: %v", err)
	}
	
	// Set source and exchange
	quote.Source = "yahoo-finance"
	quote.Exchange = "CRYPTO"
	
	// Publish to Redis Stream
	err = j.publishToRedisStream(ctx, &quote)
	if err != nil {
		return fmt.Errorf("error publishing to Redis Stream: %v", err)
	}
	
	// Cleanup
	os.Remove(filename)
	return nil
}

// publishToRedisStream publishes a quote to Redis Stream
func (j *CryptoQuoteJob) publishToRedisStream(ctx context.Context, quote *CryptoQuote) error {
	// Connect to Redis
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()
	
	// Convert quote to JSON
	data, err := json.Marshal(quote)
	if err != nil {
		return fmt.Errorf("error marshaling quote: %v", err)
	}
	
	// Publish to Redis Stream
	result, err := client.XAdd(ctx, &redis.XAddArgs{
		Stream: CryptoStream,
		ID:     "*", // Auto-generate ID
		Values: map[string]interface{}{
			"data": string(data),
		},
		MaxLen: StreamMaxLen,
	}).Result()
	
	if err != nil {
		return fmt.Errorf("error publishing to Redis Stream: %v", err)
	}
	log.Printf("Published to Redis Stream %s with ID %s", CryptoStream, result)
	
	return nil
}

// Main function - this can be called directly or used by the scheduler
func main() {
	// Parse command-line arguments
	var symbols string
	var useAPI bool
	var apiHost string
	var apiPort int

	// Process command-line flags
	for i, arg := range os.Args {
		if arg == "-symbols" && i+1 < len(os.Args) {
			symbols = os.Args[i+1]
		} else if strings.HasPrefix(arg, "-symbols=") {
			symbols = strings.TrimPrefix(arg, "-symbols=")
		} else if arg == "-api" {
			useAPI = true
		} else if strings.HasPrefix(arg, "-apihost=") {
			apiHost = strings.TrimPrefix(arg, "-apihost=")
		} else if strings.HasPrefix(arg, "-apiport=") {
			portStr := strings.TrimPrefix(arg, "-apiport=")
			port, err := strconv.Atoi(portStr)
			if err == nil {
				apiPort = port
			}
		}
	}

	// Default symbols if not provided
	if symbols == "" {
		symbols = "BTC-USD,ETH-USD"
		log.Printf("No symbols specified, using defaults: %s", symbols)
	}

	// Create the job
	job := NewCryptoQuoteJob("", false)
	
	// Configure to use API service if requested
	if useAPI {
		if apiHost == "" {
			apiHost = "localhost"
		}
		if apiPort == 0 {
			apiPort = 8080
		}
		job.WithAPIService(apiHost, apiPort)
	}

	// Execute the job
	ctx := context.Background()
	params := map[string]string{
		"symbols": symbols,
	}
	
	log.Printf("Executing crypto quote job for symbols: %s", symbols)
	err := job.Execute(ctx, params)
	if err != nil {
		log.Fatalf("Job execution failed: %v", err)
	}
	
	log.Println("Job completed successfully")
}