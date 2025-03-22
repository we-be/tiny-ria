# Quotron CLI Test Utilities

This directory contains test utilities for the Quotron CLI.

## ETL Service Tests

The ETL service is responsible for consuming data from Redis and storing it in the database. The following test utilities are available:

### Redis Publishing and Monitoring

- `crypto_redis_monitor` - Monitors Redis channels and streams for activity
- `test_crypto_etl` - Tests the crypto quote job publishing to Redis (without direct DB access)

### Integration Test

To run the integration test:

```bash
./etl_integration_test.sh
```

This will:
1. Start a Redis monitor to watch channels and streams
2. Run the crypto quote job to publish data to Redis
3. Show the Redis monitor output

## Data Flow Architecture

The correct data flow architecture is:

1. **Scheduler Service/API Service**
   - Fetch data from external sources (Yahoo Finance, etc.)
   - Publish to Redis (NOT directly to database)
   - Uses channels: `quotron:stocks`, `quotron:crypto`
   - Uses streams: `quotron:stocks:stream`, `quotron:crypto:stream`

2. **ETL Service**
   - Subscribe to Redis channels and streams
   - Process data
   - Store in PostgreSQL database

Using this architecture ensures:
- Loose coupling between components
- Scalability (multiple producers and consumers)
- Reliability (with Redis persistence and consumer groups)
- Monitoring capabilities