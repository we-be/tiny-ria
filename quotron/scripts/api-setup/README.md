# API Setup for Quotron

This directory contains scripts to help you set up API credentials for the Quotron data pipeline.

## Available Scripts

### Alpha Vantage

[Alpha Vantage](https://www.alphavantage.co/) provides free APIs for realtime and historical stock data, forex, and cryptocurrencies.

To set up Alpha Vantage:

1. Run the setup script:
   ```bash
   ./alpha_vantage_setup.sh
   ```

2. The script will guide you through:
   - Getting a free API key from Alpha Vantage
   - Testing your API key
   - Saving your API key to the environment or a config file

3. After setup, you can use the API scraper:
   ```bash
   cd ../../api-scraper
   go run cmd/main/main.go --symbol AAPL
   ```

### Usage Notes for Alpha Vantage

- **Free API Key Limits**: The free tier provides 25 API requests per day
- **Endpoints Used**:
  - `GLOBAL_QUOTE` - For stock quotes
  - `TIME_SERIES_DAILY` - For market indices

## Adding More API Providers

To add support for additional API providers:

1. Create a new setup script in this directory
2. Update the client implementation in `api-scraper/pkg/client/`
3. Document any API-specific limitations or usage notes

## Production Usage

For production use:
- Consider upgrading to paid API tiers for higher rate limits
- Use environment variables for API keys rather than config files
- Implement proper credential rotation and management