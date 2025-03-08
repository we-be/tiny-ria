package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/tiny-ria/quotron/api-scraper/pkg/client"
)

func main() {
	apiKey := flag.String("api-key", os.Getenv("FINANCE_API_KEY"), "API key for the financial data API")
	baseURL := flag.String("base-url", "https://api.example.com/v1", "Base URL for the financial API")
	symbol := flag.String("symbol", "AAPL", "Stock symbol to fetch")
	flag.Parse()

	if *apiKey == "" {
		log.Fatal("API key is required. Set FINANCE_API_KEY environment variable or provide --api-key flag")
	}

	apiClient := client.NewAPIClient(*baseURL, *apiKey, 10*time.Second)
	
	ctx := context.Background()
	quote, err := apiClient.GetStockQuote(ctx, *symbol)
	if err != nil {
		log.Fatalf("Failed to get stock quote: %v", err)
	}

	fmt.Printf("Quote for %s: $%.2f (%.2f%%)\n", quote.Symbol, quote.Price, quote.ChangePercent)

	// Fetch market data for an index
	marketData, err := apiClient.GetMarketData(ctx, "SPX")
	if err != nil {
		log.Fatalf("Failed to get market data: %v", err)
	}

	fmt.Printf("%s: %.2f (%.2f%%)\n", marketData.IndexName, marketData.Value, marketData.ChangePercent)
}