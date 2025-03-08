package jobs

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// MarketIndexJob fetches market index data
type MarketIndexJob struct {
	BaseJob
	apiKey         string
	apiScraperPath string
	outputJSON     bool
}

// NewMarketIndexJob creates a new market index job
func NewMarketIndexJob(apiKey, apiScraperPath string, outputJSON bool) *MarketIndexJob {
	return &MarketIndexJob{
		BaseJob:        NewBaseJob("market_indices", "Fetch market indices data"),
		apiKey:         apiKey,
		apiScraperPath: apiScraperPath,
		outputJSON:     outputJSON,
	}
}

// Execute runs the market index job
func (j *MarketIndexJob) Execute(ctx context.Context, params map[string]string) error {
	// Get indices from parameters
	indices, ok := params["indices"]
	if !ok || indices == "" {
		return fmt.Errorf("no indices specified")
	}

	// Track errors for reporting
	var errors []string
	
	// Split indices and process each one
	indexList := strings.Split(indices, ",")
	for i, index := range indexList {
		index = strings.TrimSpace(index)
		if index == "" {
			continue
		}

		// Add a delay between requests to avoid rate limiting
		// Alpha Vantage free tier allows 5 requests per minute
		if i > 0 {
			// Wait 15 seconds between requests to avoid rate limiting
			log.Printf("Waiting 15 seconds before next request (API rate limiting)...")
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(15 * time.Second):
				// Continue after delay
			}
		}

		log.Printf("Fetching market data for %s", index)
		if err := j.fetchMarketData(ctx, index); err != nil {
			errMsg := fmt.Sprintf("Error fetching market data for %s: %v", index, err)
			log.Print(errMsg)
			errors = append(errors, errMsg)
			
			// Check if we should abort (API key issues, rate limits, etc.)
			if strings.Contains(err.Error(), "API key") || 
			   strings.Contains(err.Error(), "rate limit") ||
			   strings.Contains(err.Error(), "Thank you for using Alpha Vantage") {
				return fmt.Errorf("stopping due to API issues: %w", err)
			}
			
			continue // Continue with next index for other errors
		}
	}

	// Update last run time regardless of individual errors
	j.SetLastRun(time.Now())
	
	// If any indices failed, report it but don't fail the whole job
	if len(errors) > 0 {
		log.Printf("Warning: %d/%d market indices had errors", len(errors), len(indexList))
		
		// Note about Alpha Vantage limitations
		if len(errors) == len(indexList) {
			log.Printf("Note: Free Alpha Vantage API has limitations on market indices. Consider upgrading API key.")
		}
	}
	
	return nil
}

// fetchMarketData fetches data for a single market index
func (j *MarketIndexJob) fetchMarketData(ctx context.Context, index string) error {
	// Prepare command to run the API scraper
	args := []string{"--api-key", j.apiKey, "--symbol", index}
	if j.outputJSON {
		args = append(args, "--json")
	}

	cmd := exec.CommandContext(ctx, j.apiScraperPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute API scraper: %w, output: %s", err, output)
	}

	// Save the output to a file for analysis
	outputDir := "data"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("Warning: couldn't create data directory: %v", err)
	} else {
		timestamp := time.Now().Format("20060102-150405")
		filename := fmt.Sprintf("%s/%s-%s.json", outputDir, index, timestamp)
		if err := os.WriteFile(filename, output, 0644); err != nil {
			log.Printf("Warning: couldn't save output to %s: %v", filename, err)
		} else {
			log.Printf("Saved output to %s", filename)
		}
	}

	log.Printf("Successfully fetched market data for %s", index)
	return nil
}