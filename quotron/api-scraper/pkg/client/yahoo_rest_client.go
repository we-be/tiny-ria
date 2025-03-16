package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/we-be/tiny-ria/quotron/api-scraper/internal/models"
)

// YahooRestClient implements financial data fetching directly from Yahoo Finance REST API
type YahooRestClient struct {
	httpClient *http.Client
	baseURL    string
}

// YahooFinanceResponse represents the response from Yahoo Finance API
type YahooFinanceResponse struct {
	QuoteResponse struct {
		Result []struct {
			Symbol             string  `json:"symbol"`
			RegularMarketPrice float64 `json:"regularMarketPrice"`
			RegularMarketChange float64 `json:"regularMarketChange"`
			RegularMarketChangePercent float64 `json:"regularMarketChangePercent"`
			RegularMarketVolume int64   `json:"regularMarketVolume"`
			RegularMarketTime   int     `json:"regularMarketTime"`
			FullExchangeName    string  `json:"fullExchangeName"`
			ShortName           string  `json:"shortName"`
		} `json:"result"`
		Error interface{} `json:"error"`
	} `json:"quoteResponse"`
}

// NewYahooRestClient creates a new Yahoo Finance REST client
func NewYahooRestClient(timeout time.Duration) *YahooRestClient {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &YahooRestClient{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: "https://query1.finance.yahoo.com",
	}
}

// GetStockQuote fetches a stock quote directly from Yahoo Finance REST API
func (c *YahooRestClient) GetStockQuote(ctx context.Context, symbol string) (*models.StockQuote, error) {
	url := fmt.Sprintf("%s/v7/finance/quote?symbols=%s", c.baseURL, symbol)
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}
	
	// Add headers to make our request look like a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	
	// Implement simple retry logic (3 attempts)
	var resp *http.Response
	var retryErr error
	
	for attempt := 0; attempt < 3; attempt++ {
		resp, retryErr = c.httpClient.Do(req)
		if retryErr == nil && resp.StatusCode == http.StatusOK {
			break
		}
		
		// If we got a response but status is not OK
		if resp != nil {
			resp.Body.Close()
			
			// If not rate limited, don't retry
			if resp.StatusCode != http.StatusTooManyRequests {
				break
			}
		}
		
		// Wait before retrying (exponential backoff)
		select {
		case <-ctx.Done():
			return nil, errors.Wrap(ctx.Err(), "context canceled during retry")
		case <-time.After(time.Duration(attempt+1) * 500 * time.Millisecond):
			// Continue with retry
		}
	}
	
	// Handle final error state
	if retryErr != nil {
		return nil, errors.Wrap(retryErr, "failed to execute request to Yahoo Finance after retries")
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("Yahoo Finance API returned non-200 status: %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}
	
	var yahooResp YahooFinanceResponse
	if err := json.Unmarshal(body, &yahooResp); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}
	
	// Check if we got a valid response with results
	if len(yahooResp.QuoteResponse.Result) == 0 {
		return nil, errors.New("no quote data returned from Yahoo Finance")
	}
	
	// Extract the quote data
	quote := yahooResp.QuoteResponse.Result[0]
	
	stockQuote := &models.StockQuote{
		Symbol:        quote.Symbol,
		Price:         quote.RegularMarketPrice,
		Change:        quote.RegularMarketChange,
		ChangePercent: quote.RegularMarketChangePercent,
		Volume:        quote.RegularMarketVolume,
		Exchange:      quote.FullExchangeName,
		Source:        "Yahoo Finance API",
	}
	
	// Set timestamp
	if quote.RegularMarketTime != 0 {
		stockQuote.Timestamp = time.Unix(int64(quote.RegularMarketTime), 0)
	} else {
		stockQuote.Timestamp = time.Now()
	}
	
	return stockQuote, nil
}

// GetMarketData fetches market index data directly from Yahoo Finance REST API
func (c *YahooRestClient) GetMarketData(ctx context.Context, index string) (*models.MarketData, error) {
	// Map common index symbols to Yahoo Finance symbols if needed
	yahooSymbol := mapIndexSymbol(index)
	
	url := fmt.Sprintf("%s/v7/finance/quote?symbols=%s", c.baseURL, yahooSymbol)
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}
	
	// Add headers to make our request look like a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	
	// Implement simple retry logic (3 attempts)
	var resp *http.Response
	var retryErr error
	
	for attempt := 0; attempt < 3; attempt++ {
		resp, retryErr = c.httpClient.Do(req)
		if retryErr == nil && resp.StatusCode == http.StatusOK {
			break
		}
		
		// If we got a response but status is not OK
		if resp != nil {
			resp.Body.Close()
			
			// If not rate limited, don't retry
			if resp.StatusCode != http.StatusTooManyRequests {
				break
			}
		}
		
		// Wait before retrying (exponential backoff)
		select {
		case <-ctx.Done():
			return nil, errors.Wrap(ctx.Err(), "context canceled during retry")
		case <-time.After(time.Duration(attempt+1) * 500 * time.Millisecond):
			// Continue with retry
		}
	}
	
	// Handle final error state
	if retryErr != nil {
		return nil, errors.Wrap(retryErr, "failed to execute request to Yahoo Finance after retries")
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("Yahoo Finance API returned non-200 status: %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}
	
	var yahooResp YahooFinanceResponse
	if err := json.Unmarshal(body, &yahooResp); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}
	
	// Check if we got a valid response with results
	if len(yahooResp.QuoteResponse.Result) == 0 {
		return nil, errors.New("no market data returned from Yahoo Finance")
	}
	
	// Extract the quote data
	quote := yahooResp.QuoteResponse.Result[0]
	
	// Get a readable name for the index
	indexName := quote.ShortName
	if indexName == "" {
		indexName = yahooIndexName(yahooSymbol)
	}
	
	marketData := &models.MarketData{
		IndexName:     indexName,
		Value:         quote.RegularMarketPrice,
		Change:        quote.RegularMarketChange,
		ChangePercent: quote.RegularMarketChangePercent,
		Source:        "Yahoo Finance API",
	}
	
	// Set timestamp
	if quote.RegularMarketTime != 0 {
		marketData.Timestamp = time.Unix(int64(quote.RegularMarketTime), 0)
	} else {
		marketData.Timestamp = time.Now()
	}
	
	return marketData, nil
}