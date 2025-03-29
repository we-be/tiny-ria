package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/we-be/tiny-ria/agent/pkg"
)

//go:embed static templates
var content embed.FS

var (
	// Command-line flags
	port          = flag.Int("port", 8090, "Web server port")
	apiKey        = flag.String("api-key", "", "OpenAI API key (if empty, OPENAI_API_KEY env var is used)")
	model         = flag.String("model", "gpt-3.5-turbo", "LLM model to use")
	apiHost       = flag.String("api-host", "localhost", "Host of the Quotron API service")
	apiPort       = flag.Int("api-port", 8080, "Port of the Quotron API service")
	redisAddr     = flag.String("redis", "localhost:6379", "Redis server address")
	consumerID    = flag.String("consumer-id", "chat-ui", "Consumer ID for Redis consumer group")
	debug         = flag.Bool("debug", false, "Enable debug mode")
	
	// Deprecated flags - kept for backward compatibility but no longer used
	_             = flag.Bool("use-real-api", false, "Deprecated: Always using Quotron API now")
	_             = flag.String("finance-api-key", "", "Deprecated: No longer needed") 
)

// WebServer handles the chat web interface
type WebServer struct {
	assistant     *pkg.AgentAssistant
	upgrader      websocket.Upgrader
	activeClients map[*websocket.Conn]bool
	logger        *log.Logger
}

// Message represents a chat message
type Message struct {
	Type    string                 `json:"type"`
	Content string                 `json:"content,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

func main() {
	// Parse command-line flags
	flag.Parse()

	// Set up logging
	logger := log.New(os.Stdout, "[Chat-UI] ", log.LstdFlags)

	// Get API key from flag or environment variable
	llmAPIKey := *apiKey
	if llmAPIKey == "" {
		llmAPIKey = os.Getenv("OPENAI_API_KEY")
		if llmAPIKey == "" {
			// Check for Anthropic API key
			llmAPIKey = os.Getenv("ANTHROPIC_API_KEY")
			if llmAPIKey == "" {
				fmt.Println("Error: API key is required. Set with --api-key flag or OPENAI_API_KEY/ANTHROPIC_API_KEY environment variable.")
				os.Exit(1)
			}
		}
	}

	// Create agent and LLM config
	agent := pkg.NewAgent(pkg.AgentConfig{
		Name:        "ChatUI",
		APIHost:     *apiHost,
		APIPort:     *apiPort,
		EnableQueue: true,
		RedisAddr:   *redisAddr,
	})

	llmConfig := pkg.LLMConfig{
		APIKey:       llmAPIKey,
		BaseURL:      "https://api.openai.com/v1/chat/completions",
		Model:        *model,
		MaxTokens:    2000,
		Temperature:  0.7,
		TimeoutSecs:  30,
		SystemPrompt: getChatSystemPrompt(),
	}

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
		activeClients: make(map[*websocket.Conn]bool),
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
	if *redisAddr != "" {
		go server.startAlertConsumer(ctx)
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
	defer conn.Close()

	// Register client
	s.activeClients[conn] = true
	defer delete(s.activeClients, conn)

	// Send welcome message
	welcomeMsg := Message{
		Type:    "system",
		Content: "Welcome to the Quotron Agent Chat Interface. How can I help you today?",
	}
	err = conn.WriteJSON(welcomeMsg)
	if err != nil {
		s.logger.Printf("Error sending welcome message: %v", err)
		return
	}

	// Listen for messages from the client
	for {
		var msg Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			s.logger.Printf("Error reading message: %v", err)
			break
		}

		// Handle the message based on its type
		switch msg.Type {
		case "user":
			// Process user message
			go s.processUserMessage(conn, msg.Content)
		case "command":
			// Process command
			go s.processCommand(conn, msg.Content, msg.Data)
		default:
			s.logger.Printf("Unknown message type: %s", msg.Type)
		}
	}
}

// processUserMessage processes a user message and sends a response
func (s *WebServer) processUserMessage(conn *websocket.Conn, content string) {
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

	// For system messages, we show a muted system message with tool usage info
	// and then also a conversational response from the assistant
	if isSystemMessage {
		// First send a system message showing the tool usage (collapsed by default)
		toolMsg := Message{
			Type:    "tool_use",
			Content: "Tool usage information: " + response,
		}
		err = conn.WriteJSON(toolMsg)
		if err != nil {
			s.logger.Printf("Error sending tool usage info: %v", err)
		}
		
		// Extract the original question from the system message if possible
		originalQuestion := ""
		if parts := strings.Split(content, "__SYSTEM__:"); len(parts) > 1 {
			originalQuestion = strings.TrimSpace(parts[1])
		}
		
		// Now generate a conversational response about the tool result
		ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel2()
		
		// Create a prompt asking the LLM to respond conversationally
		convPrompt := fmt.Sprintf("__SYSTEM__: Please respond conversationally to this question: \"%s\". "+
			"Here's the raw data I retrieved to help you: %s", originalQuestion, response)
		
		convResponse, err := s.assistant.Chat(ctx2, convPrompt)
		if err != nil {
			s.logger.Printf("Error generating conversational response: %v", err)
			return
		}
		
		// Send the conversational response
		respMsg := Message{
			Type:    "assistant",
			Content: convResponse,
		}
		err = conn.WriteJSON(respMsg)
		if err != nil {
			s.logger.Printf("Error sending conversational response: %v", err)
		}
		
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
func (s *WebServer) processCommand(conn *websocket.Conn, command string, data map[string]interface{}) {
	// Always log the command data for debugging
	s.logger.Printf("Processing command: %s with data: %v", command, data)
	
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
		
	// ... other commands as needed

	default:
		s.sendError(conn, fmt.Sprintf("Unknown command: %s", command))
	}
}

// sendError sends an error message to the client
func (s *WebServer) sendError(conn *websocket.Conn, errorMsg string) {
	errMsg := Message{
		Type:    "error",
		Content: errorMsg,
	}
	err := conn.WriteJSON(errMsg)
	if err != nil {
		s.logger.Printf("Error sending error message: %v", err)
	}
}

// startAlertConsumer starts a consumer for alerts
func (s *WebServer) startAlertConsumer(ctx context.Context) {
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
	consumer, err := pkg.NewQueueConsumer(*redisAddr, s.logger, alertHandler)
	if err != nil {
		s.logger.Printf("Failed to create alert consumer: %v", err)
		return
	}
	defer consumer.Close()

	// Start consuming alerts
	err = consumer.StartConsuming(ctx, *consumerID)
	if err != nil && err != context.Canceled {
		s.logger.Printf("Error consuming alerts: %v", err)
	}
}

// getChatSystemPrompt returns the system prompt for the chat interface
func getChatSystemPrompt() string {
	return `You are a financial assistant integrated with Quotron, a financial data system.
You have access to real-time financial data through API calls that I can make for you.

I can help you with:
1. Fetching current stock prices, cryptocurrency values, and market indices
2. Monitoring price movements and alerting on significant changes
3. Generating portfolio summaries and analysis
4. Providing financial insights based on current market data

When you ask about a financial instrument, I'll fetch the latest data for you.
If you want to track price movements, I can set up monitoring with alerts.
For portfolio analysis, provide the symbols you want to include.

I'll always try to provide you with the most up-to-date information from reliable financial data sources.

When appropriate, I can also generate visualizations and interactive charts for financial data.
`
}