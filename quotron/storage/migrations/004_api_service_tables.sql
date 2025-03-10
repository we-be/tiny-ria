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

-- Create table for market indices
CREATE TABLE IF NOT EXISTS market_indices (
    id SERIAL PRIMARY KEY,
    index_name VARCHAR(50) NOT NULL,
    value DECIMAL(12, 2) NOT NULL,
    change DECIMAL(10, 2) NOT NULL,
    change_percent DECIMAL(10, 2) NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    source VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create index on index_name and timestamp for faster queries
CREATE INDEX IF NOT EXISTS idx_market_indices_name_timestamp ON market_indices(index_name, timestamp);

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