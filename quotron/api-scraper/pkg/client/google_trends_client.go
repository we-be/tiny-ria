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

// TrendsHealthResponse represents the response from the health endpoint
type TrendsHealthResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// InterestOverTimeData represents interest over time data for a keyword
type InterestOverTimeData struct {
	Keyword   string                   `json:"keyword"`
	Timeframe string                   `json:"timeframe"`
	Data      []map[string]interface{} `json:"data"`
	Timestamp string                   `json:"timestamp"`
	Source    string                   `json:"source"`
}

// RelatedQueriesData represents related queries data for a keyword
type RelatedQueriesData struct {
	Keyword   string                   `json:"keyword"`
	Top       []map[string]interface{} `json:"top"`
	Rising    []map[string]interface{} `json:"rising"`
	Timestamp string                   `json:"timestamp"`
	Source    string                   `json:"source"`
}

// RelatedTopicsData represents related topics data for a keyword
type RelatedTopicsData struct {
	Keyword   string                   `json:"keyword"`
	Top       []map[string]interface{} `json:"top"`
	Rising    []map[string]interface{} `json:"rising"`
	Timestamp string                   `json:"timestamp"`
	Source    string                   `json:"source"`
}

// GoogleTrendsClient implements Google Trends data fetching from a local Python proxy
type GoogleTrendsClient struct {
	httpClient   *http.Client
	proxyURL     string
	proxyProcess *exec.Cmd
	timeout      time.Duration
	cacheHits    int
	cacheMisses  int
	requestCount int
}

// NewGoogleTrendsClient creates a new Google Trends client and starts the proxy server
func NewGoogleTrendsClient(timeout time.Duration) (*GoogleTrendsClient, error) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Get proxy URL from environment or use default
	proxyURL := os.Getenv("TRENDS_PROXY_URL")
	if proxyURL == "" {
		proxyURL = "http://localhost:5001"
	}

	// Check if we're already running a proxy via daemon
	// First look for the PID file
	var cmd *exec.Cmd
	if _, err := os.Stat("/tmp/gtrends_proxy.pid"); os.IsNotExist(err) {
		// No PID file, check if we have the daemon script
		daemonScript := "./scripts/gtrends_daemon.sh"
		if _, err := os.Stat(daemonScript); err == nil {
			// Use daemon script
			cmd = exec.Command(daemonScript, "--host", "localhost", "--port", "5001")
		} else {
			// Fall back to direct Python invocation
			scriptPath := "./scripts/gtrends_proxy.py"
			cmd = exec.Command("python3", scriptPath, "--host", "localhost", "--port", "5001")
		}
		
		// Start the process
		if err := cmd.Start(); err != nil {
			return nil, errors.Wrap(err, "failed to start Google Trends proxy server")
		}
	} else {
		// PID file exists, proxy may already be running
		// We'll still create a cmd for proper cleanup but won't start it
		cmd = exec.Command("echo", "Proxy already running")
	}

	// Create the client
	client := &GoogleTrendsClient{
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
		return nil, errors.Wrap(err, "failed to connect to Google Trends proxy server")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		client.Stop()
		return nil, errors.Errorf("Google Trends proxy server health check failed: %d", resp.StatusCode)
	}

	return client, nil
}

// Stop stops the proxy server process
func (c *GoogleTrendsClient) Stop() {
	if c.proxyProcess != nil && c.proxyProcess.Process != nil {
		c.proxyProcess.Process.Kill()
	}
}

// GetInterestOverTime fetches interest over time data for a keyword
func (c *GoogleTrendsClient) GetInterestOverTime(ctx context.Context, keyword string, timeframe string) (*InterestOverTimeData, error) {
	// Construct the URL with path parameter and query parameter
	queryURL, err := url.Parse(c.proxyURL + "/interest-over-time/" + url.PathEscape(keyword))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse URL")
	}

	// Add timeframe as query parameter if provided
	if timeframe != "" {
		q := queryURL.Query()
		q.Set("timeframe", timeframe)
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
		return nil, errors.Wrap(err, "failed to execute request to Google Trends proxy")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("Google Trends proxy returned non-200 status: %d", resp.StatusCode)
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
	var data InterestOverTimeData
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal interest over time response")
	}

	return &data, nil
}

// GetRelatedQueries fetches related queries for a keyword
func (c *GoogleTrendsClient) GetRelatedQueries(ctx context.Context, keyword string) (*RelatedQueriesData, error) {
	// Construct the URL with path parameter
	queryURL, err := url.Parse(c.proxyURL + "/related-queries/" + url.PathEscape(keyword))
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
		return nil, errors.Wrap(err, "failed to execute request to Google Trends proxy")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("Google Trends proxy returned non-200 status: %d", resp.StatusCode)
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
	var data RelatedQueriesData
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal related queries response")
	}

	return &data, nil
}

// GetRelatedTopics fetches related topics for a keyword
func (c *GoogleTrendsClient) GetRelatedTopics(ctx context.Context, keyword string) (*RelatedTopicsData, error) {
	// Construct the URL with path parameter
	queryURL, err := url.Parse(c.proxyURL + "/related-topics/" + url.PathEscape(keyword))
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
		return nil, errors.Wrap(err, "failed to execute request to Google Trends proxy")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("Google Trends proxy returned non-200 status: %d", resp.StatusCode)
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
	var data RelatedTopicsData
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal related topics response")
	}

	return &data, nil
}

// CheckProxyHealth checks the health status of the proxy server
func (c *GoogleTrendsClient) CheckProxyHealth(ctx context.Context) (*TrendsHealthResponse, error) {
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

	var health TrendsHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, errors.Wrap(err, "failed to decode health response")
	}

	return &health, nil
}

// GetCacheMetrics returns metrics about cache performance
func (c *GoogleTrendsClient) GetCacheMetrics() (hits int, misses int, ratio float64) {
	total := c.cacheHits + c.cacheMisses
	ratio = 0
	if total > 0 {
		ratio = float64(c.cacheHits) / float64(total)
	}
	return c.cacheHits, c.cacheMisses, ratio
}

// GetRequestCount returns the total number of requests made
func (c *GoogleTrendsClient) GetRequestCount() int {
	return c.requestCount
}

// FormatTrendsURL generates a shareable Google Trends URL for the given keyword
func FormatTrendsURL(keyword string, timeframe string) string {
	// Default timeframe
	if timeframe == "" {
		timeframe = "today 5-y"
	}
	
	// Map common timeframes to Google Trends URL parameters
	var period string
	switch timeframe {
	case "today 1-m":
		period = "1m"
	case "today 3-m":
		period = "3m"
	case "today 12-m":
		period = "12m"
	case "today 5-y":
		period = "5y"
	case "all":
		period = "all"
	default:
		period = "5y" // default to 5 years
	}
	
	// Create URL with proper encoding
	encodedKeyword := url.QueryEscape(keyword)
	return fmt.Sprintf("https://trends.google.com/trends/explore?date=%s&q=%s", period, encodedKeyword)
}