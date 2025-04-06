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
	// Skip if we're not running full tests with database
	if os.Getenv("ETL_TEST_DB") == "" {
		t.Skip("Skipping test that requires database. Set ETL_TEST_DB=1 to enable.")
	}

	// Create a test database connection
	dbConfig := db.DefaultConfig()
	database, err := db.NewDatabase(dbConfig)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
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

// TestPipelineProcessingWithMockDB tests the pipeline with a mock database
// This test doesn't require a real database connection
func TestPipelineProcessingWithMockDB(t *testing.T) {
	// Create a mock database
	mockDB := &MockDatabase{}

	// Create pipeline with default options
	pipeline := NewPipeline(mockDB, DefaultPipelineOptions())

	// Test processing stock quotes
	t.Run("ProcessStockQuotes", func(t *testing.T) {
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

// MockDatabase is a mock implementation of the Database interface for testing
type MockDatabase struct{}

// Close implements db.Database
func (m *MockDatabase) Close() error {
	return nil
}

// ExecuteSQL implements db.Database
func (m *MockDatabase) ExecuteSQL(query string) (int64, error) {
	return 1, nil
}

// InsertQuote implements db.Database
func (m *MockDatabase) InsertQuote(ctx context.Context, quote models.StockQuote) (string, error) {
	return "mock-quote-id", nil
}

// InsertQuotesBatch implements db.Database
func (m *MockDatabase) InsertQuotesBatch(ctx context.Context, quotes []models.StockQuote, batchID string) ([]string, error) {
	ids := make([]string, len(quotes))
	for i := range quotes {
		ids[i] = "mock-quote-id-" + string(rune(i))
	}
	return ids, nil
}

// InsertIndex implements db.Database
func (m *MockDatabase) InsertIndex(ctx context.Context, index models.MarketIndex) (string, error) {
	return "mock-index-id", nil
}

// InsertIndicesBatch implements db.Database
func (m *MockDatabase) InsertIndicesBatch(ctx context.Context, indices []models.MarketIndex, batchID string) ([]string, error) {
	ids := make([]string, len(indices))
	for i := range indices {
		ids[i] = "mock-index-id-" + string(rune(i))
	}
	return ids, nil
}

// InsertBatch implements db.Database
func (m *MockDatabase) InsertBatch(ctx context.Context, batch models.DataBatch) (string, error) {
	return "mock-batch-id", nil
}

// UpdateBatch implements db.Database
func (m *MockDatabase) UpdateBatch(ctx context.Context, batchID string, status string, quoteCount, indexCount int) error {
	return nil
}

// QueryRow implements db.Database
func (m *MockDatabase) QueryRow(query string, dest ...interface{}) error {
	// Set dest to default values for testing
	for i, d := range dest {
		switch v := d.(type) {
		case *bool:
			*v = true
		case *int:
			*v = i
		case *string:
			*v = "mock-value"
		}
	}
	return nil
}

// GetLatestQuotes implements db.Database
func (m *MockDatabase) GetLatestQuotes(ctx context.Context, symbols []string, limit int) ([]models.StockQuote, error) {
	return []models.StockQuote{
		{
			ID:            "mock-quote-id",
			Symbol:        "AAPL",
			Price:         150.25,
			Change:        2.5,
			ChangePercent: 1.2,
			Volume:        12345678,
			Timestamp:     time.Now(),
			Exchange:      models.NYSE,
			Source:        models.APIScraperSource,
		},
	}, nil
}

// GetLatestIndices implements db.Database
func (m *MockDatabase) GetLatestIndices(ctx context.Context, names []string, limit int) ([]models.MarketIndex, error) {
	return []models.MarketIndex{
		{
			ID:            "mock-index-id",
			Name:          "S&P 500",
			Value:         4500.25,
			Change:        25.5,
			ChangePercent: 0.5,
			Timestamp:     time.Now(),
			Source:        models.BrowserScraperSource,
		},
	}, nil
}