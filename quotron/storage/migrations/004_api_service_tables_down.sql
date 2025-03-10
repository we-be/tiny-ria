-- Drop indices first
DROP INDEX IF EXISTS idx_stock_quotes_symbol_timestamp;
DROP INDEX IF EXISTS idx_market_indices_name_timestamp;
DROP INDEX IF EXISTS idx_scheduled_jobs_id_status;

-- Drop tables
DROP TABLE IF EXISTS stock_quotes;
DROP TABLE IF EXISTS market_indices;
DROP TABLE IF EXISTS scheduled_jobs;