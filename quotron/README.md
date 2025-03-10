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

#### Option 1: Using the Master Startup Script
The easiest way to start all services at once (YFinance Proxy, Scheduler, and Dashboard):
```bash
./start_all.sh
```

#### Option 2: Starting Components Individually
To start just the YFinance Proxy and Scheduler:
```bash
./start_services.sh
```

To start just the Dashboard with all necessary environment variables:
```bash
./restart_dashboard.sh
```

#### Option 2: Manual Startup
- API Scraper: 
  ```bash
  cd api-scraper 
  go build -o api-scraper ./cmd/main/main.go
  ./api-scraper --api-key YOUR_API_KEY
  ```
- YFinance Proxy: 
  ```bash
  cd api-scraper/scripts 
  python yfinance_proxy.py --host localhost --port 5000
  ```
- Browser Scraper: 
  ```bash
  cd browser-scraper/playwright 
  python src/scraper.py
  ```
- Dashboard: 
  ```bash
  cd dashboard 
  python dashboard.py
  ```
- Scheduler: 
  ```bash
  cd scheduler
  go build -o scheduler ./cmd/scheduler/main.go
  export ALPHA_VANTAGE_API_KEY="your_api_key"  # or "demo" for Yahoo Finance
  ./scheduler -api-scraper /full/path/to/api-scraper/api-scraper
  ```

### Troubleshooting and Health Checking

#### Job Names
The scheduler supports the following job names:
- `stock_quotes`: Fetch stock quotes for configured symbols
- `market_indices`: Fetch market indices data

Use these exact names when running jobs manually from the dashboard or command line.

To test job execution, use the provided testing script:
```bash
./test_jobs.sh
```
This will run both jobs once and display the results to help verify everything is working correctly.

#### Health Check Tool
To quickly verify the status of all services, run the health check tool:
```bash
python3 test_health.py
```

This will check if the YFinance proxy and scheduler are running correctly and generate a health report.

#### Common Issues

1. **Permission Denied**: Make sure the API scraper binary is built and has executable permissions:
   ```bash
   cd api-scraper
   go build -o api-scraper ./cmd/main/main.go
   chmod +x api-scraper
   ```

2. **Proxy Connection Failed**: Check if the YFinance proxy service is running:
   ```bash
   # Use Python to check
   python3 -c "import requests; print(requests.get('http://localhost:5000/health').json())"
   ```

3. **API Key Issues**: Set the Alpha Vantage API key:
   ```bash
   export ALPHA_VANTAGE_API_KEY="your_api_key"
   ```
   For testing, you can use "demo" which will automatically use Yahoo Finance as fallback.

4. **Environment Variables**: Make sure the dashboard can locate the YFinance proxy:
   ```bash
   export YFINANCE_PROXY_URL=http://localhost:5000
   ```

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