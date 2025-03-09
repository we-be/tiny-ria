-- quotron/storage/migrations/002_investment_models_down.sql

-- Drop tables in reverse order to handle dependencies
DROP TABLE IF EXISTS sector_allocations;
DROP TABLE IF EXISTS model_holdings;
DROP TABLE IF EXISTS investment_models;