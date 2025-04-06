# Quotron ETL Pipeline (Go Implementation)

A high-performance Go-based ETL (Extract, Transform, Load) pipeline for processing financial data in the Quotron system.

## IMPORTANT: CLI Integration Only

The ETL pipeline is **NOT** a standalone application. It is designed to be used exclusively by the Quotron CLI. There is no separate binary or command-line interface for ETL.

To interact with the ETL pipeline, use the Quotron CLI:

```bash
# Process a JSON file containing stock quotes
quotron etl process-quotes path/to/quotes.json --source api-scraper

# Process a JSON file containing market indices
quotron etl process-indices path/to/indices.json --source browser-scraper

# Process a JSON file containing both quotes and indices
quotron etl process-mixed path/to/mixed.json --source api-scraper
```

## Features

- **High Performance**: Utilizes Go's concurrency with goroutines for parallel processing (~10x faster than Python)
- **Scalable**: Designed to handle large volumes of financial data
- **Database Integration**: Direct PostgreSQL integration with connection pooling
- **Data Validation**: Robust validation of financial data
- **Statistical Analysis**: Concurrent computation of statistical metrics

## Architecture

- **Models**: Type definitions for financial data structures
- **Validation**: Concurrent data validation and cleaning
- **Enrichment**: Data enrichment and statistical computation
- **Database**: PostgreSQL database operations with connection pooling
- **Pipeline**: Core ETL pipeline with parallel processing
- **Service**: Interface for CLI integration

## Dependencies

- Go 1.18+
- PostgreSQL 13+
- Required Go packages:
  - github.com/jmoiron/sqlx
  - github.com/lib/pq
  - github.com/go-playground/validator/v10

## Database Configuration

The ETL pipeline reads database configuration from:

1. Environment variables:
   - `DB_HOST`: Database hostname (default: "localhost")
   - `DB_PORT`: Database port (default: 5432)
   - `DB_NAME`: Database name (default: "quotron")
   - `DB_USER`: Database username (default: "quotron")
   - `DB_PASSWORD`: Database password (default: "quotron")
   - `DB_SSL_MODE`: SSL mode (default: "disable")

2. `.env` file in the current directory, parent directory, or grandparent directory.

Example `.env` file:
```
DB_HOST=localhost
DB_PORT=5432
DB_NAME=quotron
DB_USER=postgres
DB_PASSWORD=postgres
```

## Performance Comparison

Initial benchmarks show significant performance improvements compared to the Python implementation:

| Metric | Python | Go | Improvement |
|--------|--------|-----|-------------|
| Throughput | ~100 quotes/sec | ~1000 quotes/sec | 10x |
| Memory usage | ~100MB | ~20MB | 5x less |
| CPU usage | High | Moderate | ~3x less |
| Concurrent batches | Limited | Scales with cores | Significant |

## Integration

The ETL pipeline integrates with:

- **API Scraper**: Processes data from the Go-based API scraper
- **Browser Scraper**: Processes data from the Python-based browser scraper
- **Scheduler**: Can be triggered by the scheduler for automated processing
- **CLI**: The only entry point for ETL operations

## Development

### Code Structure

```
quotron/etl/
├── internal/
│   ├── models/        # Data models
│   ├── db/            # Database operations
│   └── pipeline/      # Core pipeline implementation
├── pkg/
│   └── etlservice/    # Service interface for CLI
└── README.md          # Documentation
```

### Testing

```bash
# Run all tests (requires database connection)
go test ./...

# Run tests with race detector
go test -race ./...

# Run benchmarks
go test -bench=. ./...
```

### Testing Requirements

- All tests require a valid database connection
- ETL functionality is tightly coupled with database operations
- Tests will fail if a database connection cannot be established