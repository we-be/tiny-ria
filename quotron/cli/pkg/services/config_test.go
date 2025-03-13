package services

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	// Get default config
	config := DefaultConfig()
	
	// Check that essential fields are set
	if config.APIHost == "" {
		t.Error("APIHost should not be empty")
	}
	if config.APIPort == 0 {
		t.Error("APIPort should not be 0")
	}
	if config.YFinanceProxyHost == "" {
		t.Error("YFinanceProxyHost should not be empty")
	}
	if config.YFinanceProxyPort == 0 {
		t.Error("YFinanceProxyPort should not be 0")
	}
	if config.QuotronRoot == "" {
		t.Error("QuotronRoot should not be empty")
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create a temporary config file
	tempDir, err := os.MkdirTemp("", "config-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	configPath := filepath.Join(tempDir, "config.json")
	configContent := `{
		"api_host": "test-host",
		"api_port": 9999,
		"yfinance_proxy_host": "test-proxy-host",
		"yfinance_proxy_port": 8888,
		"yfinance_proxy_url": "http://test-proxy-host:8888",
		"db_host": "test-db-host",
		"db_port": 7777,
		"db_name": "test-db",
		"dashboard_host": "test-dashboard-host",
		"dashboard_port": 6666
	}`
	
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	
	// Load the config
	config := DefaultConfig()
	err = config.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("Failed to load config file: %v", err)
	}
	
	// Check that the values were loaded correctly
	if config.APIHost != "test-host" {
		t.Errorf("APIHost mismatch: got %s, want %s", config.APIHost, "test-host")
	}
	if config.APIPort != 9999 {
		t.Errorf("APIPort mismatch: got %d, want %d", config.APIPort, 9999)
	}
	if config.YFinanceProxyHost != "test-proxy-host" {
		t.Errorf("YFinanceProxyHost mismatch: got %s, want %s", config.YFinanceProxyHost, "test-proxy-host")
	}
	if config.YFinanceProxyPort != 8888 {
		t.Errorf("YFinanceProxyPort mismatch: got %d, want %d", config.YFinanceProxyPort, 8888)
	}
	if config.DBHost != "test-db-host" {
		t.Errorf("DBHost mismatch: got %s, want %s", config.DBHost, "test-db-host")
	}
}

func TestSaveToFile(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "config-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	configPath := filepath.Join(tempDir, "config.json")
	
	// Create a config with custom values
	config := DefaultConfig()
	config.APIHost = "test-host"
	config.APIPort = 9999
	
	// Save the config
	err = config.SaveToFile(configPath)
	if err != nil {
		t.Fatalf("Failed to save config file: %v", err)
	}
	
	// Check that the file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatalf("Config file was not created: %s", configPath)
	}
	
	// Load the config back
	newConfig := DefaultConfig()
	err = newConfig.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved config file: %v", err)
	}
	
	// Check that the values match
	if newConfig.APIHost != config.APIHost {
		t.Errorf("APIHost mismatch: got %s, want %s", newConfig.APIHost, config.APIHost)
	}
	if newConfig.APIPort != config.APIPort {
		t.Errorf("APIPort mismatch: got %d, want %d", newConfig.APIPort, config.APIPort)
	}
}