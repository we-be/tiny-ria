package models

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPythonSchemaValidation(t *testing.T) {
	// This test does something real and meaningful - it generates the Python
	// schema, installs it, and then validates actual data against it from Python
	
	// Skip if no Python or if running in CI
	if _, err := exec.LookPath("python"); err != nil {
		t.Skip("Python not found, skipping test")
	}
	
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "model-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Generate the Python schema model
	schemaPath := filepath.Join(tempDir, "finance_schema.json")
	pythonPath := filepath.Join(tempDir, "finance_models.py")
	
	// Generate JSON schema
	err = GeneratePythonSchema(schemaPath)
	if err != nil {
		t.Fatalf("Failed to generate schema: %v", err)
	}
	
	// Generate Python models
	err = GeneratePythonModels(schemaPath, pythonPath)
	if err != nil {
		t.Fatalf("Failed to generate Python models: %v", err)
	}
	
	// Create test data
	now := time.Now().UTC()
	quote := StockQuote{
		Symbol:        "AAPL",
		Price:         150.25,
		Change:        1.25,
		ChangePercent: 0.84,
		Volume:        10000000,
		Timestamp:     now,
		Exchange:      NASDAQ,
		Source:        APIScraperSource,
	}
	
	// Convert to JSON
	quoteJSON, err := json.Marshal(quote)
	if err != nil {
		t.Fatalf("Failed to marshal quote to JSON: %v", err)
	}
	
	// Create Python test script that will validate the data
	testScript := filepath.Join(tempDir, "test_validation.py")
	script := `
import json
import sys
import os

# Add the parent directory to the Python path
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

try:
    # Import the generated models
    from finance_models import StockQuote, MarketIndex, DataSource, Exchange

    # Parse the data from stdin
    data = json.loads(sys.stdin.read())
    
    # Create a StockQuote object from the data
    quote = StockQuote(
        symbol=data["symbol"],
        price=data["price"],
        change=data["change"],
        change_percent=data["change_percent"],
        volume=data["volume"],
        timestamp=data["timestamp"],
        exchange=data["exchange"],
        source=data["source"]
    )
    
    # Validate by converting back to dict
    quote_dict = quote.dict()
    
    # Check that key fields match
    assert quote_dict["symbol"] == data["symbol"], f"Symbol mismatch: {quote_dict['symbol']} != {data['symbol']}"
    assert quote_dict["price"] == data["price"], f"Price mismatch: {quote_dict['price']} != {data['price']}"
    
    # Success
    print("Validation successful!")
    sys.exit(0)
except Exception as e:
    print(f"Validation error: {e}")
    sys.exit(1)
`
	
	err = os.WriteFile(testScript, []byte(script), 0644)
	if err != nil {
		t.Fatalf("Failed to write test script: %v", err)
	}
	
	// Install required packages for the test
	installCmd := exec.Command("pip", "install", "pydantic==1.*")
	installOutput, err := installCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to install required packages: %v\nOutput: %s", err, installOutput)
	}
	
	// Run the Python script with the test data
	cmd := exec.Command("python", testScript)
	cmd.Stdin = strings.NewReader(string(quoteJSON))
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		t.Fatalf("Python validation failed: %v\nOutput: %s", err, output)
	}
	
	if !strings.Contains(string(output), "Validation successful") {
		t.Errorf("Validation did not succeed: %s", output)
	}
}