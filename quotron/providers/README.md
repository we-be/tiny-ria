# Quotron Data Providers

This directory contains data provider packages for Quotron. These packages provide standardized interfaces and client implementations for accessing various data sources.

## Available Providers

### Yahoo Finance

The `yahoo` provider offers a unified interface for accessing Yahoo Finance data. It supports multiple backend strategies:

- **Python Proxy**: Uses the Python `yfinance` library via a Flask proxy server
- **REST API**: Makes direct HTTP requests to Yahoo Finance's REST API
- **Direct API**: Uses the `finance-go` Go library (placeholder implementation)

For more details, see the [Yahoo Finance Provider README](yahoo/README.md).

## Using Providers

Providers are designed to be used with the models package to provide a consistent interface for data access:

```go
import (
    "github.com/we-be/tiny-ria/quotron/models"
    "github.com/we-be/tiny-ria/quotron/providers/yahoo"
)

// Create a Yahoo Finance client
client, err := yahoo.NewClient(yahoo.PythonProxy)
if err != nil {
    panic(err)
}
defer client.Stop()

// Use the client to get data
quote, err := client.GetStockQuote(context.Background(), "AAPL")
if err != nil {
    panic(err)
}

// Process the data
fmt.Printf("AAPL: $%.2f\n", quote.Price)
```

## Health Monitoring

All providers implement health monitoring through a consistent interface:

```go
health, err := client.GetHealthStatus(context.Background())
if err != nil {
    panic(err)
}

fmt.Printf("Status: %s\nHealth Score: %.2f/100\n", health.Status, health.HealthScore)
```

## Future Providers

In the future, additional providers could be added to support other data sources:

- Alpha Vantage
- Finnhub
- IEX Cloud
- Polygon.io
- etc.