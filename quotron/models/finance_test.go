package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestStockQuoteSerialization(t *testing.T) {
	// Create a stock quote
	now := time.Now().UTC().Truncate(time.Millisecond)
	quote := StockQuote{
		ID:            "test-id",
		Symbol:        "AAPL",
		Price:         150.25,
		Change:        1.25,
		ChangePercent: 0.84,
		Volume:        10000000,
		Timestamp:     now,
		Exchange:      NASDAQ,
		Source:        APIScraperSource,
		CreatedAt:     now,
		BatchID:       "batch123",
	}

	// Marshal to JSON
	data, err := json.Marshal(quote)
	if err != nil {
		t.Fatalf("Failed to marshal stock quote: %v", err)
	}

	// Unmarshal back to a stock quote
	var newQuote StockQuote
	err = json.Unmarshal(data, &newQuote)
	if err != nil {
		t.Fatalf("Failed to unmarshal stock quote: %v", err)
	}

	// Compare fields
	if newQuote.ID != quote.ID {
		t.Errorf("ID mismatch: got %s, want %s", newQuote.ID, quote.ID)
	}
	if newQuote.Symbol != quote.Symbol {
		t.Errorf("Symbol mismatch: got %s, want %s", newQuote.Symbol, quote.Symbol)
	}
	if newQuote.Price != quote.Price {
		t.Errorf("Price mismatch: got %f, want %f", newQuote.Price, quote.Price)
	}
	if newQuote.Change != quote.Change {
		t.Errorf("Change mismatch: got %f, want %f", newQuote.Change, quote.Change)
	}
	if newQuote.ChangePercent != quote.ChangePercent {
		t.Errorf("ChangePercent mismatch: got %f, want %f", newQuote.ChangePercent, quote.ChangePercent)
	}
	if newQuote.Volume != quote.Volume {
		t.Errorf("Volume mismatch: got %d, want %d", newQuote.Volume, quote.Volume)
	}
	if !newQuote.Timestamp.Equal(quote.Timestamp) {
		t.Errorf("Timestamp mismatch: got %v, want %v", newQuote.Timestamp, quote.Timestamp)
	}
	if newQuote.Exchange != quote.Exchange {
		t.Errorf("Exchange mismatch: got %s, want %s", newQuote.Exchange, quote.Exchange)
	}
	if newQuote.Source != quote.Source {
		t.Errorf("Source mismatch: got %s, want %s", newQuote.Source, quote.Source)
	}
	if !newQuote.CreatedAt.Equal(quote.CreatedAt) {
		t.Errorf("CreatedAt mismatch: got %v, want %v", newQuote.CreatedAt, quote.CreatedAt)
	}
	if newQuote.BatchID != quote.BatchID {
		t.Errorf("BatchID mismatch: got %s, want %s", newQuote.BatchID, quote.BatchID)
	}
}

func TestMarketIndexSerialization(t *testing.T) {
	// Create a market index
	now := time.Now().UTC().Truncate(time.Millisecond)
	index := MarketIndex{
		ID:            "test-id",
		Name:          "S&P 500",
		Symbol:        "^GSPC",
		Value:         4500.50,
		Change:        15.75,
		ChangePercent: 0.35,
		Timestamp:     now,
		Source:        APIScraperSource,
		CreatedAt:     now,
		BatchID:       "batch123",
	}

	// Marshal to JSON
	data, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("Failed to marshal market index: %v", err)
	}

	// Unmarshal back to a market index
	var newIndex MarketIndex
	err = json.Unmarshal(data, &newIndex)
	if err != nil {
		t.Fatalf("Failed to unmarshal market index: %v", err)
	}

	// Compare fields
	if newIndex.ID != index.ID {
		t.Errorf("ID mismatch: got %s, want %s", newIndex.ID, index.ID)
	}
	if newIndex.Name != index.Name {
		t.Errorf("Name mismatch: got %s, want %s", newIndex.Name, index.Name)
	}
	if newIndex.Symbol != index.Symbol {
		t.Errorf("Symbol mismatch: got %s, want %s", newIndex.Symbol, index.Symbol)
	}
	if newIndex.Value != index.Value {
		t.Errorf("Value mismatch: got %f, want %f", newIndex.Value, index.Value)
	}
	if newIndex.Change != index.Change {
		t.Errorf("Change mismatch: got %f, want %f", newIndex.Change, index.Change)
	}
	if newIndex.ChangePercent != index.ChangePercent {
		t.Errorf("ChangePercent mismatch: got %f, want %f", newIndex.ChangePercent, index.ChangePercent)
	}
	if !newIndex.Timestamp.Equal(index.Timestamp) {
		t.Errorf("Timestamp mismatch: got %v, want %v", newIndex.Timestamp, index.Timestamp)
	}
	if newIndex.Source != index.Source {
		t.Errorf("Source mismatch: got %s, want %s", newIndex.Source, index.Source)
	}
	if !newIndex.CreatedAt.Equal(index.CreatedAt) {
		t.Errorf("CreatedAt mismatch: got %v, want %v", newIndex.CreatedAt, index.CreatedAt)
	}
	if newIndex.BatchID != index.BatchID {
		t.Errorf("BatchID mismatch: got %s, want %s", newIndex.BatchID, index.BatchID)
	}
}

func TestMarketBatchSerialization(t *testing.T) {
	// Create a market batch
	now := time.Now().UTC().Truncate(time.Millisecond)
	batch := MarketBatch{
		Quotes: []StockQuote{
			{
				Symbol:        "AAPL",
				Price:         150.25,
				Change:        1.25,
				ChangePercent: 0.84,
				Volume:        10000000,
				Timestamp:     now,
				Exchange:      NASDAQ,
				Source:        APIScraperSource,
			},
			{
				Symbol:        "MSFT",
				Price:         300.50,
				Change:        -2.50,
				ChangePercent: -0.83,
				Volume:        8000000,
				Timestamp:     now,
				Exchange:      NASDAQ,
				Source:        APIScraperSource,
			},
		},
		Indices: []MarketIndex{
			{
				Name:          "S&P 500",
				Symbol:        "^GSPC",
				Value:         4500.50,
				Change:        15.75,
				ChangePercent: 0.35,
				Timestamp:     now,
				Source:        APIScraperSource,
			},
		},
		BatchID:   "batch123",
		CreatedAt: now,
	}

	// Marshal to JSON
	data, err := json.Marshal(batch)
	if err != nil {
		t.Fatalf("Failed to marshal market batch: %v", err)
	}

	// Unmarshal back to a market batch
	var newBatch MarketBatch
	err = json.Unmarshal(data, &newBatch)
	if err != nil {
		t.Fatalf("Failed to unmarshal market batch: %v", err)
	}

	// Compare fields
	if len(newBatch.Quotes) != len(batch.Quotes) {
		t.Errorf("Quotes length mismatch: got %d, want %d", len(newBatch.Quotes), len(batch.Quotes))
	} else {
		for i, quote := range batch.Quotes {
			if newBatch.Quotes[i].Symbol != quote.Symbol {
				t.Errorf("Quote %d Symbol mismatch: got %s, want %s", i, newBatch.Quotes[i].Symbol, quote.Symbol)
			}
		}
	}

	if len(newBatch.Indices) != len(batch.Indices) {
		t.Errorf("Indices length mismatch: got %d, want %d", len(newBatch.Indices), len(batch.Indices))
	} else {
		for i, index := range batch.Indices {
			if newBatch.Indices[i].Name != index.Name {
				t.Errorf("Index %d Name mismatch: got %s, want %s", i, newBatch.Indices[i].Name, index.Name)
			}
		}
	}

	if newBatch.BatchID != batch.BatchID {
		t.Errorf("BatchID mismatch: got %s, want %s", newBatch.BatchID, batch.BatchID)
	}

	if !newBatch.CreatedAt.Equal(batch.CreatedAt) {
		t.Errorf("CreatedAt mismatch: got %v, want %v", newBatch.CreatedAt, batch.CreatedAt)
	}
}