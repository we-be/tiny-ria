-- Migration: 003_data_source_health.sql
-- Description: Add data source health tracking table

-- Create more specific enum for data sources
CREATE TYPE api_source AS ENUM ('alpha_vantage', 'yahoo_finance', 'yahoo_finance_rest', 'yahoo_finance_proxy');
CREATE TYPE web_source AS ENUM ('slickcharts', 'other');

-- Create a table for data source health
CREATE TABLE data_source_health (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_type data_source NOT NULL,
    source_name VARCHAR(50) NOT NULL,
    source_detail VARCHAR(100),
    last_check TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_success TIMESTAMP WITH TIME ZONE,
    status VARCHAR(20) NOT NULL DEFAULT 'unknown',
    error_count INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    response_time_ms INTEGER,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(source_type, source_name)
);

-- Add index for quick status checks
CREATE INDEX idx_data_source_health_status ON data_source_health(status);
CREATE INDEX idx_data_source_health_source ON data_source_health(source_type, source_name);

-- Insert default entries for known data sources
INSERT INTO data_source_health (source_type, source_name, source_detail, status) 
VALUES 
    ('api-scraper', 'alpha_vantage', 'Alpha Vantage Financial API', 'unknown'),
    ('api-scraper', 'yahoo_finance', 'Yahoo Finance Go Library', 'unknown'),
    ('api-scraper', 'yahoo_finance_rest', 'Yahoo Finance REST API', 'unknown'),
    ('api-scraper', 'yahoo_finance_proxy', 'Yahoo Finance Python Proxy', 'unknown'),
    ('browser-scraper', 'slickcharts', 'SlickCharts S&P 500 Scraper', 'unknown'),
    ('browser-scraper', 'other', 'Generic Browser Scraper', 'unknown');