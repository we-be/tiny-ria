package pipeline

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/we-be/tiny-ria/quotron/etl/internal/db"
	"github.com/we-be/tiny-ria/quotron/etl/internal/models"
)

// TestPipelineValidation tests the pipeline's validation functionality
func TestPipelineValidation(t *testing.T) {
	// Database connection is required for all ETL tests
	// ETL's entire purpose is database operations
	// ETL should only be used through the CLI, there is no standalone ETL binary

	// Create a test database connection
	dbConfig := db.DefaultConfig()
	database, err := db.NewDatabase(dbConfig)
	if err != nil {
		t.Fatalf("Database connection required for ETL tests: %v", err)
	}
	defer database.Close()

	// Create pipeline with default options
	pipeline := NewPipeline(database, DefaultPipelineOptions())

	// Test quotes validation
	t.Run("ValidateQuotes", func(t *testing.T) {
		// Create test quotes
		quotes := []models.StockQuote{
			{
				Symbol:        "AAPL",
				Price:         150.25,
				Change:        2.5,
				ChangePercent: 1.2,
				Volume:        12345678,
				Timestamp:     time.Now(),
				Exchange:      models.NYSE,
				Source:        models.APIScraperSource,
			},
		}

		// Validate quotes
		valid, invalidQuotes, err := pipeline.ValidateQuotes(quotes)
		if err != nil {
			t.Errorf("Error validating quotes: %v", err)
		}
		if !valid {
			t.Errorf("Expected quotes to be valid, but got invalid")
		}
		if len(invalidQuotes) > 0 {
			t.Errorf("Expected 0 invalid quotes, but got %d", len(invalidQuotes))
		}

		// Test with invalid quote (missing symbol)
		invalidQuotes = []models.StockQuote{
			{
				// Symbol is missing
				Price:         150.25,
				Change:        2.5,
				ChangePercent: 1.2,
				Volume:        12345678,
				Timestamp:     time.Now(),
				Exchange:      models.NYSE,
				Source:        models.APIScraperSource,
			},
		}

		valid, invalidItems, err := pipeline.ValidateQuotes(invalidQuotes)
		if err != nil {
			t.Errorf("Error validating quotes: %v", err)
		}
		if valid {
			t.Errorf("Expected quotes to be invalid, but got valid")
		}
		if len(invalidItems) != 1 {
			t.Errorf("Expected 1 invalid quote, but got %d", len(invalidItems))
		}
	})

	// Test indices validation
	t.Run("ValidateIndices", func(t *testing.T) {
		// Create test indices
		indices := []models.MarketIndex{
			{
				Name:          "S&P 500",
				Value:         4500.25,
				Change:        25.5,
				ChangePercent: 0.5,
				Timestamp:     time.Now(),
				Source:        models.BrowserScraperSource,
			},
		}

		// Validate indices
		valid, invalidIndices, err := pipeline.ValidateIndices(indices)
		if err != nil {
			t.Errorf("Error validating indices: %v", err)
		}
		if !valid {
			t.Errorf("Expected indices to be valid, but got invalid")
		}
		if len(invalidIndices) > 0 {
			t.Errorf("Expected 0 invalid indices, but got %d", len(invalidIndices))
		}

		// Test with invalid index (missing name)
		invalidIndices = []models.MarketIndex{
			{
				// Name is missing
				Value:         4500.25,
				Change:        25.5,
				ChangePercent: 0.5,
				Timestamp:     time.Now(),
				Source:        models.BrowserScraperSource,
			},
		}

		valid, invalidItems, err := pipeline.ValidateIndices(invalidIndices)
		if err != nil {
			t.Errorf("Error validating indices: %v", err)
		}
		if valid {
			t.Errorf("Expected indices to be invalid, but got valid")
		}
		if len(invalidItems) != 1 {
			t.Errorf("Expected 1 invalid index, but got %d", len(invalidItems))
		}
	})
}

// TestPipelineProcessing tests the pipeline with a real database connection
func TestPipelineProcessing(t *testing.T) {
	// Create a real database connection
	dbConfig := db.DefaultConfig()
	database, err := db.NewDatabase(dbConfig)
	if err != nil {
		t.Fatalf("Database connection required for ETL tests: %v", err)
	}
	defer database.Close()

	// Create pipeline with default options
	pipeline := NewPipeline(database, DefaultPipelineOptions())

	// Test processing stock quotes
	t.Run("ProcessStockQuotes", func(t *testing.T) {
		// Create test quotes
		quotes := []models.StockQuote{
			{
				Symbol:        "TEST_QUOTE",
				Price:         150.25,
				Change:        2.5,
				ChangePercent: 1.2,
				Volume:        12345678,
				Timestamp:     time.Now(),
				Exchange:      models.NYSE,
				Source:        models.APIScraperSource,
			},
		}

		// Process quotes
		batchID, quoteIDs, err := pipeline.ProcessStockQuotes(context.Background(), quotes, models.APIScraperSource)
		if err != nil {
			t.Errorf("Error processing quotes: %v", err)
		}
		if batchID == "" {
			t.Errorf("Expected batch ID, but got empty string")
		}
		if len(quoteIDs) != 1 {
			t.Errorf("Expected 1 quote ID, but got %d", len(quoteIDs))
		}
	})
}