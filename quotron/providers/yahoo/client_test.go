package yahoo

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
	
	"github.com/we-be/tiny-ria/quotron/models"
)

func TestRestClientRealAPI(t *testing.T) {
	// Create a real REST client
	client, err := newRestClient(&clientOptions{
		timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create REST client: %v", err)
	}
	
	// Test with a well-known symbol
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	symbol := "AAPL"
	quote, err := client.GetStockQuote(ctx, symbol)
	if err != nil {
		t.Fatalf("Failed to get stock quote: %v", err)
	}
	
	// Verify the quote has valid data
	if quote.Symbol != symbol {
		t.Errorf("Symbol mismatch: got %s, want %s", quote.Symbol, symbol)
	}
	if quote.Price <= 0 {
		t.Errorf("Invalid price: %f", quote.Price)
	}
	if quote.Volume <= 0 {
		t.Errorf("Invalid volume: %d", quote.Volume)
	}
	if quote.Timestamp.IsZero() {
		t.Error("Timestamp is zero")
	}
	if quote.Exchange == "" {
		t.Error("Exchange is empty")
	}
	
	// Test getting multiple quotes
	symbols := []string{"MSFT", "GOOG", "AMZN"}
	quotes, err := client.GetMultipleQuotes(ctx, symbols)
	if err != nil {
		t.Fatalf("Failed to get multiple quotes: %v", err)
	}
	
	// Verify all symbols were returned
	for _, s := range symbols {
		if _, ok := quotes[s]; !ok {
			t.Errorf("Symbol %s not found in response", s)
		}
	}
	
	// Verify each quote has valid data
	for s, q := range quotes {
		if q.Symbol != s {
			t.Errorf("Symbol mismatch: got %s, want %s", q.Symbol, s)
		}
		if q.Price <= 0 {
			t.Errorf("Invalid price for %s: %f", s, q.Price)
		}
	}
	
	// Test getting market data
	indexSymbol := "^GSPC" // S&P 500
	index, err := client.GetMarketData(ctx, indexSymbol)
	if err != nil {
		t.Fatalf("Failed to get market data: %v", err)
	}
	
	// Verify the index has valid data
	if index.Symbol != indexSymbol {
		t.Errorf("Symbol mismatch: got %s, want %s", index.Symbol, indexSymbol)
	}
	if index.Value <= 0 {
		t.Errorf("Invalid value: %f", index.Value)
	}
	if index.Timestamp.IsZero() {
		t.Error("Timestamp is zero")
	}
	
	// Test health status
	health, err := client.GetHealthStatus(ctx)
	if err != nil {
		t.Fatalf("Failed to get health status: %v", err)
	}
	
	// Verify health status
	if health.Status != "up" {
		t.Errorf("Expected health status 'up', got '%s'", health.Status)
	}
	if health.ResponseTime <= 0 {
		t.Errorf("Invalid response time: %d", health.ResponseTime)
	}
	
	// Test Stop method
	err = client.Stop()
	if err != nil {
		t.Errorf("Failed to stop client: %v", err)
	}
}

func TestClientOptions(t *testing.T) {
	// Test that client options are applied correctly
	timeout := 10 * time.Second
	proxyURL := "http://localhost:8888"
	retries := 5
	healthPath := "/custom-health"

	options := defaultOptions()
	
	// Apply options
	WithTimeout(timeout)(options)
	WithProxyURL(proxyURL)(options)
	WithRetries(retries)(options)
	WithHealthPath(healthPath)(options)

	// Check that options were applied
	if options.timeout != timeout {
		t.Errorf("WithTimeout() didn't set timeout correctly: got %v, want %v", options.timeout, timeout)
	}
	if options.proxyURL != proxyURL {
		t.Errorf("WithProxyURL() didn't set proxyURL correctly: got %v, want %v", options.proxyURL, proxyURL)
	}
	if options.retries != retries {
		t.Errorf("WithRetries() didn't set retries correctly: got %v, want %v", options.retries, retries)
	}
	if options.healthPath != healthPath {
		t.Errorf("WithHealthPath() didn't set healthPath correctly: got %v, want %v", options.healthPath, healthPath)
	}
}

func TestProxyClient(t *testing.T) {
	// This test requires the proxy to be running
	// Start the proxy if it's not running
	
	// Try to start the Python proxy
	cmd := exec.Command("python", "-c", "import sys; sys.path.append('/home/hunter/Desktop/tiny-ria/quotron/api-scraper/scripts'); import yfinance_proxy; print('Proxy package found')")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("Skipping proxy test: %v\nOutput: %s", err, output)
	}
	
	// Check if proxy port 5000 is available or already in use
	isPortFree := func() bool {
		conn, err := net.DialTimeout("tcp", "localhost:5000", 1*time.Second)
		if err != nil {
			return true // Port is free
		}
		conn.Close()
		return false // Port is in use
	}
	
	var proxyCmd *exec.Cmd
	if isPortFree() {
		// Start the proxy
		proxyCmd = exec.Command("/home/hunter/Desktop/tiny-ria/quotron/.venv/bin/python", "/home/hunter/Desktop/tiny-ria/quotron/api-scraper/scripts/yfinance_proxy.py")
		proxyCmd.Stdout = os.Stdout
		proxyCmd.Stderr = os.Stderr
		err = proxyCmd.Start()
		if err != nil {
			t.Skipf("Failed to start proxy: %v", err)
		}
		
		// Give the proxy time to start
		time.Sleep(3 * time.Second)
		
		// Cleanup when test finishes
		defer func() {
			if proxyCmd.Process != nil {
				proxyCmd.Process.Kill()
			}
		}()
	} else {
		fmt.Println("Proxy appears to be already running")
	}
	
	// Create a proxy client
	client, err := newProxyClient(&clientOptions{
		timeout:    10 * time.Second,
		proxyURL:   "http://localhost:5000",
		healthPath: "/health",
	})
	if err != nil {
		t.Fatalf("Failed to create proxy client: %v", err)
	}
	
	// Test getting a stock quote
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	symbol := "AAPL"
	quote, err := client.GetStockQuote(ctx, symbol)
	if err != nil {
		t.Fatalf("Failed to get stock quote: %v", err)
	}
	
	// Verify the quote has valid data
	if quote.Symbol != symbol {
		t.Errorf("Symbol mismatch: got %s, want %s", quote.Symbol, symbol)
	}
	if quote.Price <= 0 {
		t.Errorf("Invalid price: %f", quote.Price)
	}
	
	// Test health status
	health, err := client.GetHealthStatus(ctx)
	if err != nil {
		t.Fatalf("Failed to get health status: %v", err)
	}
	
	// Verify health status
	if health.Status != "ok" && health.Status != "up" {
		t.Errorf("Expected health status 'ok' or 'up', got '%s'", health.Status)
	}
	
	// Test Stop method
	err = client.Stop()
	if err != nil {
		t.Errorf("Failed to stop client: %v", err)
	}
}