package pipeline

import (
	"context"
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
	p := NewPipeline(database, DefaultPipelineOptions())

	// Test quotes validation through the Pipeline's validator
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

		// Use the validator directly
		validQuotes, _ := p.validator.ValidateBatch(quotes, nil)
		
		// Check validation results
		if len(validQuotes) != 1 {
			t.Errorf("Expected 1 valid quote, but got %d", len(validQuotes))
		}
		
		// Test with invalid quote (missing symbol)
		invalidQuotes := []models.StockQuote{
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

		validQuotes, _ = p.validator.ValidateBatch(invalidQuotes, nil)
		
		// The validator removes invalid quotes
		if len(validQuotes) != 0 {
			t.Errorf("Expected 0 valid quotes for invalid input, but got %d", len(validQuotes))
		}
	})

	// Test indices validation through the pipeline's validator
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

		// Use the validator directly
		_, validIndices := p.validator.ValidateBatch(nil, indices)
		
		// Check validation results
		if len(validIndices) != 1 {
			t.Errorf("Expected 1 valid index, but got %d", len(validIndices))
		}

		// Test with invalid index (missing name)
		invalidIndices := []models.MarketIndex{
			{
				// Name is missing
				Value:         4500.25,
				Change:        25.5,
				ChangePercent: 0.5,
				Timestamp:     time.Now(),
				Source:        models.BrowserScraperSource,
			},
		}

		_, validIndices = p.validator.ValidateBatch(nil, invalidIndices)
		
		// The validator removes invalid indices
		if len(validIndices) != 0 {
			t.Errorf("Expected 0 valid indices for invalid input, but got %d", len(validIndices))
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