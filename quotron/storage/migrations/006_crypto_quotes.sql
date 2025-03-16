-- Migration: 006_crypto_quotes.sql
-- Description: Add support for cryptocurrency quotes

-- Update the exchange enum to include CRYPTO
ALTER TYPE exchange ADD VALUE IF NOT EXISTS 'CRYPTO';

-- Create a dedicated view for cryptocurrency quotes (using the existing stock_quotes table)
CREATE VIEW crypto_quotes AS
SELECT *
FROM stock_quotes
WHERE exchange = 'CRYPTO'
ORDER BY timestamp DESC;

-- Create a view for the latest crypto prices
CREATE VIEW latest_crypto_prices AS
SELECT DISTINCT ON (symbol) *
FROM stock_quotes
WHERE exchange = 'CRYPTO'
ORDER BY symbol, timestamp DESC;

-- Add index for faster crypto filtering
CREATE INDEX idx_stock_quotes_exchange ON stock_quotes(exchange);

-- Down migration (for rollback)
-- DROP VIEW IF EXISTS latest_crypto_prices;
-- DROP VIEW IF EXISTS crypto_quotes;
-- DROP INDEX IF EXISTS idx_stock_quotes_exchange;
-- No direct way to remove enum value, would require type recreation