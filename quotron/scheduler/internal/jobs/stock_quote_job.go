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

	// Split symbols and process each one
	symbolList := strings.Split(symbols, ",")
	for _, symbol := range symbolList {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			continue
		}

		log.Printf("Fetching stock quote for %s", symbol)
		if err := j.fetchQuote(ctx, symbol); err != nil {
			log.Printf("Error fetching quote for %s: %v", symbol, err)
			continue // Continue with next symbol even if this one fails
		}
	}

	// Update last run time
	j.SetLastRun(time.Now())
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