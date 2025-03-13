package client

import (
	"time"
)

// NewYahooProxyClientWithUnifiedHealth creates a new Yahoo Finance proxy client with unified health monitoring
func NewYahooProxyClientWithUnifiedHealth() (*YahooProxyClient, HealthMonitor, error) {
	// Create Yahoo proxy client
	proxyClient, err := NewYahooProxyClient(30 * time.Second)
	if err != nil {
		return nil, nil, err
	}

	// Create unified health monitor
	healthMonitor, err := NewUnifiedHealthMonitor(
		proxyClient,
		"api-scraper",
		"yahoo_finance_proxy",
		"Yahoo Finance Python Proxy",
	)
	if err != nil {
		return proxyClient, nil, err
	}

	return proxyClient, healthMonitor, nil
}

// NewAlphaVantageClientWithUnifiedHealth creates a new Alpha Vantage client with unified health monitoring
func NewAlphaVantageClientWithUnifiedHealth(apiKey string) (*AlphaVantageClient, HealthMonitor, error) {
	// Create Alpha Vantage client
	alphaClient := NewAlphaVantageClient(apiKey)

	// Create unified health monitor
	healthMonitor, err := NewUnifiedHealthMonitor(
		alphaClient,
		"api-scraper",
		"alpha_vantage",
		"Alpha Vantage Financial API",
	)
	if err != nil {
		return alphaClient, nil, err
	}

	return alphaClient, healthMonitor, nil
}