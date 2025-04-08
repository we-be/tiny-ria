# Google Trends Feature

This document provides instructions on how to use the Google Trends feature in the Quotron API Scraper system.

## Overview

The Google Trends feature allows you to retrieve search interest data for keywords over time, as well as related queries and topics. The system uses a client-proxy architecture:

1. A Python proxy server that interfaces directly with the Google Trends API
2. A Go client that communicates with the proxy
3. API endpoints that expose the data to other services

## Setup and Requirements

### Prerequisites

- Python 3.7+
- Go 1.16+
- Required Python packages (in `requirements.txt`):
  - flask
  - pytrends
  - flask-cors
  - requests
  - python-dateutil
  - pytz

### Starting the Proxy

The proxy server can be started using the daemon script:

```bash
./gtrends_daemon.sh
```

By default, it will run on `localhost:5001`. You can customize the host and port:

```bash
./gtrends_daemon.sh --host 0.0.0.0 --port 5002
```

### Stopping the Proxy

To stop the proxy:

```bash
./gtrends_daemon.sh stop
```

Or kill all proxy instances:

```bash
./kill_all_proxies.sh
```

## Usage

### Command Line Tool

A command line formatter tool is provided for easy data viewing:

```bash
# View all data for a keyword
python3 format_trends.py bitcoin

# Show only interest over time data
python3 format_trends.py bitcoin --type interest

# Show only related queries
python3 format_trends.py bitcoin --type queries

# Show only related topics
python3 format_trends.py bitcoin --type topics

# Show shortened version (fewer data points)
python3 format_trends.py bitcoin --short
```

### Direct API Calls

#### Interest Over Time

```bash
curl http://localhost:5001/interest-over-time/bitcoin
```

Optional query parameters:
- `timeframe`: Specify time range (e.g., `today 5-y`, `today 3-m`, `today 12-m`, `all`)

#### Related Queries

```bash
curl http://localhost:5001/related-queries/bitcoin
```

#### Related Topics

```bash
curl http://localhost:5001/related-topics/bitcoin
```

#### Health Check

```bash
curl http://localhost:5001/health
```

### Web UI

A simple web UI is available at http://localhost:5001/ when the proxy is running.

## Data Reference

### Interest Over Time Data

Data showing how search interest for a keyword has changed over time.

**Example Response Fields:**
- `keyword`: The search term requested
- `timeframe`: Time period of the data (e.g., "today 5-y")
- `data`: Array of data points with:
  - `date`: Date of the data point (YYYY-MM-DD)
  - `[keyword]`: Interest value (0-100) where 100 is peak popularity
  - `isPartial`: Whether the data is partial/incomplete
- `metadata`: Additional information about the data
- `source`: Where the data came from ("Google Trends API" or "fallback generator")
- `timestamp`: When the data was fetched

### Related Queries Data

Search terms related to the specified keyword.

**Example Response Fields:**
- `keyword`: The search term requested
- `top`: Array of top related queries with:
  - `query`: The related search term
  - `value`: Relative popularity (0-100)
- `rising`: Array of trending related queries with:
  - `query`: The related search term
  - `value`: Percentage increase (e.g., "+800%")
- `metadata`: Additional information
- `timestamp`: When the data was fetched
- `source`: Data source

### Related Topics Data

Topics related to the specified keyword.

**Example Response Fields:**
- `keyword`: The search term requested
- `top`: Array of top related topics with:
  - `topic_title`: Name of the related topic
  - `topic_type`: Category (e.g., "Cryptocurrency", "Technology")
  - `value`: Relative popularity (0-100)
- `rising`: Array of trending related topics with:
  - `topic_title`: Name of the related topic
  - `topic_type`: Category (e.g., "Cryptocurrency", "Technology")
  - `value`: Percentage increase (e.g., "+800%")
- `metadata`: Additional information
- `timestamp`: When the data was fetched
- `source`: Data source

## Value Interpretation

- **Interest Over Time Values**: Range from 0-100, where 100 represents the peak popularity for the term during the specified time period.
- **Top Related Queries/Topics Values**: Range from 0-100, representing relative popularity compared to other related queries/topics.
- **Rising Related Queries/Topics Values**: Shown as percentage increases (e.g., "+800%"), representing growth in popularity.

## Fallback Data

In case the Google Trends API is unavailable, the system will generate fallback data to ensure service availability. These responses will include `"source": "fallback generator"` to indicate they are not from the actual Google Trends API.

## Timeframes

Available timeframes:
- `today 1-m`: Last month
- `today 3-m`: Last 3 months
- `today 12-m`: Last 12 months
- `today 5-y`: Last 5 years (default)
- `all`: All available data

## Caching

The proxy implements caching to reduce API calls. Responses include an `X-Cache-Status` header that will be set to `hit` or `miss` to indicate cache status.

## Limitations

- The Google Trends API has rate limits which may cause temporary unavailability
- Some niche keywords may not have enough data in Google Trends
- Historical data more than 5 years old may be limited

## Troubleshooting

### Proxy Not Starting

Check if a process is already using the port:
```bash
lsof -i :5001
```

### No Data Returned

Verify the proxy is running:
```bash
curl http://localhost:5001/health
```

### Connection Refused

Make sure the proxy is running on the expected host and port:
```bash
ps aux | grep gtrends_proxy.py
```