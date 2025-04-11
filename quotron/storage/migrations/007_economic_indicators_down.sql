-- 007_economic_indicators_down.sql
-- Downgrade migration to remove economic indicators tables

DROP TABLE IF EXISTS economic_summary;
DROP TABLE IF EXISTS economic_data;
DROP TABLE IF EXISTS economic_indicators;