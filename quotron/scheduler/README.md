# Scheduler

A Go-based scheduler for running API scraper jobs on a defined schedule.

## Features

- Cron-based scheduling of data collection jobs
- Flexible configuration of job schedules and parameters
- Support for manual job execution
- Automatic discovery of API scraper binary
- Environment variable support for API keys
- Integration with database storage pipeline
- Automatic fallback to alternative data sources

## Jobs

The scheduler currently supports the following job types:

- **Stock Quotes**: Fetches stock price data for configured symbols
- **Market Indices**: Fetches market index data for configured indices

## Database Integration

The scheduler is integrated with the Quotron data pipeline:

1. Stock and index data is saved to JSON files in the `data/` directory
2. Files are automatically processed by the ingest pipeline
3. Data is validated, enriched, and stored in the PostgreSQL database
4. Statistical data is computed and stored with each batch

## Usage

### Database Setup

Before running the scheduler, ensure the database is set up:

```bash
cd quotron/ingest-pipeline
python cli.py setup
```

### Building

```bash
cd quotron/scheduler
go mod tidy
go build -o scheduler ./cmd/scheduler
```

### Running

```bash
# Run with default configuration
./scheduler -api-scraper=/path/to/api-scraper

# Run with custom configuration
./scheduler -config=scheduler-config.json -api-scraper=/path/to/api-scraper

# Generate a default configuration file
./scheduler -gen-config

# Run all jobs once and exit
./scheduler -run-once -api-scraper=/path/to/api-scraper

# Run a specific job once and exit
./scheduler -run-job=stock_quotes -api-scraper=/path/to/api-scraper
```

## Configuration

The scheduler uses a JSON configuration file with the following structure:

```json
{
  "api_key": "YOUR_API_KEY",
  "api_base_url": "https://www.alphavantage.co/query",
  "log_level": "info",
  "timezone": "UTC",
  "retention": 604800000000000,
  "schedules": {
    "stock_quotes": {
      "cron": "*/30 * * * *",
      "enabled": true,
      "description": "Fetch stock quotes for tracked symbols",
      "parameters": {
        "symbols": "AAPL,MSFT,GOOG,AMZN"
      }
    },
    "market_indices": {
      "cron": "0 * * * *",
      "enabled": true,
      "description": "Fetch market indices data",
      "parameters": {
        "indices": "SPY,QQQ,DIA"
      }
    }
  }
}
```

### Environment Variables

- `ALPHA_VANTAGE_API_KEY`: API key for Alpha Vantage (overrides config file)

### API Limitations

The free Alpha Vantage API has the following limitations:
- 25 API requests per day
- Limited access to certain data types (some market indices may not be available)
- 5 API calls per minute

For production use, consider upgrading to a paid plan:
- Premium: Starting at $49/month for 150 API requests per minute
- Enterprise: Custom pricing for unlimited API requests

The scheduler handles API errors gracefully and will continue attempting to fetch available data.

## Cron Expression Format

The scheduler uses the standard cron format:

```
┌─────────── minute (0 - 59)
│ ┌───────── hour (0 - 23)
│ │ ┌─────── day of the month (1 - 31)
│ │ │ ┌───── month (1 - 12)
│ │ │ │ ┌─── day of the week (0 - 6) (Sunday to Saturday)
│ │ │ │ │
* * * * *
```

Examples:
- `*/30 * * * *`: Every 30 minutes
- `0 * * * *`: Every hour
- `0 0 * * *`: Every day at midnight
