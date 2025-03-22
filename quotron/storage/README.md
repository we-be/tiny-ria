# Quotron Storage Module

The storage module provides database access and management for financial data in the Quotron system.

## Components

- **sql/**: Database access and management utilities
  - `database.py`: Core database interface for the application
  - `db_manager.py`: Database migration and management utility

- **migrations/**: SQL migration files
  - `001_initial_schema.sql`: Initial database schema setup
  - `001_initial_schema_down.sql`: Rollback for initial schema

- **blob/**: Blob storage utilities (for storing large data objects like historical data files)

## Database Schema

The main database tables are:

- **stock_quotes**: Individual stock quote records with columns for symbol, price, exchange, etc.
- **market_indices**: Market index value records with `index_name` as the identifier column
- **data_batches**: Metadata for data batches
- **batch_statistics**: Statistical data computed from batches
- **crypto_quotes**: View for cryptocurrency quotes (filtered from stock_quotes where exchange='CRYPTO')

## Usage

### Setup Database

To set up the database schema:

```bash
# From the quotron directory
docker-compose up -d postgres
cd ingest-pipeline
python cli.py setup
```

### Database Operations

The `Database` class in `sql/database.py` provides a comprehensive interface for database operations:

```python
from storage.sql.database import Database

# Get a database instance
db = Database.get_instance()

# Query latest stock quotes
latest_quotes = db.get_latest_quotes()

# Query historical data
history = db.get_quotes_history("AAPL", limit=100)

# Close the connection when done
db.close()
```

### Database Migrations

The `DBManager` class in `sql/db_manager.py` provides utilities for database migrations:

```bash
# View migration status
python storage/sql/db_manager.py status

# Apply pending migrations
python storage/sql/db_manager.py up

# Roll back the most recent migration
python storage/sql/db_manager.py down
```

Alternatively, you can use the API service migration script:

```bash
cd quotron/api-service
./scripts/migrate.sh
```

#### Important Schema Notes

1. **Market Indices Table**:
   - Uses `index_name` column (NOT `name`) to identify market indices
   - Example index names: '^GSPC' (S&P 500), '^DJI' (Dow Jones)

2. **Exchange Enum**:
   - Valid values are: 'NYSE', 'NASDAQ', 'AMEX', 'OTC', 'OTHER', 'CRYPTO'
   - 'CRYPTO' is used for all cryptocurrency quotes

3. **Crypto Quotes**:
   - Stored in the same `stock_quotes` table as regular stocks
   - Use `exchange = 'CRYPTO'` to identify crypto quotes
   - Can be accessed via the `crypto_quotes` view

## Configuration

Database connection parameters are configured via environment variables:

- `DB_HOST`: Database hostname (default: "localhost")
- `DB_PORT`: Database port (default: 5432)
- `DB_NAME`: Database name (default: "quotron")
- `DB_USER`: Database username (default: "quotron")
- `DB_PASS`: Database password (default: "quotron")
- `DB_POOL_MIN`: Minimum connections in the pool (default: 1)
- `DB_POOL_MAX`: Maximum connections in the pool (default: 10)
