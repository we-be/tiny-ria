package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/tiny-ria/quotron/api-scraper/internal/models"
	"github.com/tiny-ria/quotron/api-scraper/pkg/client"
)

func main() {
	// Alpha Vantage API key from https://www.alphavantage.co/support/#api-key
	apiKey := flag.String("api-key", os.Getenv("ALPHA_VANTAGE_API_KEY"), "Alpha Vantage API key")
	baseURL := flag.String("base-url", "https://www.alphavantage.co", "Base URL for Alpha Vantage API")
	symbol := flag.String("symbol", "AAPL", "Stock symbol to fetch")
	index := flag.String("index", "^GSPC", "Market index to fetch (^GSPC, ^DJI, ^IXIC)")
	outputJson := flag.Bool("json", false, "Output results as JSON")
	flag.Parse()

	if *apiKey == "" {
		log.Fatal("API key is required. Set ALPHA_VANTAGE_API_KEY environment variable or provide --api-key flag")
	}

	apiClient := client.NewAPIClient(*baseURL, *apiKey, 30*time.Second)
	ctx := context.Background()
	
	// Fetch stock quote
	fmt.Printf("Fetching quote for %s...\n", *symbol)
	quote, err := apiClient.GetStockQuote(ctx, *symbol)
	if err != nil {
		log.Printf("Failed to get stock quote: %v", err)
	} else {
		if *outputJson {
			quoteJson, _ := json.MarshalIndent(quote, "", "  ")
			fmt.Println(string(quoteJson))
		} else {
			fmt.Printf("Quote for %s as of %s:\n", quote.Symbol, quote.Timestamp.Format("2006-01-02"))
			fmt.Printf("  Price:  $%.2f\n", quote.Price)
			fmt.Printf("  Change: $%.2f (%.2f%%)\n", quote.Change, quote.ChangePercent)
			fmt.Printf("  Volume: %d\n", quote.Volume)
			fmt.Printf("  Source: %s\n\n", quote.Source)
		}
	}

	// Use index parameter if provided, otherwise default
	indexSymbol := *index
	if indexSymbol == "" {
		indexSymbol = "^GSPC" // Default to S&P 500
	}
	
	// Fetch market data for an index
	fmt.Printf("Fetching market data for %s...\n", indexSymbol)
	marketData, err := apiClient.GetMarketData(ctx, indexSymbol)
	if err != nil {
		// Check if the error contains information about API limits or timing
		if strings.Contains(err.Error(), "API call frequency") ||
		   strings.Contains(err.Error(), "Thank you for using Alpha Vantage") {
			log.Printf("Alpha Vantage API limit reached: %v", err)
		} else {
			log.Printf("Failed to get market data: %v", err)
		}
		
		// If we got a quote, don't exit with error even if market data failed
		if quote != nil {
			// Not a critical error if at least the stock quote worked
			fmt.Println("Note: Market data may not be available with your current API key.")
		}
	} else {
		if *outputJson {
			marketJson, _ := json.MarshalIndent(marketData, "", "  ")
			fmt.Println(string(marketJson))
		} else {
			fmt.Printf("%s as of %s:\n", marketData.IndexName, marketData.Timestamp.Format("2006-01-02"))
			fmt.Printf("  Value:  %.2f\n", marketData.Value)
			fmt.Printf("  Change: %.2f (%.2f%%)\n", marketData.Change, marketData.ChangePercent)
			fmt.Printf("  Source: %s\n", marketData.Source)
		}
	}
	
	// Only exit with error if both operations failed
	if quote == nil && marketData == nil {
		os.Exit(1)
	}
}

// Print quote as a row for table output
func printQuoteRow(quote *models.StockQuote) {
	changeSign := ""
	if quote.Change > 0 {
		changeSign = "+"
	}
	fmt.Printf("%-6s | $%8.2f | %s$%6.2f | %s%6.2f%% | %12d | %s\n",
		quote.Symbol,
		quote.Price,
		changeSign,
		quote.Change,
		changeSign,
		quote.ChangePercent,
		quote.Volume,
		quote.Timestamp.Format("2006-01-02"),
	)
}