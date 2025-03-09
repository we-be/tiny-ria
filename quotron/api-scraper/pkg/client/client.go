package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/tiny-ria/quotron/api-scraper/internal/models"
)

// Client defines the interface for financial data clients
type Client interface {
	GetStockQuote(ctx context.Context, symbol string) (*models.StockQuote, error)
	GetMarketData(ctx context.Context, index string) (*models.MarketData, error)
}

// APIClient is a client for fetching financial data from APIs
type APIClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// AlphaVantageResponse represents the response from Alpha Vantage Global Quote API
type AlphaVantageQuoteResponse struct {
	GlobalQuote struct {
		Symbol           string `json:"01. symbol"`
		Price            string `json:"05. price"`
		Change           string `json:"09. change"`
		ChangePercentage string `json:"10. change percent"`
		Volume           string `json:"06. volume"`
		LatestTradingDay string `json:"07. latest trading day"`
	} `json:"Global Quote"`
}

// AlphaVantageIndexResponse represents the response from Alpha Vantage TIME_SERIES_DAILY API
type AlphaVantageIndexResponse struct {
	MetaData struct {
		Symbol string `json:"2. Symbol"`
	} `json:"Meta Data"`
	TimeSeries map[string]struct {
		Open   string `json:"1. open"`
		High   string `json:"2. high"`
		Low    string `json:"3. low"`
		Close  string `json:"4. close"`
		Volume string `json:"5. volume"`
	} `json:"Time Series (Daily)"`
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

// GetStockQuote fetches a stock quote from Alpha Vantage API
func (c *APIClient) GetStockQuote(ctx context.Context, symbol string) (*models.StockQuote, error) {
	url := fmt.Sprintf("%s/query?function=GLOBAL_QUOTE&symbol=%s&apikey=%s", c.baseURL, symbol, c.apiKey)
	
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
	
	var avResponse AlphaVantageQuoteResponse
	if err := json.Unmarshal(body, &avResponse); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}
	
	// Check if we got a valid response
	if avResponse.GlobalQuote.Symbol == "" {
		return nil, errors.New("empty response from Alpha Vantage")
	}
	
	// Parse numeric values
	price, err := strconv.ParseFloat(avResponse.GlobalQuote.Price, 64)
	if err != nil {
		return nil, errors.Wrap(err, "invalid price format")
	}
	
	change, err := strconv.ParseFloat(avResponse.GlobalQuote.Change, 64)
	if err != nil {
		return nil, errors.Wrap(err, "invalid change format")
	}
	
	// Remove % character and parse
	changePercentStr := avResponse.GlobalQuote.ChangePercentage
	if len(changePercentStr) > 0 && changePercentStr[len(changePercentStr)-1] == '%' {
		changePercentStr = changePercentStr[:len(changePercentStr)-1]
	}
	changePercent, err := strconv.ParseFloat(changePercentStr, 64)
	if err != nil {
		return nil, errors.Wrap(err, "invalid change percent format")
	}
	
	volume, err := strconv.ParseInt(avResponse.GlobalQuote.Volume, 10, 64)
	if err != nil {
		return nil, errors.Wrap(err, "invalid volume format")
	}
	
	// Parse date
	timestamp, err := time.Parse("2006-01-02", avResponse.GlobalQuote.LatestTradingDay)
	if err != nil {
		return nil, errors.Wrap(err, "invalid timestamp format")
	}
	
	quote := &models.StockQuote{
		Symbol:        avResponse.GlobalQuote.Symbol,
		Price:         price,
		Change:        change,
		ChangePercent: changePercent,
		Volume:        volume,
		Timestamp:     timestamp,
		Exchange:      "NASDAQ", // Alpha Vantage doesn't provide this directly
		Source:        "Alpha Vantage",
	}
	
	return quote, nil
}

// GetMarketData fetches market index data from Alpha Vantage API
func (c *APIClient) GetMarketData(ctx context.Context, index string) (*models.MarketData, error) {
	// Try TIME_SERIES_DAILY first (more detailed data)
	marketData, err1 := c.getMarketDataFromTimeSeries(ctx, index)
	if err1 == nil {
		return marketData, nil
	}
	
	// If TIME_SERIES_DAILY fails, try GLOBAL_QUOTE as fallback
	marketData, err2 := c.getMarketDataFromGlobalQuote(ctx, index)
	if err2 == nil {
		return marketData, nil
	}
	
	// If both methods fail, return the first error
	return nil, errors.Wrap(err1, fmt.Sprintf("failed to get market data: %v", err2))
}

// getMarketDataFromTimeSeries fetches market data using TIME_SERIES_DAILY endpoint
func (c *APIClient) getMarketDataFromTimeSeries(ctx context.Context, index string) (*models.MarketData, error) {
	// For market indices, we can use TIME_SERIES_DAILY with index symbols
	url := fmt.Sprintf("%s/query?function=TIME_SERIES_DAILY&symbol=%s&apikey=%s", c.baseURL, index, c.apiKey)
	
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
	
	// Check if we got an error note in the response
	if string(body) != "" && string(body)[0] == '{' {
		var errorResponse struct {
			Note string `json:"Note"`
		}
		if json.Unmarshal(body, &errorResponse) == nil && errorResponse.Note != "" {
			return nil, errors.New(errorResponse.Note)
		}
	}
	
	var indexResponse AlphaVantageIndexResponse
	if err := json.Unmarshal(body, &indexResponse); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}
	
	// Get the most recent date
	var mostRecentDate string
	var mostRecentData struct {
		Open   string `json:"1. open"`
		High   string `json:"2. high"`
		Low    string `json:"3. low"`
		Close  string `json:"4. close"`
		Volume string `json:"5. volume"`
	}
	
	for date, data := range indexResponse.TimeSeries {
		if mostRecentDate == "" || date > mostRecentDate {
			mostRecentDate = date
			mostRecentData = data
		}
	}
	
	if mostRecentDate == "" {
		return nil, errors.New("no data available from Alpha Vantage")
	}
	
	// Parse values
	value, err := strconv.ParseFloat(mostRecentData.Close, 64)
	if err != nil {
		return nil, errors.Wrap(err, "invalid close price format")
	}
	
	// For previous close, get the second most recent date
	var prevDate string
	var prevClose float64
	for date, data := range indexResponse.TimeSeries {
		if date < mostRecentDate && (prevDate == "" || date > prevDate) {
			prevDate = date
			prevValue, err := strconv.ParseFloat(data.Close, 64)
			if err == nil {
				prevClose = prevValue
			}
		}
	}
	
	// Calculate change and change percent
	var change, changePercent float64
	if prevClose > 0 {
		change = value - prevClose
		changePercent = (change / prevClose) * 100
	}
	
	// Parse date
	timestamp, err := time.Parse("2006-01-02", mostRecentDate)
	if err != nil {
		return nil, errors.Wrap(err, "invalid date format")
	}
	
	// Map index symbols to readable names
	indexName := getReadableIndexName(index)
	
	marketData := &models.MarketData{
		IndexName:     indexName,
		Value:         value,
		Change:        change,
		ChangePercent: changePercent,
		Timestamp:     timestamp,
		Source:        "Alpha Vantage",
	}
	
	return marketData, nil
}

// getMarketDataFromGlobalQuote fetches market data using GLOBAL_QUOTE endpoint
func (c *APIClient) getMarketDataFromGlobalQuote(ctx context.Context, index string) (*models.MarketData, error) {
	// Try using the stock quote endpoint for the index
	url := fmt.Sprintf("%s/query?function=GLOBAL_QUOTE&symbol=%s&apikey=%s", c.baseURL, index, c.apiKey)
	
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
	
	// Check if we got an error note in the response
	if string(body) != "" && string(body)[0] == '{' {
		var errorResponse struct {
			Note string `json:"Note"`
		}
		if json.Unmarshal(body, &errorResponse) == nil && errorResponse.Note != "" {
			return nil, errors.New(errorResponse.Note)
		}
	}
	
	var avResponse AlphaVantageQuoteResponse
	if err := json.Unmarshal(body, &avResponse); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}
	
	// Check if we got a valid response
	if avResponse.GlobalQuote.Symbol == "" {
		return nil, errors.New("empty response from Alpha Vantage")
	}
	
	// Parse numeric values
	value, err := strconv.ParseFloat(avResponse.GlobalQuote.Price, 64)
	if err != nil {
		return nil, errors.Wrap(err, "invalid price format")
	}
	
	change, err := strconv.ParseFloat(avResponse.GlobalQuote.Change, 64)
	if err != nil {
		return nil, errors.Wrap(err, "invalid change format")
	}
	
	// Remove % character and parse
	changePercentStr := avResponse.GlobalQuote.ChangePercentage
	if len(changePercentStr) > 0 && changePercentStr[len(changePercentStr)-1] == '%' {
		changePercentStr = changePercentStr[:len(changePercentStr)-1]
	}
	changePercent, err := strconv.ParseFloat(changePercentStr, 64)
	if err != nil {
		return nil, errors.Wrap(err, "invalid change percent format")
	}
	
	// Parse date
	timestamp, err := time.Parse("2006-01-02", avResponse.GlobalQuote.LatestTradingDay)
	if err != nil {
		return nil, errors.Wrap(err, "invalid timestamp format")
	}
	
	// Map index symbols to readable names
	indexName := getReadableIndexName(index)
	
	marketData := &models.MarketData{
		IndexName:     indexName,
		Value:         value,
		Change:        change,
		ChangePercent: changePercent,
		Timestamp:     timestamp,
		Source:        "Alpha Vantage",
	}
	
	return marketData, nil
}

// getReadableIndexName returns a human-readable name for an index symbol
func getReadableIndexName(index string) string {
	switch index {
	case "^DJI":
		return "Dow Jones Industrial Average"
	case "^GSPC":
		return "S&P 500"
	case "^IXIC":
		return "NASDAQ Composite"
	case "^RUT":
		return "Russell 2000"
	case "^VIX":
		return "CBOE Volatility Index"
	default:
		return index
	}
}