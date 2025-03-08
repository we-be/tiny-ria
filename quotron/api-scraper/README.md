# API Scraper

A Go-based client for fetching financial data from various APIs.

## Alpha Vantage Integration

The API scraper currently supports Alpha Vantage endpoints for:
- Stock price quotes
- Market data (indices)

## Usage

Build the binary:
```bash
go build -o api-scraper ./cmd/main
```

Run with an API key:
```bash
./api-scraper --api-key YOUR_API_KEY --symbol AAPL
```

For JSON output:
```bash
./api-scraper --api-key YOUR_API_KEY --symbol AAPL --json
```

## Environment Variables

You can also set the API key with an environment variable:
```bash
export ALPHA_VANTAGE_API_KEY=YOUR_API_KEY
./api-scraper --symbol AAPL
```

## Continuous Integration

This component is automatically tested via GitHub Actions:

- Tests run when changes are pushed to the `quotron/api-scraper` directory
- Live API connectivity is verified using a real Alpha Vantage API key
- Tests ensure the API client can successfully fetch and parse financial data

The workflow file is at `.github/workflows/api-scraper-tests.yml`

### Test Locally

Run the integration tests locally:
```bash
cd /quotron
ALPHA_VANTAGE_API_KEY=YOUR_API_KEY python tests/integration/test_integration.py --api
```
