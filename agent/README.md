# RIA - Responsive Investment Assistant

RIA is an autonomous financial data monitoring and analysis tool that interacts with the Quotron financial data system. It provides real-time monitoring, portfolio analysis, AI-powered chat, and data visualization capabilities using **real market data only**.

## Features

- **Real-time Price Monitoring**: Monitor stock prices with customizable thresholds and get alerts on significant price movements
- **Portfolio Summary**: Generate comprehensive summaries of stock and cryptocurrency portfolios
- **Data Retrieval**: Fetch current data for stocks, cryptocurrencies, and market indices from reliable financial APIs
- **AI Assistant**: Interactive chat-based AI assistant that can answer financial questions and retrieve market data
- **Message Queue Integration**: Publish price alerts to Redis streams for scalable processing
- **AI Price Analysis**: Automatically analyze price movements with AI to provide insights and context
- **Extensible Design**: Easily add new capabilities through the agent's modular architecture
- **Web Chat Interface**: Modern web-based chat UI for intuitive interaction with financial data and visualizations

## Installation

```bash
# Clone the repository if you haven't already
git clone https://github.com/we-be/tiny-ria.git
cd tiny-ria/agent

# Build the agent
./build.sh
```

## Usage

### Unified CLI

The unified command-line interface provides access to all RIA features:

```
RIA - Responsive Investment Assistant for financial data monitoring and AI interaction

Usage:
  ria [OPTIONS] COMMAND [ARGS...]

Commands:
  help        Show help information
  version     Show version information
  monitor     Monitor price movements for stocks and cryptocurrencies
  fetch       Fetch current data for financial instruments
  portfolio   Generate a portfolio summary
  chat        Start the interactive AI assistant in the terminal
  web         Start the web-based chat interface
  ai-alerter  Start the AI alerter service

Global Options:
  -api-host string
        Host of the Quotron API service (default "localhost")
  -api-port int
        Port of the Quotron API service (default 8080)
  -api-key string
        API key for OpenAI or Anthropic (if empty, OPENAI_API_KEY or ANTHROPIC_API_KEY env var is used)
  -redis string
        Redis server address (default "localhost:6379")
  -debug
        Enable debug mode (default false)

Note: Always uses real market data from the Quotron API service.
```

Each command has its own set of options. Use `ria help COMMAND` to see command-specific help.

## Examples

### Monitor Stock Prices

```bash
# Monitor AAPL, MSFT, and GOOG with 1.5% alert threshold
./bin/ria monitor --symbols=AAPL,MSFT,GOOG --threshold=1.5
```

### Generate Portfolio Summary

```bash
# Generate a summary for a mixed portfolio of stocks and cryptocurrencies
./bin/ria portfolio --symbols=AAPL,MSFT,GOOG --cryptos=BTC-USD,ETH-USD
```

### Fetch Current Data

```bash
# Fetch current data for specific symbols
./bin/ria fetch --symbols=AAPL,MSFT --cryptos=BTC-USD --indices=SPY
```

### Interactive AI Assistant

```bash
# Start the interactive assistant
export OPENAI_API_KEY=your_openai_api_key
./bin/ria chat

# Or provide the API key directly
./bin/ria chat --api-key=your_openai_api_key
```

### Non-Interactive AI Query

```bash
# Run a single query in non-interactive mode
./bin/ria chat --interactive=false --query="What's the current price of AAPL and MSFT?"
```

### AI Price Alerter

```bash
# Start the AI alerter to analyze price movements
export OPENAI_API_KEY=your_openai_api_key
./bin/ria ai-alerter

# Or provide the API key directly
./bin/ria ai-alerter --api-key=your_openai_api_key --redis=localhost:6379
```

### Web Chat Interface

```bash
# Start the web chat interface
export OPENAI_API_KEY=your_openai_api_key
./bin/ria web

# Or provide the API key and custom port
./bin/ria web --api-key=your_openai_api_key --port=8090 --redis=localhost:6379
```

Then open your browser to http://localhost:8090 to interact with the agent through the chat interface.

### Enable Queue Publishing in Monitor

```bash
# Monitor with queue integration enabled
./bin/ria monitor --symbols=AAPL,MSFT,GOOG,BTC-USD,ETH-USD --threshold=1.0 --enable-queue=true
```

## Prerequisites

- The Quotron API service must be running (critical for real-time data)
- Go 1.21 or later
- Redis server (for queue integration features)
- OpenAI API key (for AI assistant and alerter)

## Starting Required Services

Before using the agent, make sure the Quotron services are running:

```bash
# Start the Quotron services
cd ../quotron/cli
./quotron start
```

## Setting Up a Complete Monitoring System

You can set up a complete RIA environment with these components:

```bash
# Terminal 1: Start the web interface
./bin/ria web --api-key=your_openai_api_key

# Terminal 2: Start price monitoring with alerts published to Redis
./bin/ria monitor --symbols=AAPL,MSFT,GOOG,BTC-USD,ETH-USD --threshold=1.0 --enable-queue

# Terminal 3: Start the AI alerter for detailed analysis of price movements
./bin/ria ai-alerter --api-key=your_openai_api_key
```

This creates a comprehensive system where:
1. The monitor watches for significant price movements
2. Alerts are published to Redis streams in real-time
3. The web interface displays financial data and delivers alerts
4. The AI alerter provides detailed analysis of market movements

## Development

RIA is built with a modular design that makes it easy to extend:

- `pkg/agent.go`: Core agent implementation with monitoring and data retrieval capabilities
- `pkg/assistant.go`: AI assistant implementation for natural language interaction
- `pkg/llm.go`: LLM client for interacting with OpenAI or Anthropic APIs
- `pkg/queue.go`: Message queue integration with Redis streams
- `cmd/unified/main.go`: Unified CLI that brings all functionality together

To add new capabilities, extend the `Agent` struct in `pkg/agent.go` with additional methods.

### Message Queue Architecture

The agent uses Redis streams for message queuing to enable a scalable, decoupled architecture:

1. The agent publishes price alerts to the `quotron:alerts:stream` stream when it detects price movements above the threshold
2. Consumer groups can be created to process these alerts, allowing multiple consumers to work on different subsets of alerts
3. The AI alerter consumes alerts from the stream and uses the AI assistant to analyze price movements
4. This architecture allows for adding more consumers without modifying the monitoring code

```
+----------------+           +-------------------+           +----------------+
| RIA Monitor    |  -------> | Redis Stream     |  -------> | RIA Alerter    |
| (Publisher)    |  alerts   | (Message Queue)  |  consume  | (Consumer)     |
+----------------+           +-------------------+           +----------------+
        |                            ^                               |
        |                            |                               v
        v                            |                        +----------------+
+----------------+                   |                        | AI API         |
| Quotron API    |                   |                        | (Analysis)     |
+----------------+                   |                        +----------------+
        ^                            |                               ^
        |                            |                               |
+----------------+                   |                        +----------------+
| RIA Web        | ------------------)----------------------> | Web Browser    |
| (Web Server)   |  alerts & data    |                        | (Client)       |
+----------------+                   |                        +----------------+
```

## License

This project is licensed under the terms of the MIT license.