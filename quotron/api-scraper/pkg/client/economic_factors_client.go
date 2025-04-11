package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"time"

	"github.com/pkg/errors"
)

// EconomicHealthResponse represents the response from the health endpoint
type EconomicHealthResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// EconomicIndicatorData represents economic indicator data
type EconomicIndicatorData struct {
	Indicator string                   `json:"indicator"`
	Period    string                   `json:"period"`
	Data      []map[string]interface{} `json:"data"`
	Timestamp string                   `json:"timestamp"`
	Source    string                   `json:"source"`
}

// EconomicFactorsSummary represents the summary of all economic factors for the US
type EconomicFactorsSummary struct {
	Indicators []string               `json:"indicators"`
	Timestamp  string                 `json:"timestamp"`
	Source     string                 `json:"source"`
	Summary    map[string]interface{} `json:"summary"`
}

// EconomicFactorsClient implements US economic data fetching from a local Python proxy
type EconomicFactorsClient struct {
	httpClient   *http.Client
	proxyURL     string
	proxyProcess *exec.Cmd
	timeout      time.Duration
	cacheHits    int
	cacheMisses  int
	requestCount int
}

// NewEconomicFactorsClient creates a new economic factors client and starts the proxy server
func NewEconomicFactorsClient(timeout time.Duration) (*EconomicFactorsClient, error) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Get proxy URL from environment or use default
	proxyURL := os.Getenv("ECONOMIC_PROXY_URL")
	if proxyURL == "" {
		proxyURL = "http://localhost:5002"
	}

	// Check if we're already running a proxy via daemon
	// First look for the PID file
	var cmd *exec.Cmd
	if _, err := os.Stat("/tmp/economic_proxy.pid"); os.IsNotExist(err) {
		// No PID file, check if we have the daemon script
		daemonScript := "./scripts/economic_daemon.sh"
		if _, err := os.Stat(daemonScript); err == nil {
			// Use daemon script
			cmd = exec.Command(daemonScript, "--host", "localhost", "--port", "5002")
		} else {
			// Fall back to direct Python invocation
			scriptPath := "./scripts/economic_proxy.py"
			cmd = exec.Command("python3", scriptPath, "--host", "localhost", "--port", "5002")
		}
		
		// Start the process
		if err := cmd.Start(); err != nil {
			return nil, errors.Wrap(err, "failed to start Economic Factors proxy server")
		}
	} else {
		// PID file exists, proxy may already be running
		// We'll still create a cmd for proper cleanup but won't start it
		cmd = exec.Command("echo", "Proxy already running")
	}

	// Create the client
	client := &EconomicFactorsClient{
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
		return nil, errors.Wrap(err, "failed to connect to Economic Factors proxy server")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		client.Stop()
		return nil, errors.Errorf("Economic Factors proxy server health check failed: %d", resp.StatusCode)
	}

	return client, nil
}

// Stop stops the proxy server process
func (c *EconomicFactorsClient) Stop() {
	if c.proxyProcess != nil && c.proxyProcess.Process != nil {
		c.proxyProcess.Process.Kill()
	}
}

// GetIndicator fetches data for a specific economic indicator
func (c *EconomicFactorsClient) GetIndicator(ctx context.Context, indicator string, period string) (*EconomicIndicatorData, error) {
	// Construct the URL with path parameter and query parameter
	queryURL, err := url.Parse(c.proxyURL + "/indicator/" + url.PathEscape(indicator))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse URL")
	}

	// Add period as query parameter if provided
	if period != "" {
		q := queryURL.Query()
		q.Set("period", period)
		queryURL.RawQuery = q.Encode()
	}

	// Make the request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	c.requestCount++
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute request to Economic Factors proxy")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("Economic Factors proxy returned non-200 status: %d", resp.StatusCode)
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

	// Parse the response into our model
	var data EconomicIndicatorData
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal indicator response")
	}

	return &data, nil
}

// GetAllIndicators fetches a list of all available economic indicators
func (c *EconomicFactorsClient) GetAllIndicators(ctx context.Context) ([]string, error) {
	// Construct the URL
	queryURL := c.proxyURL + "/indicators"

	// Make the request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	c.requestCount++
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute request to Economic Factors proxy")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("Economic Factors proxy returned non-200 status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	// Parse the response into our model
	var data struct {
		Indicators []string `json:"indicators"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal indicators response")
	}

	return data.Indicators, nil
}

// GetSummary fetches a summary of key economic indicators
func (c *EconomicFactorsClient) GetSummary(ctx context.Context) (*EconomicFactorsSummary, error) {
	// Construct the URL
	queryURL := c.proxyURL + "/summary"

	// Make the request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	c.requestCount++
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute request to Economic Factors proxy")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("Economic Factors proxy returned non-200 status: %d", resp.StatusCode)
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

	// Parse the response into our model
	var data EconomicFactorsSummary
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal summary response")
	}

	return &data, nil
}

// CheckProxyHealth checks the health status of the proxy server
func (c *EconomicFactorsClient) CheckProxyHealth(ctx context.Context) (*EconomicHealthResponse, error) {
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

	var health EconomicHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, errors.Wrap(err, "failed to decode health response")
	}

	return &health, nil
}

// GetCacheMetrics returns metrics about cache performance
func (c *EconomicFactorsClient) GetCacheMetrics() (hits int, misses int, ratio float64) {
	total := c.cacheHits + c.cacheMisses
	ratio = 0
	if total > 0 {
		ratio = float64(c.cacheHits) / float64(total)
	}
	return c.cacheHits, c.cacheMisses, ratio
}

// GetRequestCount returns the total number of requests made
func (c *EconomicFactorsClient) GetRequestCount() int {
	return c.requestCount
}

// FormatIndicatorURL generates a shareable URL for the given indicator
func FormatIndicatorURL(indicator string) string {
	// For FRED, a common economic data source
	encodedIndicator := url.QueryEscape(indicator)
	return fmt.Sprintf("https://fred.stlouisfed.org/series/%s", encodedIndicator)
}