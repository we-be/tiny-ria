# US Economic Factors Feature

This document provides instructions on how to use the US Economic Factors feature in the Quotron API Scraper system.

## Overview

The US Economic Factors feature allows you to retrieve and analyze key US economic indicators such as GDP, unemployment rate, inflation, interest rates, and more. The system uses a client-proxy architecture:

1. A Python proxy server that interfaces with economic data APIs (primarily FRED - Federal Reserve Economic Data)
2. A Go client that communicates with the proxy
3. API endpoints that expose the data to other services

## Setup and Requirements

### Prerequisites

- Python 3.7+
- Go 1.16+
- Required Python packages (in `requirements.txt`):
  - flask
  - requests
  - flask-cors
  - python-dateutil
  - pytz

### Starting the Proxy

The proxy server can be started using the daemon script:

```bash
./economic_daemon.sh
```

By default, it will run on `localhost:5002`. You can customize the host and port:

```bash
./economic_daemon.sh --host 0.0.0.0 --port 5003
```

### Stopping the Proxy

To stop the proxy:

```bash
./economic_daemon.sh stop
```

Or check the status:

```bash
./economic_daemon.sh status
```

## Usage

### Direct API Calls

#### Get Specific Economic Indicator

```bash
curl http://localhost:5002/indicator/GDP
```

Optional query parameters:
- `period`: Specify time range (e.g., `5y`, `1y`, `max`)

Available indicators include:
- `GDP`: Gross Domestic Product
- `UNRATE`: Unemployment Rate
- `CPIAUCSL`: Consumer Price Index
- `FEDFUNDS`: Federal Funds Rate
- `PAYEMS`: Nonfarm Payrolls
- `JTSJOR`: Job Openings
- `RRSFS`: Retail Sales
- `HOUST`: Housing Starts
- `CSUSHPISA`: Housing Price Index
- `INDPRO`: Industrial Production

#### Get All Available Indicators

```bash
curl http://localhost:5002/indicators
```

#### Get Economic Summary

```bash
curl http://localhost:5002/summary
```

#### Health Check

```bash
curl http://localhost:5002/health
```

### Web UI

A simple web UI is available at http://localhost:5002/ when the proxy is running.

## Data Reference

### Economic Indicator Data

Data showing historical values for a specific economic indicator.

**Example Response Fields:**
- `indicator`: The indicator ID (e.g., "GDP")
- `name`: Human-readable name (e.g., "Gross Domestic Product")
- `description`: Detailed description of the indicator
- `period`: Time period of the data (e.g., "5y")
- `data`: Array of data points with:
  - `date`: Date of the data point (YYYY-MM-DD)
  - `value`: Value of the indicator at that date
  - `unit`: Unit of measurement (e.g., "Billions of Dollars")
- `metadata`: Additional information about the data
- `source`: Where the data came from ("FRED" or "fallback generator")
- `timestamp`: When the data was fetched

### Available Indicators

List of all available economic indicators.

**Example Response Fields:**
- `indicators`: Array of available indicators with:
  - `id`: The indicator ID used in API calls
  - `name`: Human-readable name
  - `description`: Detailed description
  - `unit`: Unit of measurement
  - `frequency`: How often the data is updated (e.g., "Monthly", "Quarterly")
- `count`: Total number of available indicators
- `timestamp`: When the data was fetched

### Economic Summary

A summary of key economic indicators and overall economic health.

**Example Response Fields:**
- `indicators`: List of indicator names included in the summary
- `summary`: Object containing summarized data for each indicator:
  - `name`: Human-readable name
  - `value`: Latest value
  - `date`: Date of the latest value
  - `unit`: Unit of measurement
  - `change_pct`: Percentage change over the period
  - `trend`: Direction of change ("up", "down", or "stable")
- `overall`: Assessment of overall economic health:
  - `health`: Text description ("Strong", "Moderate", "Stable", or "Weak")
  - `trending_up`: Number of indicators trending up
  - `trending_down`: Number of indicators trending down
  - `score`: Economic health score
- `timestamp`: When the data was fetched
- `source`: Data source

## Value Interpretation

- **GDP**: Higher values generally indicate a growing economy
- **Unemployment Rate**: Lower values are generally better
- **Consumer Price Index**: Measures inflation, moderate increases are normal
- **Federal Funds Rate**: Interest rate set by the Federal Reserve
- **Nonfarm Payrolls**: Higher values indicate more jobs
- **Job Openings**: Higher values indicate strong labor demand
- **Retail Sales**: Higher values indicate strong consumer spending
- **Housing Starts**: Higher values indicate strong construction activity
- **Housing Price Index**: Measures home price changes
- **Industrial Production**: Measures output of manufacturing, mining, and utilities

## Data Sources and Fallback Data

This service can use real economic data from FRED (Federal Reserve Economic Data). To use real data, you need to:

1. Get a free API key from the Federal Reserve Bank of St. Louis: https://fred.stlouisfed.org/docs/api/api_key.html
2. Set the API key as an environment variable before starting the proxy:
   ```bash
   export FRED_API_KEY=your_api_key_here
   ./economic_daemon.sh
   ```

If no FRED API key is provided, or if there's an error accessing the FRED API, the system will generate fallback data to ensure service availability. These responses will include `"source": "fallback generator"` to indicate they are not from the actual economic data API.

When using real data, responses will include `"source": "FRED"` to indicate they are from the Federal Reserve Economic Data API.

## Caching

The proxy implements caching to reduce API calls. Responses include an `X-Cache-Status` header that will be set to `hit` or `miss` to indicate cache status.

## Limitations

- Some economic indicators are released with a lag (e.g., GDP is released quarterly with a delay)
- Historical data may have different frequencies (e.g., monthly vs. quarterly)
- Economic indicators may be revised after initial release

## Troubleshooting

### Proxy Not Starting

Check if a process is already using the port:
```bash
lsof -i :5002
```

### No Data Returned

Verify the proxy is running:
```bash
curl http://localhost:5002/health
```

### Connection Refused

Make sure the proxy is running on the expected host and port:
```bash
ps aux | grep economic_proxy.py
```