package realdata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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

// YahooFinanceClient is a client for fetching real financial data directly from Yahoo Finance
type YahooFinanceClient struct {
	httpClient *http.Client
}

// NewYahooFinanceClient creates a new client for Yahoo Finance data
func NewYahooFinanceClient() *YahooFinanceClient {
	return &YahooFinanceClient{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// GetStockQuote fetches stock data from Yahoo Finance
func (c *YahooFinanceClient) GetStockQuote(ctx context.Context, symbol string) (*StockQuote, error) {
	return c.getYahooFinanceData(ctx, symbol)
}

// GetCryptoQuote fetches cryptocurrency data from Yahoo Finance
func (c *YahooFinanceClient) GetCryptoQuote(ctx context.Context, symbol string) (*StockQuote, error) {
	// Ensure symbol has USD suffix
	if !strings.Contains(symbol, "-") {
		symbol = symbol + "-USD"
	}
	
	// Use the same method as for stocks
	return c.getYahooFinanceData(ctx, symbol)
}

// GetMarketData fetches market index data from Yahoo Finance
func (c *YahooFinanceClient) GetMarketData(ctx context.Context, indexSymbol string) (*MarketData, error) {
	// Make sure index symbol has the ^ prefix for Yahoo Finance
	if !strings.HasPrefix(indexSymbol, "^") && !strings.HasPrefix(indexSymbol, "%5E") {
		// Check if it's one of the special indices (SPY, QQQ, DIA)
		switch strings.ToUpper(indexSymbol) {
		case "SPY", "S&P 500", "SP500":
			indexSymbol = "^GSPC" // S&P 500
		case "QQQ", "NASDAQ":
			indexSymbol = "^IXIC" // NASDAQ Composite
		case "DIA", "DOW", "DOW JONES", "DJIA":
			indexSymbol = "^DJI" // Dow Jones Industrial Average
		case "RUSSELL", "RUSSELL 2000":
			indexSymbol = "^RUT" // Russell 2000
		case "VIX":
			indexSymbol = "^VIX" // CBOE Volatility Index
		}
		
		// If it's not one of the special cases and doesn't start with ^, add it
		if !strings.HasPrefix(indexSymbol, "^") {
			indexSymbol = "^" + indexSymbol
		}
	}
	
	// Use the same method as for stocks but convert to MarketData
	stockQuote, err := c.getYahooFinanceData(ctx, indexSymbol)
	if err != nil {
		return nil, err
	}
	
	// Convert StockQuote to MarketData
	marketData := &MarketData{
		IndexName:     indexSymbol,
		Value:         stockQuote.Price,
		Change:        stockQuote.Change,
		ChangePercent: stockQuote.ChangePercent,
		Timestamp:     stockQuote.Timestamp,
		Source:        "Yahoo Finance",
	}
	
	return marketData, nil
}

// getYahooFinanceData fetches stock data from Yahoo Finance API
func (c *YahooFinanceClient) getYahooFinanceData(ctx context.Context, symbol string) (*StockQuote, error) {
	// Base URL for Yahoo Finance API
	apiURL := "https://query1.finance.yahoo.com/v8/finance/chart/"
	
	// Build the URL with symbol and parameters
	fullURL := fmt.Sprintf("%s%s?interval=1d&range=1d", apiURL, url.PathEscape(symbol))
	
	// Create the request
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating Yahoo Finance request: %w", err)
	}
	
	// Set headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	
	// Execute the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making Yahoo Finance request: %w", err)
	}
	defer resp.Body.Close()
	
	// Check for successful response
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Yahoo Finance API returned non-OK status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}
	
	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading Yahoo Finance response: %w", err)
	}
	
	// Parse the JSON response
	var yahooResp yahooFinanceResponse
	if err := json.Unmarshal(bodyBytes, &yahooResp); err != nil {
		return nil, fmt.Errorf("error parsing Yahoo Finance response: %w", err)
	}
	
	// Check for errors in the Yahoo response
	if yahooResp.Chart.Error != nil {
		errorObj, ok := yahooResp.Chart.Error.(map[string]interface{})
		if ok {
			if code, exists := errorObj["code"].(string); exists {
				if description, descExists := errorObj["description"].(string); descExists {
					return nil, fmt.Errorf("Yahoo Finance API error: %s - %s", code, description)
				}
				return nil, fmt.Errorf("Yahoo Finance API error code: %s", code)
			}
		}
		return nil, fmt.Errorf("unknown Yahoo Finance API error")
	}
	
	// Extract the relevant data
	quote, err := c.parseYahooFinanceResponse(yahooResp, symbol)
	if err != nil {
		return nil, fmt.Errorf("error extracting data from Yahoo Finance response: %w", err)
	}
	
	return quote, nil
}

// Parse Yahoo Finance response into StockQuote
func (c *YahooFinanceClient) parseYahooFinanceResponse(resp yahooFinanceResponse, symbol string) (*StockQuote, error) {
	// Check if there are results
	if len(resp.Chart.Result) == 0 {
		return nil, fmt.Errorf("no data found for symbol: %s", symbol)
	}
	
	result := resp.Chart.Result[0]
	meta := result.Meta
	
	// Check if we have quote data
	if len(result.Indicators.Quote) == 0 {
		return nil, fmt.Errorf("no quote data found for symbol: %s", symbol)
	}
	
	quote := result.Indicators.Quote[0]
	
	// Check if we have any data points
	lastIndex := len(quote.Close) - 1
	if lastIndex < 0 {
		return nil, fmt.Errorf("no price data found for symbol: %s", symbol)
	}
	
	// Get the current price (last close)
	currentPrice := quote.Close[lastIndex]
	
	// Calculate change and change percent
	previousClose := meta.PreviousClose
	change := currentPrice - previousClose
	changePercent := (change / previousClose) * 100
	
	// Get volume (if available)
	var volume int64
	if lastIndex < len(quote.Volume) {
		volume = quote.Volume[lastIndex]
	}
	
	// Get timestamp
	var timestamp time.Time
	if len(result.Timestamp) > lastIndex {
		timestamp = time.Unix(result.Timestamp[lastIndex], 0)
	} else {
		timestamp = time.Unix(meta.RegularMarketTime, 0)
	}
	
	// Create the stock quote
	stockQuote := &StockQuote{
		Symbol:        symbol,
		Price:         currentPrice,
		Change:        change,
		ChangePercent: changePercent,
		Volume:        volume,
		Timestamp:     timestamp,
		Exchange:      meta.ExchangeName,
		Source:        "Yahoo Finance",
	}
	
	return stockQuote, nil
}

// Yahoo Finance API Response Structures

type yahooFinanceResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Currency             string  `json:"currency"`
				Symbol               string  `json:"symbol"`
				ExchangeName         string  `json:"exchangeName"`
				InstrumentType       string  `json:"instrumentType"`
				FirstTradeDate       int64   `json:"firstTradeDate"`
				RegularMarketTime    int64   `json:"regularMarketTime"`
				GMTOffset            int     `json:"gmtoffset"`
				Timezone             string  `json:"timezone"`
				RegularMarketPrice   float64 `json:"regularMarketPrice"`
				ChartPreviousClose   float64 `json:"chartPreviousClose"`
				PreviousClose        float64 `json:"previousClose"`
				Scale                int     `json:"scale"`
				PriceHint            int     `json:"priceHint"`
				CurrentTradingPeriod struct {
					Pre struct {
						Timezone  string `json:"timezone"`
						Start     int64  `json:"start"`
						End       int64  `json:"end"`
						GMTOffset int    `json:"gmtoffset"`
					} `json:"pre"`
					Regular struct {
						Timezone  string `json:"timezone"`
						Start     int64  `json:"start"`
						End       int64  `json:"end"`
						GMTOffset int    `json:"gmtoffset"`
					} `json:"regular"`
					Post struct {
						Timezone  string `json:"timezone"`
						Start     int64  `json:"start"`
						End       int64  `json:"end"`
						GMTOffset int    `json:"gmtoffset"`
					} `json:"post"`
				} `json:"currentTradingPeriod"`
				TradingPeriods  [][]interface{} `json:"tradingPeriods"`
				DataGranularity string          `json:"dataGranularity"`
				Range           string          `json:"range"`
				ValidRanges     []string        `json:"validRanges"`
			} `json:"meta"`
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					High   []float64 `json:"high"`
					Volume []int64   `json:"volume"`
					Open   []float64 `json:"open"`
					Low    []float64 `json:"low"`
					Close  []float64 `json:"close"`
				} `json:"quote"`
				Adjclose []struct {
					Adjclose []float64 `json:"adjclose"`
				} `json:"adjclose"`
			} `json:"indicators"`
		} `json:"result"`
		Error interface{} `json:"error"`
	} `json:"chart"`
}