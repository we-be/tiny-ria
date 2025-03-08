package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/tiny-ria/quotron/api-scraper/internal/models"
)

// APIClient is a client for fetching financial data from APIs
type APIClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewAPIClient creates a new API client
func NewAPIClient(baseURL, apiKey string, timeout time.Duration) *APIClient {
	return &APIClient{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

// GetStockQuote fetches a stock quote from the API
func (c *APIClient) GetStockQuote(ctx context.Context, symbol string) (*models.StockQuote, error) {
	url := fmt.Sprintf("%s/quote?symbol=%s&apikey=%s", c.baseURL, symbol, c.apiKey)
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute request")
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("API returned non-200 status: %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}
	
	var quote models.StockQuote
	if err := json.Unmarshal(body, &quote); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}
	
	return &quote, nil
}

// GetMarketData fetches market data from the API
func (c *APIClient) GetMarketData(ctx context.Context, index string) (*models.MarketData, error) {
	url := fmt.Sprintf("%s/market?index=%s&apikey=%s", c.baseURL, index, c.apiKey)
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute request")
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("API returned non-200 status: %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}
	
	var marketData models.MarketData
	if err := json.Unmarshal(body, &marketData); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}
	
	return &marketData, nil
}