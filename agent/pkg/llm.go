package pkg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
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
	Provider     string // "openai" or "anthropic"
}

// DefaultLLMConfig returns a default LLM configuration
func DefaultLLMConfig() LLMConfig {
	// Default to OpenAI
	apiKey := os.Getenv("OPENAI_API_KEY")
	provider := "openai"
	baseURL := "https://api.openai.com/v1/chat/completions"
	model := "gpt-3.5-turbo"
	
	// Check for Anthropic API key environment variable
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" && anthropicKey != "" {
		// Only use Anthropic if OpenAI key is not set but Anthropic key is
		apiKey = anthropicKey
		provider = "anthropic"
		baseURL = "https://api.anthropic.com/v1/messages"
		model = "claude-3-sonnet-20240229"
	}
	
	return LLMConfig{
		APIKey:       apiKey,
		BaseURL:      baseURL,
		Model:        model,
		MaxTokens:    2000,
		Temperature:  0.7,
		TimeoutSecs:  30,
		SystemPrompt: DefaultSystemPrompt,
		Provider:     provider,
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

// OpenAIChatRequest represents a request to the OpenAI chat completions API
type OpenAIChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature"`
}

// OpenAIChatResponse represents a response from the OpenAI chat completions API
type OpenAIChatResponse struct {
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

// AnthropicChatRequest represents a request to the Anthropic chat API
type AnthropicChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature"`
}

// AnthropicChatResponse represents a response from the Anthropic chat API
type AnthropicChatResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model       string    `json:"model"`
	StopReason  string    `json:"stop_reason"`
	StopSequence string   `json:"stop_sequence"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
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

	if c.config.Provider == "anthropic" {
		return c.generateAnthropicResponse(ctx, messages)
	}
	
	// Default to OpenAI
	return c.generateOpenAIResponse(ctx, messages)
}

// generateOpenAIResponse sends a request to the OpenAI API and returns the response
func (c *LLMClient) generateOpenAIResponse(ctx context.Context, messages []Message) (string, error) {
	chatReq := OpenAIChatRequest{
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
		return "", fmt.Errorf("error sending request to OpenAI API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return "", fmt.Errorf("error decoding error response: %w", err)
		}
		return "", fmt.Errorf("OpenAI API returned non-OK status: %d, error: %v", resp.StatusCode, errResp)
	}

	var chatResp OpenAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("error decoding response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("OpenAI API returned no choices")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// generateAnthropicResponse sends a request to the Anthropic API and returns the response
func (c *LLMClient) generateAnthropicResponse(ctx context.Context, messages []Message) (string, error) {
	// Convert to Anthropic format
	chatReq := AnthropicChatRequest{
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
	req.Header.Set("anthropic-version", "2023-12-15")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request to Anthropic API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return "", fmt.Errorf("error decoding error response: %w", err)
		}
		return "", fmt.Errorf("Anthropic API returned non-OK status: %d, error: %v", resp.StatusCode, errResp)
	}

	var chatResp AnthropicChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("error decoding response: %w", err)
	}

	// Extract text from the content
	if len(chatResp.Content) == 0 {
		return "", fmt.Errorf("Anthropic API returned no content")
	}

	var result strings.Builder
	for _, part := range chatResp.Content {
		if part.Type == "text" {
			result.WriteString(part.Text)
		}
	}

	return result.String(), nil
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