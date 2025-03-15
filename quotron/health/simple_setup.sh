#!/bin/bash
# Simple database setup script just for demo purposes

# Create a simple table
psql -h localhost -p 5432 -U quotron -d quotron << EOF
-- Create a simple health table for demonstration
CREATE TABLE IF NOT EXISTS data_source_health (
    id SERIAL PRIMARY KEY,
    source_type VARCHAR(50) NOT NULL,
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

-- Add some default entries
INSERT INTO data_source_health (source_type, source_name, source_detail, status)
VALUES 
    ('api-scraper', 'yahoo_finance', 'Yahoo Finance Client', 'unknown'),
    ('api-scraper', 'yahoo_finance_proxy', 'Yahoo Finance Proxy', 'unknown'),
    ('test-source', 'test-client', 'Test Client', 'unknown')
ON CONFLICT (source_type, source_name) DO NOTHING;
EOF

echo "Created simple health table for demonstration"