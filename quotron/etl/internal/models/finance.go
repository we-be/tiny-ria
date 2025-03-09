package models

import (
	"time"
)

// DataSource represents the source of financial data
type DataSource string

const (
	APIScraperSource     DataSource = "api-scraper"
	BrowserScraperSource DataSource = "browser-scraper"
	ManualSource         DataSource = "manual"
)

// Exchange represents a stock exchange
type Exchange string

const (
	NYSE    Exchange = "NYSE"
	NASDAQ  Exchange = "NASDAQ"
	AMEX    Exchange = "AMEX"
	OTC     Exchange = "OTC"
	OTHER   Exchange = "OTHER"
)

// StockQuote represents a single stock quote
type StockQuote struct {
	ID            string     `json:"id,omitempty" db:"id"`
	Symbol        string     `json:"symbol" db:"symbol" validate:"required"`
	Price         float64    `json:"price" db:"price" validate:"required,gt=0"`
	Change        float64    `json:"change" db:"change" validate:"required"`
	ChangePercent float64    `json:"change_percent" db:"change_percent" validate:"required"`
	Volume        int64      `json:"volume" db:"volume" validate:"required,gte=0"`
	Timestamp     time.Time  `json:"timestamp" db:"timestamp" validate:"required"`
	Exchange      Exchange   `json:"exchange" db:"exchange" validate:"required"`
	Source        DataSource `json:"source" db:"source" validate:"required"`
	CreatedAt     time.Time  `json:"created_at,omitempty" db:"created_at"`
	BatchID       string     `json:"batch_id,omitempty" db:"batch_id"`
}

// MarketIndex represents a market index like S&P 500, NASDAQ, etc.
type MarketIndex struct {
	ID            string     `json:"id,omitempty" db:"id"`
	Name          string     `json:"name" db:"name" validate:"required"`
	Value         float64    `json:"value" db:"value" validate:"required,gt=0"`
	Change        float64    `json:"change" db:"change" validate:"required"`
	ChangePercent float64    `json:"change_percent" db:"change_percent" validate:"required"`
	Timestamp     time.Time  `json:"timestamp" db:"timestamp" validate:"required"`
	Source        DataSource `json:"source" db:"source" validate:"required"`
	CreatedAt     time.Time  `json:"created_at,omitempty" db:"created_at"`
	BatchID       string     `json:"batch_id,omitempty" db:"batch_id"`
}

// DataBatch represents a batch of financial data
type DataBatch struct {
	ID          string     `json:"id" db:"id" validate:"required"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty" db:"processed_at"`
	Status      string     `json:"status" db:"status" validate:"required"`
	QuoteCount  int        `json:"quote_count" db:"quote_count"`
	IndexCount  int        `json:"index_count" db:"index_count"`
	Source      DataSource `json:"source" db:"source" validate:"required"`
	Metadata    []byte     `json:"metadata,omitempty" db:"metadata"` // JSONB in PostgreSQL
}

// BatchStatistics represents statistical data for a batch
type BatchStatistics struct {
	ID                 string     `json:"id,omitempty" db:"id"`
	BatchID            string     `json:"batch_id" db:"batch_id" validate:"required"`
	MeanPrice          *float64   `json:"mean_price,omitempty" db:"mean_price"`
	MedianPrice        *float64   `json:"median_price,omitempty" db:"median_price"`
	MeanChangePercent  *float64   `json:"mean_change_percent,omitempty" db:"mean_change_percent"`
	PositiveChangeCount *int       `json:"positive_change_count,omitempty" db:"positive_change_count"`
	NegativeChangeCount *int       `json:"negative_change_count,omitempty" db:"negative_change_count"`
	UnchangedCount     *int       `json:"unchanged_count,omitempty" db:"unchanged_count"`
	TotalVolume        *int64     `json:"total_volume,omitempty" db:"total_volume"`
	StatisticsJSON     []byte     `json:"statistics_json,omitempty" db:"statistics_json"` // JSONB in PostgreSQL
	CreatedAt          time.Time  `json:"created_at,omitempty" db:"created_at"`
}

// MarketBatch represents a collection of stock quotes and market indices
type MarketBatch struct {
	Quotes   []StockQuote  `json:"quotes"`
	Indices  []MarketIndex `json:"indices"`
	BatchID  string        `json:"batch_id" validate:"required"`
	CreatedAt time.Time    `json:"created_at"`
}