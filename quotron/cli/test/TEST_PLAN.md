# Test Plan for ETL Architecture Fix

This test plan outlines the steps to verify that our changes have properly fixed the issue where the scheduler service was bypassing the ETL pipeline by writing directly to the database.

## 1. Unit Tests

### Redis Publishing Tests
- Verify that `PublishToStockStream()` method correctly adds messages to the Redis stock stream
- Verify that `PublishToCryptoStream()` method correctly adds messages to the Redis crypto stream
- Verify that `PublishToMarketIndexStream()` method correctly adds messages to the Redis market index stream

### Scheduler Job Tests
- Verify that `StockQuoteJob.fetchQuoteFromAPI()` publishes to Redis and doesn't access the database directly
- Verify that `CryptoQuoteJob.fetchQuoteFromAPI()` publishes to Redis and doesn't access the database directly
- Verify that `MarketIndexJob.fetchMarketDataFromAPI()` publishes to Redis and doesn't access the database directly

### ETL Service Tests
- Verify that the ETL service correctly subscribes to all Redis channels including the new market index channel
- Verify that the ETL service correctly processes stock, crypto, and market index messages from Redis streams
- Verify that the `processMarketIndex()` function correctly inserts data into the database

## 2. Integration Tests

### Redis Publishing Flow
- Run the test script `./test_crypto_etl` with multiple symbols
- Verify that messages are published to both Redis PubSub and Streams using `./crypto_redis_monitor`
- Check that different types of data (stocks, crypto, market indices) are properly published to their respective channels/streams

### End-to-End Flow
- Run `./etl_integration_test.sh` to test the complete flow
- Verify that the ETL service consumes messages from all Redis channels and streams
- Check the Redis monitor output to confirm proper message flow
- Query the database to confirm data has been inserted correctly

## 3. Regression Tests

### Legacy PubSub Flow
- Verify that legacy PubSub publishers still work with the updated ETL service
- Confirm that the PubSub-to-Stream bridge in the ETL service handles all message types correctly

### Database Schema Compatibility
- Verify that the database schema is compatible with the new data types
- Check that the market index schema exists and can be written to

## 4. Manual Tests

### Scheduler Service Tests
- Manually run the scheduler service with the updated code
- Verify that no direct database calls are made by enabling debug logs
- Check Redis to ensure messages are flowing through the proper channels/streams

### Complete System Flow
- Start all components (scheduler, ETL, Redis) together
- Run a scheduler job that processes stocks, crypto, and market indices
- Verify data flows correctly from scheduler → Redis → ETL → Database

## 5. Performance Tests

### Redis Performance
- Measure message throughput through Redis under load
- Verify that multiple consumers (ETL workers) can process messages efficiently

### Database Performance
- Check database query performance when receiving data through the ETL pipeline
- Verify that batch processing works correctly for high-volume scenarios

## 6. Monitoring Tests

### Redis Monitoring
- Verify that Redis monitoring tools show the correct channels and streams
- Confirm that subscriber counts are accurate in the Redis monitor

### ETL Service Monitoring
- Check ETL logs to ensure that all message types are processed correctly
- Verify that error handling works properly for malformed messages