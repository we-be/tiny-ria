# tiny-ria

A modular financial data scraping, analysis, and trading system.

![CLI Build](https://github.com/we-be/tiny-ria/actions/workflows/cli-release.yml/badge.svg)
![Yahoo Finance Tests](https://github.com/we-be/tiny-ria/actions/workflows/yahoo-finance-tests.yml/badge.svg)
![API Scraper Tests](https://github.com/we-be/tiny-ria/actions/workflows/api-scraper-tests.yml/badge.svg)

## Components

- **Quotron**: Financial data scraping and ingestion pipeline
  - API Scraper (Go): Connects to financial APIs (Alpha Vantage implemented)
  - Browser Scraper (Python/Playwright): Handles JS-heavy websites
  - Auth Engine: Manages authentication for data sources
  - Ingest Pipeline: Validates and normalizes financial data
  - Events System: Distributes data through the system

## Development

### API Keys

To run the API scrapers with real data, you'll need:
- Alpha Vantage API key (set as `ALPHA_VANTAGE_API_KEY` environment variable)

### GitHub Actions

The repo includes several CI workflows:

1. **CLI Release Workflow**: Builds and publishes the Quotron CLI as a GitHub release
2. **Yahoo Finance Tests**: Verifies the Yahoo Finance proxy and direct API integration
3. **API Scraper Tests**: Tests the API scraper with Alpha Vantage and integration tests

Status badges at the top of this README show the current health of these workflows.

#### Setting Up API Keys

To enable API tests in GitHub Actions, add your Alpha Vantage API key as a repository secret:
- Go to Settings → Secrets → Actions → New repository secret
- Name: `ALPHA_VANTAGE_API_KEY`
- Value: Your API key

#### Using the CLI

The Quotron CLI binary is published as a GitHub release and can be used to manage all services:

```bash
# Start services
./quotron start yfinance-proxy
./quotron start api-service

# Check service status
./quotron status

# Stop services
./quotron stop yfinance-proxy
./quotron stop api-service

# Check health
./quotron health
```

---
Darvas
