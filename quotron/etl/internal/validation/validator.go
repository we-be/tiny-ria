package validation

import (
	"log"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/we-be/tiny-ria/quotron/etl/internal/models"
)

// DataValidator handles validation of financial data
type DataValidator struct {
	validate        *validator.Validate
	priceMax        float64
	priceMin        float64
	volumeMax       int64
	minValidTime    time.Time
	allowHistorical bool
}

// NewDataValidator creates a new validator for financial data
func NewDataValidator(allowHistorical bool) *DataValidator {
	var minTime time.Time
	if allowHistorical {
		// If historical data is allowed, use a date far in the past
		minTime = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	} else {
		// Otherwise, only accept data from the last day
		minTime = time.Now().UTC().Add(-24 * time.Hour)
	}

	return &DataValidator{
		validate:        validator.New(),
		priceMax:        100000.0,  // Max reasonable stock price
		priceMin:        0.0001,    // Min reasonable stock price
		volumeMax:       10000000000, // Max reasonable volume (10B)
		minValidTime:    minTime,
		allowHistorical: allowHistorical,
	}
}

// ValidateStockQuote validates a stock quote and returns an error if invalid
func (v *DataValidator) ValidateStockQuote(quote *models.StockQuote) error {
	// Use struct validation tags
	if err := v.validate.Struct(quote); err != nil {
		log.Printf("Validation failed for %s: %v", quote.Symbol, err)
		return err
	}

	// Additional validation
	if !v.isPriceReasonable(quote.Price) {
		log.Printf("Unreasonable price for %s: %f", quote.Symbol, quote.Price)
		return ErrUnreasonablePrice
	}

	if !v.isVolumeReasonable(quote.Volume) {
		log.Printf("Unreasonable volume for %s: %d", quote.Symbol, quote.Volume)
		return ErrUnreasonableVolume
	}

	if !v.isTimestampRecent(quote.Timestamp) {
		log.Printf("Outdated timestamp for %s: %v", quote.Symbol, quote.Timestamp)
		return ErrOutdatedTimestamp
	}

	log.Printf("Quote for %s passed validation", quote.Symbol)
	return nil
}

// ValidateMarketIndex validates a market index and returns an error if invalid
func (v *DataValidator) ValidateMarketIndex(index *models.MarketIndex) error {
	// Use struct validation tags
	if err := v.validate.Struct(index); err != nil {
		log.Printf("Validation failed for %s: %v", index.Name, err)
		return err
	}

	// Additional validation
	if !v.isTimestampRecent(index.Timestamp) {
		log.Printf("Outdated timestamp for %s: %v", index.Name, index.Timestamp)
		return ErrOutdatedTimestamp
	}

	log.Printf("Index %s passed validation", index.Name)
	return nil
}

// ValidateBatch validates a batch of quotes and indices
func (v *DataValidator) ValidateBatch(quotes []models.StockQuote, indices []models.MarketIndex) ([]models.StockQuote, []models.MarketIndex) {
	validQuotes := make([]models.StockQuote, 0, len(quotes))
	validIndices := make([]models.MarketIndex, 0, len(indices))

	// Process quotes concurrently
	quoteCh := make(chan models.StockQuote, len(quotes))
	for i := range quotes {
		go func(q models.StockQuote) {
			if err := v.ValidateStockQuote(&q); err == nil {
				quoteCh <- q
			} else {
				quoteCh <- models.StockQuote{} // Send empty quote on error
			}
		}(quotes[i])
	}

	// Collect valid quotes
	for i := 0; i < len(quotes); i++ {
		quote := <-quoteCh
		if quote.Symbol != "" { // If not empty, it's valid
			validQuotes = append(validQuotes, quote)
		}
	}

	// Process indices concurrently
	indexCh := make(chan models.MarketIndex, len(indices))
	for i := range indices {
		go func(idx models.MarketIndex) {
			if err := v.ValidateMarketIndex(&idx); err == nil {
				indexCh <- idx
			} else {
				indexCh <- models.MarketIndex{} // Send empty index on error
			}
		}(indices[i])
	}

	// Collect valid indices
	for i := 0; i < len(indices); i++ {
		index := <-indexCh
		if index.Name != "" { // If not empty, it's valid
			validIndices = append(validIndices, index)
		}
	}

	return validQuotes, validIndices
}

// Helper methods
func (v *DataValidator) isPriceReasonable(price float64) bool {
	return price >= v.priceMin && price <= v.priceMax
}

func (v *DataValidator) isVolumeReasonable(volume int64) bool {
	return volume >= 0 && volume <= v.volumeMax
}

func (v *DataValidator) isTimestampRecent(timestamp time.Time) bool {
	return timestamp.After(v.minValidTime) || timestamp.Equal(v.minValidTime)
}