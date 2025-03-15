#!/bin/bash
# Setup database for health service

set -e

# Set the working directory to the health service root
cd "$(dirname "$0")"

# Database connection parameters
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5432}
DB_NAME=${DB_NAME:-quotron}
DB_USER=${DB_USER:-quotron}
DB_PASSWORD=${DB_PASSWORD:-quotron}

# Create the schema file
cat > health_schema.sql << EOF
-- Check if the uuid-ossp extension is available
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Drop old tables if they exist
DROP TABLE IF EXISTS health_reports;
DROP TABLE IF EXISTS system_health_status;
DROP TABLE IF EXISTS data_source_health;

-- Drop any existing enums
DROP TYPE IF EXISTS data_source;
DROP TYPE IF EXISTS api_source;
DROP TYPE IF EXISTS web_source;

-- Create source type enum
CREATE TYPE data_source AS ENUM ('api-scraper', 'browser-scraper', 'test-source');

-- Create the health reports table
CREATE TABLE IF NOT EXISTS health_reports (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_type VARCHAR(50) NOT NULL,
    source_name VARCHAR(50) NOT NULL,
    source_detail VARCHAR(100),
    status VARCHAR(20) NOT NULL DEFAULT 'unknown',
    last_check TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_success TIMESTAMP WITH TIME ZONE,
    error_count INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    response_time_ms INTEGER,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(source_type, source_name)
);

-- Create the system health status table
CREATE TABLE IF NOT EXISTS system_health_status (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    status VARCHAR(20) NOT NULL DEFAULT 'unknown',
    score INTEGER NOT NULL DEFAULT 0,
    last_check TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    message TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Add indexes for quick lookups
CREATE INDEX IF NOT EXISTS idx_health_reports_status ON health_reports(status);
CREATE INDEX IF NOT EXISTS idx_health_reports_source ON health_reports(source_type, source_name);
CREATE INDEX IF NOT EXISTS idx_health_reports_last_check ON health_reports(last_check);
CREATE INDEX IF NOT EXISTS idx_system_health_last_check ON system_health_status(last_check);

-- Insert dummy data
INSERT INTO health_reports (source_type, source_name, source_detail, status) 
VALUES 
    ('api-scraper', 'alpha_vantage', 'Alpha Vantage Financial API', 'unknown'),
    ('api-scraper', 'yahoo_finance', 'Yahoo Finance Go Library', 'unknown'),
    ('api-scraper', 'yahoo_finance_proxy', 'Yahoo Finance Python Proxy', 'unknown'),
    ('browser-scraper', 'slickcharts', 'SlickCharts S&P 500 Scraper', 'unknown')
ON CONFLICT (source_type, source_name) DO NOTHING;
EOF

echo "Created database schema file"

# Execute the SQL file using psql
echo "Setting up database tables..."
PGPASSWORD=${DB_PASSWORD} psql -h ${DB_HOST} -p ${DB_PORT} -U ${DB_USER} -d ${DB_NAME} -f health_schema.sql

echo "Database setup complete"