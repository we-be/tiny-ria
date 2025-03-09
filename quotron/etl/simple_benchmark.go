package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/we-be/tiny-ria/quotron/etl/internal/models"
)

// generateRandomQuotes generates random stock quotes for testing
func generateRandomQuotes(count int, r *rand.Rand) []models.StockQuote {
	symbols := []string{"AAPL", "MSFT", "GOOG", "AMZN", "META"}
	exchanges := []models.Exchange{models.NYSE, models.NASDAQ}
	now := time.Now()
	
	quotes := make([]models.StockQuote, count)
	for i := 0; i < count; i++ {
		symbol := symbols[r.Intn(len(symbols))]
		price := 50.0 + r.Float64()*450.0
		change := -10.0 + r.Float64()*20.0
		changePercent := (change / (price - change)) * 100.0
		volume := r.Int63n(10000000) + 1000
		exchange := exchanges[r.Intn(len(exchanges))]
		
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

// generateRandomIndices generates random market indices for testing
func generateRandomIndices(count int, r *rand.Rand) []models.MarketIndex {
	names := []string{"S&P 500", "Dow Jones", "NASDAQ"}
	now := time.Now()
	
	indices := make([]models.MarketIndex, count)
	for i := 0; i < count; i++ {
		name := names[i%len(names)]
		value := 1000.0 + r.Float64()*29000.0
		change := -100.0 + r.Float64()*200.0
		changePercent := (change / (value - change)) * 100.0
		
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

func main() {
	// Initialize random number generator
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	
	batchSizes := []int{10, 100, 1000}
	
	for _, size := range batchSizes {
		// Generate random data
		quotes := generateRandomQuotes(size, r)
		indices := generateRandomIndices(size/10, r)
		
		// Simulate processing time
		startTime := time.Now()
		
		// Simulate concurrent processing with goroutines
		done := make(chan bool)
		
		// Process quotes concurrently
		go func() {
			for i := range quotes {
				// Simulate validation and processing
				if i%10 == 0 {
					time.Sleep(time.Millisecond)
				}
			}
			done <- true
		}()
		
		// Process indices concurrently
		go func() {
			for i := range indices {
				// Simulate validation and processing
				if i%5 == 0 {
					time.Sleep(time.Millisecond)
				}
			}
			done <- true
		}()
		
		// Wait for both goroutines to complete
		<-done
		<-done
		
		duration := time.Since(startTime)
		itemsPerSecond := float64(len(quotes)+len(indices)) / duration.Seconds()
		
		fmt.Printf("Batch size %d: processed %d items in %v (%.2f items/sec)\n", 
			size, len(quotes)+len(indices), duration, itemsPerSecond)
	}
	
	log.Println("Benchmark complete")
}
