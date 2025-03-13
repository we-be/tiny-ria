package yahoo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/we-be/tiny-ria/quotron/models"
)

// proxyClient implements Client using the Python yfinance proxy server
type proxyClient struct {
	client      *http.Client
	options     *clientOptions
	baseURL     string
	cmd         *exec.Cmd
	proxyStopCh chan struct{}
}

// newProxyClient creates a new Yahoo Finance proxy client
func newProxyClient(options *clientOptions) (Client, error) {
	client := &http.Client{
		Timeout: options.timeout,
	}
	
	// Use default proxy URL if not specified
	proxyURL := options.proxyURL
	if proxyURL == "" {
		proxyURL = "http://localhost:5000"
	}
	
	// Create the proxy client
	pc := &proxyClient{
		client:      client,
		options:     options,
		baseURL:     proxyURL,
		proxyStopCh: make(chan struct{}),
	}
	
	// Check if proxy is already running
	err := pc.checkProxyHealth(context.Background())
	if err != nil {
		// Proxy not running, start it
		err = pc.startProxy()
		if err != nil {
			return nil, fmt.Errorf("failed to start proxy: %w", err)
		}
		
		// Wait for proxy to become available
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		// Poll until proxy is healthy or timeout occurs
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("timed out waiting for proxy to start")
			case <-ticker.C:
				err := pc.checkProxyHealth(ctx)
				if err == nil {
					// Proxy is healthy
					return pc, nil
				}
			}
		}
	}
	
	// Proxy is already running
	return pc, nil
}

// startProxy starts the Python yfinance proxy server
func (c *proxyClient) startProxy() error {
	// Try to find the proxy script
	scriptPaths := []string{
		"quotron/api-scraper/scripts/yfinance_proxy.py",
		"api-scraper/scripts/yfinance_proxy.py",
		"../api-scraper/scripts/yfinance_proxy.py",
		"../../api-scraper/scripts/yfinance_proxy.py",
	}
	
	var scriptPath string
	for _, path := range scriptPaths {
		if _, err := os.Stat(path); err == nil {
			scriptPath = path
			break
		}
	}
	
	if scriptPath == "" {
		return fmt.Errorf("could not find yfinance_proxy.py script")
	}
	
	// Extract host and port from URL
	parts := strings.Split(strings.TrimPrefix(c.baseURL, "http://"), ":")
	host := parts[0]
	port := "5000"
	if len(parts) > 1 {
		port = parts[1]
	}
	
	// Start the proxy script
	cmd := exec.Command("python", scriptPath, "--host", host, "--port", port)
	
	// Redirect output to log file
	logFile, err := os.OpenFile("/tmp/yfinance_proxy.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	
	// Start the process
	err = cmd.Start()
	if err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start proxy: %w", err)
	}
	
	// Save the command for cleanup
	c.cmd = cmd
	
	return nil
}

// checkProxyHealth checks if the proxy server is running and healthy
func (c *proxyClient) checkProxyHealth(ctx context.Context) error {
	// Create request with context
	url := fmt.Sprintf("%s/health", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}
	
	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send health check request: %w", err)
	}
	defer resp.Body.Close()
	
	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected health check status code: %d", resp.StatusCode)
	}
	
	return nil
}

// GetStockQuote retrieves a stock quote for the given symbol
func (c *proxyClient) GetStockQuote(ctx context.Context, symbol string) (*models.StockQuote, error) {
	// Create request with context
	url := fmt.Sprintf("%s/quote/%s", c.baseURL, symbol)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
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
	var quote models.StockQuote
	if err := json.Unmarshal(body, &quote); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	return &quote, nil
}

// GetMarketData retrieves market data for the given index symbol
func (c *proxyClient) GetMarketData(ctx context.Context, symbol string) (*models.MarketIndex, error) {
	// Create request with context
	url := fmt.Sprintf("%s/index/%s", c.baseURL, symbol)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
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
	var index models.MarketIndex
	if err := json.Unmarshal(body, &index); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	return &index, nil
}

// GetMultipleQuotes retrieves quotes for multiple symbols
func (c *proxyClient) GetMultipleQuotes(ctx context.Context, symbols []string) (map[string]*models.StockQuote, error) {
	// Create request with context
	url := fmt.Sprintf("%s/quotes", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(fmt.Sprintf(`{"symbols": ["%s"]}`, strings.Join(symbols, "\",\""))))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set content type
	req.Header.Set("Content-Type", "application/json")
	
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
	var response struct {
		Quotes []models.StockQuote `json:"quotes"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	// Convert to map
	quotes := make(map[string]*models.StockQuote)
	for i := range response.Quotes {
		q := response.Quotes[i]
		quotes[q.Symbol] = &q
	}
	
	return quotes, nil
}

// GetHealthStatus returns the health status of the client
func (c *proxyClient) GetHealthStatus(ctx context.Context) (*models.DataSourceHealth, error) {
	// Create request with context
	url := fmt.Sprintf("%s/health", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create health check request: %w", err)
	}
	
	// Record start time
	startTime := time.Now()
	
	// Send request
	resp, err := c.client.Do(req)
	
	// Calculate response time
	responseTime := time.Since(startTime).Milliseconds()
	
	// Create health status
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
		return health, nil
	}
	defer resp.Body.Close()
	
	// Check response status
	if resp.StatusCode != http.StatusOK {
		health.Status = "degraded"
		health.ErrorCount = 1
		health.LastError = fmt.Sprintf("unexpected health check status code: %d", resp.StatusCode)
		health.LastErrorTime = &health.LastChecked
		health.SuccessCount = 0
		health.HealthScore = 50
		return health, nil
	}
	
	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		health.Status = "degraded"
		health.ErrorCount = 1
		health.LastError = fmt.Sprintf("failed to read health check response: %s", err)
		health.LastErrorTime = &health.LastChecked
		health.SuccessCount = 0
		health.HealthScore = 50
		return health, nil
	}
	
	// Parse response
	var healthResp struct {
		Status    string    `json:"status"`
		Timestamp time.Time `json:"timestamp"`
		Uptime    float64   `json:"uptime"`
	}
	if err := json.Unmarshal(body, &healthResp); err != nil {
		health.Status = "degraded"
		health.ErrorCount = 1
		health.LastError = fmt.Sprintf("failed to parse health check response: %s", err)
		health.LastErrorTime = &health.LastChecked
		health.SuccessCount = 0
		health.HealthScore = 50
		return health, nil
	}
	
	// Update health status
	health.Status = healthResp.Status
	health.SuccessCount = 1
	health.ErrorCount = 0
	
	// Calculate up since time
	if healthResp.Uptime > 0 {
		upSince := healthResp.Timestamp.Add(-time.Duration(healthResp.Uptime * float64(time.Second)))
		health.UpSince = &upSince
	}
	
	// Set health score based on status
	if healthResp.Status == "ok" {
		health.HealthScore = 100
	} else {
		health.HealthScore = 50
	}
	
	return health, nil
}

// Stop stops the client and releases resources
func (c *proxyClient) Stop() error {
	// Kill the proxy process if we started it
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	
	return nil
}