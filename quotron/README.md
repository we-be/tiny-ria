# Quotron

Quotron is the dedicated scraping and data ingestion pipeline within the tiny-ria project, aimed at systematically collecting, cleaning, and ingesting financial data from diverse web sources.

## Project Components

### 1. Core Shared Components
Located in the root directory, these components provide shared functionality:

#### Models
Located in `models/`, this package provides unified data models for the entire system:
- Centralized finance data structures used across all components
- Schema generation for Python from Go types
- Consistent field naming and data validation

#### Providers
Located in `providers/`, these packages offer standardized interfaces to data sources:
- Yahoo Finance provider with multiple implementation strategies
- Consistent interface for all data access
- Integrated health monitoring

### 2. API Scraper (Go)
Located in `api-scraper/`, this component handles data collection from REST APIs.
- Makes HTTP requests to financial data APIs
- Handles rate limiting and authentication
- Standardizes data into common formats

### 3. Browser Scraper (Python)
Located in `browser-scraper/`, this component handles JavaScript-heavy websites.
- Uses Playwright or Selenium for browser automation
- Extracts data from complex UI elements
- Handles dynamic content loading

### 4. Authentication Engine (Python)
Located in `auth-engine/`, this component manages authentication for various services.
- Handles login credentials and session cookies
- Maintains authenticated sessions
- Provides middleware for authenticated requests

### 5. Configuration System
Located throughout the project, configuration is managed through:
- Environment variables
- JSON configuration files
- Command line flags
- Default values with smart detection of project root

### 6. Ingest Pipeline (Python)
Located in `ingest-pipeline/`, this component processes raw data.
- Validates incoming data against schemas
- Enriches data with additional information
- Prepares data for storage

### 7. Events System (Python)
Located in `events/`, this component manages asynchronous communication.
- Produces events when new data is available
- Consumes events to trigger processing
- Uses Kafka for reliable messaging

### 8. Storage (SQL/NoSQL)
Located in `storage/`, this component manages data persistence.
- Stores structured data in SQL databases
- Stores unstructured data in blob storage
- Handles database migrations

### 9. Scheduler (Go)
Located in `scheduler/`, this component manages automated jobs.
- Schedules periodic data collection
- Manages retries and error handling
- Coordinates between different scrapers

### 10. CLI
Located in `cli/`, this unified command-line interface replaces multiple bash scripts:
- Single entry point for all commands
- Consistent interface for all operations
- Environment management and configuration

## Getting Started

### Prerequisites
- Go 1.21+
- Python 3.11+
- Docker and Docker Compose
- PostgreSQL 14+

### Setup
1. Clone the repository
2. Copy the example environment file and customize as needed:
   ```bash
   cp .env.example .env
   nano .env  # Edit values as needed
   ```
3. Start the required infrastructure: `docker-compose up -d`

### Running Components

Quotron provides a unified Go CLI for managing all services and operations:

```bash
./quotron COMMAND [OPTIONS]
```

#### Available Commands

##### Starting Services
```bash
# Start all services
./quotron start

# Start specific services
./quotron start api      # API service with integrated dashboard
./quotron start proxy scheduler

# Start services in monitor mode (auto-restart)
./quotron start --monitor
```

##### Stopping Services
```bash
# Stop all services
./quotron stop

# Stop specific services
./quotron stop api       # Stops API service with integrated dashboard
./quotron stop proxy
```

##### Checking Status
```bash
# Check status of all services
./quotron status
```

##### Running Tests
```bash
# Run all tests
./quotron test

# Run specific tests
./quotron test api
./quotron test integration

# Test a specific job
./quotron test job stock_quotes
```

##### Importing Data
```bash
# Import S&P 500 data
./quotron import-sp500
```

##### Configuration
```bash
# Generate default config file
./quotron --gen-config
```

##### Getting Help
```bash
./quotron help
```

#### Configuration

The CLI can be configured via:

1. Default values
2. Environment variables 
3. Config file (JSON format)
4. Command-line flags

##### Configuration System

The configuration system provides a unified approach to configuring all services:

- **Automatic Project Root Detection**: Locates the quotron root directory automatically
- **Environment Variable Support**: Override settings using environment variables
- **JSON Configuration**: Detailed configuration via JSON files
- **Command-line Flags**: Quick overrides for specific settings
- **Service-specific Configs**: Each service can have its own configuration file
- **Centralized Settings**: Core settings like database connection details are shared

To generate a default config file:
```bash
./quotron --gen-config
```

This will create a `quotron.json` file that you can edit to customize the configuration.

**Example Configurations:**
- `quotron.example.json`: Example configuration for the main CLI
- `scheduler-config.example.json`: Example configuration for the scheduler

To use these examples:
```bash
cp quotron.example.json quotron.json
cp scheduler-config.example.json scheduler-config.json
# Edit the config files to match your environment
```

##### Key Configuration Settings

The following are key settings that can be configured:

- **API Service**: Host, port, and data source settings
- **Database**: Connection details for PostgreSQL
- **Redis**: Connection details for Redis cache
- **YFinance Proxy**: Host, port, and URL settings
- **Paths**: Locations of core components and files
- **Log Files**: Paths to log files for each service
- **PID Files**: Paths to process ID files for service management
- **External API Keys**: API keys for third-party services

#### Manual Component Usage

If needed, you can still interact with individual components directly:

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
- Dashboard (now integrated into API service): 
  ```bash
  cd api-service
  go build -o api-service ./cmd/server
  ./api-service --port=8080
  # Dashboard UI available at http://localhost:8501
  ```
- Scheduler: 
  ```bash
  cd scheduler
  go build -o scheduler ./cmd/scheduler/main.go
  ./scheduler -api-scraper /full/path/to/api-scraper/api-scraper
  ```

### Troubleshooting and Health Checking

#### Running Tests
To run all tests for the service:
```bash
./quotron test
```

This comprehensive test suite will:
1. Start necessary services
2. Run API service tests
3. Verify database connectivity
4. Test Yahoo Finance direct connectivity
5. Run integration tests
6. Test scheduler jobs

#### Checking Service Status
To check the status of all services:
```bash
./quotron status
```

This will show:
- Which services are running (with PIDs)
- Whether services are responding on their ports
- Color-coded status indicators

#### Job Names
The scheduler supports the following job names:
- `stock_quotes`: Fetch stock quotes for configured symbols
- `market_indices`: Fetch market indices data

Use these exact names when running jobs manually:
```bash
./quotron test job stock_quotes
./quotron test job market_indices
```

#### Health Check Tool
To quickly verify the status of all services, run the health check tool:
```bash
python3 test_health.py
```

This will check if the YFinance proxy and scheduler are running correctly and generate a health report.

#### Environment Configuration
Environment variables can be used to configure the CLI. You can also use a JSON configuration file:
```bash
./quotron --config path/to/config.json start
```

Key configurations include:
- API endpoints and ports
- Database connection details
- Log file locations
- External API keys

#### Common Issues

1. **Permission Denied**: Make sure the binary has executable permissions:
   ```bash
   chmod +x quotron
   ```

2. **Proxy Connection Failed**: Check if the YFinance proxy service is running:
   ```bash
   ./quotron status
   # Or use Python to check
   python3 -c "import requests; print(requests.get('http://localhost:5000/health').json())"
   ```

3. **API Key Issues**: Set the Alpha Vantage API key in your config file or environment:
   ```
   export ALPHA_VANTAGE_API_KEY=your_api_key
   ```
   For testing, you can use "demo" which will automatically use Yahoo Finance as fallback.

4. **Service Not Starting**: Check the log files for detailed error information:
   ```bash
   tail -f /tmp/yfinance_proxy.log
   tail -f /tmp/scheduler.log
   tail -f /tmp/api_service.log
   ```

## Data Source Health Monitoring

The system includes a robust data source health monitoring system:

### Health Dashboard
The health dashboard (integrated into the API service) provides a visual overview of all data sources with:
- At-a-glance status indicators for each source
- Health score metrics and trends
- Visual cards showing current status
- Detailed error information for failing sources
- Accessible at http://localhost:8080/health or through the API service UI

### Automatic Recovery
The system can automatically recover failing data sources:
- One-click recovery options for individual sources
- Bulk recovery for all failing sources
- Intelligent recovery strategies based on source type
- Health status tracking after recovery attempts

### AI Diagnostics
When you click the "AI Diagnose" button in the API service's health dashboard, a comprehensive diagnostics report is generated at `quotron/diagnostics_report.md`. This report includes:
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

### Building the CLI
```bash
cd cli
./build.sh
```

For more details on the CLI, see the [CLI README](cli/README.md).

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

### API Service (with integrated dashboard)
```
cd api-service
go build -o api-service ./cmd/server
./api-service --port=8080
# Dashboard UI available at http://localhost:8080
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