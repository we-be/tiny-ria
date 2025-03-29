package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/we-be/tiny-ria/agent/pkg"
)

var (
	apiKey       = flag.String("api-key", "", "OpenAI API key (if empty, OPENAI_API_KEY env var is used)")
	model        = flag.String("model", "gpt-3.5-turbo", "LLM model to use")
	apiHost      = flag.String("api-host", "localhost", "Host of the Quotron API service")
	apiPort      = flag.Int("api-port", 8080, "Port of the Quotron API service")
	temperature  = flag.Float64("temperature", 0.7, "LLM temperature (higher = more creative)")
	interactMode = flag.Bool("interactive", true, "Run in interactive mode")
	singleQuery  = flag.String("query", "", "Single query to run (non-interactive mode)")
)

func main() {
	// Parse command-line flags
	flag.Parse()

	// Get API key from flag or environment variable
	llmAPIKey := *apiKey
	if llmAPIKey == "" {
		llmAPIKey = os.Getenv("OPENAI_API_KEY")
		if llmAPIKey == "" {
			fmt.Println("Error: OpenAI API key is required. Set with --api-key flag or OPENAI_API_KEY environment variable.")
			os.Exit(1)
		}
	}

	// Create agent and LLM config
	agent := pkg.NewAgent(pkg.AgentConfig{
		Name:    "FinanceAssistant",
		APIHost: *apiHost,
		APIPort: *apiPort,
	})

	llmConfig := pkg.LLMConfig{
		APIKey:       llmAPIKey,
		BaseURL:      "https://api.openai.com/v1/chat/completions",
		Model:        *model,
		MaxTokens:    2000,
		Temperature:  *temperature,
		TimeoutSecs:  30,
		SystemPrompt: pkg.DefaultLLMConfig().SystemPrompt,
	}

	assistant := pkg.NewAgentAssistant(agent, llmConfig)

	// Set up context with cancellation for cleanup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	setupSignalHandling(cancel)

	// Run in appropriate mode
	if *interactMode {
		runInteractiveMode(ctx, assistant)
	} else if *singleQuery != "" {
		runSingleQuery(ctx, assistant, *singleQuery)
	} else {
		fmt.Println("Error: In non-interactive mode, a query must be provided with --query")
		os.Exit(1)
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
		os.Exit(0)
	}()
}

// runInteractiveMode runs the assistant in interactive chat mode
func runInteractiveMode(ctx context.Context, assistant *pkg.AgentAssistant) {
	fmt.Println("=== Quotron Financial Assistant ===")
	fmt.Println("Enter your financial questions or commands below.")
	fmt.Println("Type 'exit' or 'quit' to end the session.")
	fmt.Println("Examples:")
	fmt.Println(" - What's the current price of AAPL?")
	fmt.Println(" - How are tech stocks MSFT, AAPL, and GOOG doing today?")
	fmt.Println(" - Compare Bitcoin and Ethereum prices")
	fmt.Println(" - portfolio AAPL MSFT GOOG BTC-USD")
	fmt.Println(" - monitor TSLA with 1.5% threshold")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("\n> ")
		if !scanner.Scan() {
			break
		}

		userInput := scanner.Text()
		if strings.TrimSpace(userInput) == "" {
			continue
		}

		// Check for exit command
		lowerInput := strings.ToLower(userInput)
		if lowerInput == "exit" || lowerInput == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		// Process the user input
		fmt.Println("\nProcessing...")
		response, err := assistant.Chat(ctx, userInput)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		// Display the response
		fmt.Println("\n" + response)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
	}
}

// runSingleQuery runs the assistant with a single query (non-interactive mode)
func runSingleQuery(ctx context.Context, assistant *pkg.AgentAssistant, query string) {
	response, err := assistant.Chat(ctx, query)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(response)
}