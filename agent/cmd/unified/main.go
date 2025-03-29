package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/we-be/tiny-ria/agent/pkg"
	"github.com/we-be/tiny-ria/quotron/scheduler/pkg/client"
)

//go:embed static templates
var content embed.FS

// Command is a command that can be executed by the CLI
type Command struct {
	Name        string
	Description string
	Execute     func([]string)
}

// commonFlags holds flags that are shared across multiple commands
type commonFlags struct {
	apiHost       string
	apiPort       int
	redisAddr     string
	apiKey        string
	debug         bool
}

var (
	// Global variables
	logger       *log.Logger
	commands     map[string]*Command
	commonArgs   commonFlags
	agentService *pkg.Agent
	llmConfig    pkg.LLMConfig
	binPath      string
)

func init() {
	// Set up logging to both stdout and a file
	logFile, err := os.OpenFile("/tmp/ria_logs/agent.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		// Fall back to stdout only
		logger = log.New(os.Stdout, "[Quotron] ", log.LstdFlags)
	} else {
		// Use MultiWriter to log to both stdout and the file
		multiWriter := io.MultiWriter(os.Stdout, logFile)
		logger = log.New(multiWriter, "[Quotron] ", log.LstdFlags)
	}

	// Get the binary path
	binPath, _ = os.Executable()

	// Initialize common flags
	commonArgs = commonFlags{
		apiHost:   "localhost",
		apiPort:   8080,
		redisAddr: "localhost:6379",
		apiKey:    "",
		debug:     false,
	}

	// Setup commands
	commands = map[string]*Command{
		"help": {
			Name:        "help",
			Description: "Show help information",
			Execute:     cmdHelp,
		},
		"version": {
			Name:        "version",
			Description: "Show version information",
			Execute:     cmdVersion,
		},
		"monitor": {
			Name:        "monitor",
			Description: "Monitor price movements for stocks and cryptocurrencies",
			Execute:     cmdMonitor,
		},
		"fetch": {
			Name:        "fetch",
			Description: "Fetch current data for financial instruments",
			Execute:     cmdFetch,
		},
		"portfolio": {
			Name:        "portfolio",
			Description: "Generate a portfolio summary",
			Execute:     cmdPortfolio,
		},
		"chat": {
			Name:        "chat",
			Description: "Start the interactive AI assistant in the terminal",
			Execute:     cmdChat,
		},
		"web": {
			Name:        "web",
			Description: "Start the web-based chat interface",
			Execute:     cmdWeb,
		},
		"ai-alerter": {
			Name:        "ai-alerter",
			Description: "Start the AI alerter service",
			Execute:     cmdAIAlerter,
		},
	}
}

func main() {
	// Set up flags
	flag.StringVar(&commonArgs.apiHost, "api-host", "localhost", "Host of the Quotron API service")
	flag.IntVar(&commonArgs.apiPort, "api-port", 8080, "Port of the Quotron API service") 
	flag.StringVar(&commonArgs.redisAddr, "redis", "localhost:6379", "Redis server address")
	flag.StringVar(&commonArgs.apiKey, "api-key", "", "API key for OpenAI or Anthropic (if empty, OPENAI_API_KEY or ANTHROPIC_API_KEY env var is used)")
	flag.BoolVar(&commonArgs.debug, "debug", false, "Enable debug mode")
	
	// Remove these flags that were previously defined
	// No longer using these flags as we always use the Quotron API
	_ = flag.Bool("use-real-api", false, "Deprecated: Always using Quotron API now")
	_ = flag.String("finance-api-key", "", "Deprecated: No longer needed")

	// Custom usage
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "RIA - Responsive Investment Assistant for financial data monitoring and AI interaction\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n  ria [OPTIONS] COMMAND [ARGS...]\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		
		// Sort and print commands
		cmdNames := make([]string, 0, len(commands))
		for name := range commands {
			cmdNames = append(cmdNames, name)
		}
		
		for _, name := range cmdNames {
			cmd := commands[name]
			fmt.Fprintf(os.Stderr, "  %-12s %s\n", cmd.Name, cmd.Description)
		}
		
		fmt.Fprintf(os.Stderr, "\nGlobal Options:\n")
		flag.PrintDefaults()
		
		fmt.Fprintf(os.Stderr, "\nRun 'ria help COMMAND' for more information on a command\n")
	}

	// Parse flags
	flag.Parse()

	// Get the API key from flag or environment variable
	if commonArgs.apiKey == "" {
		commonArgs.apiKey = os.Getenv("OPENAI_API_KEY")
		if commonArgs.apiKey == "" {
			commonArgs.apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
	}

	// Determine the command
	args := flag.Args()
	if len(args) < 1 {
		cmdHelp(args)
		os.Exit(1)
	}

	// Get the command
	cmdName := args[0]
	cmd, exists := commands[cmdName]
	if !exists {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmdName)
		fmt.Fprintf(os.Stderr, "Run 'ria help' for usage\n")
		os.Exit(1)
	}

	// Execute the command
	cmd.Execute(args[1:])
}

// cmdHelp handles the help command
func cmdHelp(args []string) {
	if len(args) == 0 {
		flag.Usage()
		return
	}

	// Show help for a specific command
	cmdName := args[0]
	cmd, exists := commands[cmdName]
	if !exists {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmdName)
		fmt.Fprintf(os.Stderr, "Run 'ria help' for usage\n")
		os.Exit(1)
	}

	fmt.Printf("Help for command '%s':\n", cmdName)
	fmt.Printf("  %s\n\n", cmd.Description)

	// Command-specific help
	switch cmdName {
	case "monitor":
		fmt.Println("Usage: ria monitor [OPTIONS]")
		fmt.Println("Options:")
		fmt.Println("  --symbols string      Comma-separated list of stock symbols (default \"AAPL,MSFT,GOOG\")")
		fmt.Println("  --cryptos string      Comma-separated list of crypto symbols (default \"BTC-USD,ETH-USD\")")
		fmt.Println("  --threshold float     Alert threshold percentage for price movements (default 2.0)")
		fmt.Println("  --interval duration   Monitoring interval duration (default 1m)")
		fmt.Println("  --enable-queue        Enable publishing alerts to message queue (default false)")
		fmt.Println("  --redis string        Redis server address for queue (default \"localhost:6379\")")
		fmt.Println("\nExample:")
		fmt.Println("  ria monitor --symbols=AAPL,MSFT,GOOG --threshold=1.5 --enable-queue")
	
	case "fetch":
		fmt.Println("Usage: ria fetch [OPTIONS]")
		fmt.Println("Options:")
		fmt.Println("  --symbols string      Comma-separated list of stock symbols (default \"AAPL,MSFT,GOOG\")")
		fmt.Println("  --cryptos string      Comma-separated list of crypto symbols (default \"BTC-USD,ETH-USD\")")
		fmt.Println("  --indices string      Comma-separated list of market indices (default \"SPY,QQQ,DIA\")")
		fmt.Println("\nExample:")
		fmt.Println("  ria fetch --symbols=AAPL,MSFT --cryptos=BTC-USD --indices=SPY")
	
	case "portfolio":
		fmt.Println("Usage: ria portfolio [OPTIONS]")
		fmt.Println("Options:")
		fmt.Println("  --symbols string      Comma-separated list of stock symbols (default \"AAPL,MSFT,GOOG\")")
		fmt.Println("  --cryptos string      Comma-separated list of crypto symbols (default \"BTC-USD,ETH-USD\")")
		fmt.Println("\nExample:")
		fmt.Println("  ria portfolio --symbols=AAPL,MSFT,GOOG --cryptos=BTC-USD,ETH-USD")
	
	case "chat":
		fmt.Println("Usage: ria chat [OPTIONS]")
		fmt.Println("Options:")
		fmt.Println("  --api-key string      OpenAI or Anthropic API key (if empty, OPENAI_API_KEY or ANTHROPIC_API_KEY env var is used)")
		fmt.Println("  --interactive         Run in interactive mode (default true)")
		fmt.Println("  --query string        Single query to run (non-interactive mode)")
		fmt.Println("  --model string        LLM model to use (default \"gpt-3.5-turbo\")")
		fmt.Println("  --temperature float   LLM temperature (higher = more creative) (default 0.7)")
		fmt.Println("\nExample:")
		fmt.Println("  ria chat --api-key=YOUR_API_KEY")
		fmt.Println("  ria chat --query=\"What's the current price of AAPL and MSFT?\" --interactive=false")
	
	case "web":
		fmt.Println("Usage: ria web [OPTIONS]")
		fmt.Println("Options:")
		fmt.Println("  --api-key string       OpenAI or Anthropic API key (if empty, OPENAI_API_KEY or ANTHROPIC_API_KEY env var is used)")
		fmt.Println("  --port int             Web server port (default 8090)")
		fmt.Println("  --consumer-id string   Consumer ID for Redis consumer group (default \"chat-ui\")")
		fmt.Println("  --use-real-api         Use real financial API instead of local API service (default false)")
		fmt.Println("  --finance-api-key      API key for real financial data service (if needed)")
		fmt.Println("\nExample:")
		fmt.Println("  ria web --api-key=YOUR_API_KEY --port=8090")
		fmt.Println("  ria web --api-key=YOUR_API_KEY --use-real-api --finance-api-key=YOUR_FINANCE_API_KEY")
	
	case "ai-alerter":
		fmt.Println("Usage: ria ai-alerter [OPTIONS]")
		fmt.Println("Options:")
		fmt.Println("  --api-key string      OpenAI or Anthropic API key (if empty, OPENAI_API_KEY or ANTHROPIC_API_KEY env var is used)")
		fmt.Println("  --consumer-id string  Consumer ID for Redis consumer group (default \"ai-alerter\")")
		fmt.Println("  --model string        LLM model to use (default \"gpt-3.5-turbo\")")
		fmt.Println("\nExample:")
		fmt.Println("  ria ai-alerter --api-key=YOUR_API_KEY")
	
	default:
		fmt.Println("No detailed help available for this command.")
	}
}

// cmdVersion handles the version command
func cmdVersion(args []string) {
	fmt.Println("RIA - Responsive Investment Assistant v1.0.0")
	fmt.Println("© 2025 TinyRIA. All rights reserved.")
}

// cmdMonitor handles the monitor command
func cmdMonitor(args []string) {
	// Set up flags
	monitorCmd := flag.NewFlagSet("monitor", flag.ExitOnError)
	symbols := monitorCmd.String("symbols", "AAPL,MSFT,GOOG", "Comma-separated list of stock symbols")
	cryptos := monitorCmd.String("cryptos", "BTC-USD,ETH-USD", "Comma-separated list of crypto symbols")
	threshold := monitorCmd.Float64("threshold", 2.0, "Alert threshold percentage for price movements")
	interval := monitorCmd.Duration("interval", 1*time.Minute, "Monitoring interval duration")
	enableQueue := monitorCmd.Bool("enable-queue", false, "Enable publishing alerts to message queue")

	// Parse flags
	if err := monitorCmd.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing monitor flags: %v\n", err)
		os.Exit(1)
	}

	// Split symbols and cryptos
	stockSymbols := strings.Split(*symbols, ",")
	cryptoSymbols := strings.Split(*cryptos, ",")

	// Create agent with queue if enabled
	agent := pkg.NewAgent(pkg.AgentConfig{
		Name:        "Monitor",
		APIHost:     commonArgs.apiHost,
		APIPort:     commonArgs.apiPort,
		EnableQueue: *enableQueue,
		RedisAddr:   commonArgs.redisAddr,
	})

	// Set up context with cancellation for cleanup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChan
		fmt.Println("\nReceived interrupt signal, shutting down...")
		cancel()
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()

	// Start monitoring
	fmt.Printf("Starting to monitor %d stocks and %d cryptos with %.1f%% threshold...\n", 
		len(stockSymbols), len(cryptoSymbols), *threshold)
	fmt.Printf("Checking every %s\n", interval.String())
	fmt.Printf("Press Ctrl+C to stop\n\n")

	// Callback function for price movements
	alertCallback := func(symbol string, quote interface{}, percentChange float64) {
		fmt.Printf("ALERT: %s has moved %.2f%%\n", symbol, percentChange)
	}

	// Monitor stocks with callback
	agent.MonitorStocks(ctx, stockSymbols, *interval, *threshold, alertCallback)
}

// cmdFetch handles the fetch command
func cmdFetch(args []string) {
	// Set up flags
	fetchCmd := flag.NewFlagSet("fetch", flag.ExitOnError)
	symbols := fetchCmd.String("symbols", "AAPL,MSFT,GOOG", "Comma-separated list of stock symbols")
	cryptos := fetchCmd.String("cryptos", "BTC-USD,ETH-USD", "Comma-separated list of crypto symbols")
	indices := fetchCmd.String("indices", "SPY,QQQ,DIA", "Comma-separated list of market indices")

	// Parse flags
	if err := fetchCmd.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing fetch flags: %v\n", err)
		os.Exit(1)
	}

	// Split symbols, cryptos, and indices
	stockSymbols := strings.Split(*symbols, ",")
	cryptoSymbols := strings.Split(*cryptos, ",")
	indexSymbols := strings.Split(*indices, ",")

	// Create agent
	agent := pkg.NewAgent(pkg.AgentConfig{
		Name:    "Fetch",
		APIHost: commonArgs.apiHost,
		APIPort: commonArgs.apiPort,
	})

	// Create context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Fetch stock data
	if len(stockSymbols) > 0 && stockSymbols[0] != "" {
		fmt.Printf("Fetching data for %d stocks...\n", len(stockSymbols))
		stocks, err := agent.FetchStockData(ctx, stockSymbols)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching stock data: %v\n", err)
		} else {
			fmt.Println("\nStock Data:")
			fmt.Println("----------------------------------------------------------")
			fmt.Printf("%-8s %-10s %-10s %-10s %-15s\n", "Symbol", "Price", "Change", "Change %", "Volume")
			fmt.Println("----------------------------------------------------------")
			for _, symbol := range stockSymbols {
				if quote, ok := stocks[symbol]; ok {
					fmt.Printf("%-8s $%-9.2f %-+10.2f %-+10.2f %-15d\n",
						symbol, quote.Price, quote.Change, quote.ChangePercent, quote.Volume)
				} else {
					fmt.Printf("%-8s %-10s %-10s %-10s %-15s\n", symbol, "N/A", "N/A", "N/A", "N/A")
				}
			}
			fmt.Println()
		}
	}

	// Fetch crypto data
	if len(cryptoSymbols) > 0 && cryptoSymbols[0] != "" {
		fmt.Printf("Fetching data for %d cryptocurrencies...\n", len(cryptoSymbols))
		cryptos, err := agent.FetchCryptoData(ctx, cryptoSymbols)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching crypto data: %v\n", err)
		} else {
			fmt.Println("\nCryptocurrency Data:")
			fmt.Println("----------------------------------------------------------")
			fmt.Printf("%-10s %-10s %-10s %-10s %-15s\n", "Symbol", "Price", "Change", "Change %", "Volume")
			fmt.Println("----------------------------------------------------------")
			for _, symbol := range cryptoSymbols {
				if quote, ok := cryptos[symbol]; ok {
					fmt.Printf("%-10s $%-9.2f %-+10.2f %-+10.2f %-15d\n",
						symbol, quote.Price, quote.Change, quote.ChangePercent, quote.Volume)
				} else {
					fmt.Printf("%-10s %-10s %-10s %-10s %-15s\n", symbol, "N/A", "N/A", "N/A", "N/A")
				}
			}
			fmt.Println()
		}
	}

	// Fetch market index data
	if len(indexSymbols) > 0 && indexSymbols[0] != "" {
		fmt.Printf("Fetching data for %d market indices...\n", len(indexSymbols))
		indices, err := agent.FetchMarketData(ctx, indexSymbols)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching market data: %v\n", err)
		} else {
			fmt.Println("\nMarket Index Data:")
			fmt.Println("------------------------------------------")
			fmt.Printf("%-10s %-10s %-10s %-10s\n", "Index", "Value", "Change", "Change %")
			fmt.Println("------------------------------------------")
			for _, symbol := range indexSymbols {
				if data, ok := indices[symbol]; ok {
					fmt.Printf("%-10s %-10.2f %-+10.2f %-+10.2f\n",
						symbol, data.Value, data.Change, data.ChangePercent)
				} else {
					fmt.Printf("%-10s %-10s %-10s %-10s\n", symbol, "N/A", "N/A", "N/A")
				}
			}
			fmt.Println()
		}
	}
}

// cmdPortfolio handles the portfolio command
func cmdPortfolio(args []string) {
	// Set up flags
	portfolioCmd := flag.NewFlagSet("portfolio", flag.ExitOnError)
	symbols := portfolioCmd.String("symbols", "AAPL,MSFT,GOOG", "Comma-separated list of stock symbols")
	cryptos := portfolioCmd.String("cryptos", "BTC-USD,ETH-USD", "Comma-separated list of crypto symbols")

	// Parse flags
	if err := portfolioCmd.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing portfolio flags: %v\n", err)
		os.Exit(1)
	}

	// Split symbols and cryptos
	stockSymbols := strings.Split(*symbols, ",")
	cryptoSymbols := strings.Split(*cryptos, ",")

	// Create agent
	agent := pkg.NewAgent(pkg.AgentConfig{
		Name:    "Portfolio",
		APIHost: commonArgs.apiHost,
		APIPort: commonArgs.apiPort,
	})

	// Create context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Generate portfolio summary
	fmt.Printf("Generating portfolio summary for %d stocks and %d cryptos...\n", 
		len(stockSymbols), len(cryptoSymbols))
	
	summary, err := agent.GetPortfolioSummary(ctx, stockSymbols, cryptoSymbols)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating portfolio summary: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nPortfolio Summary:")
	fmt.Println("==================================================")
	fmt.Println(summary)
}

// cmdChat handles the chat command
func cmdChat(args []string) {
	// Set up flags
	chatCmd := flag.NewFlagSet("chat", flag.ExitOnError)
	interactive := chatCmd.Bool("interactive", true, "Run in interactive mode")
	query := chatCmd.String("query", "", "Single query to run (non-interactive mode)")
	model := chatCmd.String("model", "", "LLM model to use")
	temperature := chatCmd.Float64("temperature", 0.7, "LLM temperature (higher = more creative)")

	// Parse flags
	if err := chatCmd.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing chat flags: %v\n", err)
		os.Exit(1)
	}

	// Check for API key
	if commonArgs.apiKey == "" {
		fmt.Println("Error: API key is required. Set with --api-key flag or OPENAI_API_KEY/ANTHROPIC_API_KEY environment variable.")
		os.Exit(1)
	}

	// Create agent
	agent := pkg.NewAgent(pkg.AgentConfig{
		Name:    "ChatAssistant",
		APIHost: commonArgs.apiHost,
		APIPort: commonArgs.apiPort,
	})

	// Default to OpenAI model if not specified
	if *model == "" {
		*model = "gpt-3.5-turbo"
	}

	// Create LLM config based on defaults
	llmConfig := pkg.DefaultLLMConfig()
	llmConfig.APIKey = commonArgs.apiKey
	llmConfig.Model = *model
	llmConfig.Temperature = *temperature

	// Create assistant
	assistant := pkg.NewAgentAssistant(agent, llmConfig)

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChan
		fmt.Println("\nReceived interrupt signal, shutting down...")
		cancel()
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()

	// Run in appropriate mode
	if !*interactive {
		// Non-interactive mode
		if *query == "" {
			fmt.Println("Error: query parameter is required in non-interactive mode")
			os.Exit(1)
		}

		response, err := assistant.Chat(ctx, *query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(response)
	} else {
		// Interactive mode
		fmt.Println("Quotron Financial Assistant")
		fmt.Println("Type 'exit' or 'quit' to exit, or press Ctrl+C")
		fmt.Println("--------------------------------------------")

		for {
			fmt.Print("\nYou: ")
			var input string
			if _, err := fmt.Scanln(&input); err != nil {
				fmt.Println()
				continue
			}

			// Check for exit commands
			input = strings.TrimSpace(input)
			if input == "exit" || input == "quit" {
				fmt.Println("Goodbye!")
				break
			}

			// Handle empty input
			if input == "" {
				continue
			}

			// Send to assistant
			response, err := assistant.Chat(ctx, input)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				continue
			}

			fmt.Printf("\nAssistant: %s\n", response)
		}
	}
}

// cmdWeb handles the web command
func cmdWeb(args []string) {
	// Set up flags
	webCmd := flag.NewFlagSet("web", flag.ExitOnError)
	port := webCmd.Int("port", 8090, "Web server port")
	consumerID := webCmd.String("consumer-id", "chat-ui", "Consumer ID for Redis consumer group")

	// Parse flags
	if err := webCmd.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing web flags: %v\n", err)
		os.Exit(1)
	}

	// Check for API key
	if commonArgs.apiKey == "" {
		fmt.Println("Error: API key is required. Set with --api-key flag or OPENAI_API_KEY/ANTHROPIC_API_KEY environment variable.")
		os.Exit(1)
	}

	// Create agent
	agent := pkg.NewAgent(pkg.AgentConfig{
		Name:        "ChatUI",
		APIHost:     commonArgs.apiHost,
		APIPort:     commonArgs.apiPort,
		EnableQueue: true,
		RedisAddr:   commonArgs.redisAddr,
	})

	// Default to OpenAI model
	model := "gpt-3.5-turbo"

	// Create LLM config based on defaults
	llmConfig := pkg.DefaultLLMConfig()
	llmConfig.APIKey = commonArgs.apiKey
	llmConfig.Model = model
	llmConfig.SystemPrompt = getChatSystemPrompt()

	assistant := pkg.NewAgentAssistant(agent, llmConfig)

	// Create web server
	server := &WebServer{
		assistant: assistant,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow connections from any origin
			},
		},
		activeClients: make(map[*ThreadSafeConn]bool),
		logger:        logger,
	}

	// Set up context with cancellation for cleanup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChan
		fmt.Println("\nReceived interrupt signal, shutting down...")
		cancel()
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()

	// Start alert consumer if Redis is configured
	if commonArgs.redisAddr != "" {
		go server.startAlertConsumer(ctx, *consumerID)
	}

	// Set up routes
	http.HandleFunc("/ws", server.handleWebSocket)
	http.HandleFunc("/", server.handleIndex)

	// Serve static files
	staticFS, err := fs.Sub(content, "static")
	if err != nil {
		logger.Fatalf("Failed to setup static file server: %v", err)
	}
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Start server
	addr := fmt.Sprintf(":%d", *port)
	logger.Printf("Starting web server on %s", addr)
	logger.Printf("Open your browser to http://localhost:%d", *port)
	err = http.ListenAndServe(addr, nil)
	if err != nil {
		logger.Fatalf("Error starting server: %v", err)
	}
}

// cmdAIAlerter handles the ai-alerter command
func cmdAIAlerter(args []string) {
	// Set up flags
	alerterCmd := flag.NewFlagSet("ai-alerter", flag.ExitOnError)
	consumerID := alerterCmd.String("consumer-id", "ai-alerter", "Consumer ID for Redis consumer group")
	model := alerterCmd.String("model", "", "LLM model to use")

	// Parse flags
	if err := alerterCmd.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing ai-alerter flags: %v\n", err)
		os.Exit(1)
	}

	// Check for API key
	if commonArgs.apiKey == "" {
		fmt.Println("Error: API key is required. Set with --api-key flag or OPENAI_API_KEY/ANTHROPIC_API_KEY environment variable.")
		os.Exit(1)
	}

	// Set up logging
	logger := log.New(os.Stdout, "[AI-Alerter] ", log.LstdFlags)

	// Create agent
	agent := pkg.NewAgent(pkg.AgentConfig{
		Name:        "AIAlerter",
		APIHost:     commonArgs.apiHost,
		APIPort:     commonArgs.apiPort,
		EnableQueue: false, // We don't need to publish, only consume
	})

	// Default to OpenAI model if not specified
	if *model == "" {
		*model = "gpt-3.5-turbo"
	}

	// Create LLM config
	llmConfig := pkg.DefaultLLMConfig()
	llmConfig.APIKey = commonArgs.apiKey
	llmConfig.Model = *model
	llmConfig.SystemPrompt = getAlertSystemPrompt()

	assistant := pkg.NewAgentAssistant(agent, llmConfig)

	// Set up context with cancellation for cleanup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChan
		fmt.Println("\nReceived interrupt signal, shutting down...")
		cancel()
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()

	// Create alert handler function
	alertHandler := func(alert pkg.AlertMessage) error {
		// Create a query based on the alert
		direction := "increased"
		if alert.PercentChange < 0 {
			direction = "decreased"
		}

		query := fmt.Sprintf(
			"Alert: %s has %s by %.2f%% from $%.2f to $%.2f. "+
				"Current price: $%.2f, Volume: %d. "+
				"Provide a brief analysis of this price movement.",
			alert.Symbol, direction, alert.PercentChange, 
			alert.PreviousPrice, alert.Price, alert.Price, alert.Volume,
		)

		// Log the alert and query
		logger.Printf("Processing alert for %s (%.2f%%)", alert.Symbol, alert.PercentChange)
		logger.Printf("Query: %s", query)

		// Send to AI assistant for analysis
		response, err := assistant.Chat(ctx, query)
		if err != nil {
			logger.Printf("Error getting AI analysis: %v", err)
			return err
		}

		// Print the analysis
		fmt.Printf("\n==== AI ANALYSIS: %s (%.2f%%) ====\n", alert.Symbol, alert.PercentChange)
		fmt.Println(response)
		fmt.Print("==================================\n")

		return nil
	}

	// Create and start the consumer
	logger.Printf("Starting alert consumer on Redis %s", commonArgs.redisAddr)
	consumer, err := pkg.NewQueueConsumer(commonArgs.redisAddr, logger, alertHandler)
	if err != nil {
		logger.Fatalf("Failed to create consumer: %v", err)
	}
	defer consumer.Close()

	logger.Printf("Waiting for price alerts...")
	logger.Printf("Press Ctrl+C to exit")

	// Start consuming alerts
	err = consumer.StartConsuming(ctx, *consumerID)
	if err != nil && err != context.Canceled {
		logger.Fatalf("Error consuming alerts: %v", err)
	}

	logger.Printf("Shutting down...")
}

//
// Web Server Implementation (copied from chat-ui)
//

// WebServer handles the chat web interface
type WebServer struct {
	assistant     *pkg.AgentAssistant
	upgrader      websocket.Upgrader
	activeClients map[*ThreadSafeConn]bool
	logger        *log.Logger
	clientsMutex  sync.Mutex // Mutex to protect activeClients map
}

// Message represents a chat message
type Message struct {
	Type    string                 `json:"type"`
	Content string                 `json:"content,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// handleIndex serves the main page
func (s *WebServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	// If path is not root, serve the specific file
	if r.URL.Path != "/" {
		http.Error(w, "Page not found", http.StatusNotFound)
		return
	}

	// Parse the template
	tmpl, err := template.ParseFS(content, "templates/index.html")
	if err != nil {
		s.logger.Printf("Error parsing template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Execute the template
	err = tmpl.Execute(w, nil)
	if err != nil {
		s.logger.Printf("Error executing template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleWebSocket handles WebSocket connections
func (s *WebServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Printf("Error upgrading to WebSocket: %v", err)
		return
	}
	
	// Create thread-safe connection wrapper
	safeConn := NewThreadSafeConn(conn)
	defer safeConn.Close()

	// Thread-safe client registration
	s.clientsMutex.Lock()
	s.activeClients[safeConn] = true
	s.clientsMutex.Unlock()
	
	defer func() {
		s.clientsMutex.Lock()
		delete(s.activeClients, safeConn)
		s.clientsMutex.Unlock()
	}()

	// Send welcome message
	welcomeMsg := Message{
		Type:    "system",
		Content: "Welcome to the Quotron Agent Chat Interface. How can I help you today?",
	}
	
	if err := safeConn.WriteJSON(welcomeMsg); err != nil {
		s.logger.Printf("Error sending welcome message: %v", err)
		return
	}

	// Listen for messages from the client
	for {
		var msg Message
		err := safeConn.ReadJSON(&msg)
		if err != nil {
			s.logger.Printf("Error reading message: %v", err)
			break
		}

		// Handle the message based on its type
		switch msg.Type {
		case "user":
			// Process user message
			go s.processUserMessage(safeConn, msg.Content)
		case "command":
			// Process command
			go s.processCommand(safeConn, msg.Content, msg.Data)
		default:
			s.logger.Printf("Unknown message type: %s", msg.Type)
		}
	}
}

// processUserMessage processes a user message and sends a response
func (s *WebServer) processUserMessage(conn *ThreadSafeConn, content string) {
	// Create context
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Check if this is a system message (internal tool use)
	isSystemMessage := strings.HasPrefix(content, "__SYSTEM__:")
	
	// Send status indicators to the client
	if isSystemMessage {
		// Inform user that the agent is working on fixing an error
		fixingMsg := Message{
			Type:    "system",
			Content: "I noticed an issue with the data. Let me fix that for you...",
		}
		err := conn.WriteJSON(fixingMsg)
		if err != nil {
			s.logger.Printf("Error sending fixing indicator: %v", err)
		}
	} else {
		// Regular typing indicator for user messages
		typingMsg := Message{
			Type: "typing",
			Data: map[string]interface{}{"status": "start"},
		}
		err := conn.WriteJSON(typingMsg)
		if err != nil {
			s.logger.Printf("Error sending typing indicator: %v", err)
		}
		defer func() {
			// Stop typing indicator
			typingMsg.Data["status"] = "stop"
			_ = conn.WriteJSON(typingMsg)
		}()
	}

	// Get response from assistant
	response, err := s.assistant.Chat(ctx, content)

	if err != nil {
		// Send error message
		errMsg := Message{
			Type:    "error",
			Content: fmt.Sprintf("Error: %v", err),
		}
		_ = conn.WriteJSON(errMsg)
		return
	}

	// For system messages, we don't show the response directly to the user
	if isSystemMessage {
		s.logger.Printf("System processing complete, result: %s", response)
		return
	}

	// Send response for regular user messages
	respMsg := Message{
		Type:    "assistant",
		Content: response,
	}
	err = conn.WriteJSON(respMsg)
	if err != nil {
		s.logger.Printf("Error sending response: %v", err)
	}
}

// processCommand handles special commands
func (s *WebServer) processCommand(conn *ThreadSafeConn, command string, data map[string]interface{}) {
	switch command {
	case "monitor":
		// Extract symbols from data
		symbols, ok := data["symbols"].([]interface{})
		if !ok || len(symbols) == 0 {
			s.sendError(conn, "No symbols provided for monitoring")
			return
		}

		// Convert interface slice to string slice
		symbolsStr := make([]string, len(symbols))
		for i, sym := range symbols {
			symbolsStr[i], _ = sym.(string)
		}

		// Extract threshold
		threshold, _ := data["threshold"].(float64)
		if threshold <= 0 {
			threshold = 1.0 // Default threshold
		}

		// Send acknowledgment
		ackMsg := Message{
			Type:    "system",
			Content: fmt.Sprintf("Setting up monitoring for %v with %.1f%% threshold", symbolsStr, threshold),
		}
		_ = conn.WriteJSON(ackMsg)
		
		// TODO: Implement actual monitoring setup
		
	case "fetch_price":
		// Extract symbol from data
		symbol, ok := data["symbol"].(string)
		if !ok || symbol == "" {
			s.sendError(conn, "No symbol provided for price fetch")
			return
		}
		
		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		// Determine if it's a crypto symbol (contains dash)
		var quote *client.StockQuote
		var err error
		
		if strings.Contains(symbol, "-") {
			// Fetch crypto quote
			cryptoResult, err := s.assistant.GetAgent().FetchCryptoData(ctx, []string{symbol})
			if err != nil {
				s.logger.Printf("Error fetching crypto data for %s: %v", symbol, err)
				s.sendError(conn, fmt.Sprintf("Error fetching data for %s: %v", symbol, err))
				return
			}
			
			if q, ok := cryptoResult[symbol]; ok {
				quote = q
			} else {
				s.sendError(conn, fmt.Sprintf("No data found for symbol: %s", symbol))
				return
			}
		} else {
			// Fetch stock quote
			stockResult, err := s.assistant.GetAgent().FetchStockData(ctx, []string{symbol})
			if err != nil {
				s.logger.Printf("Error fetching stock data for %s: %v", symbol, err)
				s.sendError(conn, fmt.Sprintf("Error fetching data for %s: %v", symbol, err))
				return
			}
			
			if q, ok := stockResult[symbol]; ok {
				quote = q
			} else {
				s.sendError(conn, fmt.Sprintf("No data found for symbol: %s", symbol))
				return
			}
		}
		
		// Send the price data to the client
		priceMsg := Message{
			Type: "price_data",
			Data: map[string]interface{}{
				"symbol":        quote.Symbol,
				"price":         quote.Price,
				"change":        quote.Change,
				"changePercent": quote.ChangePercent,
				"volume":        quote.Volume,
				"timestamp":     quote.Timestamp,
			},
		}
		
		err = conn.WriteJSON(priceMsg)
		if err != nil {
			s.logger.Printf("Error sending price data: %v", err)
		}
		
	case "fetch_indices":
		// Extract indices from data, or use defaults
		var indicesStr []string
		indices, ok := data["indices"].([]interface{})
		if !ok || len(indices) == 0 {
			// Use default indices
			indicesStr = []string{"S&P 500", "DOW", "NASDAQ"}
		} else {
			// Convert interface slice to string slice
			indicesStr = make([]string, len(indices))
			for i, idx := range indices {
				indicesStr[i], _ = idx.(string)
			}
		}
		
		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		// Fetch market data
		marketResult, err := s.assistant.GetAgent().FetchMarketData(ctx, indicesStr)
		if err != nil {
			s.logger.Printf("Error fetching market data: %v", err)
			s.sendError(conn, fmt.Sprintf("Error fetching market data: %v", err))
			return
		}
		
		// Convert to a format suitable for the UI
		marketData := make([]map[string]interface{}, 0, len(marketResult))
		for name, data := range marketResult {
			marketData = append(marketData, map[string]interface{}{
				"name":     name,
				"value":    data.Value,
				"change":   data.Change,
				"percent":  data.ChangePercent,
				"timestamp": data.Timestamp,
			})
		}
		
		// Send the market data to the client
		indexMsg := Message{
			Type: "index_data",
			Data: map[string]interface{}{"indices": marketData},
		}
		
		err = conn.WriteJSON(indexMsg)
		if err != nil {
			s.logger.Printf("Error sending index data: %v", err)
		}

	default:
		s.sendError(conn, fmt.Sprintf("Unknown command: %s", command))
	}
}

// sendError sends an error message to the client
func (s *WebServer) sendError(conn *ThreadSafeConn, errorMsg string) {
	// Create a simplified user-friendly error message
	userMsg := "Data temporarily unavailable. Please try again later."
	
	// Log the full error for debugging
	s.logger.Printf("Error details: %s", errorMsg)
	
	errMsg := Message{
		Type:    "error",
		Content: userMsg,
	}
	err := conn.WriteJSON(errMsg)
	if err != nil {
		s.logger.Printf("Error sending error message: %v", err)
	}
}

// startAlertConsumer starts a consumer for alerts
func (s *WebServer) startAlertConsumer(ctx context.Context, consumerID string) {
	// Create alert handler function
	alertHandler := func(alert pkg.AlertMessage) error {
		// Create alert message
		direction := "increased"
		if alert.PercentChange < 0 {
			direction = "decreased"
		}

		alertMsg := Message{
			Type: "alert",
			Data: map[string]interface{}{
				"symbol":        alert.Symbol,
				"price":         alert.Price,
				"previousPrice": alert.PreviousPrice,
				"percentChange": alert.PercentChange,
				"direction":     direction,
				"volume":        alert.Volume,
				"timestamp":     alert.Timestamp,
			},
		}

		// Broadcast to all clients
		for client := range s.activeClients {
			err := client.WriteJSON(alertMsg)
			if err != nil {
				s.logger.Printf("Error sending alert to client: %v", err)
				// Don't remove client here to avoid concurrent map write during iteration
			}
		}

		return nil
	}

	// Create and start the consumer
	consumer, err := pkg.NewQueueConsumer(commonArgs.redisAddr, s.logger, alertHandler)
	if err != nil {
		s.logger.Printf("Failed to create alert consumer: %v", err)
		return
	}
	defer consumer.Close()

	// Start consuming alerts
	err = consumer.StartConsuming(ctx, consumerID)
	if err != nil && err != context.Canceled {
		s.logger.Printf("Error consuming alerts: %v", err)
	}
}

// getChatSystemPrompt returns the system prompt for the chat interface
func getChatSystemPrompt() string {
	return `You are a concise financial assistant integrated with Quotron, a financial data system.
You have access to real-time financial data through API calls that I can make for you.

IMPORTANT GUIDELINES:
- Give direct, factual responses focused on data and numbers
- Avoid unnecessary explanations, advice, or commentary
- When presenting financial data, focus on key numbers and facts only
- Keep responses very brief (1-3 sentences when possible)
- Do not offer investment advice or recommendations
- Do not explain every aspect of market movements
- Present data in a clean, minimal format

When asked about stocks, crypto, or indices, simply provide the current price, change, and volume.
Skip explanations of what the data means, market sentiment, or other commentary.

Example response for stock data:
"AAPL: $170.55 (+1.2%, Vol: 45.7M)"

Example response for market question:
"S&P 500: 4,783.35 (-0.2%), NASDAQ: 16,302.76 (+0.1%), DOW: 38,905.66 (-0.4%)"
`
}

// getAlertSystemPrompt returns the system prompt for the AI alerter
func getAlertSystemPrompt() string {
	return `You are a financial assistant specialized in analyzing price movements.
When a significant price movement is detected, you'll receive an alert with the details.
Your job is to:

1. Analyze the potential reasons for the price movement
2. Provide context about the company or cryptocurrency
3. Suggest possible implications for investors
4. Keep your analysis concise and focused (2-3 paragraphs)

Be factual and informative. If you're not certain about something, make that clear.
Focus on providing value to investors who need quick insights about sudden price changes.`
}