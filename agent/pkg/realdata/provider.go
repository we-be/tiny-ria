package realdata

import (
	"context"
	"fmt"
)

// FinancialDataProvider defines the interface for fetching financial data
type FinancialDataProvider interface {
	// GetStockQuote fetches stock data for a given symbol
	GetStockQuote(ctx context.Context, symbol string) (*StockQuote, error)
	
	// GetCryptoQuote fetches cryptocurrency data for a given symbol
	GetCryptoQuote(ctx context.Context, symbol string) (*StockQuote, error)
	
	// GetMarketData fetches market index data for a given index
	GetMarketData(ctx context.Context, index string) (*MarketData, error)
}

// GetProvider returns a financial data provider based on the provider name
func GetProvider(providerName string) (FinancialDataProvider, error) {
	switch providerName {
	case "yahoo", "yahoo-finance", "":
		// Default to Yahoo Finance
		return NewYahooFinanceClient(), nil
	default:
		return nil, fmt.Errorf("unsupported financial data provider: %s", providerName)
	}
}