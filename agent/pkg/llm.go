package pkg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// LLMConfig holds configuration for the LLM client
type LLMConfig struct {
	APIKey       string
	BaseURL      string
	Model        string
	MaxTokens    int
	Temperature  float64
	TimeoutSecs  int
	SystemPrompt string
}

// DefaultLLMConfig returns a default LLM configuration
func DefaultLLMConfig() LLMConfig {
	return LLMConfig{
		APIKey:       os.Getenv("OPENAI_API_KEY"),
		BaseURL:      "https://api.openai.com/v1/chat/completions",
		Model:        "gpt-3.5-turbo",
		MaxTokens:    2000,
		Temperature:  0.7,
		TimeoutSecs:  30,
		SystemPrompt: DefaultSystemPrompt,
	}
}

// LLMClient manages interactions with the LLM API
type LLMClient struct {
	config     LLMConfig
	httpClient *http.Client
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents a request to the chat completions API
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature"`
}

// ChatResponse represents a response from the chat completions API
type ChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// NewLLMClient creates a new LLM client
func NewLLMClient(config LLMConfig) *LLMClient {
	return &LLMClient{
		config: config,
		httpClient: &http.Client{
			Timeout: time.Duration(config.TimeoutSecs) * time.Second,
		},
	}
}

// GenerateResponse sends a request to the LLM and returns the generated response
func (c *LLMClient) GenerateResponse(ctx context.Context, messages []Message) (string, error) {
	// Ensure the system prompt is included
	if len(messages) == 0 || messages[0].Role != "system" {
		messages = append([]Message{{Role: "system", Content: c.config.SystemPrompt}}, messages...)
	}

	chatReq := ChatRequest{
		Model:       c.config.Model,
		Messages:    messages,
		MaxTokens:   c.config.MaxTokens,
		Temperature: c.config.Temperature,
	}

	jsonData, err := json.Marshal(chatReq)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.config.BaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request to LLM API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return "", fmt.Errorf("error decoding error response: %w", err)
		}
		return "", fmt.Errorf("LLM API returned non-OK status: %d, error: %v", resp.StatusCode, errResp)
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("error decoding response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("LLM API returned no choices")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// DefaultSystemPrompt is the default system prompt for the financial assistant
const DefaultSystemPrompt = `You are a financial assistant integrated with Quotron, a financial data system. 
You have access to real-time financial data through API calls that I can make for you.

I can help you with:
1. Fetching current stock prices, cryptocurrency values, and market indices
2. Monitoring price movements and alerting on significant changes
3. Generating portfolio summaries and analysis
4. Providing financial insights based on current market data

When you ask about a financial instrument, I'll fetch the latest data for you.
If you want to track price movements, I can set up monitoring with alerts.
For portfolio analysis, provide the symbols you want to include.

I'll always try to provide you with the most up-to-date information from reliable financial data sources.`

// chatHistory stores a conversation's message history
type chatHistory struct {
	Messages []Message
	MaxSize  int
}

// newChatHistory creates a new chat history with the given system prompt
func newChatHistory(systemPrompt string, maxSize int) *chatHistory {
	return &chatHistory{
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
		},
		MaxSize: maxSize,
	}
}

// AddUserMessage adds a user message to the history
func (h *chatHistory) AddUserMessage(content string) {
	h.Messages = append(h.Messages, Message{Role: "user", Content: content})
	h.pruneIfNeeded()
}

// AddAssistantMessage adds an assistant message to the history
func (h *chatHistory) AddAssistantMessage(content string) {
	h.Messages = append(h.Messages, Message{Role: "assistant", Content: content})
	h.pruneIfNeeded()
}

// GetMessages returns all messages
func (h *chatHistory) GetMessages() []Message {
	return h.Messages
}

// pruneIfNeeded removes oldest messages if history exceeds max size
// Always keeps the system message (first message)
func (h *chatHistory) pruneIfNeeded() {
	if h.MaxSize <= 0 || len(h.Messages) <= h.MaxSize {
		return
	}

	// Keep system message and remove oldest messages
	excess := len(h.Messages) - h.MaxSize
	h.Messages = append(h.Messages[:1], h.Messages[1+excess:]...)
}