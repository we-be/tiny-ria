# Quotron

Quotron is the dedicated scraping and data ingestion pipeline within the tiny-ria project, aimed at systematically collecting, cleaning, and ingesting financial data from diverse web sources.

## Project Components

### 1. API Scraper (Go)
Located in `api-scraper/`, this component handles data collection from REST APIs.
- Makes HTTP requests to financial data APIs
- Handles rate limiting and authentication
- Standardizes data into common formats

### 2. Browser Scraper (Python)
Located in `browser-scraper/`, this component handles JavaScript-heavy websites.
- Uses Playwright or Selenium for browser automation
- Extracts data from complex UI elements
- Handles dynamic content loading

### 3. Authentication Engine (Python)
Located in `auth-engine/`, this component manages authentication for various services.
- Handles login credentials and session cookies
- Maintains authenticated sessions
- Provides middleware for authenticated requests

### 4. Ingest Pipeline (Python)
Located in `ingest-pipeline/`, this component processes raw data.
- Validates incoming data against schemas
- Enriches data with additional information
- Prepares data for storage

### 5. Events System (Python)
Located in `events/`, this component manages asynchronous communication.
- Produces events when new data is available
- Consumes events to trigger processing
- Uses Kafka for reliable messaging

### 6. Storage (SQL/NoSQL)
Located in `storage/`, this component manages data persistence.
- Stores structured data in SQL databases
- Stores unstructured data in blob storage
- Handles database migrations

## Getting Started

### Prerequisites
- Go 1.21+
- Python 3.11+
- Docker and Docker Compose

### Setup
1. Clone the repository
2. Set up environment variables (see `.env.example` in each component)
3. Start the required infrastructure: `docker-compose up -d`

### Running Components
- API Scraper: `cd api-scraper && go run cmd/main/main.go`
- Browser Scraper: `cd browser-scraper/playwright && python src/scraper.py`
- Auth Engine: `cd auth-engine && python service/auth_service.py`

## Development

### API Scraper
```
cd api-scraper
go mod download
go run cmd/main/main.go --api-key YOUR_API_KEY
```

### Browser Scraper
```
cd browser-scraper/playwright
pip install -r requirements.txt
python src/scraper.py
```

### Auth Engine
```
cd auth-engine
pip install -r requirements.txt
python service/auth_service.py
```

## Testing
Each component has its own tests in the respective directory or in the main `tests/` directory.

## Deployment
Docker images for each component can be built from their respective Dockerfiles.