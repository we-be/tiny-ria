# API Service

This service provides a HTTP-based interface to financial data sources, serving as a unified middleware layer between data providers and consumer applications. It also includes an integrated dashboard for monitoring and visualization.

## Features

### API Service
- RESTful API endpoints for stock quotes and market indices
- Automatic failover between data sources (Yahoo Finance proxy and Alpha Vantage)
- Health monitoring endpoints with data source status reporting
- Database integration for data persistence (works even without DB connection)
- Data normalization from multiple sources into consistent response format

### Integrated Dashboard
- **Scheduler Control**
  - Start and stop the scheduler
  - Run individual jobs on demand
  - Monitor scheduler status
- **Market Data Overview**
  - View latest market indices
  - Track top movers in the stock market
  - Color-coded indicators for price changes
- **Data Source Health**
  - Monitor health of data sources
  - Track data freshness and update frequency
  - View batch processing statistics
- **Investment Model Explorer**
  - Browse available investment models
  - Visualize sector allocations with pie charts
  - Explore model holdings with interactive charts

## API Endpoints

### Quote Endpoints
- `GET /api/quote/{symbol}` - Get stock quote for a symbol
- `POST /api/quotes/batch` - Get multiple stock quotes in a single request
- `GET /api/quotes/history/{symbol}?days=7` - Get historical quotes for a symbol

### Market Index Endpoints
- `GET /api/index/{index}` - Get market index data
- `POST /api/indices/batch` - Get multiple market indices in a single request

### Health and Status Endpoints
- `GET /api/health` - Check service health status
- `GET /api/data-source/health` - Check health status of data sources
- `GET /` - API information and documentation

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
- `--db`: PostgreSQL connection URL (default: "postgres://postgres:postgres@localhost:5432/quotron?sslmode=disable")
- `--yahoo`: Use Yahoo Finance as data source (default: true)
- `--alpha-key`: Alpha Vantage API key
- `--yahoo-host`: Yahoo Finance proxy host (default: "localhost")
- `--yahoo-port`: Yahoo Finance proxy port (default: 5000)
- `--health`: Enable unified health reporting (default: false)
- `--health-service`: Unified health service URL (empty to disable)
- `--name`: Service name for health reporting (default: "api-service")

## Architecture

The API service consists of the following components:

1. **HTTP Server**: Handles incoming requests and routes them to the appropriate handlers
2. **Client Manager**: Manages data source clients with fallback capability
3. **Data Clients**: Implementations for different data sources (Alpha Vantage, Yahoo Finance)
4. **Database Integration**: Stores results in PostgreSQL for persistence

## Design Decisions

1. **HTTP Instead of Command Execution**: Previous design ran API scrapers as separate processes, which caused permission issues. This service provides a network-based approach that avoids those issues.

2. **Resilience Features**:
   - Automatic failover between data sources (primary and secondary)
   - Graceful handling of database connection failures (continues operation without DB)
   - Database persistence for result history when available
   - Connection pooling for database efficiency
   - Timeouts to prevent hanging operations
   - Detailed health reporting
   - Concurrent processing for batch operations

3. **Client/Server Model**: Scheduler now acts as a client to the API service, making it easier to distribute across containers/machines.

4. **Unified Health Monitoring**: Integration with the health service for centralized monitoring of all data sources and services.

5. **Batch Processing**: Support for batch operations reduces network overhead when retrieving multiple quotes or indices.

6. **Historical Data Access**: The service provides access to historical data stored in the database, enabling time-series analysis and charting.

## Dashboard
The integrated dashboard offers visualization and control capabilities for the Quotron system.

### Dashboard Technologies
- Streamlit for the UI framework
- Plotly for data visualization
- Pandas for data manipulation
- psycopg2 for PostgreSQL database connections
- Python subprocess for scheduler control

### Running the Dashboard
The dashboard is now integrated into the API service and runs on port 8501 by default:

```bash
# Start the API service with dashboard
./quotron start api

# Access the dashboard
open http://localhost:8501
```

### Dashboard Database Configuration
The dashboard connects to PostgreSQL with the following settings:

- Host: localhost (configurable)
- Port: 5432 (configurable)
- Database name: quotron (configurable)
- Username: quotron (configurable)
- Password: quotron (configurable)

You can test this connection with:
```bash
PGPASSWORD=quotron psql -U quotron -h localhost -d quotron -c "SELECT 1"
```

## Testing

The API service and dashboard can be tested using the CLI:

```bash
# Start the service if not already running
./quotron start api

# Run API-specific tests
./quotron test api

# Run all tests including the API service
./quotron test
```

Integration tests verify both the Yahoo Finance proxy and the API service, ensuring proper data flow through the entire system.