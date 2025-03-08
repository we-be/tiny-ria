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

- **stock_quotes**: Individual stock quote records
- **market_indices**: Market index value records
- **data_batches**: Metadata for data batches
- **batch_statistics**: Statistical data computed from batches

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

## Configuration

Database connection parameters are configured via environment variables:

- `DB_HOST`: Database hostname (default: "localhost")
- `DB_PORT`: Database port (default: 5432)
- `DB_NAME`: Database name (default: "quotron")
- `DB_USER`: Database username (default: "quotron")
- `DB_PASS`: Database password (default: "quotron")
- `DB_POOL_MIN`: Minimum connections in the pool (default: 1)
- `DB_POOL_MAX`: Maximum connections in the pool (default: 10)
