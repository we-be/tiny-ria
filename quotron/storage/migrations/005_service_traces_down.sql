-- Migration: 005_service_traces_down.sql
-- Description: Remove service trace tracking tables

DROP TABLE IF EXISTS trace_spans;
DROP TABLE IF EXISTS service_traces;
DROP TABLE IF EXISTS service_dependencies;