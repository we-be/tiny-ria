# Quotron Test Utilities

This directory contains test utilities for various aspects of the Quotron system.

## Cryptocurrency Test Tools

The following tools are available for testing the cryptocurrency quote functionality:

1. **test_crypto_quote.go**: Tests fetching and publishing a single cryptocurrency quote
   ```
   go run test_crypto_quote.go -symbol BTC-USD
   ```

2. **test_crypto_job_manual.go**: Manually runs the crypto job with multiple symbols
   ```
   go run test_crypto_job_manual.go -symbols BTC-USD,ETH-USD,SOL-USD
   ```

3. **crypto_monitor**: Monitors the Redis channel for cryptocurrency quotes
   ```
   ./crypto_monitor
   ```

## Redis Test Tools

Redis-related test utilities:

1. **redis_sub_main.go**: Subscribes to stock quotes channel
   ```
   go run redis_sub_main.go
   ```

2. **redis_monitor_main.go**: Monitors Redis channels for activity
   ```
   go run redis_monitor_main.go
   ```

3. **redis_test_main.go**: Basic Redis functionality test
   ```
   go run redis_test_main.go
   ```

## ETL Test Tools

ETL pipeline test utilities:

1. **etl_publisher_main.go**: Publishes test messages to the ETL queue
   ```
   go run etl_publisher_main.go
   ```

2. **etl_worker_main.go**: Simple ETL worker that processes messages
   ```
   go run etl_worker_main.go
   ```