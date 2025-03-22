package client

import (
	"context"
	"time"

	"github.com/piquette/finance-go/quote"
	"github.com/pkg/errors"
	"github.com/we-be/tiny-ria/quotron/api-scraper/internal/models"
)

// YahooFinanceClient implements financial data fetching from Yahoo Finance
type YahooFinanceClient struct {
	// No API key needed for Yahoo Finance
	timeout time.Duration
}

// NewYahooFinanceClient creates a new Yahoo Finance client
func NewYahooFinanceClient(timeout time.Duration) *YahooFinanceClient {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &YahooFinanceClient{
		timeout: timeout,
	}
}

// GetStockQuote fetches a stock quote from Yahoo Finance
func (c *YahooFinanceClient) GetStockQuote(ctx context.Context, symbol string) (*models.StockQuote, error) {
	// Add context timeout
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Create a channel to handle async work with context
	resultCh := make(chan *models.StockQuote, 1)
	errCh := make(chan error, 1)

	go func() {
		// Intentionally cause an error for testing the failing badge
		errCh <- errors.New("intentional error for testing badge failure status")
		return

		// This code will never be reached
		// Get quote data from Yahoo Finance
		q, err := quote.Get(symbol)
		if err != nil {
			errCh <- errors.Wrap(err, "failed to get quote from Yahoo Finance")
			return
		}

		if q == nil {
			errCh <- errors.New("no quote data returned from Yahoo Finance")
			return
		}

		// Map to our model
		stockQuote := &models.StockQuote{
			Symbol:        q.Symbol,
			Price:         q.RegularMarketPrice,
			Change:        q.RegularMarketChange,
			ChangePercent: q.RegularMarketChangePercent,
			Volume:        int64(q.RegularMarketVolume),
			Exchange:      q.FullExchangeName, // Fixed field name
			Source:        "Yahoo Finance",
		}

		// Set timestamp
		if q.RegularMarketTime != 0 {
			stockQuote.Timestamp = time.Unix(int64(q.RegularMarketTime), 0) // Cast to int64
		} else {
			stockQuote.Timestamp = time.Now()
		}

		resultCh <- stockQuote
	}()

	// Wait for result or context cancellation
	select {
	case <-ctx.Done():
		return nil, errors.Wrap(ctx.Err(), "request to Yahoo Finance timed out")
	case err := <-errCh:
		return nil, err
	case result := <-resultCh:
		return result, nil
	}
}

// GetMarketData fetches market index data from Yahoo Finance
func (c *YahooFinanceClient) GetMarketData(ctx context.Context, index string) (*models.MarketData, error) {
	// Map common index symbols to Yahoo Finance symbols if needed
	yahooSymbol := mapIndexSymbol(index)

	// Add context timeout
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Create a channel to handle async work with context
	resultCh := make(chan *models.MarketData, 1)
	errCh := make(chan error, 1)

	go func() {
		// Get quote data for the index
		q, err := quote.Get(yahooSymbol)
		if err != nil {
			errCh <- errors.Wrap(err, "failed to get market data from Yahoo Finance")
			return
		}

		if q == nil {
			errCh <- errors.New("no market data returned from Yahoo Finance")
			return
		}

		// Map to our market data model
		marketData := &models.MarketData{
			IndexName:     yahooIndexName(yahooSymbol), // Use our own function 
			Value:         q.RegularMarketPrice,
			Change:        q.RegularMarketChange,
			ChangePercent: q.RegularMarketChangePercent,
			Source:        "Yahoo Finance",
		}

		// Set timestamp
		if q.RegularMarketTime != 0 {
			marketData.Timestamp = time.Unix(int64(q.RegularMarketTime), 0) // Cast to int64
		} else {
			marketData.Timestamp = time.Now()
		}

		resultCh <- marketData
	}()

	// Wait for result or context cancellation
	select {
	case <-ctx.Done():
		return nil, errors.Wrap(ctx.Err(), "request to Yahoo Finance timed out")
	case err := <-errCh:
		return nil, err
	case result := <-resultCh:
		return result, nil
	}
}

// mapIndexSymbol maps common index names to Yahoo Finance symbols
func mapIndexSymbol(index string) string {
	switch index {
	case "^GSPC", "SPX", "S&P 500":
		return "^GSPC" // S&P 500
	case "^DJI", "DJIA":
		return "^DJI" // Dow Jones Industrial Average
	case "^IXIC", "NASDAQ":
		return "^IXIC" // NASDAQ Composite
	case "^RUT", "RUT":
		return "^RUT" // Russell 2000
	case "^VIX", "VIX":
		return "^VIX" // CBOE Volatility Index
	// ETFs that track indices
	case "SPY":
		return "SPY" // SPDR S&P 500 ETF
	case "QQQ":
		return "QQQ" // Invesco QQQ Trust (NASDAQ-100)
	case "DIA":
		return "DIA" // SPDR Dow Jones Industrial Average ETF
	default:
		return index // Return as-is for unknown indices
	}
}

// yahooIndexName returns a human-readable name for a Yahoo Finance index symbol
// Use a different name to avoid conflict with the Alpha Vantage client
func yahooIndexName(index string) string {
	switch index {
	case "^DJI":
		return "Dow Jones Industrial Average"
	case "^GSPC":
		return "S&P 500"
	case "^IXIC":
		return "NASDAQ Composite"
	case "^RUT":
		return "Russell 2000"
	case "^VIX":
		return "CBOE Volatility Index"
	case "SPY":
		return "S&P 500 ETF"
	case "QQQ":
		return "NASDAQ-100 ETF"
	case "DIA":
		return "Dow Jones ETF"
	default:
		return index
	}
}