# Quotron Ingest Pipeline

The ingest pipeline is responsible for processing financial data from various sources, validating it, and storing it in the database.

## Components

- **schemas/**: Data validation schemas
  - `finance_schema.py`: Pydantic models for financial data

- **validation/**: Data validation and enrichment
  - `validators.py`: Validator and enricher classes

- **ingestor.py**: Core data processing logic

- **cli.py**: Command-line interface for the pipeline

## Data Flow

1. Raw data is collected by the API scraper or browser scraper
2. Data is passed to the ingestor for processing
3. Data is validated and cleaned
4. Valid data is enriched with additional information
5. Data is stored in the database
6. Statistics are computed and stored

## Usage

### Process Data Files

Process a file containing stock quotes:

```bash
python cli.py quotes sample_data.json --source api-scraper
```

Process a file containing market indices:

```bash
python cli.py indices sample_data.json --source api-scraper
```

Process a file containing both quotes and indices:

```bash
python cli.py mixed sample_data.json --source api-scraper
```

### Process Real-time Data

Simulate processing real-time data (for testing):

```bash
python cli.py realtime --source api-scraper --duration 60
```

### List Latest Data

List the latest quotes and indices:

```bash
python cli.py list --limit 10
```

## Development

### Dependencies

Install the required dependencies:

```bash
pip install -r requirements.txt
```

### Testing

The ingest pipeline can be tested using sample data:

```bash
# Start the PostgreSQL database
cd .. && docker-compose up -d postgres

# Set up the database schema
python cli.py setup

# Process sample data
python cli.py mixed sample_data.json --source manual

# View the results
python cli.py list
```

## Configuration

The pipeline can be configured via environment variables:

- Database connection parameters (see storage module documentation)
- Logging levels and formats
