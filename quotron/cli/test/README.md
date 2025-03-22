# Quotron ETL Tests

This directory contains tests and utilities for the Quotron ETL architecture. The architecture follows a publisher-consumer pattern where:

1. Publishers (e.g., schedulers, API scrapers) publish financial data to Redis
2. The ETL service consumes this data and stores it in the database

## Architecture

The data flow is:

```
Publishers (Scheduler, API Scrapers) → Redis → ETL Service → Database
```

Redis serves as a message broker with two mechanisms:
- **PubSub**: For backward compatibility
- **Streams**: For more reliable message delivery with consumer groups

## Key Components

- `test_crypto_etl_flow.go`: Demonstrates the complete ETL flow (publisher → Redis → consumer)
- `crypto_redis_monitor.go`: Monitors Redis channels and streams, displaying message counts
- `setup_redis_streams.sh`: Sets up Redis streams and consumer groups for the ETL service
- `etl_integration_test.sh`: Integration test script that verifies the entire data flow
- `quotron_crypto_job.go`: Sample job implementation used by the scheduler

## Running Tests

### Setup Redis Streams

```bash
./setup_redis_streams.sh
```

This script creates the required Redis streams and consumer groups for the ETL service.

### Monitor Redis

```bash
go run crypto_redis_monitor.go
# or with message content:
go run crypto_redis_monitor.go -v
```

This utility displays message counts for all channels and streams and logs messages as they arrive.

### Test ETL Flow

```bash
go run test_crypto_etl_flow.go
```

This simulates both the publisher (scheduler) and consumer (ETL) in a single process, demonstrating the complete flow.

### Integration Test

```bash
./etl_integration_test.sh
```

This script orchestrates a complete integration test, verifying all components work together.

## Troubleshooting

If the ETL service isn't processing messages from all channels:

1. Run `./setup_redis_streams.sh` to ensure streams and consumer groups exist
2. Run `go run crypto_redis_monitor.go` to verify messages are being published
3. Check ETL logs for subscription failures
4. Restart the ETL service: `quotron stop etl && quotron start etl`