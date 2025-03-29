package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/we-be/tiny-ria/agent/pkg"
)

var (
	// Command-line flags
	apiKey     = flag.String("api-key", "", "OpenAI API key (if empty, OPENAI_API_KEY env var is used)")
	model      = flag.String("model", "gpt-3.5-turbo", "LLM model to use")
	apiHost    = flag.String("api-host", "localhost", "Host of the Quotron API service")
	apiPort    = flag.Int("api-port", 8080, "Port of the Quotron API service")
	redisAddr  = flag.String("redis", "localhost:6379", "Redis server address")
	consumerID = flag.String("consumer-id", "ai-alerter", "Consumer ID for Redis consumer group")
)

func main() {
	// Parse command-line flags
	flag.Parse()

	// Set up logging
	logger := log.New(os.Stdout, "[AI-Alerter] ", log.LstdFlags)

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
		Name:        "AIAlerter",
		APIHost:     *apiHost,
		APIPort:     *apiPort,
		EnableQueue: false, // We don't need to publish, only consume
	})

	llmConfig := pkg.LLMConfig{
		APIKey:       llmAPIKey,
		BaseURL:      "https://api.openai.com/v1/chat/completions",
		Model:        *model,
		MaxTokens:    2000,
		Temperature:  0.7,
		TimeoutSecs:  30,
		SystemPrompt: getAlertSystemPrompt(),
	}

	assistant := pkg.NewAgentAssistant(agent, llmConfig)

	// Set up context with cancellation for cleanup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	setupSignalHandling(cancel)

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
		fmt.Println("==================================\n")

		return nil
	}

	// Create and start the consumer
	logger.Printf("Starting alert consumer on Redis %s", *redisAddr)
	consumer, err := pkg.NewQueueConsumer(*redisAddr, logger, alertHandler)
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