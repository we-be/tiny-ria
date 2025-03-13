// Package yahoo provides a unified client for Yahoo Finance data access
package yahoo

import (
	"context"
	"time"

	"github.com/we-be/tiny-ria/quotron/models"
)

// ProviderType defines the type of Yahoo Finance provider implementation
type ProviderType string

const (
	// DirectAPI uses the finance-go library for direct API access
	DirectAPI ProviderType = "direct"
	// RestAPI uses direct HTTP calls to Yahoo Finance REST API
	RestAPI ProviderType = "rest"
	// PythonProxy uses the Python yfinance proxy server
	PythonProxy ProviderType = "proxy"
)

// Client defines the interface for Yahoo Finance data access
type Client interface {
	// GetStockQuote retrieves a stock quote for the given symbol
	GetStockQuote(ctx context.Context, symbol string) (*models.StockQuote, error)
	
	// GetMarketData retrieves market data for the given index symbol
	GetMarketData(ctx context.Context, symbol string) (*models.MarketIndex, error)
	
	// GetMultipleQuotes retrieves quotes for multiple symbols
	GetMultipleQuotes(ctx context.Context, symbols []string) (map[string]*models.StockQuote, error)
	
	// GetHealthStatus returns the health status of the client
	GetHealthStatus(ctx context.Context) (*models.DataSourceHealth, error)
	
	// Stop stops the client and releases resources (particularly for the proxy client)
	Stop() error
}

// ClientOption defines options for creating a Yahoo Finance client
type ClientOption func(*clientOptions)

// clientOptions holds configuration options for the client
type clientOptions struct {
	timeout    time.Duration
	proxyURL   string
	retries    int
	healthPath string
}

// WithTimeout sets the timeout for API requests
func WithTimeout(timeout time.Duration) ClientOption {
	return func(o *clientOptions) {
		o.timeout = timeout
	}
}

// WithProxyURL sets the URL for the Python proxy server
func WithProxyURL(url string) ClientOption {
	return func(o *clientOptions) {
		o.proxyURL = url
	}
}

// WithRetries sets the number of retries for failed requests
func WithRetries(retries int) ClientOption {
	return func(o *clientOptions) {
		o.retries = retries
	}
}

// WithHealthPath sets the health check endpoint for the proxy server
func WithHealthPath(path string) ClientOption {
	return func(o *clientOptions) {
		o.healthPath = path
	}
}

// defaultOptions returns the default client options
func defaultOptions() *clientOptions {
	return &clientOptions{
		timeout:    30 * time.Second,
		proxyURL:   "http://localhost:5000",
		retries:    3,
		healthPath: "/health",
	}
}

// NewClient creates a new Yahoo Finance client with the specified provider type
func NewClient(providerType ProviderType, opts ...ClientOption) (Client, error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	switch providerType {
	case DirectAPI:
		return newYahooClient(options)
	case RestAPI:
		return newRestClient(options)
	case PythonProxy:
		return newProxyClient(options)
	default:
		// Default to Python proxy as it's the most reliable
		return newProxyClient(options)
	}
}