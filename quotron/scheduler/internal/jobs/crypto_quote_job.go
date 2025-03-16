package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/we-be/tiny-ria/quotron/scheduler/pkg/client"
)

const (
	CryptoQuoteChannel = "quotron:crypto"
)

// CryptoQuoteJob fetches cryptocurrency quotes for specified symbols
type CryptoQuoteJob struct {
	BaseJob
	apiScraperPath  string
	outputJSON      bool
	apiHost         string
	apiPort         int
	useAPIService   bool // Whether to use the API service instead of direct execution
}

// NewCryptoQuoteJob creates a new cryptocurrency quote job
func NewCryptoQuoteJob(apiScraperPath string, outputJSON bool) *CryptoQuoteJob {
	return &CryptoQuoteJob{
		BaseJob:        NewBaseJob("crypto_quotes", "Fetch cryptocurrency quotes for tracked symbols"),
		apiScraperPath: apiScraperPath,
		outputJSON:     outputJSON,
		apiHost:        "localhost",
		apiPort:        8080,
		useAPIService:  false, // Default to legacy mode
	}
}

// WithAPIService configures the job to use the API service
func (j *CryptoQuoteJob) WithAPIService(host string, port int) *CryptoQuoteJob {
	j.useAPIService = true
	j.apiHost = host
	j.apiPort = port
	return j
}

// Execute runs the cryptocurrency quote job
func (j *CryptoQuoteJob) Execute(ctx context.Context, params map[string]string) error {
	// Get symbols from parameters
	symbols, ok := params["symbols"]
	if !ok || symbols == "" {
		return fmt.Errorf("no symbols specified")
	}

	// Track errors for reporting
	var errors []string
	
	// Split symbols and process each one
	symbolList := strings.Split(symbols, ",")
	for i, symbol := range symbolList {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			continue
		}

		// Format crypto symbol if needed (ensure BTC-USD format)
		if !strings.Contains(symbol, "-") {
			symbol = symbol + "-USD"
			log.Printf("Formatted crypto symbol to %s", symbol)
		}

		// Add a delay between requests to avoid rate limiting if using direct API access
		// No need for delay when using our API service which handles rate limiting internally
		if i > 0 && !j.useAPIService {
			// Wait 5 seconds between requests to avoid rate limiting
			log.Printf("Waiting 5 seconds before next request (API rate limiting)...")
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				// Continue after delay
			}
		}

		log.Printf("Fetching crypto quote for %s", symbol)
		
		var err error
		if j.useAPIService {
			// Use API service
			err = j.fetchQuoteFromAPI(ctx, symbol)
		} else {
			// Legacy mode - direct Yahoo Finance API scraper execution
			err = j.fetchQuoteYahoo(ctx, symbol)
		}
		
		if err != nil {
			errMsg := fmt.Sprintf("Error fetching quote for %s: %v", symbol, err)
			log.Print(errMsg)
			errors = append(errors, errMsg)
			continue // Continue with next symbol for other errors
		}
	}

	// Update last run time regardless of individual errors
	j.SetLastRun(time.Now())
	
	// If any symbols failed, report it but don't fail the whole job
	if len(errors) > 0 {
		log.Printf("Warning: %d/%d crypto quotes had errors", len(errors), len(symbolList))
	}
	
	return nil
}

// fetchQuoteFromAPI fetches crypto quote data using the API service
func (j *CryptoQuoteJob) fetchQuoteFromAPI(ctx context.Context, symbol string) error {
	// Create API client
	apiClient := client.NewAPIClient(j.apiHost, j.apiPort)
	
	// Get crypto quote from API service
	quote, err := apiClient.GetCryptoQuote(ctx, symbol)
	if err != nil {
		return fmt.Errorf("failed to fetch crypto quote from API service: %w", err)
	}
	
	// Convert to JSON for storage
	jsonData, err := json.MarshalIndent(quote, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal crypto quote: %w", err)
	}
	
	// Save to file and import to database
	outputDir := "data"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("Warning: couldn't create data directory: %v", err)
	} else {
		timestamp := time.Now().Format("20060102-150405")
		filename := fmt.Sprintf("%s/%s-yahoo-%s.json", outputDir, symbol, timestamp)
		
		if err := os.WriteFile(filename, jsonData, 0644); err != nil {
			log.Printf("Warning: couldn't save output to %s: %v", filename, err)
		} else {
			log.Printf("Saved output to %s", filename)
			
			// Import to database using the ingest pipeline
			ingestCmd := exec.CommandContext(ctx, "python3", "../ingest-pipeline/cli.py", "crypto", filename, "--source", "api-scraper", "--allow-old-data")
			ingestOutput, ingestErr := ingestCmd.CombinedOutput()
			if ingestErr != nil {
				log.Printf("Warning: couldn't import data to database: %v, output: %s", ingestErr, ingestOutput)
			} else {
				log.Printf("Imported data to database: %s", ingestOutput)
			}
		}
	}
	
	// Publish to Redis
	redisClient := client.NewRedisClient("")
	defer redisClient.Close()
	
	// Set exchange to CRYPTO
	quote.Exchange = "CRYPTO"
	
	if err := redisClient.PublishCryptoQuote(ctx, quote); err != nil {
		log.Printf("Warning: failed to publish to Redis: %v", err)
	} else {
		log.Printf("Published crypto quote for %s to Redis", symbol)
	}
	
	log.Printf("Successfully fetched crypto quote for %s via API service (%s)", symbol, quote.Source)
	return nil
}

// fetchQuoteYahoo fetches a crypto quote for a single symbol using Yahoo Finance
func (j *CryptoQuoteJob) fetchQuoteYahoo(ctx context.Context, symbol string) error {
	// Prepare command to run the API scraper with Yahoo Finance
	args := []string{"--yahoo", "--symbol", symbol}
	if j.outputJSON {
		args = append(args, "--json")
	}

	cmd := exec.CommandContext(ctx, j.apiScraperPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute API scraper with Yahoo Finance: %w, output: %s", err, output)
	}

	// Save the output to a file for analysis and database
	outputDir := "data"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("Warning: couldn't create data directory: %v", err)
	} else {
		timestamp := time.Now().Format("20060102-150405")
		filename := fmt.Sprintf("%s/%s-yahoo-%s.json", outputDir, symbol, timestamp)
		if err := os.WriteFile(filename, output, 0644); err != nil {
			log.Printf("Warning: couldn't save output to %s: %v", filename, err)
		} else {
			log.Printf("Saved output to %s", filename)
			
			// Parse the JSON to extract quote data for Redis
			var quoteData map[string]interface{}
			if err := json.Unmarshal(output, &quoteData); err != nil {
				log.Printf("Warning: couldn't parse JSON for Redis publishing: %v", err)
			} else {
				// Create StockQuote from JSON data
				cryptoQuote := &client.StockQuote{
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
					cryptoQuote.Price = price
				}
				if change, ok := quoteData["change"].(float64); ok {
					cryptoQuote.Change = change
				}
				if changePercent, ok := quoteData["changePercent"].(float64); ok {
					cryptoQuote.ChangePercent = changePercent
				}
				if volume, ok := quoteData["volume"].(float64); ok {
					cryptoQuote.Volume = int64(volume)
				}
				
				// Publish to Redis
				redisClient := client.NewRedisClient("")
				defer redisClient.Close()
				
				if err := redisClient.PublishCryptoQuote(ctx, cryptoQuote); err != nil {
					log.Printf("Warning: failed to publish to Redis: %v", err)
				} else {
					log.Printf("Published crypto quote for %s to Redis", symbol)
				}
			}
			
			// Import to database using the ingest pipeline
			ingestCmd := exec.CommandContext(ctx, "python3", "../ingest-pipeline/cli.py", "crypto", filename, "--source", "api-scraper", "--allow-old-data")
			ingestOutput, ingestErr := ingestCmd.CombinedOutput()
			if ingestErr != nil {
				log.Printf("Warning: couldn't import data to database: %v, output: %s", ingestErr, ingestOutput)
			} else {
				log.Printf("Imported data to database: %s", ingestOutput)
			}
		}
	}

	log.Printf("Successfully fetched crypto quote for %s using Yahoo Finance", symbol)
	return nil
}