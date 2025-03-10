# Service Tracing Implementation Guide

This document provides instructions for implementing the service trace visualization system in the Quotron application.

## Overview

The service trace visualization system allows you to visualize how requests flow through different services in the Quotron ecosystem. It provides:

1. Visual representations of service dependencies
2. Timing analysis for each service
3. Error tracing and bottleneck identification
4. Historical data for performance optimization

## Components

The tracing system consists of three main components:

1. **Database Schema**: Tables for storing trace data and service dependencies
2. **Middleware**: Components that collect and store trace data in each service
3. **Dashboard**: A visualization interface in the Streamlit dashboard

## Implementation Steps

### 1. Apply Database Migrations

Run the migration to create the necessary tables:

```bash
# From the root directory
cd quotron/storage
psql -U quotron -d quotron -f migrations/005_service_traces.sql
```

This creates three tables:
- `service_traces`: Stores individual trace spans
- `trace_spans`: Maps parent-child relationships between spans
- `service_dependencies`: Defines service dependency relationships

### 2. Implement Tracing in Go Services

For Go-based services (API Service, Scheduler), add the tracing middleware:

#### API Service

1. Add the middleware to your HTTP router:

```go
// In your main.go or server setup
import "path/to/middleware"

// Initialize the middleware
tracingMiddleware := middleware.TracingMiddleware{
    DB:          db,
    ServiceName: "api-service",
}

// Add to your router
router.Use(tracingMiddleware.Trace)
```

### 3. Implement Tracing in Python Services

For Python-based services (Yahoo Finance Proxy):

1. Initialize the middleware in your Flask app:

```python
from middleware.tracing import TracingMiddleware

app = Flask(__name__)
tracing = TracingMiddleware(app, service_name="yahoo_finance_proxy")
```

2. Use the function tracer for specific functions:

```python
from middleware.tracing import trace_function

@trace_function(name="fetch_stock_data", service="yahoo_finance_proxy")
def fetch_stock_data(symbol):
    # Your function code here
    pass
```

### 4. Configure Client Services to Propagate Trace IDs

When a client service (like the Scheduler) makes requests to another service:

```go
// Go example
req, _ := http.NewRequest("GET", url, nil)

// Add trace headers from parent request if available
if parentTraceID != "" {
    req.Header.Set("X-Trace-ID", parentTraceID)
    req.Header.Set("X-Span-ID", parentSpanID)
}

client := &http.Client{}
resp, err := client.Do(req)
```

```python
# Python example
headers = {}
if hasattr(g, 'trace_id'):
    headers['X-Trace-ID'] = g.trace_id
    headers['X-Span-ID'] = g.span_id

response = requests.get(url, headers=headers)
```

### 5. View Traces in the Dashboard

1. Navigate to the "Service Traces" tab in the dashboard
2. Use the time range selector to view traces from different periods
3. Explore the Service Flow diagram to see how services interact
4. Use Timeline View to analyze performance of specific requests

## Advanced Configuration

### Adding Custom Metadata

You can add custom metadata to traces to provide more context:

```go
// Go example
metadata := map[string]interface{}{
    "custom_field": value,
    "request_id": requestID,
}
```

```python
# Python example
metadata["custom_field"] = value
metadata["request_id"] = request_id
```

### Sampling

For high-traffic services, you may want to implement sampling to reduce the volume of trace data:

```go
// Only trace 10% of requests
if rand.Float64() < 0.1 {
    // Trace this request
}
```

## Troubleshooting

Common issues:

1. **Missing Traces**: Ensure all services are properly configured with the middleware
2. **Broken Service Flow**: Check that trace IDs are being propagated correctly between services
3. **Database Errors**: Verify database connection parameters in each service
4. **High Database Load**: Implement sampling to reduce trace volume

## Next Steps

Future enhancements to consider:

1. Integration with OpenTelemetry for standardized tracing
2. Real-time trace streaming and alerting
3. Anomaly detection for performance issues
4. Service health scoring based on trace data