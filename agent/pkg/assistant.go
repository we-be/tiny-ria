package pkg

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// AgentAssistant combines LLM capabilities with Quotron data access
type AgentAssistant struct {
	agent      *Agent
	llmClient  *LLMClient
	history    *chatHistory
	symbolsRgx *regexp.Regexp
}

// GetAgent returns the underlying Agent
func (a *AgentAssistant) GetAgent() *Agent {
	return a.agent
}

// NewAgentAssistant creates a new agent assistant
func NewAgentAssistant(agent *Agent, llmConfig LLMConfig) *AgentAssistant {
	// Compile regex for finding stock symbols
	// Matches both regular symbols (AAPL) and crypto (BTC-USD)
	symbolsRgx := regexp.MustCompile(`\b[A-Z]{1,5}(?:-[A-Z]{3})?\b`)

	return &AgentAssistant{
		agent:      agent,
		llmClient:  NewLLMClient(llmConfig),
		history:    newChatHistory(llmConfig.SystemPrompt, 15), // Keep last 15 messages
		symbolsRgx: symbolsRgx,
	}
}

// Chat performs a single chat interaction
func (a *AgentAssistant) Chat(ctx context.Context, userMessage string) (string, error) {
	// Check if this is a system message (internal tool use)
	isSystemMessage := strings.HasPrefix(userMessage, "__SYSTEM__:")
	
	if !isSystemMessage {
		// Add regular user message to history
		a.history.AddUserMessage(userMessage)
	}

	// Special handling for "what can you do" type questions
	if matchesCapabilityQuestion(userMessage) {
		return getCapabilitiesResponse(), nil
	}
	
	// Check for commands that need direct processing before sending to LLM
	if isDirectCommand(userMessage) {
		response, err := a.processDirectCommand(ctx, userMessage)
		if err != nil {
			return "", err
		}
		a.history.AddAssistantMessage(response)
		return response, nil
	}

	// First, use the LLM to categorize the request and plan actions
	planPrompt := fmt.Sprintf(`
%s

Please first analyze what kind of financial information is being requested here.
Then, determine which specific stocks, cryptocurrencies, or indices I should fetch data for.
Finally, plan what analysis to perform on the data.

Format your response in the following structure:
ANALYSIS: [brief description of what the user is asking for]
STOCKS: [comma-separated list of stock symbols to fetch, or "none"]
CRYPTOS: [comma-separated list of crypto symbols to fetch, or "none"]
INDICES: [comma-separated list of market indices to fetch, or "none"]
ACTIONS: [brief description of the analysis steps to take]`, userMessage)
	
	// Get initial response for planning
	planMessages := []Message{
		{Role: "system", Content: "You are a financial data planning assistant. Analyze user requests and determine what financial data to retrieve."},
		{Role: "user", Content: planPrompt},
	}
	
	planResponse, err := a.llmClient.GenerateResponse(ctx, planMessages)
	if err != nil {
		return "", fmt.Errorf("error generating plan: %w", err)
	}

	// Extract data needs from the plan
	stocks, cryptos, indices := a.extractSymbolsFromPlan(planResponse)

	// Fetch any required financial data
	dataContext, err := a.fetchFinancialData(ctx, stocks, cryptos, indices)
	if err != nil {
		// If there's an error and this isn't already a system message, create a system message to handle it
		if !isSystemMessage && (len(stocks) > 0 || len(cryptos) > 0 || len(indices) > 0) {
			errorMsg := fmt.Sprintf("__SYSTEM__: Error fetching data: %v. Try to fix the symbols and fetch again.", err)
			return a.Chat(ctx, errorMsg) // Recursively call Chat with a system message
		}
		// Continue even with partial data
		dataContext += "\nNote: Some data could not be retrieved due to errors."
	}

	// Combine the original user query with the financial data for the final response
	enhancedPrompt := fmt.Sprintf(`
I've retrieved the following financial data to help answer your question:

%s

Based on this information, please provide a comprehensive and insightful response to the original question: "%s"
`, dataContext, userMessage)

	// Add the enhanced prompt to history and get final response
	finalMessages := a.history.GetMessages()
	if len(finalMessages) > 0 {
		// If we already have messages in history, replace the last one
		finalMessages[len(finalMessages)-1] = Message{Role: "user", Content: enhancedPrompt}
	} else {
		// Otherwise just add it
		finalMessages = append(finalMessages, Message{Role: "user", Content: enhancedPrompt})
	}
	
	response, err := a.llmClient.GenerateResponse(ctx, finalMessages)
	if err != nil {
		return "", fmt.Errorf("error generating response: %w", err)
	}

	// Only add to chat history if this wasn't a system message
	if !isSystemMessage {
		// Add assistant response to history
		a.history.AddAssistantMessage(response)
	}
	
	return response, nil
}

// matchesCapabilityQuestion checks if the user is asking about capabilities
func matchesCapabilityQuestion(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "what can you do") ||
		strings.Contains(message, "how can you help") || 
		strings.Contains(message, "what are your capabilities") ||
		strings.Contains(message, "what tools") ||
		(strings.Contains(message, "help") && len(message) < 10)
}

// getCapabilitiesResponse returns a response about the assistant's capabilities
func getCapabilitiesResponse() string {
	return `I can help you with various financial tasks:

1. **Market Information**: Ask about specific stocks, market indices, or cryptocurrency prices
2. **Portfolio Monitoring**: I can track stocks and alert you to price changes
3. **Financial Analysis**: Ask me to analyze stock trends or compare companies 
4. **Price Alerts**: Set up alerts for price changes with "monitor [SYMBOL] with [X]% threshold"
5. **Market Summaries**: Get quick updates on market performance

Examples:
- "How is AAPL performing today?"
- "Show me the top tech stocks"
- "Monitor TSLA, MSFT, GOOGL with 2% threshold"
- "What's happening with Bitcoin?"
- "Compare AAPL and MSFT performance"
- "Show me the latest S&P 500 data"
`
}

// isDirectCommand checks if the user message is a direct command
func isDirectCommand(message string) bool {
	message = strings.ToLower(message)
	return strings.HasPrefix(message, "monitor ") ||
		strings.HasPrefix(message, "track ") ||
		strings.HasPrefix(message, "alert ") ||
		strings.HasPrefix(message, "portfolio ")
}

// processDirectCommand handles direct commands without LLM
func (a *AgentAssistant) processDirectCommand(ctx context.Context, command string) (string, error) {
	command = strings.ToLower(command)
	
	// Handle monitor/track/alert commands
	if strings.HasPrefix(command, "monitor ") || strings.HasPrefix(command, "track ") || strings.HasPrefix(command, "alert ") {
		// Extract symbols and threshold
		matches := a.symbolsRgx.FindAllString(command, -1)
		if len(matches) == 0 {
			// Log for debugging
			fmt.Printf("Command '%s' didn't match any stock symbols using regex: %v\n", command, a.symbolsRgx)
			
			// Try a more direct approach for common stock symbols
			commandUpper := strings.ToUpper(command)
			for _, commonSymbol := range []string{"AAPL", "MSFT", "AMZN", "GOOGL", "GOOG", "META", "TSLA"} {
				if strings.Contains(commandUpper, commonSymbol) {
					matches = append(matches, commonSymbol)
				}
			}
			
			if len(matches) == 0 {
				return "I couldn't find any stock symbols to monitor. Please specify symbols like AAPL or MSFT.", nil
			}
		}
		
		// Default threshold
		threshold := 1.0
		// Try to extract custom threshold
		thresholdRgx := regexp.MustCompile(`(\d+(\.\d+)?)%`)
		thresholdMatches := thresholdRgx.FindStringSubmatch(command)
		if len(thresholdMatches) > 1 {
			fmt.Sscanf(thresholdMatches[1], "%f", &threshold)
		}
		
		return fmt.Sprintf("I've set up monitoring for %s with a %.1f%% threshold. I'll alert you when there are significant price movements.", 
			strings.Join(matches, ", "), threshold), nil
	}
	
	// Handle portfolio command
	if strings.HasPrefix(command, "portfolio ") {
		// Extract stocks and cryptos
		stocks := []string{}
		cryptos := []string{}
		
		matches := a.symbolsRgx.FindAllString(command, -1)
		for _, match := range matches {
			if strings.Contains(match, "-") {
				cryptos = append(cryptos, match)
			} else {
				stocks = append(stocks, match)
			}
		}
		
		if len(stocks) == 0 && len(cryptos) == 0 {
			return "I couldn't find any symbols for your portfolio. Please specify stock symbols like AAPL or crypto symbols like BTC-USD.", nil
		}
		
		// Generate portfolio summary
		summary, err := a.agent.GetPortfolioSummary(ctx, stocks, cryptos)
		if err != nil {
			return fmt.Sprintf("Error generating portfolio summary: %v", err), nil
		}
		
		return summary, nil
	}
	
	// If we didn't recognize the command, let the LLM handle it
	return "", fmt.Errorf("not a recognized direct command")
}

// extractSymbolsFromPlan parses the LLM's planning response to extract financial symbols
func (a *AgentAssistant) extractSymbolsFromPlan(planResponse string) ([]string, []string, []string) {
	stocks := []string{}
	cryptos := []string{}
	indices := []string{}
	
	// Extract from STOCKS section
	stocksMatch := regexp.MustCompile(`STOCKS:\s*(.+)`).FindStringSubmatch(planResponse)
	if len(stocksMatch) > 1 && stocksMatch[1] != "none" {
		for _, symbol := range strings.Split(stocksMatch[1], ",") {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" && !strings.EqualFold(symbol, "none") {
				stocks = append(stocks, symbol)
			}
		}
	}
	
	// Extract from CRYPTOS section
	cryptosMatch := regexp.MustCompile(`CRYPTOS:\s*(.+)`).FindStringSubmatch(planResponse)
	if len(cryptosMatch) > 1 && cryptosMatch[1] != "none" {
		for _, symbol := range strings.Split(cryptosMatch[1], ",") {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" && !strings.EqualFold(symbol, "none") {
				// Ensure crypto symbols have USD suffix if not already present
				if !strings.Contains(symbol, "-") {
					symbol = symbol + "-USD"
				}
				cryptos = append(cryptos, symbol)
			}
		}
	}
	
	// Extract from INDICES section
	indicesMatch := regexp.MustCompile(`INDICES:\s*(.+)`).FindStringSubmatch(planResponse)
	if len(indicesMatch) > 1 && indicesMatch[1] != "none" {
		for _, symbol := range strings.Split(indicesMatch[1], ",") {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" && !strings.EqualFold(symbol, "none") {
				indices = append(indices, symbol)
			}
		}
	}
	
	return stocks, cryptos, indices
}

// fetchFinancialData retrieves data for the requested financial instruments
func (a *AgentAssistant) fetchFinancialData(ctx context.Context, stocks, cryptos, indices []string) (string, error) {
	var result strings.Builder
	var errs []string
	var invalidSymbols []string
	
	// Fetch stock data
	if len(stocks) > 0 {
		result.WriteString("## Stock Data\n\n")
		stockData, err := a.agent.FetchStockData(ctx, stocks)
		if err != nil {
			errs = append(errs, fmt.Sprintf("Error fetching stock data: %v", err))
		}
		
		if len(stockData) > 0 {
			result.WriteString("| Symbol | Price | Change | Change % | Volume | Timestamp |\n")
			result.WriteString("|--------|-------|--------|----------|--------|----------|\n")
			
			for _, symbol := range stocks {
				if quote, ok := stockData[symbol]; ok {
					result.WriteString(fmt.Sprintf("| %s | $%.2f | %.2f | %.2f%% | %s | %s |\n",
						symbol, quote.Price, quote.Change, quote.ChangePercent,
						formatNumber(quote.Volume), quote.Timestamp.Format(time.RFC3339)))
				} else {
					result.WriteString(fmt.Sprintf("| %s | N/A | N/A | N/A | N/A | N/A |\n", symbol))
					invalidSymbols = append(invalidSymbols, symbol)
				}
			}
			result.WriteString("\n")
		} else {
			result.WriteString("No stock data available\n\n")
			invalidSymbols = append(invalidSymbols, stocks...)
		}
	}
	
	// Fetch crypto data
	if len(cryptos) > 0 {
		result.WriteString("## Cryptocurrency Data\n\n")
		cryptoData, err := a.agent.FetchCryptoData(ctx, cryptos)
		if err != nil {
			errs = append(errs, fmt.Sprintf("Error fetching crypto data: %v", err))
		}
		
		if len(cryptoData) > 0 {
			result.WriteString("| Symbol | Price | Change | Change % | Volume | Timestamp |\n")
			result.WriteString("|--------|-------|--------|----------|--------|----------|\n")
			
			for _, symbol := range cryptos {
				if quote, ok := cryptoData[symbol]; ok {
					result.WriteString(fmt.Sprintf("| %s | $%.2f | %.2f | %.2f%% | %s | %s |\n",
						symbol, quote.Price, quote.Change, quote.ChangePercent,
						formatNumber(quote.Volume), quote.Timestamp.Format(time.RFC3339)))
				} else {
					result.WriteString(fmt.Sprintf("| %s | N/A | N/A | N/A | N/A | N/A |\n", symbol))
					invalidSymbols = append(invalidSymbols, symbol)
				}
			}
			result.WriteString("\n")
		} else {
			result.WriteString("No cryptocurrency data available\n\n")
			invalidSymbols = append(invalidSymbols, cryptos...)
		}
	}
	
	// Fetch market index data
	if len(indices) > 0 {
		result.WriteString("## Market Index Data\n\n")
		indexData, err := a.agent.FetchMarketData(ctx, indices)
		if err != nil {
			errs = append(errs, fmt.Sprintf("Error fetching market index data: %v", err))
		}
		
		if len(indexData) > 0 {
			result.WriteString("| Index | Value | Change | Change % | Timestamp |\n")
			result.WriteString("|-------|-------|--------|----------|----------|\n")
			
			for _, symbol := range indices {
				if data, ok := indexData[symbol]; ok {
					result.WriteString(fmt.Sprintf("| %s | %.2f | %.2f | %.2f%% | %s |\n",
						symbol, data.Value, data.Change, data.ChangePercent,
						data.Timestamp.Format(time.RFC3339)))
				} else {
					result.WriteString(fmt.Sprintf("| %s | N/A | N/A | N/A | N/A |\n", symbol))
					invalidSymbols = append(invalidSymbols, symbol)
				}
			}
			result.WriteString("\n")
		} else {
			result.WriteString("No market index data available\n\n")
			invalidSymbols = append(invalidSymbols, indices...)
		}
	}
	
	// Add suggestions for invalid symbols
	if len(invalidSymbols) > 0 {
		result.WriteString("## Data Retrieval Issues\n\n")
		result.WriteString("There were issues retrieving data for the following symbols:\n")
		for _, symbol := range invalidSymbols {
			result.WriteString(fmt.Sprintf("- %s\n", symbol))
		}
		
		result.WriteString("\nPossible fixes:\n")
		result.WriteString("- Check for typos in the symbols\n")
		result.WriteString("- For stocks, use standard ticker symbols (e.g., AAPL, MSFT)\n")
		result.WriteString("- For cryptocurrencies, add '-USD' suffix (e.g., BTC-USD, ETH-USD)\n")
		result.WriteString("- For indices, use standard names (e.g., S&P 500, Dow Jones, NASDAQ)\n")
		result.WriteString("\n")
	}
	
	if len(errs) > 0 {
		return result.String(), fmt.Errorf("%s. Invalid symbols: %s", strings.Join(errs, "; "), strings.Join(invalidSymbols, ", "))
	}
	
	return result.String(), nil
}