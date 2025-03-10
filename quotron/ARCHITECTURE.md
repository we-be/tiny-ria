# Quotron Architecture

## System Overview

Quotron is a financial data processing system designed to collect, process, and analyze stock market data. It consists of several key components that work together to provide a comprehensive view of the market.

## Component Architecture

### API Service

The API Service provides HTTP endpoints for retrieving financial data. It serves as a network-based replacement for direct command execution, avoiding permission issues.

- **Technology**: Go, HTTP server
- **Endpoints**:
  - `GET /api/quote/{symbol}` - Get stock quote for a symbol
  - `GET /api/index/{index}` - Get market index data
  - `GET /api/health` - Check service health status
  - `GET /api/data-source/health` - Check health status of data sources
- **Dependencies**: PostgreSQL, Yahoo Finance Proxy

### API Scraper

A tool that fetches data from financial APIs.

- **Technology**: Go
- **Data Sources**: Alpha Vantage API, Yahoo Finance
- **Features**: Rate limiting handling, failover between data sources

### Yahoo Finance Proxy

A proxy server for Yahoo Finance data to enhance reliability and caching.

- **Technology**: Python, Flask
- **Features**: Caching, circuit breaker pattern, retries, metrics

### Scheduler

Manages periodic data collection jobs.

- **Technology**: Go, cron
- **Jobs**: Stock quotes, market indices
- **Features**: Configurable schedules, API-based data collection

### Dashboard

Web interface for monitoring and controlling the system.

- **Technology**: Python, Streamlit
- **Features**: System health monitoring, data visualization, job management

### Infrastructure

- **Database**: PostgreSQL
- **Event Streaming**: Kafka
- **Blob Storage**: MinIO

## Communication Patterns

1. **HTTP-based API**: Components communicate via HTTP APIs
2. **Database Integration**: Results are stored in PostgreSQL for persistence
3. **Event-driven**: Kafka for event distribution
4. **Container orchestration**: Docker Compose for local development

## Deployment Architecture

The system is containerized using Docker with the following services:

- api-service
- yahoo-proxy
- api-scraper
- browser-scraper
- auth-engine
- kafka
- postgres
- minio
- dashboard
- scheduler

## Recent Architectural Improvements

### HTTP-based API Service

- **Problem**: Previous architecture used command execution which led to permission issues
- **Solution**: Created a new API Service that provides HTTP endpoints
- **Benefits**:
  - Eliminated permission issues
  - Improved error handling and resilience
  - Enabled better load distribution and scaling
  - Enhanced monitoring capabilities

### Client/Server Model

- **Previous**: Scheduler directly executed API scraper binaries
- **Current**: Scheduler acts as a client to the API Service
- **Benefits**:
  - Decoupled components
  - Simplified error handling
  - Improved resilience with retries and timeouts
  - Reduced direct filesystem dependencies

### Enhanced Data Persistence

- Added PostgreSQL tables for stock quotes and market indices
- Implemented shared data store for job status tracking
- Created health monitoring records for data sources

### Infrastructure as Code

- Docker Compose for consistent deployment
- Simplified local development and testing
- Enabled easy horizontal scaling