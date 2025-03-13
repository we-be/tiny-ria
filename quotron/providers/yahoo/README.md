# Yahoo Finance Provider

This package provides a unified client for Yahoo Finance data access. It consolidates multiple Yahoo Finance client implementations into a single interface with multiple backend strategies.

## Overview

The Yahoo Finance Provider package offers:

1. A consistent interface for accessing Yahoo Finance data
2. Multiple implementation strategies (direct API, REST API, Python proxy)
3. Automatic health monitoring and reporting
4. Seamless integration with the Quotron models package

## Usage

```go
import (
	"context"
	"fmt"
	"time"
	
	"github.com/we-be/tiny-ria/quotron/models"
	"github.com/we-be/tiny-ria/quotron/providers/yahoo"
)

func main() {
	// Create a client using the Python proxy (most reliable)
	client, err := yahoo.NewClient(yahoo.PythonProxy, 
		yahoo.WithTimeout(30 * time.Second),
		yahoo.WithProxyURL("http://localhost:5000"))
	if err != nil {
		panic(err)
	}
	defer client.Stop()
	
	// Get a stock quote
	ctx := context.Background()
	quote, err := client.GetStockQuote(ctx, "AAPL")
	if err != nil {
		panic(err)
	}
	
	fmt.Printf("AAPL: $%.2f (%.2f%%)\n", quote.Price, quote.ChangePercent)
	
	// Get multiple quotes
	quotes, err := client.GetMultipleQuotes(ctx, []string{"MSFT", "GOOG", "AMZN"})
	if err != nil {
		panic(err)
	}
	
	for symbol, quote := range quotes {
		fmt.Printf("%s: $%.2f (%.2f%%)\n", symbol, quote.Price, quote.ChangePercent)
	}
}
```

## Available Providers

### Python Proxy (Recommended)

Uses the Python `yfinance` library through a Flask proxy server. This is the most reliable option as it uses the robust `yfinance` Python package which handles Yahoo Finance's idiosyncrasies.

```go
client, err := yahoo.NewClient(yahoo.PythonProxy)
```

### REST API

Makes direct HTTP requests to Yahoo Finance's REST API. This avoids the Python dependency but may be less reliable.

```go
client, err := yahoo.NewClient(yahoo.RestAPI)
```

### Direct API

Uses the `finance-go` Go library for direct API access. This is currently not fully implemented.

```go
client, err := yahoo.NewClient(yahoo.DirectAPI)
```

## Health Monitoring

All clients implement the `GetHealthStatus` method which returns a `models.DataSourceHealth` object:

```go
health, err := client.GetHealthStatus(ctx)
if err != nil {
	fmt.Println("Error checking health:", err)
	return
}

fmt.Printf("Yahoo Finance status: %s\n", health.Status)
fmt.Printf("Health score: %.2f/100\n", health.HealthScore)
```

## Configuration Options

- `WithTimeout(duration)`: Sets the request timeout
- `WithProxyURL(url)`: Sets the URL for the Python proxy server
- `WithRetries(count)`: Sets the number of retries for failed requests
- `WithHealthPath(path)`: Sets the health check endpoint path