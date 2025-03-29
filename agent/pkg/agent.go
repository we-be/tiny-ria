package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/we-be/tiny-ria/quotron/scheduler/pkg/client"
)

// Agent represents an autonomous agent that can interact with Quotron services
type Agent struct {
	name          string
	apiClient     *client.APIClient
	logger        *log.Logger
	queuePublisher *QueuePublisher
}

// AgentConfig holds configuration parameters for the agent
type AgentConfig struct {
	Name       string
	APIHost    string
	APIPort    int
	LogPrefix  string
	RedisAddr  string // Redis server address (optional)
	EnableQueue bool  // Enable publishing to message queue
}

// NewAgent creates a new Agent instance
func NewAgent(config AgentConfig) *Agent {
	logPrefix := config.LogPrefix
	if logPrefix == "" {
		logPrefix = fmt.Sprintf("[Agent:%s] ", config.Name)
	}

	logger := log.New(log.Writer(), logPrefix, log.LstdFlags)
	
	agent := &Agent{
		name:      config.Name,
		apiClient: client.NewAPIClient(config.APIHost, config.APIPort),
		logger:    logger,
	}
	
	// Set up Redis publisher if enabled
	if config.EnableQueue {
		redisAddr := config.RedisAddr
		if redisAddr == "" {
			redisAddr = "localhost:6379" // Default Redis address
		}
		
		publisher, err := NewQueuePublisher(redisAddr, logger)
		if err != nil {
			logger.Printf("Warning: Failed to initialize Redis publisher: %v", err)
			logger.Printf("Alerts will not be published to the message queue")
		} else {
			agent.queuePublisher = publisher
			logger.Printf("Redis publisher initialized, alerts will be published to the queue")
		}
	}
	
	return agent
}

// FetchStockData fetches stock data for given symbols
func (a *Agent) FetchStockData(ctx context.Context, symbols []string) (map[string]*client.StockQuote, error) {
	a.logger.Printf("Fetching stock data for %d symbols: %s", len(symbols), strings.Join(symbols, ", "))
	
	results := make(map[string]*client.StockQuote)
	var errors []string

	for _, symbol := range symbols {
		quote, err := a.apiClient.GetStockQuote(ctx, symbol)
		if err != nil {
			a.logger.Printf("Error fetching stock quote for %s: %v", symbol, err)
			errors = append(errors, fmt.Sprintf("%s: %v", symbol, err))
			continue
		}
		
		results[symbol] = quote
		a.logger.Printf("Successfully fetched stock quote for %s: Price=$%.2f", symbol, quote.Price)
	}
	
	if len(errors) > 0 {
		return results, fmt.Errorf("errors fetching some stock quotes: %s", strings.Join(errors, "; "))
	}
	
	return results, nil
}

// FetchCryptoData fetches cryptocurrency data for given symbols
func (a *Agent) FetchCryptoData(ctx context.Context, symbols []string) (map[string]*client.StockQuote, error) {
	a.logger.Printf("Fetching crypto data for %d symbols: %s", len(symbols), strings.Join(symbols, ", "))
	
	results := make(map[string]*client.StockQuote)
	var errors []string

	for _, symbol := range symbols {
		quote, err := a.apiClient.GetCryptoQuote(ctx, symbol)
		if err != nil {
			a.logger.Printf("Error fetching crypto quote for %s: %v", symbol, err)
			errors = append(errors, fmt.Sprintf("%s: %v", symbol, err))
			continue
		}
		
		results[symbol] = quote
		a.logger.Printf("Successfully fetched crypto quote for %s: Price=$%.2f", symbol, quote.Price)
	}
	
	if len(errors) > 0 {
		return results, fmt.Errorf("errors fetching some crypto quotes: %s", strings.Join(errors, "; "))
	}
	
	return results, nil
}

// FetchMarketData fetches market index data for given indices
func (a *Agent) FetchMarketData(ctx context.Context, indices []string) (map[string]*client.MarketData, error) {
	a.logger.Printf("Fetching market data for %d indices: %s", len(indices), strings.Join(indices, ", "))
	
	results := make(map[string]*client.MarketData)
	var errors []string

	for _, index := range indices {
		data, err := a.apiClient.GetMarketData(ctx, index)
		if err != nil {
			a.logger.Printf("Error fetching market data for %s: %v", index, err)
			errors = append(errors, fmt.Sprintf("%s: %v", index, err))
			continue
		}
		
		results[index] = data
		a.logger.Printf("Successfully fetched market data for %s: Value=%.2f", index, data.Value)
	}
	
	if len(errors) > 0 {
		return results, fmt.Errorf("errors fetching some market data: %s", strings.Join(errors, "; "))
	}
	
	return results, nil
}

// MonitorStocks continually monitors stock prices and performs actions based on price movements
func (a *Agent) MonitorStocks(ctx context.Context, symbols []string, interval time.Duration, 
	alertThresholdPercent float64, callback func(symbol string, quote interface{}, percentChange float64)) {
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	// Store the baseline prices to compare against
	baselinePrices := make(map[string]float64)
	
	// Initial fetch to establish baseline
	a.logger.Printf("Establishing baseline prices for monitoring...")
	quotes, err := a.FetchStockData(ctx, symbols)
	if err != nil {
		a.logger.Printf("Warning: Error establishing all baseline prices: %v", err)
	}
	
	for symbol, quote := range quotes {
		baselinePrices[symbol] = quote.Price
		a.logger.Printf("Baseline price for %s: $%.2f", symbol, quote.Price)
	}
	
	a.logger.Printf("Starting price monitoring for %d symbols with %.2f%% threshold...", 
		len(symbols), alertThresholdPercent)
	
	for {
		select {
		case <-ctx.Done():
			a.logger.Printf("Monitoring stopped due to context cancellation")
			return
			
		case <-ticker.C:
			a.logger.Printf("Checking current prices...")
			
			currentQuotes, err := a.FetchStockData(ctx, symbols)
			if err != nil {
				a.logger.Printf("Warning: Error fetching some current prices: %v", err)
			}
			
			for symbol, quote := range currentQuotes {
				if baseline, ok := baselinePrices[symbol]; ok {
					percentChange := ((quote.Price - baseline) / baseline) * 100
					
					absPctChange := percentChange
					if absPctChange < 0 {
						absPctChange = -absPctChange
					}
					
					if absPctChange >= alertThresholdPercent {
						direction := "increased"
						if percentChange < 0 {
							direction = "decreased"
						}
						
						a.logger.Printf("ALERT: %s has moved %.2f%% from baseline $%.2f to $%.2f", 
							symbol, percentChange, baseline, quote.Price)
						
						// Call user-provided callback if any
						if callback != nil {
							callback(symbol, quote, percentChange)
						}
						
						// Publish to message queue if available
						if a.queuePublisher != nil {
							// Create alert message
							alert := AlertMessage{
								Symbol:        symbol,
								Price:         quote.Price,
								PreviousPrice: baseline,
								PercentChange: percentChange,
								Volume:        quote.Volume,
								Timestamp:     time.Now(),
								Direction:     direction,
							}
							
							// Publish to queue
							err := a.queuePublisher.PublishAlert(ctx, alert)
							if err != nil {
								a.logger.Printf("Error publishing alert to queue: %v", err)
							}
						}
						
						// Update baseline after a significant movement
						baselinePrices[symbol] = quote.Price
						a.logger.Printf("Updated baseline for %s to $%.2f", symbol, quote.Price)
					}
				}
			}
		}
	}
}

// GetPortfolioSummary retrieves and summarizes data for a portfolio of stocks and cryptos
func (a *Agent) GetPortfolioSummary(ctx context.Context, stocks []string, cryptos []string) (string, error) {
	a.logger.Printf("Generating portfolio summary for %d stocks and %d cryptos", 
		len(stocks), len(cryptos))
	
	// Fetch stock data
	stockQuotes, err := a.FetchStockData(ctx, stocks)
	if err != nil {
		a.logger.Printf("Warning: Error fetching some stock data: %v", err)
	}
	
	// Fetch crypto data
	cryptoQuotes, err := a.FetchCryptoData(ctx, cryptos)
	if err != nil {
		a.logger.Printf("Warning: Error fetching some crypto data: %v", err)
	}
	
	// Generate summary
	var b strings.Builder
	
	b.WriteString("# Portfolio Summary\n")
	b.WriteString(fmt.Sprintf("Generated at: %s\n\n", time.Now().Format(time.RFC1123)))
	
	// Stock summary
	b.WriteString("## Stocks\n\n")
	if len(stockQuotes) > 0 {
		b.WriteString("| Symbol | Price | Change | Change % | Volume |\n")
		b.WriteString("|--------|-------|--------|----------|--------|\n")
		
		for _, symbol := range stocks {
			if quote, ok := stockQuotes[symbol]; ok {
				b.WriteString(fmt.Sprintf("| %s | $%.2f | %.2f | %.2f%% | %s |\n", 
					symbol, quote.Price, quote.Change, quote.ChangePercent, 
					formatNumber(quote.Volume)))
			} else {
				b.WriteString(fmt.Sprintf("| %s | N/A | N/A | N/A | N/A |\n", symbol))
			}
		}
	} else {
		b.WriteString("No stock data available\n")
	}
	
	b.WriteString("\n")
	
	// Crypto summary
	b.WriteString("## Cryptocurrencies\n\n")
	if len(cryptoQuotes) > 0 {
		b.WriteString("| Symbol | Price | Change | Change % | Volume |\n")
		b.WriteString("|--------|-------|--------|----------|--------|\n")
		
		for _, symbol := range cryptos {
			if quote, ok := cryptoQuotes[symbol]; ok {
				b.WriteString(fmt.Sprintf("| %s | $%.2f | %.2f | %.2f%% | %s |\n", 
					symbol, quote.Price, quote.Change, quote.ChangePercent,
					formatNumber(quote.Volume)))
			} else {
				b.WriteString(fmt.Sprintf("| %s | N/A | N/A | N/A | N/A |\n", symbol))
			}
		}
	} else {
		b.WriteString("No cryptocurrency data available\n")
	}
	
	// Calculate totals
	var totalValue float64
	var totalChange float64
	
	for _, quote := range stockQuotes {
		totalValue += quote.Price
		totalChange += quote.Change
	}
	
	for _, quote := range cryptoQuotes {
		totalValue += quote.Price
		totalChange += quote.Change
	}
	
	b.WriteString("\n## Summary\n\n")
	b.WriteString(fmt.Sprintf("Total portfolio value: $%.2f\n", totalValue))
	b.WriteString(fmt.Sprintf("Total change: $%.2f\n", totalChange))
	
	if totalChange >= 0 {
		b.WriteString("Status: POSITIVE 📈\n")
	} else {
		b.WriteString("Status: NEGATIVE 📉\n")
	}
	
	return b.String(), nil
}

// SaveDataToJSON saves agent data to a JSON file
func (a *Agent) SaveDataToJSON(data interface{}, filePath string) error {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling data to JSON: %w", err)
	}
	
	a.logger.Printf("Saving data to file: %s", filePath)
	// This would normally write to a file, but for now, just log the JSON
	a.logger.Printf("JSON data: %s", string(jsonBytes))
	
	return nil
}

// Helper function to format large numbers
func formatNumber(n int64) string {
	if n >= 1_000_000_000 {
		return fmt.Sprintf("%.1fB", float64(n)/1_000_000_000)
	} else if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	} else if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

// Close closes the agent and its resources
func (a *Agent) Close() error {
	if a.queuePublisher != nil {
		return a.queuePublisher.Close()
	}
	return nil
}