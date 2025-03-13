# Quotron Models

This package contains centralized data models for the Quotron system. It serves as the source of truth for all data structures used across the various components.

## Overview

The models package provides:

1. Go data structures for financial data models
2. JSON Schema definitions
3. Auto-generated Python models for use in Python components
4. Utility functions for schema generation and validation

## Key Models

- `StockQuote`: Represents a single stock quote with price, change, volume, etc.
- `MarketIndex`: Represents a market index like S&P 500, NASDAQ, etc.
- `DataBatch`: Represents a batch of financial data being processed
- `BatchStatistics`: Statistical information about a batch of data
- `MarketBatch`: Collection of quotes and indices
- `DataSourceHealth`: Health monitoring information for data sources

## Usage in Go Components

To use these models in a Go component:

```go
import "github.com/we-be/tiny-ria/quotron/models"

// Create a new stock quote
quote := models.StockQuote{
    Symbol:        "AAPL",
    Price:         150.25,
    Change:        1.25,
    ChangePercent: 0.84,
    Volume:        10000000,
    Timestamp:     time.Now(),
    Exchange:      models.NASDAQ,
    Source:        models.APIScraperSource,
}
```

## Usage in Python Components

For Python components, import the auto-generated models:

```python
from quotron.ingest_pipeline.schemas.finance_models import StockQuote, MarketIndex, DataSource

# Create a new stock quote
quote = StockQuote(
    symbol="AAPL",
    price=150.25,
    change=1.25,
    change_percent=0.84,
    volume=10000000,
    timestamp=datetime.now(),
    exchange="NASDAQ",
    source="api-scraper"
)
```

## Generating Schemas

To regenerate the schemas after changing the Go models:

```bash
cd cmd/gen
go run main.go
```

This will update the JSON schema and Python model files.

## Important Notes

1. Always update the Go models first, then regenerate the schemas
2. Never manually edit the generated Python models
3. Schema changes should be backward compatible when possible