# API Service

This service provides a HTTP-based interface to the API Scraper functionality, replacing direct command-line execution to avoid permission issues.

## Features

- RESTful API endpoints for stock quotes and market indices
- Health monitoring endpoints
- Integrated with existing Yahoo Finance and Alpha Vantage data sources
- Results stored in PostgreSQL for resilience

## API Endpoints

- `GET /api/quote/{symbol}` - Get stock quote for a symbol
- `GET /api/index/{index}` - Get market index data
- `GET /api/health` - Check service health status
- `GET /api/data-source/health` - Check health status of data sources

## Technologies

- Go HTTP server
- PostgreSQL for data storage
- Circuit breaker pattern for external API resilience

## Getting Started

### Building and Running Locally

```bash
# Navigate to the api-service directory
cd api-service

# Build the service
go build -o api-service ./cmd/server

# Run the service
./api-service --port=8080 --yahoo-host=localhost --yahoo-port=5000
```

### Using Docker

```bash
# Build and run using docker-compose
docker-compose up api-service
```

### Configuration

The API service can be configured using command-line flags:

- `--port`: Port number for the HTTP server (default: 8080)
- `--db`: PostgreSQL connection URL (default: "postgres://postgres:postgres@localhost:5433/quotron?sslmode=disable")
- `--yahoo`: Use Yahoo Finance as data source (default: true)
- `--alpha-key`: Alpha Vantage API key
- `--yahoo-host`: Yahoo Finance proxy host (default: "localhost")
- `--yahoo-port`: Yahoo Finance proxy port (default: 5000)

## Architecture

The API service consists of the following components:

1. **HTTP Server**: Handles incoming requests and routes them to the appropriate handlers
2. **Client Manager**: Manages data source clients with fallback capability
3. **Data Clients**: Implementations for different data sources (Alpha Vantage, Yahoo Finance)
4. **Database Integration**: Stores results in PostgreSQL for persistence

## Design Decisions

1. **HTTP Instead of Command Execution**: Previous design ran API scrapers as separate processes, which caused permission issues. This service provides a network-based approach that avoids those issues.

2. **Resilience Features**:
   - Automatic failover between data sources
   - Database persistence for result history
   - Connection pooling for database efficiency
   - Timeouts to prevent hanging operations

3. **Client/Server Model**: Scheduler now acts as a client to the API service, making it easier to distribute across containers/machines.