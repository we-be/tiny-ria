# Test Plan for ETL Architecture Fix

This test plan outlines the steps to verify that our changes have properly fixed the issue where the scheduler service was bypassing the ETL pipeline by writing directly to the database.

## 1. Setup and Preparation

### Redis Stream Setup
- Run the `./setup_redis_streams.sh` script to initialize Redis streams and consumer groups
- Verify that all required streams (`quotron:stocks:stream`, `quotron:crypto:stream`, `quotron:indices:stream`) exist
- Confirm that the consumer group `quotron:etl` is properly created for each stream

### Redis Monitoring
- Run `./crypto_redis_monitor` to observe Redis channels and streams
- Verify that the monitor can detect messages on both PubSub channels and Streams
- Check that message counts are tracked correctly

## 2. Component Tests

### Publisher Tests
- Use `test_crypto_etl_flow.go` to test publishing to Redis
- Verify that messages are correctly published to both PubSub and Streams
- Confirm that message format is consistent with what the ETL service expects

### Consumer Tests
- Use `test_crypto_etl_flow.go` to test consuming from Redis
- Verify that messages can be consumed from both PubSub and Streams
- Check that consumer groups work properly with acknowledgments

### Scheduler Job Tests
- Run `go run quotron_crypto_job.go -symbols=BTC-USD,ETH-USD`
- Verify that the job correctly fetches data and publishes to Redis
- Confirm that no direct database access occurs

## 3. Integration Tests

### Complete ETL Flow Test
- Run `./etl_integration_test.sh` to test the entire pipeline
- Verify the test generates data, publishes to Redis, and consumes it
- Check that both PubSub and Stream mechanisms work together

### Redis-to-ETL Communication
- Start the ETL service: `quotron start etl`
- Run the crypto job: `go run quotron_crypto_job.go`
- Verify that messages flow from the job to Redis to ETL
- Check ETL logs to confirm messages are processed

### Scheduler-to-Database Flow
- Start all services: `quotron start scheduler etl`
- Verify scheduler jobs publish to Redis instead of writing directly to DB
- Confirm ETL service correctly processes the messages and writes to DB
- Query the database to verify data is stored correctly

## 4. Error Handling and Recovery

### Stream Recovery
- Test ETL service recovery after disconnection from Redis
- Verify that unacknowledged messages are redelivered
- Check that the service resumes processing from where it left off

### Invalid Message Handling
- Test with malformed messages sent to Redis
- Verify that ETL service properly handles parsing errors
- Confirm that valid messages continue to be processed

## 5. Monitoring and Logging

### Real-time Monitoring
- Run `./crypto_redis_monitor -v` during system operation
- Verify that all message types (stocks, crypto, indices) are flowing
- Check for any errors or anomalies in the message flow

### Log Analysis
- Examine ETL service logs to verify processing of all message types
- Check for subscription confirmations to all channels
- Verify proper initialization of all streams and consumer groups

## 6. Performance and Load Testing

### Multi-worker Test
- Configure ETL with multiple workers
- Generate high volume of messages to test parallel processing
- Verify that processing time scales appropriately with worker count

### Stream Backlog Test
- Generate a backlog of messages in Redis streams
- Start the ETL service and verify it processes the backlog
- Check that no messages are lost during catch-up

## 7. Validation Checklist

Before considering the fix complete, verify:

- [ ] ETL service subscribes to ALL three channels (stocks, crypto, indices)
- [ ] Messages flow correctly through BOTH PubSub and Streams
- [ ] Scheduler publishes to Redis only (no direct DB access)
- [ ] Consumer groups are properly maintained for reliable processing
- [ ] No duplicate message processing occurs
- [ ] Database schema correctly stores all message types
- [ ] Error handling and recovery mechanisms work properly