package yahoo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/we-be/tiny-ria/quotron/models"
)

// restClient implements Client using direct REST API calls
type restClient struct {
	client  *http.Client
	options *clientOptions
}

// newRestClient creates a new Yahoo Finance REST client
func newRestClient(options *clientOptions) (Client, error) {
	client := &http.Client{
		Timeout: options.timeout,
	}
	
	return &restClient{
		client:  client,
		options: options,
	}, nil
}

// GetStockQuote retrieves a stock quote for the given symbol
func (c *restClient) GetStockQuote(ctx context.Context, symbol string) (*models.StockQuote, error) {
	// Create request with context
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v7/finance/quote?symbols=%s", symbol)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")
	
	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	// Parse response
	var yahooResp struct {
		QuoteResponse struct {
			Result []struct {
				Symbol        string  `json:"symbol"`
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				RegularMarketChange float64 `json:"regularMarketChange"`
				RegularMarketChangePercent float64 `json:"regularMarketChangePercent"`
				RegularMarketVolume int64   `json:"regularMarketVolume"`
				RegularMarketTime   int64   `json:"regularMarketTime"`
				Exchange      string  `json:"exchange"`
			} `json:"result"`
			Error interface{} `json:"error"`
		} `json:"quoteResponse"`
	}
	
	if err := json.Unmarshal(body, &yahooResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	// Check for API error
	if yahooResp.QuoteResponse.Error != nil {
		return nil, fmt.Errorf("Yahoo API error: %v", yahooResp.QuoteResponse.Error)
	}
	
	// Check if there are any results
	if len(yahooResp.QuoteResponse.Result) == 0 {
		return nil, fmt.Errorf("no results for symbol: %s", symbol)
	}
	
	// Extract stock quote
	result := yahooResp.QuoteResponse.Result[0]
	
	// Parse exchange code
	var exchange models.Exchange
	switch {
	case strings.Contains(result.Exchange, "NASDAQ"):
		exchange = models.NASDAQ
	case strings.Contains(result.Exchange, "NYSE"):
		exchange = models.NYSE
	case strings.Contains(result.Exchange, "AMEX"):
		exchange = models.AMEX
	case strings.Contains(result.Exchange, "OTC"):
		exchange = models.OTC
	default:
		exchange = models.OTHER
	}
	
	// Create stock quote
	quote := &models.StockQuote{
		Symbol:        result.Symbol,
		Price:         result.RegularMarketPrice,
		Change:        result.RegularMarketChange,
		ChangePercent: result.RegularMarketChangePercent,
		Volume:        result.RegularMarketVolume,
		Timestamp:     time.Unix(result.RegularMarketTime, 0),
		Exchange:      exchange,
		Source:        models.APIScraperSource,
	}
	
	return quote, nil
}

// GetMarketData retrieves market data for the given index symbol
func (c *restClient) GetMarketData(ctx context.Context, symbol string) (*models.MarketIndex, error) {
	// Use the same endpoint as GetStockQuote, but interpret the result as a market index
	quote, err := c.GetStockQuote(ctx, symbol)
	if err != nil {
		return nil, err
	}
	
	// Convert to MarketIndex
	index := &models.MarketIndex{
		Name:          quote.Symbol,
		Symbol:        quote.Symbol,
		Value:         quote.Price,
		Change:        quote.Change,
		ChangePercent: quote.ChangePercent,
		Timestamp:     quote.Timestamp,
		Source:        quote.Source,
	}
	
	return index, nil
}

// GetMultipleQuotes retrieves quotes for multiple symbols
func (c *restClient) GetMultipleQuotes(ctx context.Context, symbols []string) (map[string]*models.StockQuote, error) {
	// Create a comma-separated list of symbols
	symbolList := strings.Join(symbols, ",")
	
	// Create request with context
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v7/finance/quote?symbols=%s", symbolList)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")
	
	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	// Parse response
	var yahooResp struct {
		QuoteResponse struct {
			Result []struct {
				Symbol        string  `json:"symbol"`
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				RegularMarketChange float64 `json:"regularMarketChange"`
				RegularMarketChangePercent float64 `json:"regularMarketChangePercent"`
				RegularMarketVolume int64   `json:"regularMarketVolume"`
				RegularMarketTime   int64   `json:"regularMarketTime"`
				Exchange      string  `json:"exchange"`
			} `json:"result"`
			Error interface{} `json:"error"`
		} `json:"quoteResponse"`
	}
	
	if err := json.Unmarshal(body, &yahooResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	// Check for API error
	if yahooResp.QuoteResponse.Error != nil {
		return nil, fmt.Errorf("Yahoo API error: %v", yahooResp.QuoteResponse.Error)
	}
	
	// Process results
	quotes := make(map[string]*models.StockQuote)
	for _, result := range yahooResp.QuoteResponse.Result {
		// Parse exchange code
		var exchange models.Exchange
		switch {
		case strings.Contains(result.Exchange, "NASDAQ"):
			exchange = models.NASDAQ
		case strings.Contains(result.Exchange, "NYSE"):
			exchange = models.NYSE
		case strings.Contains(result.Exchange, "AMEX"):
			exchange = models.AMEX
		case strings.Contains(result.Exchange, "OTC"):
			exchange = models.OTC
		default:
			exchange = models.OTHER
		}
		
		// Create stock quote
		quote := &models.StockQuote{
			Symbol:        result.Symbol,
			Price:         result.RegularMarketPrice,
			Change:        result.RegularMarketChange,
			ChangePercent: result.RegularMarketChangePercent,
			Volume:        result.RegularMarketVolume,
			Timestamp:     time.Unix(result.RegularMarketTime, 0),
			Exchange:      exchange,
			Source:        models.APIScraperSource,
		}
		
		quotes[result.Symbol] = quote
	}
	
	return quotes, nil
}

// GetHealthStatus returns the health status of the client
func (c *restClient) GetHealthStatus(ctx context.Context) (*models.DataSourceHealth, error) {
	// Check if the API is working by fetching a well-known symbol
	startTime := time.Now()
	_, err := c.GetStockQuote(ctx, "AAPL")
	responseTime := time.Since(startTime).Milliseconds()
	
	health := &models.DataSourceHealth{
		Source:         models.APIScraperSource,
		LastChecked:    time.Now(),
		ResponseTime:   int(responseTime),
		AverageLatency: float64(responseTime),
	}
	
	if err != nil {
		health.Status = "down"
		health.ErrorCount = 1
		health.LastError = err.Error()
		health.LastErrorTime = &health.LastChecked
		health.SuccessCount = 0
		health.HealthScore = 0
	} else {
		health.Status = "up"
		health.ErrorCount = 0
		health.SuccessCount = 1
		now := time.Now()
		health.UpSince = &now
		health.HealthScore = 100
	}
	
	return health, nil
}

// Stop stops the client and releases resources
func (c *restClient) Stop() error {
	// No resources to clean up for the REST client
	return nil
}