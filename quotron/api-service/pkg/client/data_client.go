package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// StockQuote represents stock quote data
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

// MarketData represents market index data
type MarketData struct {
	IndexName     string    `json:"indexName"`
	Value         float64   `json:"value"`
	Change        float64   `json:"change"`
	ChangePercent float64   `json:"changePercent"`
	Timestamp     time.Time `json:"timestamp"`
	Source        string    `json:"source"`
}

// DataClient defines the interface for fetching financial data
type DataClient interface {
	GetStockQuote(ctx context.Context, symbol string) (*StockQuote, error)
	GetMarketData(ctx context.Context, index string) (*MarketData, error)
	GetName() string
	GetHealth() string
}

// YahooProxyClient implements DataClient interface using the Yahoo Finance proxy
type YahooProxyClient struct {
	baseURL   string
	clientID  string
	useHTTPS  bool
	isHealthy bool
}

// NewYahooProxyClient creates a new Yahoo proxy client
func NewYahooProxyClient(host string, port int, clientID string, useHTTPS bool) *YahooProxyClient {
	protocol := "http"
	if useHTTPS {
		protocol = "https"
	}
	return &YahooProxyClient{
		baseURL:   fmt.Sprintf("%s://%s:%d", protocol, host, port),
		clientID:  clientID,
		useHTTPS:  useHTTPS,
		isHealthy: true,
	}
}

// GetStockQuote fetches a stock quote from the Yahoo Finance proxy
func (c *YahooProxyClient) GetStockQuote(ctx context.Context, symbol string) (*StockQuote, error) {
	// Create request URL to the proxy server
	url := fmt.Sprintf("%s/quote/%s", c.baseURL, symbol)
	
	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %w", err)
	}
	
	// Add client ID header
	req.Header.Add("X-Client-ID", c.clientID)
	
	// Execute request
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.isHealthy = false
		return nil, fmt.Errorf("error executing HTTP request: %w", err)
	}
	defer resp.Body.Close()
	
	// Check status code
	if resp.StatusCode != http.StatusOK {
		c.isHealthy = false
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("proxy returned non-OK status: %d, body: %s", resp.StatusCode, string(body))
	}
	
	// Parse JSON response
	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		c.isHealthy = false
		return nil, fmt.Errorf("error parsing JSON response: %w", err)
	}
	
	// Set client health status
	c.isHealthy = true
	
	// Extract and convert data
	price, _ := data["price"].(float64)
	change, _ := data["change"].(float64)
	changePercent, _ := data["changePercent"].(float64)
	volume, _ := data["volume"].(float64)
	exchange, _ := data["exchange"].(string)
	
	// Create stock quote
	quote := &StockQuote{
		Symbol:        symbol,
		Price:         price,
		Change:        change,
		ChangePercent: changePercent,
		Volume:        int64(volume),
		Timestamp:     time.Now(),
		Exchange:      exchange,
		Source:        "Yahoo Finance",
	}
	
	return quote, nil
}

// GetMarketData fetches market index data from the Yahoo Finance proxy
func (c *YahooProxyClient) GetMarketData(ctx context.Context, index string) (*MarketData, error) {
	// Create request URL to the proxy server
	url := fmt.Sprintf("%s/market/%s", c.baseURL, index)
	
	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %w", err)
	}
	
	// Add client ID header
	req.Header.Add("X-Client-ID", c.clientID)
	
	// Execute request
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.isHealthy = false
		return nil, fmt.Errorf("error executing HTTP request: %w", err)
	}
	defer resp.Body.Close()
	
	// Check status code
	if resp.StatusCode != http.StatusOK {
		c.isHealthy = false
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("proxy returned non-OK status: %d, body: %s", resp.StatusCode, string(body))
	}
	
	// Parse JSON response
	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		c.isHealthy = false
		return nil, fmt.Errorf("error parsing JSON response: %w", err)
	}
	
	// Set client health status
	c.isHealthy = true
	
	// Extract and convert data - handle different field names from the proxy
	value, _ := data["value"].(float64)
	if value == 0 {
		// Try alternate field name
		value, _ = data["regularMarketPrice"].(float64)
	}
	
	change, _ := data["change"].(float64)
	if change == 0 {
		// Try alternate field name
		change, _ = data["regularMarketChange"].(float64)
	}
	
	changePercent, _ := data["changePercent"].(float64)
	if changePercent == 0 {
		// Try alternate field name
		changePercent, _ = data["regularMarketChangePercent"].(float64)
	}
	
	// Extract index name
	indexName, _ := data["index_name"].(string)
	if indexName == "" {
		// Fallback to symbol if not provided
		indexName = index
	}

	// Create market data object
	marketData := &MarketData{
		IndexName:     indexName,
		Value:         value,
		Change:        change,
		ChangePercent: changePercent,
		Timestamp:     time.Now(),
		Source:        "Yahoo Finance",
	}
	
	return marketData, nil
}

// GetName returns the client name
func (c *YahooProxyClient) GetName() string {
	return "Yahoo Finance Proxy"
}

// GetHealth returns the health status of the client
func (c *YahooProxyClient) GetHealth() string {
	if c.isHealthy {
		return "healthy"
	}
	return "unhealthy"
}

// AlphaVantageClient implements DataClient interface using Alpha Vantage API
type AlphaVantageClient struct {
	apiKey    string
	isHealthy bool
}

// NewAlphaVantageClient creates a new Alpha Vantage client
func NewAlphaVantageClient(apiKey string) *AlphaVantageClient {
	return &AlphaVantageClient{
		apiKey:    apiKey,
		isHealthy: true,
	}
}

// GetStockQuote fetches a stock quote from Alpha Vantage
func (c *AlphaVantageClient) GetStockQuote(ctx context.Context, symbol string) (*StockQuote, error) {
	// Create Alpha Vantage API URL
	url := fmt.Sprintf("https://www.alphavantage.co/query?function=GLOBAL_QUOTE&symbol=%s&apikey=%s", 
		symbol, c.apiKey)
	
	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %w", err)
	}
	
	// Execute request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.isHealthy = false
		return nil, fmt.Errorf("error executing HTTP request: %w", err)
	}
	defer resp.Body.Close()
	
	// Check status code
	if resp.StatusCode != http.StatusOK {
		c.isHealthy = false
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Alpha Vantage returned non-OK status: %d, body: %s", resp.StatusCode, string(body))
	}
	
	// Parse JSON response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.isHealthy = false
		return nil, fmt.Errorf("error parsing JSON response: %w", err)
	}
	
	// Check for error messages in the response
	if _, exists := result["Error Message"]; exists {
		c.isHealthy = false
		return nil, fmt.Errorf("Alpha Vantage API error: %v", result["Error Message"])
	}
	
	// Check for rate limit messages
	if _, exists := result["Note"]; exists {
		c.isHealthy = false
		return nil, fmt.Errorf("Alpha Vantage API rate limit: %v", result["Note"])
	}
	
	// Extract data from the nested structure
	globalQuote, exists := result["Global Quote"].(map[string]interface{})
	if !exists || globalQuote == nil {
		c.isHealthy = false
		return nil, fmt.Errorf("unexpected response format from Alpha Vantage")
	}
	
	// Convert string values to proper types
	symbolStr, _ := globalQuote["01. symbol"].(string)
	priceStr, _ := globalQuote["05. price"].(string)
	changeStr, _ := globalQuote["09. change"].(string)
	changePercentStr, _ := globalQuote["10. change percent"].(string)
	volumeStr, _ := globalQuote["06. volume"].(string)
	
	// Parse strings to numbers
	price, _ := parseFloat64(priceStr)
	change, _ := parseFloat64(changeStr)
	changePercent, _ := parsePercentage(changePercentStr)
	volume, _ := parseInt64(volumeStr)
	
	// Set client health status
	c.isHealthy = true
	
	return &StockQuote{
		Symbol:        symbolStr,
		Price:         price,
		Change:        change,
		ChangePercent: changePercent,
		Volume:        volume,
		Timestamp:     time.Now(),
		Exchange:      "NYSE", // Alpha Vantage doesn't provide exchange in Global Quote
		Source:        "Alpha Vantage",
	}, nil
}

// GetMarketData fetches market index data from Alpha Vantage
func (c *AlphaVantageClient) GetMarketData(ctx context.Context, index string) (*MarketData, error) {
	// Create Alpha Vantage API URL for Time Series Daily
	url := fmt.Sprintf("https://www.alphavantage.co/query?function=TIME_SERIES_DAILY&symbol=%s&apikey=%s", 
		index, c.apiKey)
	
	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %w", err)
	}
	
	// Execute request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.isHealthy = false
		return nil, fmt.Errorf("error executing HTTP request: %w", err)
	}
	defer resp.Body.Close()
	
	// Check status code
	if resp.StatusCode != http.StatusOK {
		c.isHealthy = false
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Alpha Vantage returned non-OK status: %d, body: %s", resp.StatusCode, string(body))
	}
	
	// Parse JSON response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.isHealthy = false
		return nil, fmt.Errorf("error parsing JSON response: %w", err)
	}
	
	// Check for error messages in the response
	if _, exists := result["Error Message"]; exists {
		c.isHealthy = false
		return nil, fmt.Errorf("Alpha Vantage API error: %v", result["Error Message"])
	}
	
	// Check for rate limit messages
	if _, exists := result["Note"]; exists {
		c.isHealthy = false
		return nil, fmt.Errorf("Alpha Vantage API rate limit: %v", result["Note"])
	}
	
	// Extract time series data
	timeSeries, exists := result["Time Series (Daily)"].(map[string]interface{})
	if !exists || timeSeries == nil {
		c.isHealthy = false
		return nil, fmt.Errorf("unexpected response format from Alpha Vantage")
	}
	
	// Find the latest data point (first key in the time series)
	var latestDate string
	var latestData map[string]interface{}
	
	for date, data := range timeSeries {
		if latestDate == "" || date > latestDate {
			latestDate = date
			latestData = data.(map[string]interface{})
		}
	}
	
	if latestData == nil {
		c.isHealthy = false
		return nil, fmt.Errorf("no data found in Alpha Vantage response")
	}
	
	// Get the data point before the latest to calculate change
	var prevDate string
	var prevData map[string]interface{}
	
	for date, data := range timeSeries {
		if date != latestDate && (prevDate == "" || date > prevDate) {
			prevDate = date
			prevData = data.(map[string]interface{})
		}
	}
	
	// Extract values
	closeStr, _ := latestData["4. close"].(string)
	closeValue, _ := parseFloat64(closeStr)
	
	// Calculate change if we have previous data
	var change, changePercent float64
	if prevData != nil {
		prevCloseStr, _ := prevData["4. close"].(string)
		prevClose, _ := parseFloat64(prevCloseStr)
		change = closeValue - prevClose
		if prevClose > 0 {
			changePercent = (change / prevClose) * 100
		}
	}
	
	// Set client health status
	c.isHealthy = true
	
	return &MarketData{
		IndexName:     index,
		Value:         closeValue,
		Change:        change,
		ChangePercent: changePercent,
		Timestamp:     time.Now(),
		Source:        "Alpha Vantage",
	}, nil
}

// Helper functions for string conversion
func parseFloat64(s string) (float64, error) {
	var v float64
	_, err := fmt.Sscanf(s, "%f", &v)
	return v, err
}

func parseInt64(s string) (int64, error) {
	var v int64
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

func parsePercentage(s string) (float64, error) {
	// Remove % character if present
	s = strings.TrimSuffix(s, "%")
	return parseFloat64(s)
}

// GetName returns the client name
func (c *AlphaVantageClient) GetName() string {
	return "Alpha Vantage"
}

// GetHealth returns the health status of the client
func (c *AlphaVantageClient) GetHealth() string {
	if c.isHealthy {
		return "healthy"
	}
	return "unhealthy"
}

// ClientManager manages multiple data clients with fallback
type ClientManager struct {
	primaryClient   DataClient
	secondaryClient DataClient
}

// NewClientManager creates a new client manager
func NewClientManager(primary, secondary DataClient) *ClientManager {
	return &ClientManager{
		primaryClient:   primary,
		secondaryClient: secondary,
	}
}

// GetStockQuote tries to get a stock quote from the primary client, falls back to secondary
func (m *ClientManager) GetStockQuote(ctx context.Context, symbol string) (*StockQuote, error) {
	quote, err := m.primaryClient.GetStockQuote(ctx, symbol)
	if err != nil {
		// Log the error and try secondary client
		quote, err = m.secondaryClient.GetStockQuote(ctx, symbol)
		if err != nil {
			return nil, fmt.Errorf("all data sources failed: %w", err)
		}
	}
	return quote, nil
}

// GetMarketData tries to get market data from the primary client, falls back to secondary
func (m *ClientManager) GetMarketData(ctx context.Context, index string) (*MarketData, error) {
	data, err := m.primaryClient.GetMarketData(ctx, index)
	if err != nil {
		// Log the error and try secondary client
		data, err = m.secondaryClient.GetMarketData(ctx, index)
		if err != nil {
			return nil, fmt.Errorf("all data sources failed: %w", err)
		}
	}
	return data, nil
}

// GetClientHealth returns the health status of both clients
func (m *ClientManager) GetClientHealth() map[string]string {
	return map[string]string{
		m.primaryClient.GetName():   m.primaryClient.GetHealth(),
		m.secondaryClient.GetName(): m.secondaryClient.GetHealth(),
	}
}