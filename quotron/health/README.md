# Unified Health Monitoring System

This directory contains the Quotron unified health monitoring system. This system replaces the various health monitoring implementations that were scattered throughout the codebase with a centralized, unified approach.

## Components

- **Core Health Service**: Stores and retrieves health status information
- **HTTP API**: Provides a RESTful API for health status reporting and retrieval
- **Client Libraries**: Go and Python libraries for integrating with the health service
- **Middleware**: HTTP middleware for automatic health reporting
- **CLI Integration**: Command-line tools for health status checking

## Getting Started

### Managing the Health Service

```bash
# Build the health service
./build.sh build

# Start the health service
./build.sh start

# Test the health service
./build.sh test
```

### Using the Go Client

```go
import (
    "context"
    "time"
    healthClient "github.com/we-be/tiny-ria/quotron/health/client"
    "github.com/we-be/tiny-ria/quotron/health"
)

// Create a health client
client := healthClient.NewHealthClient("http://localhost:8085")

// Report health status
report := health.HealthReport{
    SourceType:     "api-scraper",
    SourceName:     "yahoo_finance",
    SourceDetail:   "Yahoo Finance API Client",
    Status:         health.StatusHealthy,
    LastCheck:      time.Now(),
    ResponseTimeMs: 150,
    ErrorMessage:   "",
    Metadata: map[string]interface{}{
        "version": "1.0.0",
    },
}

err := client.ReportHealth(context.Background(), report)
```

### Using the Python Client

```python
from client import HealthClient, HealthStatus

# Create a health client
client = HealthClient("http://localhost:8085")

# Report health status
client.report_health(
    source_type="browser-scraper",
    source_name="slickcharts",
    source_detail="SlickCharts S&P 500 Scraper",
    status=HealthStatus.HEALTHY,
    response_time_ms=200,
    metadata={"pages_scraped": 1}
)

# Use the decorator for automatic health reporting
@client.monitor_health(source_type="browser-scraper", source_name="playwright")
def scrape_website(url):
    # Scraping code here
    return "Successfully scraped"
```

### Using the CLI

#### Using the Test Client

```bash
# Check system health
./bin/test-client -cmd system

# Check all services
./bin/test-client -cmd all

# Get health for a specific service
./bin/test-client -cmd get -source "api-scraper" -name "yahoo_finance"

# Report health status
./bin/test-client -cmd report -source "api-scraper" -name "yahoo_finance" -status "healthy"
```

#### Using the Quotron CLI

The health service is now integrated with the main Quotron CLI:

```bash
# Check all service health
./quotron health

# Get system health summary
./quotron health --action system

# Check specific service health
./quotron health --action service api-scraper/yahoo_finance

# Get health status in JSON format
./quotron health --format json
```

## Migrating from Old Health Monitoring

The old health monitoring implementations have been deprecated and should be replaced with the unified health monitoring system. Here's how to migrate:

### Go Services

1. Remove references to the old health monitoring
2. Import the unified health client
3. Use the unified health client to report and retrieve health status

Example:

```go
// Old way
monitor := NewYahooFinanceHealthMonitor(client)
status, err, responseTime := monitor.CheckHealth()

// New way
import healthClient "github.com/we-be/tiny-ria/quotron/health/client"

healthClient := healthClient.NewHealthClient("http://localhost:8085")
report := health.HealthReport{
    SourceType:     "api-scraper",
    SourceName:     "yahoo_finance",
    Status:         health.StatusHealthy,
    ResponseTimeMs: responseTime,
}
healthClient.ReportHealth(context.Background(), report)
```

### Python Services

1. Remove imports of the old health monitoring
2. Import the unified health client
3. Use the unified health client to report and retrieve health status

Example:

```python
# Old way
from health_monitor import HealthMonitor, HealthStatus
monitor = HealthMonitor("browser-scraper", "slickcharts", "SlickCharts S&P 500 Scraper")
monitor.update_health_status(HealthStatus.HEALTHY)

# New way
from client import HealthClient, HealthStatus
client = HealthClient("http://localhost:8085")
client.report_health(
    source_type="browser-scraper",
    source_name="slickcharts",
    source_detail="SlickCharts S&P 500 Scraper",
    status=HealthStatus.HEALTHY
)
```

## Health Service API

### Report Health

```
POST /health
```

Request Body:
```json
{
    "source_type": "api-scraper",
    "source_name": "yahoo_finance",
    "source_detail": "Yahoo Finance API Client",
    "status": "healthy",
    "response_time_ms": 150,
    "error_message": "",
    "metadata": {
        "version": "1.0.0"
    }
}
```

### Get Health for a Service

```
GET /health/{source_type}/{source_name}
```

### Get Health for All Services

```
GET /health
```

### Get System Health

```
GET /health/system
```

## Health Status Values

- `healthy`: Service is fully operational
- `degraded`: Service is operational but with reduced performance or capabilities
- `failed`: Service is not operational
- `limited`: Service is operational but with limited functionality
- `unknown`: Service health status cannot be determined

## Testing

To test the health monitoring system, use the included test script:

```bash
./test_health_system.sh
```