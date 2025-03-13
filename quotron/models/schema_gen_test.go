package models

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratePythonSchema(t *testing.T) {
	// Create a temporary output path
	tempDir, err := os.MkdirTemp("", "schema-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Generate schema to the temp directory
	schemaPath := filepath.Join(tempDir, "finance_schema.json")
	err = GeneratePythonSchema(schemaPath)
	if err != nil {
		t.Fatalf("Failed to generate Python schema: %v", err)
	}
	
	// Check if the file exists
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		t.Fatalf("Schema file was not created: %s", schemaPath)
	}
	
	// Read the file and check its content
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("Failed to read schema file: %v", err)
	}
	
	// Check for key expected content
	schemaStr := string(schemaData)
	expectedContent := []string{
		"StockQuote",
		"MarketIndex",
		"MarketBatch",
		"properties",
		"required",
	}
	
	for _, expected := range expectedContent {
		if !contains(schemaStr, expected) {
			t.Errorf("Schema file doesn't contain expected content: %s", expected)
		}
	}
}

func TestGeneratePythonModels(t *testing.T) {
	// Create a temporary output path
	tempDir, err := os.MkdirTemp("", "schema-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Generate schema to the temp directory
	schemaPath := filepath.Join(tempDir, "finance_schema.json")
	err = GeneratePythonSchema(schemaPath)
	if err != nil {
		t.Fatalf("Failed to generate Python schema: %v", err)
	}
	
	// Generate Python models
	pythonPath := filepath.Join(tempDir, "finance_models.py")
	err = GeneratePythonModels(schemaPath, pythonPath)
	if err != nil {
		t.Fatalf("Failed to generate Python models: %v", err)
	}
	
	// Check if the file exists
	if _, err := os.Stat(pythonPath); os.IsNotExist(err) {
		t.Fatalf("Python model file was not created: %s", pythonPath)
	}
	
	// Read the file and check its content
	pythonData, err := os.ReadFile(pythonPath)
	if err != nil {
		t.Fatalf("Failed to read Python model file: %v", err)
	}
	
	// Check for key expected content
	pythonStr := string(pythonData)
	expectedContent := []string{
		"class DataSource",
		"class Exchange",
		"class StockQuote",
		"class MarketIndex",
		"class MarketBatch",
		"BaseModel",
		"Field",
	}
	
	for _, expected := range expectedContent {
		if !contains(pythonStr, expected) {
			t.Errorf("Python model file doesn't contain expected content: %s", expected)
		}
	}
}

// Helper function for the tests
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || len(s) > len(substr) && s[1:] != "" && contains(s[1:], substr)
}