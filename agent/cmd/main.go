package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/we-be/tiny-ria/agent/pkg"
	"github.com/we-be/tiny-ria/quotron/scheduler/pkg/client"
)

var (
	// Command-line flags
	agentName   = flag.String("name", "FinanceWatcher", "Name of the agent")
	apiHost     = flag.String("api-host", "localhost", "Host of the Quotron API service")
	apiPort     = flag.Int("api-port", 8080, "Port of the Quotron API service")
	command     = flag.String("command", "help", "Command to execute (help, monitor, portfolio)")
	symbols     = flag.String("symbols", "AAPL,MSFT,GOOG", "Comma-separated list of stock symbols")
	cryptos     = flag.String("cryptos", "BTC-USD,ETH-USD", "Comma-separated list of crypto symbols")
	indices     = flag.String("indices", "SPY,QQQ,DIA", "Comma-separated list of market indices")
	threshold   = flag.Float64("threshold", 2.0, "Alert threshold percentage for price movements")
	interval    = flag.Duration("interval", 1*time.Minute, "Monitoring interval duration")
	enableQueue = flag.Bool("enable-queue", false, "Enable publishing alerts to message queue")
	redisAddr   = flag.String("redis-addr", "localhost:6379", "Redis server address for queue")
)

func main() {
	// Parse command-line flags
	flag.Parse()

	// Create the agent
	agent := pkg.NewAgent(pkg.AgentConfig{
		Name:        *agentName,
		APIHost:     *apiHost,
		APIPort:     *apiPort,
		EnableQueue: *enableQueue,
		RedisAddr:   *redisAddr,
	})

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	setupSignalHandling(cancel)

	// Execute the requested command
	switch *command {
	case "help", "":
		printHelp()
	case "monitor":
		runMonitor(ctx, agent)
	case "portfolio":
		runPortfolio(ctx, agent)
	case "fetch":
		runFetch(ctx, agent)
	default:
		log.Fatalf("Unknown command: %s", *command)
	}
}

// setupSignalHandling sets up handling for OS signals
func setupSignalHandling(cancel context.CancelFunc) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-signalChan
		fmt.Println("\nReceived interrupt signal, shutting down...")
		cancel()
		// Give some time for goroutines to clean up
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
}

// printHelp displays usage information
func printHelp() {
	fmt.Println("Quotron Agent - Autonomous financial data monitoring and analysis")
	fmt.Println("\nUsage:")
	fmt.Println("  quotron-agent [OPTIONS] --command=COMMAND")
	fmt.Println("\nCommands:")
	fmt.Println("  help        Display this help message")
	fmt.Println("  monitor     Monitor stock prices and alert on significant movements")
	fmt.Println("  portfolio   Generate a portfolio summary")
	fmt.Println("  fetch       Fetch data for specified symbols")
	fmt.Println("\nOptions:")
	flag.PrintDefaults()
	fmt.Println("\nExamples:")
	fmt.Println("  quotron-agent --command=monitor --symbols=AAPL,MSFT,GOOG --threshold=1.5")
	fmt.Println("  quotron-agent --command=portfolio --symbols=AAPL,MSFT,GOOG --cryptos=BTC-USD,ETH-USD")
	fmt.Println("  quotron-agent --command=fetch --symbols=AAPL,MSFT --cryptos=BTC-USD --indices=SPY")
}

// runMonitor executes the stock monitoring command
func runMonitor(ctx context.Context, agent *pkg.Agent) {
	symbolList := strings.Split(*symbols, ",")
	if len(symbolList) == 0 || (len(symbolList) == 1 && symbolList[0] == "") {
		log.Fatal("No stock symbols specified for monitoring")
	}

	fmt.Printf("Starting price monitoring for %d symbols with %.2f%% threshold...\n", 
		len(symbolList), *threshold)
	fmt.Printf("Press Ctrl+C to stop monitoring\n\n")

	// Define alert callback function
	alertCallback := func(symbol string, quote interface{}, percentChange float64) {
		direction := "increased"
		if percentChange < 0 {
			direction = "decreased"
		}
		
		// We know this is a *client.StockQuote, but we're accepting interface{} to match the callback signature
		stockQuote := quote.(*client.StockQuote)
		
		fmt.Printf("🚨 ALERT: %s has %s by %.2f%%\n", symbol, direction, percentChange)
		fmt.Printf("   Current price: $%.2f, Volume: %d\n", stockQuote.Price, stockQuote.Volume)
	}

	// Start monitoring
	agent.MonitorStocks(ctx, symbolList, *interval, *threshold, alertCallback)
}

// runPortfolio executes the portfolio summary command
func runPortfolio(ctx context.Context, agent *pkg.Agent) {
	stockList := strings.Split(*symbols, ",")
	if len(stockList) == 1 && stockList[0] == "" {
		stockList = []string{}
	}

	cryptoList := strings.Split(*cryptos, ",")
	if len(cryptoList) == 1 && cryptoList[0] == "" {
		cryptoList = []string{}
	}

	if len(stockList) == 0 && len(cryptoList) == 0 {
		log.Fatal("No stocks or cryptos specified for portfolio")
	}

	fmt.Printf("Generating portfolio summary for %d stocks and %d cryptos...\n", 
		len(stockList), len(cryptoList))

	summary, err := agent.GetPortfolioSummary(ctx, stockList, cryptoList)
	if err != nil {
		log.Printf("Warning: Error generating portfolio summary: %v", err)
	}

	fmt.Println(summary)
}

// runFetch executes the data fetching command
func runFetch(ctx context.Context, agent *pkg.Agent) {
	stockList := strings.Split(*symbols, ",")
	if len(stockList) == 1 && stockList[0] == "" {
		stockList = []string{}
	}

	cryptoList := strings.Split(*cryptos, ",")
	if len(cryptoList) == 1 && cryptoList[0] == "" {
		cryptoList = []string{}
	}

	indexList := strings.Split(*indices, ",")
	if len(indexList) == 1 && indexList[0] == "" {
		indexList = []string{}
	}

	if len(stockList) == 0 && len(cryptoList) == 0 && len(indexList) == 0 {
		log.Fatal("No symbols specified for fetching")
	}

	// Fetch stocks
	if len(stockList) > 0 {
		fmt.Printf("Fetching data for %d stocks...\n", len(stockList))
		stockQuotes, err := agent.FetchStockData(ctx, stockList)
		if err != nil {
			log.Printf("Warning: %v", err)
		}

		fmt.Println("\n=== Stock Quotes ===")
		for symbol, quote := range stockQuotes {
			fmt.Printf("%s: $%.2f (%.2f%%) - Volume: %s\n", 
				symbol, quote.Price, quote.ChangePercent, formatNumber(quote.Volume))
		}
		fmt.Println()
	}

	// Fetch cryptos
	if len(cryptoList) > 0 {
		fmt.Printf("Fetching data for %d cryptocurrencies...\n", len(cryptoList))
		cryptoQuotes, err := agent.FetchCryptoData(ctx, cryptoList)
		if err != nil {
			log.Printf("Warning: %v", err)
		}

		fmt.Println("\n=== Cryptocurrency Quotes ===")
		for symbol, quote := range cryptoQuotes {
			fmt.Printf("%s: $%.2f (%.2f%%) - Volume: %s\n", 
				symbol, quote.Price, quote.ChangePercent, formatNumber(quote.Volume))
		}
		fmt.Println()
	}

	// Fetch indices
	if len(indexList) > 0 {
		fmt.Printf("Fetching data for %d market indices...\n", len(indexList))
		marketData, err := agent.FetchMarketData(ctx, indexList)
		if err != nil {
			log.Printf("Warning: %v", err)
		}

		fmt.Println("\n=== Market Indices ===")
		for index, data := range marketData {
			fmt.Printf("%s: %.2f (%.2f%%)\n", 
				index, data.Value, data.ChangePercent)
		}
		fmt.Println()
	}
}

// Helper function to format large numbers
func formatNumber(n int64) string {
	if n >= 1_000_000_000 {
		return fmt.Sprintf("%.1fB", float64(n)/1_000_000_000)
	} else if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	} else if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}