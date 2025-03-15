# CLAUDE.md - Assistant Guidelines

## Project Structure
- `agent/`: Agent components
- `mcp/`: Master Control Program 
- `quotron/`: Financial data scraping and ingestion pipeline
  - `api-scraper/`: Golang-based API scraping
  - `browser-scraper/`: Python-based browser automation (Playwright/Selenium)
  - `auth-engine/`: Authentication and session management
  - `cli/`: Unified command-line interface for managing services
  - `health/`: Service health monitoring and reporting
  - `ingest-pipeline/`: ETL processing
  - `events/`: Event-driven data distribution

## Commands
- CLI: `./quotron [command] [options]`
- Start services: `./quotron start [service...]`
- Stop services: `./quotron stop [service...]`
- Check health: `./quotron health`
- Run tests: `./quotron test [test_type]`
- Golang (api-scraper): `cd quotron/api-scraper && go test ./...`
- Python (browser/auth): `cd quotron/{module} && python -m pytest`
- Single test: `python -m pytest path/to/test_file.py::test_function`

## Code Style
- Go: Follow Go standard formatting with `gofmt`
- Python: Use Black formatter, flake8 linter, and type hints
- Imports: Group standard lib, third-party, local packages
- Naming: snake_case for Python, camelCase for Go
- Error handling: Use structured errors with context
- Documentation: Use docstrings for all public functions

## Commit Messages
- Format: `<type>(quotron/<module>): <subject>`
- Types: feat, fix, docs, style, refactor, test, chore
- Example: `feat(quotron/api-scraper): implement Yahoo Finance connector`

This document will be updated as the codebase evolves.