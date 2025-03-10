-- Migration: 005_service_traces.sql
-- Description: Add service trace tracking tables

-- Create service trace tables
CREATE TABLE service_traces (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    trace_id VARCHAR(64) NOT NULL,
    parent_id VARCHAR(64),
    name VARCHAR(100) NOT NULL,
    service VARCHAR(50) NOT NULL,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE NOT NULL,
    duration_ms INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'success',
    error_message TEXT,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create table for storing relationships between traces
CREATE TABLE trace_spans (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    trace_id VARCHAR(64) NOT NULL,
    parent_span_id VARCHAR(64) NOT NULL,
    child_span_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create table for storing service dependencies
CREATE TABLE service_dependencies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_service VARCHAR(50) NOT NULL,
    target_service VARCHAR(50) NOT NULL,
    dependency_type VARCHAR(20) NOT NULL DEFAULT 'http',
    is_critical BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(source_service, target_service)
);

-- Insert known service dependencies
INSERT INTO service_dependencies (source_service, target_service, dependency_type, is_critical) 
VALUES 
    ('scheduler', 'api-service', 'http', TRUE),
    ('api-service', 'yahoo_finance_proxy', 'http', TRUE),
    ('api-service', 'alpha_vantage', 'http', TRUE),
    ('api-service', 'database', 'sql', TRUE),
    ('browser-scraper', 'slickcharts', 'http', FALSE),
    ('dashboard', 'database', 'sql', FALSE);

-- Add indexes for performance
CREATE INDEX idx_service_traces_trace_id ON service_traces(trace_id);
CREATE INDEX idx_service_traces_service ON service_traces(service);
CREATE INDEX idx_service_traces_time ON service_traces(start_time);
CREATE INDEX idx_trace_spans_trace_id ON trace_spans(trace_id);