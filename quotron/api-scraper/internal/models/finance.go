package models

import "time"

// StockQuote represents a single stock quote
type StockQuote struct {
	Symbol        string    `json:"symbol"`
	Price         float64   `json:"price"`
	Change        float64   `json:"change"`
	ChangePercent float64   `json:"changePercent"`
	Volume        int64     `json:"volume"`
	Timestamp     time.Time `json:"timestamp"`
	Exchange      string    `json:"exchange"`
	Source        string    `json:"source"`
}

// MapExchangeToEnum maps various exchange codes to the standard enum values
// Enum values are: NYSE, NASDAQ, AMEX, OTC, OTHER
func MapExchangeToEnum(exchange string) string {
	switch exchange {
	case "NYSE":
		return "NYSE"
	case "NASDAQ", "NMS", "NGS", "NAS", "NCM":
		return "NASDAQ"
	case "AMEX", "ASE", "CBOE":
		return "AMEX"
	case "OTC", "OTCBB", "OTC PINK":
		return "OTC"
	default:
		return "OTHER"
	}
}

// MarketData represents overall market data
type MarketData struct {
	IndexName     string    `json:"indexName"`
	Value         float64   `json:"value"`
	Change        float64   `json:"change"`
	ChangePercent float64   `json:"changePercent"`
	Timestamp     time.Time `json:"timestamp"`
	Source        string    `json:"source"`
}