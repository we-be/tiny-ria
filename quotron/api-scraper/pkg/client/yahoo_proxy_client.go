package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"time"

	"github.com/pkg/errors"
	"github.com/we-be/tiny-ria/quotron/api-scraper/internal/models"
)

// ProxyHealthResponse represents the response from the health endpoint
type ProxyHealthResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// YahooProxyClient implements financial data fetching from a local Python yfinance proxy
type YahooProxyClient struct {
	httpClient   *http.Client
	proxyURL     string
	proxyProcess *exec.Cmd
	timeout      time.Duration
	cacheHits    int
	cacheMisses  int
	requestCount int
}

// NewYahooProxyClient creates a new Yahoo Finance proxy client and starts the proxy server
func NewYahooProxyClient(timeout time.Duration) (Client, error) {
	return NewYahooFinanceClient(timeout), nil
}

// Stop stops the proxy server process
func (c *YahooProxyClient) Stop() {
	if c.proxyProcess != nil && c.proxyProcess.Process != nil {
		c.proxyProcess.Process.Kill()
	}
}

// GetStockQuote fetches a stock quote from the Yahoo Finance proxy
func (c *YahooProxyClient) GetStockQuote(ctx context.Context, symbol string) (*models.StockQuote, error) {
	// Construct the URL with path parameter
	queryURL, err := url.Parse(c.proxyURL + "/quote/" + symbol)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse URL")
	}

	// Make the request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	c.requestCount++
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute request to Yahoo Finance proxy")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("Yahoo Finance proxy returned non-200 status: %d", resp.StatusCode)
	}

	// Track cache hits/misses if header is present
	cacheStatus := resp.Header.Get("X-Cache-Status")
	if cacheStatus == "hit" {
		c.cacheHits++
	} else if cacheStatus == "miss" {
		c.cacheMisses++
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	// Check if response contains an error
	var errorResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Error != "" {
		return nil, errors.New(errorResp.Error)
	}

	// Parse the response into our model with string timestamp
	var quote struct {
		Symbol        string  `json:"symbol"`
		Price         float64 `json:"price"`
		Change        float64 `json:"change"`
		ChangePercent float64 `json:"changePercent"`
		Volume        int64   `json:"volume"`
		Timestamp     string  `json:"timestamp"` // String timestamp
		Exchange      string  `json:"exchange"`
		Source        string  `json:"source"`
	}

	if err := json.Unmarshal(body, &quote); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal quote response")
	}

	// Convert timestamp to time.Time
	timestamp, err := time.Parse(time.RFC3339, quote.Timestamp)
	if err != nil {
		// If parsing fails, use current time
		timestamp = time.Now()
	}

	// Convert to our StockQuote model
	stockQuote := &models.StockQuote{
		Symbol:        quote.Symbol,
		Price:         quote.Price,
		Change:        quote.Change,
		ChangePercent: quote.ChangePercent,
		Volume:        quote.Volume,
		Timestamp:     timestamp,
		Exchange:      quote.Exchange,
		Source:        quote.Source,
	}

	return stockQuote, nil
}

// GetMarketData fetches market index data from the Yahoo Finance proxy
func (c *YahooProxyClient) GetMarketData(ctx context.Context, index string) (*models.MarketData, error) {
	// Map the index symbol if needed
	yahooSymbol := mapIndexSymbol(index)

	// Construct the URL with path parameter
	queryURL, err := url.Parse(c.proxyURL + "/market/" + yahooSymbol)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse URL")
	}

	// Make the request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	c.requestCount++
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute request to Yahoo Finance proxy")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("Yahoo Finance proxy returned non-200 status: %d", resp.StatusCode)
	}

	// Track cache hits/misses if header is present
	cacheStatus := resp.Header.Get("X-Cache-Status")
	if cacheStatus == "hit" {
		c.cacheHits++
	} else if cacheStatus == "miss" {
		c.cacheMisses++
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	// Check if response contains an error
	var errorResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Error != "" {
		return nil, errors.New(errorResp.Error)
	}

	// Parse the response into our model with string timestamp
	var marketData struct {
		IndexName     string  `json:"indexName"`
		Value         float64 `json:"value"`
		Change        float64 `json:"change"`
		ChangePercent float64 `json:"changePercent"`
		Timestamp     string  `json:"timestamp"` // String timestamp
		Source        string  `json:"source"`
	}

	if err := json.Unmarshal(body, &marketData); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal market data response")
	}

	// Convert timestamp to time.Time
	timestamp, err := time.Parse(time.RFC3339, marketData.Timestamp)
	if err != nil {
		// If parsing fails, use current time
		timestamp = time.Now()
	}

	// Convert to our MarketData model
	md := &models.MarketData{
		IndexName:     marketData.IndexName,
		Value:         marketData.Value,
		Change:        marketData.Change,
		ChangePercent: marketData.ChangePercent,
		Timestamp:     timestamp,
		Source:        marketData.Source,
	}

	return md, nil
}

// CheckProxyHealth checks the health status of the proxy server
func (c *YahooProxyClient) CheckProxyHealth(ctx context.Context) (*ProxyHealthResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.proxyURL+"/health", nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create health check request")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute health check request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("health check returned non-200 status: %d", resp.StatusCode)
	}

	var health ProxyHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, errors.Wrap(err, "failed to decode health response")
	}

	return &health, nil
}

// GetCacheMetrics returns metrics about cache performance
func (c *YahooProxyClient) GetCacheMetrics() (hits int, misses int, ratio float64) {
	total := c.cacheHits + c.cacheMisses
	ratio = 0
	if total > 0 {
		ratio = float64(c.cacheHits) / float64(total)
	}
	return c.cacheHits, c.cacheMisses, ratio
}

// GetRequestCount returns the total number of requests made
func (c *YahooProxyClient) GetRequestCount() int {
	return c.requestCount
}