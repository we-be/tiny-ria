# Quotron Agent

The Quotron Agent is an autonomous financial data monitoring and analysis tool that interacts with the Quotron financial data system. It provides real-time monitoring, portfolio analysis, and data retrieval capabilities.

## Features

- **Real-time Price Monitoring**: Monitor stock prices with customizable thresholds and get alerts on significant price movements
- **Portfolio Summary**: Generate comprehensive summaries of stock and cryptocurrency portfolios
- **Data Retrieval**: Fetch current data for stocks, cryptocurrencies, and market indices
- **AI Assistant**: Interactive chat-based AI assistant that can answer financial questions and retrieve market data
- **Message Queue Integration**: Publish price alerts to Redis streams for scalable processing
- **AI Price Analysis**: Automatically analyze price movements with AI to provide insights and context
- **Extensible Design**: Easily add new capabilities through the agent's modular architecture

## Installation

```bash
# Clone the repository if you haven't already
git clone https://github.com/we-be/tiny-ria.git
cd tiny-ria/agent

# Build the agent
./build.sh
```

## Usage

### Agent CLI

The agent provides a command-line interface with several commands:

```
Quotron Agent - Autonomous financial data monitoring and analysis

Usage:
  quotron-agent [OPTIONS] --command=COMMAND

Commands:
  help        Display this help message
  monitor     Monitor stock prices and alert on significant movements
  portfolio   Generate a portfolio summary
  fetch       Fetch data for specified symbols

Options:
  -api-host string
        Host of the Quotron API service (default "localhost")
  -api-port int
        Port of the Quotron API service (default 8080)
  -command string
        Command to execute (help, monitor, portfolio) (default "help")
  -cryptos string
        Comma-separated list of crypto symbols (default "BTC-USD,ETH-USD")
  -indices string
        Comma-separated list of market indices (default "SPY,QQQ,DIA")
  -interval duration
        Monitoring interval duration (default 1m0s)
  -name string
        Name of the agent (default "FinanceWatcher")
  -symbols string
        Comma-separated list of stock symbols (default "AAPL,MSFT,GOOG")
  -threshold float
        Alert threshold percentage for price movements (default 2)
  -enable-queue bool
        Enable publishing alerts to message queue (default false)
  -redis-addr string
        Redis server address for queue (default "localhost:6379")
```

### AI Assistant

The AI assistant provides an interactive chat interface for financial data:

```
Usage:
  quotron-assistant [OPTIONS]

Options:
  -api-key string
        OpenAI API key (if empty, OPENAI_API_KEY env var is used)
  -api-host string
        Host of the Quotron API service (default "localhost")
  -api-port int
        Port of the Quotron API service (default 8080)
  -interactive
        Run in interactive mode (default true)
  -model string
        LLM model to use (default "gpt-3.5-turbo")
  -query string
        Single query to run (non-interactive mode)
  -temperature float
        LLM temperature (higher = more creative) (default 0.7)
```

## Examples

### Monitor Stock Prices

```bash
# Monitor AAPL, MSFT, and GOOG with 1.5% alert threshold
./bin/quotron-agent --command=monitor --symbols=AAPL,MSFT,GOOG --threshold=1.5
```

### Generate Portfolio Summary

```bash
# Generate a summary for a mixed portfolio of stocks and cryptocurrencies
./bin/quotron-agent --command=portfolio --symbols=AAPL,MSFT,GOOG --cryptos=BTC-USD,ETH-USD
```

### Fetch Current Data

```bash
# Fetch current data for specific symbols
./bin/quotron-agent --command=fetch --symbols=AAPL,MSFT --cryptos=BTC-USD --indices=SPY
```

### Interactive AI Assistant

```bash
# Start the interactive assistant
export OPENAI_API_KEY=your_openai_api_key
./bin/quotron-assistant

# Or provide the API key directly
./bin/quotron-assistant --api-key=your_openai_api_key
```

### Non-Interactive AI Query

```bash
# Run a single query in non-interactive mode
./bin/quotron-assistant --interactive=false --query="What's the current price of AAPL and MSFT?"
```

### AI Price Alerter

```bash
# Start the AI alerter to analyze price movements
export OPENAI_API_KEY=your_openai_api_key
./bin/quotron-ai-alerter

# Or provide the API key directly
./bin/quotron-ai-alerter --api-key=your_openai_api_key --redis=localhost:6379
```

### Enable Queue Publishing in Monitor

```bash
# Monitor with queue integration enabled
./bin/quotron-agent --command=monitor --symbols=AAPL,MSFT,GOOG,BTC-USD,ETH-USD --threshold=1.0 --enable-queue=true
```

## Prerequisites

- The Quotron API service must be running
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

## Development

The agent is built with a modular design that makes it easy to extend:

- `pkg/agent.go`: Core agent implementation with monitoring and data retrieval capabilities
- `pkg/assistant.go`: AI assistant implementation for natural language interaction
- `pkg/llm.go`: LLM client for interacting with OpenAI's API
- `pkg/queue.go`: Message queue integration with Redis streams
- `cmd/main.go`: Command-line interface and command handlers for the agent
- `cmd/assistant/main.go`: Command-line interface for the AI assistant
- `cmd/ai-alerter/main.go`: AI-powered price movement analyzer

To add new capabilities, extend the `Agent` struct in `pkg/agent.go` with additional methods.

### Message Queue Architecture

The agent uses Redis streams for message queuing to enable a scalable, decoupled architecture:

1. The agent publishes price alerts to the `quotron:alerts:stream` stream when it detects price movements above the threshold
2. Consumer groups can be created to process these alerts, allowing multiple consumers to work on different subsets of alerts
3. The AI alerter consumes alerts from the stream and uses the AI assistant to analyze price movements
4. This architecture allows for adding more consumers without modifying the monitoring code

```
+----------------+           +-------------------+           +----------------+
| Agent Monitor  |  -------> | Redis Stream     |  -------> | AI Alerter     |
| (Publisher)    |  alerts   | (Message Queue)  |  consume  | (Consumer)     |
+----------------+           +-------------------+           +----------------+
                                                                     |
                                                                     v
                                                             +----------------+
                                                             | OpenAI API     |
                                                             | (Analysis)     |
                                                             +----------------+
```

## License

This project is licensed under the terms of the MIT license.