-- 007_economic_indicators.sql
-- Migration to add economic indicators for US economy

-- Create economic_indicators table
CREATE TABLE IF NOT EXISTS economic_indicators (
    id SERIAL PRIMARY KEY,
    indicator_id VARCHAR(50) NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    unit VARCHAR(100),
    frequency VARCHAR(50),
    source VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create index on indicator_id for fast lookups
CREATE INDEX IF NOT EXISTS economic_indicators_indicator_id_idx ON economic_indicators (indicator_id);

-- Create economic_data table to store time series data
CREATE TABLE IF NOT EXISTS economic_data (
    id SERIAL PRIMARY KEY,
    indicator_id VARCHAR(50) NOT NULL,
    date DATE NOT NULL,
    value NUMERIC(20, 6) NOT NULL,
    is_estimate BOOLEAN DEFAULT FALSE,
    source VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create composite index for fast time series lookups
CREATE INDEX IF NOT EXISTS economic_data_indicator_date_idx ON economic_data (indicator_id, date);

-- Create economic_summary table to store calculated summaries
CREATE TABLE IF NOT EXISTS economic_summary (
    id SERIAL PRIMARY KEY,
    summary_date DATE NOT NULL,
    overall_health VARCHAR(50) NOT NULL,
    health_score INTEGER NOT NULL,
    indicators_up INTEGER NOT NULL,
    indicators_down INTEGER NOT NULL,
    indicators_stable INTEGER NOT NULL,
    report_json JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create index on summary_date for fast lookups
CREATE INDEX IF NOT EXISTS economic_summary_date_idx ON economic_summary (summary_date);

-- Insert basic economic indicators
INSERT INTO economic_indicators (indicator_id, name, description, unit, frequency, source) VALUES
('GDP', 'Gross Domestic Product', 'Real Gross Domestic Product', 'Billions of Dollars', 'Quarterly', 'FRED'),
('UNRATE', 'Unemployment Rate', 'Civilian Unemployment Rate', 'Percent', 'Monthly', 'FRED'),
('CPIAUCSL', 'Consumer Price Index', 'Consumer Price Index for All Urban Consumers: All Items', 'Index 1982-1984=100', 'Monthly', 'FRED'),
('FEDFUNDS', 'Federal Funds Rate', 'Federal Funds Effective Rate', 'Percent', 'Monthly', 'FRED'),
('PAYEMS', 'Nonfarm Payrolls', 'All Employees, Total Nonfarm', 'Thousands of Persons', 'Monthly', 'FRED'),
('JTSJOR', 'Job Openings', 'Job Openings: Total Nonfarm', 'Thousands', 'Monthly', 'FRED'),
('RRSFS', 'Retail Sales', 'Advance Retail Sales: Retail and Food Services, Total', 'Millions of Dollars', 'Monthly', 'FRED'),
('HOUST', 'Housing Starts', 'Housing Starts: Total New Privately Owned', 'Thousands of Units', 'Monthly', 'FRED'),
('CSUSHPISA', 'Housing Price Index', 'S&P/Case-Shiller U.S. National Home Price Index', 'Index Jan 2000=100', 'Monthly', 'FRED'),
('INDPRO', 'Industrial Production', 'Industrial Production Index', 'Index 2017=100', 'Monthly', 'FRED')
ON CONFLICT (indicator_id) DO NOTHING;