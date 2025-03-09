package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/we-be/tiny-ria/quotron/etl/internal/db"
	"github.com/we-be/tiny-ria/quotron/etl/internal/models"
	"github.com/we-be/tiny-ria/quotron/etl/internal/pipeline"
)

// generateRandomStockQuotes generates random stock quotes for testing
func generateRandomStockQuotes(count int, r *rand.Rand) []models.StockQuote {
	symbols := []string{"AAPL", "MSFT", "GOOG", "AMZN", "META", "TSLA", "NFLX", "NVDA", "AMD", "INTC"}
	exchanges := []models.Exchange{models.NYSE, models.NASDAQ, models.AMEX}
	now := time.Now()
	
	quotes := make([]models.StockQuote, count)
	for i := 0; i < count; i++ {
		// Get random symbol from the list
		symbol := symbols[r.Intn(len(symbols))]
		
		// Generate random price between 50 and 500
		price := 50.0 + r.Float64()*450.0
		
		// Generate random change between -10 and 10
		change := -10.0 + r.Float64()*20.0
		
		// Calculate change percent
		changePercent := (change / (price - change)) * 100.0
		
		// Generate random volume between 1,000 and 10,000,000
		volume := r.Int63n(10000000) + 1000
		
		// Use a random exchange
		exchange := exchanges[r.Intn(len(exchanges))]
		
		// Create the stock quote
		quotes[i] = models.StockQuote{
			Symbol:        symbol,
			Price:         price,
			Change:        change,
			ChangePercent: changePercent,
			Volume:        volume,
			Timestamp:     now,
			Exchange:      exchange,
			Source:        models.APIScraperSource,
		}
	}
	
	return quotes
}

// generateRandomMarketIndices generates random market indices for testing
func generateRandomMarketIndices(count int, r *rand.Rand) []models.MarketIndex {
	names := []string{"S&P 500", "Dow Jones", "NASDAQ", "Russell 2000", "FTSE 100", "DAX", "Nikkei 225"}
	now := time.Now()
	
	indices := make([]models.MarketIndex, count)
	for i := 0; i < count; i++ {
		// Use a cyclic pattern for names if we have more indices than names
		name := names[i%len(names)]
		
		// Generate random value between 1,000 and 30,000
		value := 1000.0 + r.Float64()*29000.0
		
		// Generate random change between -100 and 100
		change := -100.0 + r.Float64()*200.0
		
		// Calculate change percent
		changePercent := (change / (value - change)) * 100.0
		
		// Create the market index
		indices[i] = models.MarketIndex{
			Name:          name,
			Value:         value,
			Change:        change,
			ChangePercent: changePercent,
			Timestamp:     now,
			Source:        models.APIScraperSource,
		}
	}
	
	return indices
}

// saveBenchmarkResults saves the benchmark results to a JSON file
func saveBenchmarkResults(results map[string]interface{}, filename string) error {
	// Convert results to JSON
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal benchmark results: %w", err)
	}
	
	// Write to file
	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write benchmark results: %w", err)
	}
	
	return nil
}

func main() {
	// Initialize random number generator
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	
	// Define batch sizes for testing
	batchSizes := []int{10, 100, 1000}
	
	// Create a mock database (this would be replaced with a real database in production)
	database, err := db.NewDatabase(db.DefaultConfig())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()
	
	// Create pipeline with default options
	pipelineOpts := pipeline.DefaultPipelineOptions()
	pipelineOpts.AllowHistoricalData = true
	p := pipeline.NewPipeline(database, pipelineOpts)
	
	// Store benchmark results
	results := make(map[string]interface{})
	results["timestamp"] = time.Now().Format(time.RFC3339)
	results["host"] = os.Getenv("HOSTNAME")
	results["batch_sizes"] = batchSizes
	batchResults := make([]map[string]interface{}, 0)
	
	// Run benchmarks for different batch sizes
	for _, size := range batchSizes {
		// Generate random data
		quotes := generateRandomStockQuotes(size, r)
		indices := generateRandomMarketIndices(size / 10, r) // Fewer indices than quotes
		
		// Benchmark stock quotes processing
		startTime := time.Now()
		ctx := context.Background()
		batchID, quoteIDs, err := p.ProcessStockQuotes(ctx, quotes, models.APIScraperSource)
		quotesDuration := time.Since(startTime)
		
		batchResult := map[string]interface{}{
			"batch_size":        size,
			"quotes_count":      len(quotes),
			"quotes_processed":  len(quoteIDs),
			"quotes_duration_ms": quotesDuration.Milliseconds(),
			"quotes_per_second": float64(len(quoteIDs)) / quotesDuration.Seconds(),
		}
		
		if err != nil {
			batchResult["quotes_error"] = err.Error()
		}
		
		// Benchmark market indices processing
		startTime = time.Now()
		batchID, indexIDs, err := p.ProcessMarketIndices(ctx, indices, models.APIScraperSource)
		indicesDuration := time.Since(startTime)
		
		batchResult["indices_count"] = len(indices)
		batchResult["indices_processed"] = len(indexIDs)
		batchResult["indices_duration_ms"] = indicesDuration.Milliseconds()
		batchResult["indices_per_second"] = float64(len(indexIDs)) / indicesDuration.Seconds()
		
		if err != nil {
			batchResult["indices_error"] = err.Error()
		}
		
		// Benchmark mixed batch processing
		startTime = time.Now()
		batchID, quoteIDs, indexIDs, err = p.ProcessMixedBatch(ctx, quotes, indices, models.APIScraperSource)
		mixedDuration := time.Since(startTime)
		
		batchResult["mixed_duration_ms"] = mixedDuration.Milliseconds()
		batchResult["mixed_items_per_second"] = float64(len(quoteIDs)+len(indexIDs)) / mixedDuration.Seconds()
		
		if err != nil {
			batchResult["mixed_error"] = err.Error()
		}
		
		batchResults = append(batchResults, batchResult)
		
		fmt.Printf("Batch size %d:\n", size)
		fmt.Printf("  Quotes:  %d processed in %v (%.2f quotes/sec)\n", 
			len(quoteIDs), quotesDuration, float64(len(quoteIDs))/quotesDuration.Seconds())
		fmt.Printf("  Indices: %d processed in %v (%.2f indices/sec)\n", 
			len(indexIDs), indicesDuration, float64(len(indexIDs))/indicesDuration.Seconds())
		fmt.Printf("  Mixed:   %d items processed in %v (%.2f items/sec)\n\n", 
			len(quoteIDs)+len(indexIDs), mixedDuration, float64(len(quoteIDs)+len(indexIDs))/mixedDuration.Seconds())
	}
	
	// Save results to JSON file
	results["batches"] = batchResults
	err = saveBenchmarkResults(results, "benchmark_results.json")
	if err != nil {
		log.Printf("Warning: failed to save benchmark results: %v", err)
	} else {
		fmt.Println("Benchmark results saved to benchmark_results.json")
	}
}