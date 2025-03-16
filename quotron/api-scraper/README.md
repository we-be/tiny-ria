# API Scraper

[![API Tests](https://img.shields.io/github/actions/workflow/status/we-be/tiny-ria/api-scraper-tests.yml?label=API%20Tests&logo=golang)](https://github.com/we-be/tiny-ria/actions/workflows/api-scraper-tests.yml)
[![YFinance](https://img.shields.io/github/actions/workflow/status/we-be/tiny-ria/yahoo-finance-tests.yml?label=YFinance&logo=yahoo)](https://github.com/we-be/tiny-ria/actions/workflows/yahoo-finance-tests.yml)

A Go-based client for fetching financial data from various financial APIs.

## Supported APIs

### Alpha Vantage Integration

The API scraper supports Alpha Vantage endpoints for:
- Stock price quotes
- Market data (indices)

**Note**: Alpha Vantage has rate limits:
- Free tier: 5 requests/minute, 25 requests/day
- Limited market data access

### Yahoo Finance Integration

The API scraper now supports Yahoo Finance as an alternative:
- No API key required
- Higher rate limits
- Two modes:
  - REST API (pure Go implementation)
  - Python-based proxy (using yfinance package)

## Usage

Build the binary:
```bash
make build  # or: go build -o api-scraper ./cmd/main
```

### Alpha Vantage

Run with an Alpha Vantage API key:
```bash
./api-scraper --api-key YOUR_API_KEY --symbol AAPL
```

### Yahoo Finance

Run with Yahoo Finance (no API key needed):
```bash
./api-scraper --yahoo --symbol AAPL
```

### Database Integration

The API scraper works with the Quotron storage pipeline:

1. When run from the scheduler, data is automatically saved to JSON files
2. The ingest pipeline processes these files to:
   - Validate and clean the data
   - Enrich it with additional information
   - Store it in the PostgreSQL database
   - Compute and store statistical information

To manually process data files:
```bash
cd ../ingest-pipeline
python cli.py mixed path/to/data.json --source api-scraper --allow-old-data
```

### JSON Output

For JSON output with any provider:
```bash
./api-scraper --yahoo --symbol AAPL --json
```

## Environment Variables

You can set the Alpha Vantage API key with an environment variable:
```bash
export ALPHA_VANTAGE_API_KEY=YOUR_API_KEY
./api-scraper --symbol AAPL
```

## Python Proxy Setup

The Yahoo Finance proxy requires Python 3 with yfinance:

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -r scripts/requirements.txt
```

The proxy server will automatically start and stop when using the `--yahoo` flag.

## Continuous Integration

This component is automatically tested via GitHub Actions:

- Tests run when changes are pushed to the `quotron/api-scraper` directory
- Live API connectivity is verified using a real Alpha Vantage API key
- Tests ensure the API client can successfully fetch and parse financial data
- Yahoo Finance proxy server is tested with health checks

The workflow file is at `.github/workflows/api-scraper-tests.yml`

### Test Locally

Run the integration tests locally:
```bash
cd /quotron
ALPHA_VANTAGE_API_KEY=YOUR_API_KEY python tests/integration/test_integration.py --api
```