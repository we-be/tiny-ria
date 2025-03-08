-- Migration: 001_initial_schema.sql
-- Description: Initial database schema for financial data storage

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create enum types
CREATE TYPE data_source AS ENUM ('api-scraper', 'browser-scraper', 'manual');
CREATE TYPE exchange AS ENUM ('NYSE', 'NASDAQ', 'AMEX', 'OTC', 'OTHER');

-- Create a table for stock quotes
CREATE TABLE stock_quotes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    symbol VARCHAR(20) NOT NULL,
    price DECIMAL(19, 4) NOT NULL CHECK (price >= 0),
    change DECIMAL(19, 4) NOT NULL,
    change_percent DECIMAL(10, 4) NOT NULL,
    volume BIGINT NOT NULL CHECK (volume >= 0),
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    exchange exchange NOT NULL,
    source data_source NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    batch_id VARCHAR(50),
    -- Create indexes for common queries
    CONSTRAINT stock_quotes_reasonable_price CHECK (price BETWEEN 0.0001 AND 1000000)
);

-- Create a table for market indices
CREATE TABLE market_indices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(50) NOT NULL,
    value DECIMAL(19, 4) NOT NULL CHECK (value >= 0),
    change DECIMAL(19, 4) NOT NULL,
    change_percent DECIMAL(10, 4) NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    source data_source NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    batch_id VARCHAR(50)
);

-- Create a table for data batches
CREATE TABLE data_batches (
    id VARCHAR(50) PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE,
    status VARCHAR(20) NOT NULL DEFAULT 'created',
    quote_count INTEGER NOT NULL DEFAULT 0,
    index_count INTEGER NOT NULL DEFAULT 0,
    source data_source NOT NULL,
    metadata JSONB
);

-- Create a table for statistics
CREATE TABLE batch_statistics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    batch_id VARCHAR(50) NOT NULL REFERENCES data_batches(id),
    mean_price DECIMAL(19, 4),
    median_price DECIMAL(19, 4),
    mean_change_percent DECIMAL(10, 4),
    positive_change_count INTEGER,
    negative_change_count INTEGER,
    unchanged_count INTEGER,
    total_volume BIGINT,
    statistics_json JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create indexes for common queries
CREATE INDEX idx_stock_quotes_symbol ON stock_quotes(symbol);
CREATE INDEX idx_stock_quotes_timestamp ON stock_quotes(timestamp);
CREATE INDEX idx_stock_quotes_batch_id ON stock_quotes(batch_id);
CREATE INDEX idx_market_indices_name ON market_indices(name);
CREATE INDEX idx_market_indices_timestamp ON market_indices(timestamp);
CREATE INDEX idx_market_indices_batch_id ON market_indices(batch_id);
CREATE INDEX idx_data_batches_status ON data_batches(status);
CREATE INDEX idx_data_batches_created_at ON data_batches(created_at);

-- Create a view for the latest stock prices
CREATE VIEW latest_stock_prices AS
SELECT DISTINCT ON (symbol) *
FROM stock_quotes
ORDER BY symbol, timestamp DESC;

-- Create a view for the latest market indices
CREATE VIEW latest_market_indices AS
SELECT DISTINCT ON (name) *
FROM market_indices
ORDER BY name, timestamp DESC;

-- Down migration (for rollback)
-- The down migration would be in a separate file named 001_initial_schema_down.sql