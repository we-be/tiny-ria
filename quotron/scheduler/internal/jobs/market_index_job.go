package jobs

import (
	"context"
	"fmt"
	"log"
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

	// Split indices and process each one
	indexList := strings.Split(indices, ",")
	for _, index := range indexList {
		index = strings.TrimSpace(index)
		if index == "" {
			continue
		}

		log.Printf("Fetching market data for %s", index)
		if err := j.fetchMarketData(ctx, index); err != nil {
			log.Printf("Error fetching market data for %s: %v", index, err)
			continue // Continue with next index even if this one fails
		}
	}

	// Update last run time
	j.SetLastRun(time.Now())
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

	log.Printf("Successfully fetched market data for %s", index)
	return nil
}