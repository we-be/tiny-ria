# Test Plan for API Service Implementation

## 1. Component Tests

### 1.1 API Service Tests
- Start API service and verify it's running
- Test `/api/health` endpoint returns 200 OK
- Test `/api/quote/{symbol}` endpoint with various symbols (AAPL, MSFT, etc.)
- Test `/api/index/{index}` endpoint with various indices (SPY, QQQ, DIA)
- Test `/api/data-source/health` endpoint returns proper status

### 1.2 Yahoo Finance Proxy Tests
- Verify proxy server starts correctly
- Test the proxy's health endpoint
- Test direct quote retrieval from the proxy
- Verify caching works as expected

### 1.3 Database Tests
- Verify database tables are created correctly (run migrations)
- Test data insertion for stock quotes
- Test data insertion for market indices
- Verify data source health records are being updated

## 2. Integration Tests

### 2.1 Scheduler → API Service Integration
- Run scheduler with API service configuration
- Verify stock quote job works with API service
- Verify market index job works with API service
- Check that data is properly stored in JSON files and database

### 2.2 End-to-End Flow Test
- Start all services (API service, Yahoo proxy, scheduler)
- Run a full job execution cycle
- Verify data flows correctly through all components
- Check database records

## 3. Performance Tests

### 3.1 Load Testing
- Test concurrent requests to API service
- Verify rate limiting and queueing work correctly
- Check response times under load

### 3.2 Resilience Testing
- Test fallback from Alpha Vantage to Yahoo Finance
- Test service restart recovery
- Verify database connection resilience

## 4. Deployment Tests

### 4.1 Docker Tests
- Build Docker images for all services
- Test Docker Compose configuration
- Verify inter-container communication
- Check environment variable configuration

## 5. Running Tests

```bash
# Start API service for testing
./quotron start api

# Run basic API service tests
./quotron test api

# Run integration tests
cd tests/integration
python test_integration.py --api

# Test Yahoo Finance integration
cd tests
python yahoo_finance_test.py

# Test scheduler jobs with API service
./quotron test job stock_quotes
./quotron test job market_indices
```