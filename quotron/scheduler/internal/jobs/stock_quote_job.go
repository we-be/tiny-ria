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

// StockQuoteJob fetches stock quotes for specified symbols
type StockQuoteJob struct {
	BaseJob
	apiKey          string
	apiScraperPath  string
	outputJSON      bool
	fallbackEnabled bool  // Whether to use Yahoo Finance as fallback
	apiHost         string
	apiPort         int
	useAPIService   bool // Whether to use the API service instead of direct execution
}

// NewStockQuoteJob creates a new stock quote job
func NewStockQuoteJob(apiKey, apiScraperPath string, outputJSON bool) *StockQuoteJob {
	return &StockQuoteJob{
		BaseJob:         NewBaseJob("stock_quotes", "Fetch stock quotes for tracked symbols"),
		apiKey:          apiKey,
		apiScraperPath:  apiScraperPath,
		outputJSON:      outputJSON,
		fallbackEnabled: true, // Enable Yahoo Finance fallback by default
		apiHost:         "localhost",
		apiPort:         8080,
		useAPIService:   false, // Default to legacy mode
	}
}

// WithAPIService configures the job to use the API service
func (j *StockQuoteJob) WithAPIService(host string, port int) *StockQuoteJob {
	j.useAPIService = true
	j.apiHost = host
	j.apiPort = port
	return j
}

// Execute runs the stock quote job
func (j *StockQuoteJob) Execute(ctx context.Context, params map[string]string) error {
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

		// Add a delay between requests to avoid rate limiting if using direct API access
		// No need for delay when using our API service which handles rate limiting internally
		if i > 0 && !j.useAPIService {
			// Wait 15 seconds between requests to avoid rate limiting
			log.Printf("Waiting 15 seconds before next request (API rate limiting)...")
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(15 * time.Second):
				// Continue after delay
			}
		}

		log.Printf("Fetching stock quote for %s", symbol)
		
		var err error
		if j.useAPIService {
			// Use API service
			err = j.fetchQuoteFromAPI(ctx, symbol)
		} else {
			// Legacy mode - direct API scraper execution
			err = j.fetchQuote(ctx, symbol)
			if err != nil {
				// Try fallback if appropriate
				if (strings.Contains(err.Error(), "API key") || 
				   strings.Contains(err.Error(), "rate limit") ||
				   strings.Contains(err.Error(), "Thank you for using Alpha Vantage") ||
				   strings.Contains(err.Error(), "permission denied")) && j.fallbackEnabled {
					
					log.Printf("Alpha Vantage API issue detected, trying Yahoo Finance fallback for %s", symbol)
					if fallbackErr := j.fetchQuoteYahoo(ctx, symbol); fallbackErr != nil {
						errMsg := fmt.Sprintf("Fallback error for %s: %v", symbol, fallbackErr)
						log.Print(errMsg)
						errors = append(errors, errMsg)
						
						// If both primary and fallback failed, report but continue with next symbol
						continue
					} else {
						// Fallback succeeded, remove the error for this symbol
						log.Printf("Yahoo Finance fallback succeeded for %s", symbol)
						// Remove the last error since the fallback succeeded
						if len(errors) > 0 {
							errors = errors[:len(errors)-1]
						}
						continue
					}
				} else if strings.Contains(err.Error(), "API key") || 
				   strings.Contains(err.Error(), "rate limit") ||
				   strings.Contains(err.Error(), "Thank you for using Alpha Vantage") {
					return fmt.Errorf("stopping due to API issues: %w", err)
				}
			}
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
		log.Printf("Warning: %d/%d stock quotes had errors", len(errors), len(symbolList))
		
		// If more than 5 requests failed, it might be a rate limit issue
		if len(errors) > 5 {
			log.Printf("Note: Free Alpha Vantage API has a limit of 25 requests per day. Consider upgrading API key.")
		}
	}
	
	return nil
}

// fetchQuoteFromAPI fetches stock quote data using the API service
func (j *StockQuoteJob) fetchQuoteFromAPI(ctx context.Context, symbol string) error {
	// Create API client
	apiClient := client.NewAPIClient(j.apiHost, j.apiPort)
	
	// Get stock quote from API service
	quote, err := apiClient.GetStockQuote(ctx, symbol)
	if err != nil {
		return fmt.Errorf("failed to fetch stock quote from API service: %w", err)
	}
	
	// Map exchange value to valid enum
	quote.Exchange = client.MapExchangeToEnum(quote.Exchange)
	
	// Convert to JSON for storage
	jsonData, err := json.MarshalIndent(quote, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal stock quote: %w", err)
	}
	
	// Save to file only (no direct database import)
	outputDir := "data"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("Warning: couldn't create data directory: %v", err)
	} else {
		timestamp := time.Now().Format("20060102-150405")
		var filename string
		if strings.Contains(strings.ToLower(quote.Source), "yahoo") {
			filename = fmt.Sprintf("%s/%s-yahoo-%s.json", outputDir, symbol, timestamp)
		} else {
			filename = fmt.Sprintf("%s/%s-%s.json", outputDir, symbol, timestamp)
		}
		
		if err := os.WriteFile(filename, jsonData, 0644); err != nil {
			log.Printf("Warning: couldn't save output to %s: %v", filename, err)
		} else {
			log.Printf("Saved output to %s", filename)
			// Note: No direct database import - the ETL service will handle that
		}
	}
	
	// Publish to Redis (both PubSub and Stream for backward compatibility)
	redisClient := client.NewRedisClient("")
	defer redisClient.Close()
	
	quoteData := client.StockQuoteToQuoteData(quote)
	
	// 1. Publish to Redis PubSub channel
	if err := redisClient.PublishStockQuote(ctx, quoteData); err != nil {
		log.Printf("Warning: failed to publish to Redis PubSub: %v", err)
	} else {
		log.Printf("Published stock quote for %s to Redis PubSub channel", symbol)
	}
	
	// 2. Publish to Redis Stream
	if err := redisClient.PublishToStockStream(ctx, quoteData); err != nil {
		log.Printf("Warning: failed to publish to Redis Stream: %v", err)
	} else {
		log.Printf("Published stock quote for %s to Redis Stream", symbol)
	}
	
	log.Printf("Successfully fetched quote for %s via API service (%s)", symbol, quote.Source)
	return nil
}

// fetchQuote fetches a stock quote for a single symbol using Alpha Vantage (legacy mode)
func (j *StockQuoteJob) fetchQuote(ctx context.Context, symbol string) error {
	// Check if apiKey is "demo" and use Yahoo Finance directly
	if j.apiKey == "demo" && j.fallbackEnabled {
		return j.fetchQuoteYahoo(ctx, symbol)
	}

	// Prepare command to run the API scraper
	args := []string{"--api-key", j.apiKey, "--symbol", symbol}
	if j.outputJSON {
		args = append(args, "--json")
	}

	cmd := exec.CommandContext(ctx, j.apiScraperPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute API scraper: %w, output: %s", err, output)
	}

	// Save the output to a file (no direct database import)
	outputDir := "data"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("Warning: couldn't create data directory: %v", err)
	} else {
		timestamp := time.Now().Format("20060102-150405")
		filename := fmt.Sprintf("%s/%s-%s.json", outputDir, symbol, timestamp)
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
				stockQuote := &client.StockQuote{
					Symbol:        symbol,
					Price:         0.0,  // Will be populated from JSON
					Change:        0.0,  // Will be populated from JSON
					ChangePercent: 0.0,  // Will be populated from JSON
					Volume:        0,    // Will be populated from JSON
					Timestamp:     time.Now(),
					Exchange:      "OTHER", // Will be populated from JSON if available
					Source:        "Alpha Vantage",
				}
				
				// Extract values from JSON
				if price, ok := quoteData["price"].(float64); ok {
					stockQuote.Price = price
				}
				if change, ok := quoteData["change"].(float64); ok {
					stockQuote.Change = change
				}
				if changePercent, ok := quoteData["changePercent"].(float64); ok {
					stockQuote.ChangePercent = changePercent
				}
				if volume, ok := quoteData["volume"].(float64); ok {
					stockQuote.Volume = int64(volume)
				}
				if exchange, ok := quoteData["exchange"].(string); ok {
					stockQuote.Exchange = client.MapExchangeToEnum(exchange)
				}
				
				// Publish to Redis (both PubSub and Stream)
				redisClient := client.NewRedisClient("")
				defer redisClient.Close()
				
				quoteData := client.StockQuoteToQuoteData(stockQuote)
				
				// 1. Publish to Redis PubSub channel
				if err := redisClient.PublishStockQuote(ctx, quoteData); err != nil {
					log.Printf("Warning: failed to publish to Redis PubSub: %v", err)
				} else {
					log.Printf("Published stock quote for %s to Redis PubSub channel", symbol)
				}
				
				// 2. Publish to Redis Stream
				if err := redisClient.PublishToStockStream(ctx, quoteData); err != nil {
					log.Printf("Warning: failed to publish to Redis Stream: %v", err)
				} else {
					log.Printf("Published stock quote for %s to Redis Stream", symbol)
				}
			}
			
			// Note: No direct database import - the ETL service will handle that
		}
	}

	log.Printf("Successfully fetched quote for %s", symbol)
	return nil
}

// fetchQuoteYahoo fetches a stock quote for a single symbol using Yahoo Finance (legacy mode)
func (j *StockQuoteJob) fetchQuoteYahoo(ctx context.Context, symbol string) error {
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

	// Save the output to a file (no direct database import)
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
				stockQuote := &client.StockQuote{
					Symbol:        symbol,
					Price:         0.0,  // Will be populated from JSON
					Change:        0.0,  // Will be populated from JSON
					ChangePercent: 0.0,  // Will be populated from JSON
					Volume:        0,    // Will be populated from JSON
					Timestamp:     time.Now(),
					Exchange:      "OTHER", // Will be populated from JSON if available
					Source:        "Yahoo Finance",
				}
				
				// Extract values from JSON
				if price, ok := quoteData["price"].(float64); ok {
					stockQuote.Price = price
				}
				if change, ok := quoteData["change"].(float64); ok {
					stockQuote.Change = change
				}
				if changePercent, ok := quoteData["changePercent"].(float64); ok {
					stockQuote.ChangePercent = changePercent
				}
				if volume, ok := quoteData["volume"].(float64); ok {
					stockQuote.Volume = int64(volume)
				}
				if exchange, ok := quoteData["exchange"].(string); ok {
					stockQuote.Exchange = client.MapExchangeToEnum(exchange)
				}
				
				// Publish to Redis (both PubSub and Stream)
				redisClient := client.NewRedisClient("")
				defer redisClient.Close()
				
				quoteData := client.StockQuoteToQuoteData(stockQuote)
				
				// 1. Publish to Redis PubSub channel
				if err := redisClient.PublishStockQuote(ctx, quoteData); err != nil {
					log.Printf("Warning: failed to publish to Redis PubSub: %v", err)
				} else {
					log.Printf("Published stock quote for %s to Redis PubSub channel", symbol)
				}
				
				// 2. Publish to Redis Stream
				if err := redisClient.PublishToStockStream(ctx, quoteData); err != nil {
					log.Printf("Warning: failed to publish to Redis Stream: %v", err)
				} else {
					log.Printf("Published stock quote for %s to Redis Stream", symbol)
				}
			}
			
			// Note: No direct database import - the ETL service will handle that
		}
	}

	log.Printf("Successfully fetched quote for %s using Yahoo Finance", symbol)
	return nil
}

// fixExchangeInJSONFile updates the exchange field in a stock quote JSON file to ensure
// it uses a valid enum value for the database
func fixExchangeInJSONFile(filename string) {
	// Read the file to map the exchange
	fileData, err := os.ReadFile(filename)
	if err != nil {
		log.Printf("Warning: couldn't read file to fix exchange: %v", err)
		return
	}

	var quoteData map[string]interface{}
	if err := json.Unmarshal(fileData, &quoteData); err != nil {
		log.Printf("Warning: couldn't parse JSON to fix exchange: %v", err)
		return
	}

	// Check if exchange exists and map it
	if exchangeVal, ok := quoteData["exchange"].(string); ok {
		// Map using our exchange mapping function
		mappedExchange := client.MapExchangeToEnum(exchangeVal)
		
		// Only update if the exchange value changed
		if mappedExchange != exchangeVal {
			quoteData["exchange"] = mappedExchange
			
			// Write the updated JSON back to the file
			if updatedData, err := json.MarshalIndent(quoteData, "", "  "); err == nil {
				if err := os.WriteFile(filename, updatedData, 0644); err == nil {
					log.Printf("Mapped exchange '%s' to '%s' in %s", exchangeVal, mappedExchange, filepath.Base(filename))
				}
			}
		}
	}
}