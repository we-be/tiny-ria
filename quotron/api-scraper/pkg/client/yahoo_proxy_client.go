package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"time"

	"github.com/pkg/errors"
	"github.com/tiny-ria/quotron/api-scraper/internal/models"
)

// YahooProxyClient implements financial data fetching from a local Python yfinance proxy
type YahooProxyClient struct {
	httpClient   *http.Client
	proxyURL     string
	proxyProcess *exec.Cmd
	timeout      time.Duration
}

// NewYahooProxyClient creates a new Yahoo Finance proxy client and starts the proxy server
func NewYahooProxyClient(timeout time.Duration) (*YahooProxyClient, error) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Default proxy URL (localhost:8080)
	proxyURL := "http://localhost:8080"

	// Start the Python proxy server in the background with Python
	scriptPath := "./scripts/yfinance_proxy.py"
	cmd := exec.Command("python3", scriptPath, "--host", "localhost", "--port", "8080")
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start Yahoo Finance proxy server")
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
	fmt.Printf("Waiting for Yahoo Finance proxy server to start...\n")
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
	// Construct the URL with query parameters
	queryURL, err := url.Parse(c.proxyURL + "/quote")
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse URL")
	}

	params := url.Values{}
	params.Add("symbol", symbol)
	queryURL.RawQuery = params.Encode()

	// Make the request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute request to Yahoo Finance proxy")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("Yahoo Finance proxy returned non-200 status: %d", resp.StatusCode)
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

	// Construct the URL with query parameters
	queryURL, err := url.Parse(c.proxyURL + "/market")
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse URL")
	}

	params := url.Values{}
	params.Add("index", yahooSymbol)
	queryURL.RawQuery = params.Encode()

	// Make the request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute request to Yahoo Finance proxy")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("Yahoo Finance proxy returned non-200 status: %d", resp.StatusCode)
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