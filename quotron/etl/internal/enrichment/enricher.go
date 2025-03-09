package enrichment

import (
	"encoding/json"
	"log"
	"math"
	"sort"
	"sync"

	"github.com/we-be/tiny-ria/quotron/etl/internal/models"
)

// DataEnricher handles data enrichment and statistical computations
type DataEnricher struct{}

// NewDataEnricher creates a new data enricher
func NewDataEnricher() *DataEnricher {
	return &DataEnricher{}
}

// EnrichBatch adds additional information to a batch of financial data
func (e *DataEnricher) EnrichBatch(batch *models.MarketBatch) *models.MarketBatch {
	log.Printf("Enriching batch %s", batch.BatchID)
	
	// Currently just a placeholder, but in a real implementation, we would:
	// 1. Add additional data to stocks (sectors, market cap, etc.)
	// 2. Add additional data to indices
	// 3. Compute derived metrics

	return batch
}

// BatchStatistics contains statistical information about a batch of data
type BatchStatistics struct {
	MeanPrice           float64 `json:"mean_price"`
	MedianPrice         float64 `json:"median_price"`
	MeanChangePercent   float64 `json:"mean_change_percent"`
	PositiveChangeCount int     `json:"positive_change_count"`
	NegativeChangeCount int     `json:"negative_change_count"`
	UnchangedCount      int     `json:"unchanged_count"`
	TotalVolume         int64   `json:"total_volume"`
	// Additional statistics can be added here
}

// ComputeStatistics calculates statistics for a batch of financial data
func (e *DataEnricher) ComputeStatistics(batch *models.MarketBatch) (*BatchStatistics, []byte) {
	if len(batch.Quotes) == 0 {
		return nil, nil
	}

	// Use a wait group to synchronize parallel computations
	var wg sync.WaitGroup
	var stats BatchStatistics
	var pricesMutex sync.Mutex
	var volumeMutex sync.Mutex
	var countsMutex sync.Mutex

	// Data arrays for calculation
	prices := make([]float64, len(batch.Quotes))
	changePercents := make([]float64, len(batch.Quotes))

	// Extract data into arrays in parallel
	for i, quote := range batch.Quotes {
		prices[i] = quote.Price
		changePercents[i] = quote.ChangePercent
	}

	// Calculate mean price concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		var sum float64
		for _, price := range prices {
			sum += price
		}
		pricesMutex.Lock()
		stats.MeanPrice = sum / float64(len(prices))
		pricesMutex.Unlock()
	}()

	// Calculate median price concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Make a copy to avoid modifying the original array
		pricesCopy := make([]float64, len(prices))
		copy(pricesCopy, prices)
		sort.Float64s(pricesCopy)
		
		pricesMutex.Lock()
		if len(pricesCopy) % 2 == 0 {
			// Even number of elements
			mid := len(pricesCopy) / 2
			stats.MedianPrice = (pricesCopy[mid-1] + pricesCopy[mid]) / 2
		} else {
			// Odd number of elements
			stats.MedianPrice = pricesCopy[len(pricesCopy)/2]
		}
		pricesMutex.Unlock()
	}()

	// Calculate mean change percent concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		var sum float64
		for _, changePercent := range changePercents {
			sum += changePercent
		}
		pricesMutex.Lock()
		stats.MeanChangePercent = sum / float64(len(changePercents))
		pricesMutex.Unlock()
	}()

	// Calculate total volume and count changes concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		var positive, negative, unchanged int
		var totalVolume int64

		for _, quote := range batch.Quotes {
			volumeMutex.Lock()
			totalVolume += quote.Volume
			volumeMutex.Unlock()

			countsMutex.Lock()
			if quote.Change > 0 {
				positive++
			} else if quote.Change < 0 {
				negative++
			} else {
				unchanged++
			}
			countsMutex.Unlock()
		}

		volumeMutex.Lock()
		countsMutex.Lock()
		stats.TotalVolume = totalVolume
		stats.PositiveChangeCount = positive
		stats.NegativeChangeCount = negative
		stats.UnchangedCount = unchanged
		countsMutex.Unlock()
		volumeMutex.Unlock()
	}()

	// Wait for all calculations to complete
	wg.Wait()

	// Fix NaN values if any
	if math.IsNaN(stats.MeanPrice) {
		stats.MeanPrice = 0
	}
	if math.IsNaN(stats.MedianPrice) {
		stats.MedianPrice = 0
	}
	if math.IsNaN(stats.MeanChangePercent) {
		stats.MeanChangePercent = 0
	}

	// Convert statistics to JSON
	statsJSON, err := json.Marshal(stats)
	if err != nil {
		log.Printf("Error marshaling statistics: %v", err)
		return &stats, nil
	}

	log.Printf("Computed statistics for batch %s", batch.BatchID)
	return &stats, statsJSON
}