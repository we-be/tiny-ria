# Quotron

Quotron is the dedicated scraping and data ingestion pipeline within the tiny-ria project, aimed at systematically collecting, cleaning, and ingesting financial data from diverse web sources.

## Project Components

### 1. API Scraper (Go)
Located in `api-scraper/`, this component handles data collection from REST APIs.
- Makes HTTP requests to financial data APIs
- Handles rate limiting and authentication
- Standardizes data into common formats

### 2. Browser Scraper (Python)
Located in `browser-scraper/`, this component handles JavaScript-heavy websites.
- Uses Playwright or Selenium for browser automation
- Extracts data from complex UI elements
- Handles dynamic content loading

### 3. Authentication Engine (Python)
Located in `auth-engine/`, this component manages authentication for various services.
- Handles login credentials and session cookies
- Maintains authenticated sessions
- Provides middleware for authenticated requests

### 4. Dashboard (Python)
Located in `dashboard/`, this component provides visualization and monitoring.
- Streamlit-based web interface
- Displays market data and sources status
- Controls for starting/stopping services
- Health monitoring and diagnostics

### 5. Ingest Pipeline (Python)
Located in `ingest-pipeline/`, this component processes raw data.
- Validates incoming data against schemas
- Enriches data with additional information
- Prepares data for storage

### 6. Events System (Python)
Located in `events/`, this component manages asynchronous communication.
- Produces events when new data is available
- Consumes events to trigger processing
- Uses Kafka for reliable messaging

### 7. Storage (SQL/NoSQL)
Located in `storage/`, this component manages data persistence.
- Stores structured data in SQL databases
- Stores unstructured data in blob storage
- Handles database migrations

### 8. Scheduler (Go)
Located in `scheduler/`, this component manages automated jobs.
- Schedules periodic data collection
- Manages retries and error handling
- Coordinates between different scrapers

## Getting Started

### Prerequisites
- Go 1.21+
- Python 3.11+
- Docker and Docker Compose
- PostgreSQL 14+

### Setup
1. Clone the repository
2. Set up environment variables:
   ```
   DB_HOST=localhost
   DB_PORT=5432
   DB_NAME=quotron
   DB_USER=quotron
   DB_PASSWORD=quotron
   YFINANCE_PROXY_URL=http://localhost:5000
   ```
3. Start the required infrastructure: `docker-compose up -d`

### Running Components
- API Scraper: `cd api-scraper && go run cmd/main/main.go`
- YFinance Proxy: `cd api-scraper/scripts && python yfinance_proxy.py`
- Browser Scraper: `cd browser-scraper/playwright && python src/scraper.py`
- Dashboard: `cd dashboard && python dashboard.py`
- Scheduler: `cd scheduler && go run cmd/scheduler/main.go`

## Data Source Health Monitoring

The system includes a robust data source health monitoring system:

### Health Dashboard
The dashboard provides a visual overview of all data sources with:
- At-a-glance status indicators for each source
- Health score metrics and trends
- Visual cards showing current status
- Detailed error information for failing sources

### Automatic Recovery
The system can automatically recover failing data sources:
- One-click recovery options for individual sources
- Bulk recovery for all failing sources
- Intelligent recovery strategies based on source type
- Health status tracking after recovery attempts

### AI Diagnostics
When you click the "AI Diagnose" button in the dashboard, a comprehensive diagnostics report is generated at `quotron/diagnostics_report.md`. This report includes:
- Overall system health score
- Status of all data sources
- Detailed analysis of failing sources
- Recommendations for resolving issues

### YFinance Proxy
The Yahoo Finance proxy is a critical component for fetching stock data. It includes:
- Caching with TTL to reduce API calls
- Circuit breaker pattern to prevent overwhelming the API
- Exponential backoff for retries on failure
- Heartbeat health monitoring
- REST API endpoints for quotes, market data, and health checks

## Development

### API Scraper
```
cd api-scraper
go mod download
go run cmd/main/main.go --api-key YOUR_API_KEY
```

### YFinance Proxy
```
cd api-scraper/scripts
pip install -r requirements.txt
python yfinance_proxy.py
```

### Dashboard
```
cd dashboard
pip install -r requirements.txt
python dashboard.py
```

### Browser Scraper
```
cd browser-scraper/playwright
pip install -r requirements.txt
python src/scraper.py
```

## Testing
Each component has its own tests in the respective directory or in the main `tests/` directory.

## Deployment
Docker images for each component can be built from their respective Dockerfiles.