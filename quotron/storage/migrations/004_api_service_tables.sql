-- Create table for stock quotes
CREATE TABLE IF NOT EXISTS stock_quotes (
    id SERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    change DECIMAL(10, 2) NOT NULL,
    change_percent DECIMAL(10, 2) NOT NULL,
    volume BIGINT NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    exchange VARCHAR(20) NOT NULL,
    source VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create index on symbol and timestamp for faster queries
CREATE INDEX IF NOT EXISTS idx_stock_quotes_symbol_timestamp ON stock_quotes(symbol, timestamp);

-- Note: market_indices table was previously defined in 001_initial_schema.sql
-- It was renamed from 'name' to 'index_name' in migration 007_fix_market_indices_column.sql

-- Create a table for job status tracking
CREATE TABLE IF NOT EXISTS scheduled_jobs (
    id SERIAL PRIMARY KEY,
    job_id VARCHAR(50) NOT NULL,
    job_type VARCHAR(50) NOT NULL,
    parameters JSONB NOT NULL,
    status VARCHAR(20) NOT NULL,
    result JSONB,
    error_message TEXT,
    started_at TIMESTAMP WITH TIME ZONE NOT NULL,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create index on job_id and status for faster queries
CREATE INDEX IF NOT EXISTS idx_scheduled_jobs_id_status ON scheduled_jobs(job_id, status);