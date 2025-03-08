-- Down Migration: 001_initial_schema_down.sql
-- Description: Rollback the initial database schema

-- Drop views
DROP VIEW IF EXISTS latest_stock_prices;
DROP VIEW IF EXISTS latest_market_indices;

-- Drop tables (in correct order to handle dependencies)
DROP TABLE IF EXISTS batch_statistics;
DROP TABLE IF EXISTS market_indices;
DROP TABLE IF EXISTS stock_quotes;
DROP TABLE IF EXISTS data_batches;

-- Drop enum types
DROP TYPE IF EXISTS exchange;
DROP TYPE IF EXISTS data_source;