-- Drop indices first
DROP INDEX IF EXISTS idx_stock_quotes_symbol_timestamp;
DROP INDEX IF EXISTS idx_scheduled_jobs_id_status;

-- Note: market_indices table was defined in 001_initial_schema.sql
-- Its index is now handled by 007_fix_market_indices_column.sql

-- Drop tables
DROP TABLE IF EXISTS stock_quotes;
DROP TABLE IF EXISTS scheduled_jobs;