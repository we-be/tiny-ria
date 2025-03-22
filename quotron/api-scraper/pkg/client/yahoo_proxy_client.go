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
func NewYahooProxyClient(timeout time.Duration) (*YahooProxyClient, error) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Get proxy URL from environment or use default
	proxyURL := os.Getenv("YAHOO_PROXY_URL")
	if proxyURL == "" {
		proxyURL = "http://localhost:5000"
	}

	// Check if we're already running a proxy via daemon_proxy.sh
	// First look for the PID file
	var cmd *exec.Cmd
	if _, err := os.Stat("/tmp/yfinance_proxy.pid"); os.IsNotExist(err) {
		// No PID file, check if we have run_proxy.sh
		scriptPath := "./scripts/run_proxy.sh"
		if _, err := os.Stat(scriptPath); err == nil {
			// Use run_proxy.sh instead of direct Python call
			cmd = exec.Command(scriptPath, "--host", "localhost", "--port", "5000")
		} else {
			// Try daemon_proxy.sh
			daemonScript := "./scripts/daemon_proxy.sh"
			if _, err := os.Stat(daemonScript); err == nil {
				// Use daemon script
				cmd = exec.Command(daemonScript, "--host", "localhost", "--port", "5000")
			} else {
				// Fall back to direct Python invocation
				scriptPath := "./scripts/yfinance_proxy.py"
				cmd = exec.Command("python3", scriptPath, "--host", "localhost", "--port", "5000")
			}
		}
		
		// Start the process
		if err := cmd.Start(); err != nil {
			return nil, errors.Wrap(err, "failed to start Yahoo Finance proxy server")
		}
	} else {
		// PID file exists, proxy may already be running
		// We'll still create a cmd for proper cleanup but won't start it
		cmd = exec.Command("echo", "Proxy already running")
	}

	// Create the client
	client := &YahooProxyClient{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		proxyURL:     proxyURL,
		proxyProcess: cmd,
		timeout:      timeout,
	}

	// Wait for the server to start (simple approach)
	time.Sleep(5 * time.Second)

	// Check if the server is running
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, proxyURL+"/health", nil)
	if err != nil {
		client.Stop()
		return nil, errors.Wrap(err, "failed to create health check request")
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		client.Stop()
		return nil, errors.Wrap(err, "failed to connect to Yahoo Finance proxy server")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		client.Stop()
		return nil, errors.Errorf("Yahoo Finance proxy server health check failed: %d", resp.StatusCode)
	}

	return client, nil
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