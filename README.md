# tiny-ria

A modular financial data scraping, analysis, and trading system.

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

The repo includes CI workflows that:
1. Run tests when changes are pushed to relevant directories
2. Verify API scrapers are working correctly

To enable API tests in GitHub Actions, add your Alpha Vantage API key as a repository secret:
- Go to Settings → Secrets → Actions → New repository secret
- Name: `ALPHA_VANTAGE_API_KEY`
- Value: Your API key

---
Darvas
