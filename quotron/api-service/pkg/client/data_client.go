package client

import (
	"context"
	"fmt"
	"time"
)

// StockQuote represents stock quote data
type StockQuote struct {
	Symbol        string    `json:"symbol"`
	Price         float64   `json:"price"`
	Change        float64   `json:"change"`
	ChangePercent float64   `json:"change_percent"`
	Volume        int64     `json:"volume"`
	Timestamp     time.Time `json:"timestamp"`
	Exchange      string    `json:"exchange"`
	Source        string    `json:"source"`
}

// MarketData represents market index data
type MarketData struct {
	IndexName     string    `json:"index_name"`
	Value         float64   `json:"value"`
	Change        float64   `json:"change"`
	ChangePercent float64   `json:"change_percent"`
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
	// For now, return mock data
	// In a real implementation, this would call the YFinance proxy
	return &StockQuote{
		Symbol:        symbol,
		Price:         150.25,
		Change:        2.75,
		ChangePercent: 1.87,
		Volume:        5000000,
		Timestamp:     time.Now(),
		Exchange:      "NASDAQ",
		Source:        "Yahoo Finance",
	}, nil
}

// GetMarketData fetches market index data from the Yahoo Finance proxy
func (c *YahooProxyClient) GetMarketData(ctx context.Context, index string) (*MarketData, error) {
	// For now, return mock data
	// In a real implementation, this would call the YFinance proxy
	return &MarketData{
		IndexName:     index,
		Value:         4200.50,
		Change:        25.75,
		ChangePercent: 0.62,
		Timestamp:     time.Now(),
		Source:        "Yahoo Finance",
	}, nil
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
	// For now, return mock data
	// In a real implementation, this would call the Alpha Vantage API
	return &StockQuote{
		Symbol:        symbol,
		Price:         149.99,
		Change:        -0.50,
		ChangePercent: -0.33,
		Volume:        6500000,
		Timestamp:     time.Now(),
		Exchange:      "NYSE",
		Source:        "Alpha Vantage",
	}, nil
}

// GetMarketData fetches market index data from Alpha Vantage
func (c *AlphaVantageClient) GetMarketData(ctx context.Context, index string) (*MarketData, error) {
	// For now, return mock data
	// In a real implementation, this would call the Alpha Vantage API
	return &MarketData{
		IndexName:     index,
		Value:         4195.75,
		Change:        -5.25,
		ChangePercent: -0.12,
		Timestamp:     time.Now(),
		Source:        "Alpha Vantage",
	}, nil
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