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

// NewAPIClientWithUnifiedHealth creates a new Alpha Vantage client with unified health monitoring
func NewAPIClientWithUnifiedHealth(baseURL, apiKey string) (Client, HealthMonitor, error) {
	// Create Alpha Vantage client
	apiClient := NewAPIClient(baseURL, apiKey, 30*time.Second)

	// Create unified health monitor
	healthMonitor, err := NewUnifiedHealthMonitor(
		apiClient,
		"api-scraper",
		"alpha_vantage",
		"Alpha Vantage Financial API",
	)
	if err != nil {
		return apiClient, nil, err
	}

	return apiClient, healthMonitor, nil
}