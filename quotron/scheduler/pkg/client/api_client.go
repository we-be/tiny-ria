package client

// For go mod init github.com/we-be/tiny-ria/quotron/scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// APIClient is a client for the API service
type APIClient struct {
	baseURL    string
	httpClient *http.Client
	realClient *RealFinanceAPIClient
}

// NewAPIClient creates a new API client
func NewAPIClient(host string, port int) *APIClient {
	return &APIClient{
		baseURL: fmt.Sprintf("http://%s:%d", host, port),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetStockQuote fetches a stock quote from the API service
func (c *APIClient) GetStockQuote(ctx context.Context, symbol string) (*StockQuote, error) {
	// If real finance client is available, use it
	if c.realClient != nil {
		return c.realClient.GetYahooFinanceData(ctx, symbol)
	}

	// Otherwise use the local API service
	url := fmt.Sprintf("%s/api/quote/%s", c.baseURL, url.PathEscape(symbol))
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned non-OK status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var quote StockQuote
	if err := json.NewDecoder(resp.Body).Decode(&quote); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return &quote, nil
}

// GetMarketData fetches market index data from the API service
func (c *APIClient) GetMarketData(ctx context.Context, index string) (*MarketData, error) {
	// If real finance client is available, use it
	if c.realClient != nil {
		return c.realClient.GetMarketIndex(ctx, index)
	}

	// Otherwise use the local API service
	url := fmt.Sprintf("%s/api/index/%s", c.baseURL, url.PathEscape(index))
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned non-OK status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var marketData MarketData
	if err := json.NewDecoder(resp.Body).Decode(&marketData); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return &marketData, nil
}

// GetHealth checks the health of the API service
func (c *APIClient) GetHealth(ctx context.Context) (bool, error) {
	url := fmt.Sprintf("%s/api/health", c.baseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("error creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// GetCryptoQuote fetches a crypto quote from the API service
func (c *APIClient) GetCryptoQuote(ctx context.Context, symbol string) (*StockQuote, error) {
	// If real finance client is available, use it
	if c.realClient != nil {
		return c.realClient.GetCryptoData(ctx, symbol)
	}

	// Otherwise use the local API service
	url := fmt.Sprintf("%s/api/crypto/%s", c.baseURL, url.PathEscape(symbol))
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned non-OK status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var quote StockQuote
	if err := json.NewDecoder(resp.Body).Decode(&quote); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return &quote, nil
}

// MapExchangeToEnum maps various exchange codes to the standard enum values
// Enum values are: NYSE, NASDAQ, AMEX, OTC, OTHER
func MapExchangeToEnum(exchange string) string {
	switch exchange {
	case "NYSE":
		return "NYSE"
	case "NASDAQ", "NMS", "NGS", "NAS", "NCM":
		return "NASDAQ"
	case "AMEX", "ASE", "CBOE":
		return "AMEX"
	case "OTC", "OTCBB", "OTC PINK":
		return "OTC"
	default:
		return "OTHER"
	}
}