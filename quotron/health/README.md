# Health Monitoring Service

A unified health monitoring service for Quotron components. This replaces the various scattered health monitoring implementations with a centralized service that all components can use.

## Components

1. **Health Service**: Core service for storing and retrieving health data
2. **Health API**: HTTP API for health monitoring
3. **Go Client**: Client library for Go services
4. **Python Client**: Client library for Python services
5. **Middleware**: Automatic health reporting for web services

## Getting Started

### 1. Build the components

```bash
./build.sh
```

### 2. Run the server

```bash
./health-server --port 8085 --db-host localhost --db-port 5432 --db-name quotron --db-user quotron --db-password quotron
```

### 3. Use the Go client

```bash
./health-client --url http://localhost:8085 --type test-client --name go-client --action report --status healthy
./health-client --url http://localhost:8085 --type test-client --name go-client --action get
```

### 4. Use the Python client

```bash
./test_client.py --url http://localhost:8085 --type test-client --name python-client --action report --status healthy
./test_client.py --url http://localhost:8085 --type test-client --name python-client --action get
```

## Integration with Services

### Go Services

Add to your Go service:

```go
import (
    "github.com/we-be/tiny-ria/quotron/health/client"
    "github.com/we-be/tiny-ria/quotron/health"
)

// Create a client
healthClient := client.NewHealthClient("http://localhost:8085")

// Add middleware to your HTTP router
router.Use(health.HealthReportingMiddleware(healthClient, "api-service", "main-api"))

// Or report health manually
report := health.HealthReport{
    SourceType:     "api-service",
    SourceName:     "data-api",
    Status:         health.StatusHealthy,
    ResponseTimeMs: 100,
}
healthClient.ReportHealth(ctx, report)
```

### Python Services

Add to your Python service:

```python
from quotron.health.client import HealthClient, HealthStatus, measure_request_time

# Create a client
client = HealthClient("http://localhost:8085")

# Report health
client.report_health(
    source_type="browser-scraper",
    source_name="playwright",
    status=HealthStatus.HEALTHY,
    response_time_ms=150
)

# Decorate functions to automatically report health
@measure_request_time
def fetch_data(url, client=client, source_type="api-scraper", source_name="yahoo_proxy"):
    response = requests.get(url)
    return response.json()
```

## Dashboard Integration

The health data is stored in the `data_source_health` table and can be visualized in the Quotron dashboard.