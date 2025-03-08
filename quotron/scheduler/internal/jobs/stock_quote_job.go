package jobs

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

// StockQuoteJob fetches stock quotes for specified symbols
type StockQuoteJob struct {
	BaseJob
	apiKey     string
	apiScraperPath string
	outputJSON bool
}

// NewStockQuoteJob creates a new stock quote job
func NewStockQuoteJob(apiKey, apiScraperPath string, outputJSON bool) *StockQuoteJob {
	return &StockQuoteJob{
		BaseJob:    NewBaseJob("stock_quotes", "Fetch stock quotes for tracked symbols"),
		apiKey:     apiKey,
		apiScraperPath: apiScraperPath,
		outputJSON: outputJSON,
	}
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
	for _, symbol := range symbolList {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			continue
		}

		log.Printf("Fetching stock quote for %s", symbol)
		if err := j.fetchQuote(ctx, symbol); err != nil {
			errMsg := fmt.Sprintf("Error fetching quote for %s: %v", symbol, err)
			log.Print(errMsg)
			errors = append(errors, errMsg)
			
			// Check if we should abort (API key issues, rate limits, etc.)
			if strings.Contains(err.Error(), "API key") || 
			   strings.Contains(err.Error(), "rate limit") {
				return fmt.Errorf("stopping due to API issues: %w", err)
			}
			
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

// fetchQuote fetches a stock quote for a single symbol
func (j *StockQuoteJob) fetchQuote(ctx context.Context, symbol string) error {
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

	log.Printf("Successfully fetched quote for %s", symbol)
	return nil
}