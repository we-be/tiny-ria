-- quotron/storage/migrations/002_investment_models.sql

-- Create tables for investment models and their holdings

CREATE TABLE investment_models (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider VARCHAR(255) NOT NULL,
    model_name TEXT NOT NULL,
    detail_level VARCHAR(50) NOT NULL, -- 'full', 'partial', or 'minimal'
    source VARCHAR(50),
    fetched_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(provider, model_name, fetched_at)
);

CREATE TABLE model_holdings (
    id SERIAL PRIMARY KEY,
    model_id UUID REFERENCES investment_models(id) ON DELETE CASCADE,
    ticker VARCHAR(20),
    position_name TEXT,
    allocation DECIMAL(10, 4),
    asset_class VARCHAR(50),
    sector VARCHAR(50),
    additional_metadata JSONB
);

CREATE TABLE sector_allocations (
    id SERIAL PRIMARY KEY,
    model_id UUID REFERENCES investment_models(id) ON DELETE CASCADE,
    sector VARCHAR(50),
    allocation_percent DECIMAL(10, 4)
);

-- Create indexes for queries we expect to perform often
CREATE INDEX idx_model_holdings_model_id ON model_holdings(model_id);
CREATE INDEX idx_sector_allocations_model_id ON sector_allocations(model_id);
CREATE INDEX idx_investment_models_provider ON investment_models(provider);