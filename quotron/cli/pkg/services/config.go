package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds global configuration for all services
type Config struct {
	// API Service configuration
	APIHost string `json:"api_host"`
	APIPort int    `json:"api_port"`

	// YFinance Proxy configuration
	YFinanceProxyHost string `json:"yfinance_proxy_host"`
	YFinanceProxyPort int    `json:"yfinance_proxy_port"`
	YFinanceProxyURL  string `json:"yfinance_proxy_url"`

	// Redis configuration
	RedisHost     string `json:"redis_host"`
	RedisPort     int    `json:"redis_port"`
	RedisPassword string `json:"redis_password"`

	// Database configuration
	DBHost     string `json:"db_host"`
	DBPort     int    `json:"db_port"`
	DBName     string `json:"db_name"`
	DBUser     string `json:"db_user"`
	DBPassword string `json:"db_password"`

	// Dashboard configuration
	DashboardHost string `json:"dashboard_host"`
	DashboardPort int    `json:"dashboard_port"`
	
	// Health Service configuration
	HealthServiceHost string `json:"health_service_host"`
	HealthServicePort int    `json:"health_service_port"`
	HealthServiceURL  string `json:"health_service_url"`

	// Paths
	QuotronRoot        string `json:"quotron_root"`
	APIScraperPath     string `json:"api_scraper_path"`
	SchedulerPath      string `json:"scheduler_path"`
	PythonScraperDir   string `json:"python_scraper_dir"`
	ETLDir             string `json:"etl_dir"`

	// Log file paths
	DashboardLogFile    string `json:"dashboard_log_file"`
	YFinanceLogFile     string `json:"yfinance_log_file"`
	SchedulerLogFile    string `json:"scheduler_log_file"`
	APIServiceLogFile   string `json:"api_service_log_file"`
	HealthServiceLogFile string `json:"health_service_log_file"`
	ETLServiceLogFile   string `json:"etl_service_log_file"`

	// PID files
	DashboardPIDFile     string `json:"dashboard_pid_file"`
	SchedulerPIDFile     string `json:"scheduler_pid_file"`
	YFinanceProxyPIDFile string `json:"yfinance_proxy_pid_file"`
	APIServicePIDFile    string `json:"api_service_pid_file"`
	HealthServicePIDFile string `json:"health_service_pid_file"`
	ETLServicePIDFile    string `json:"etl_service_pid_file"`

	// External API keys
	AlphaVantageAPIKey string `json:"alpha_vantage_api_key"`
}

// DefaultConfig creates a default configuration
func DefaultConfig() *Config {
	// Get the project root directory
	quotronRoot := "/home/hunter/Desktop/tiny-ria/quotron"

	// Create default config
	config := &Config{
		// API Service configuration
		APIHost: "localhost",
		APIPort: 8080,

		// YFinance Proxy configuration
		YFinanceProxyHost: "localhost",
		YFinanceProxyPort: 5000,
		YFinanceProxyURL:  "http://localhost:5000",

		// Redis configuration
		RedisHost:     "localhost",
		RedisPort:     6379,
		RedisPassword: "",

		// Database configuration
		DBHost:     "localhost",
		DBPort:     5432,
		DBName:     "quotron",
		DBUser:     "quotron",
		DBPassword: "quotron",

		// Dashboard configuration
		DashboardHost: "localhost",
		DashboardPort: 8501,
		
		// Health Service configuration
		HealthServiceHost: "localhost",
		HealthServicePort: 8085,
		HealthServiceURL:  "http://localhost:8085",

		// Paths
		QuotronRoot:      quotronRoot,
		APIScraperPath:   filepath.Join(quotronRoot, "api-scraper", "api-scraper"),
		SchedulerPath:    filepath.Join(quotronRoot, "scheduler"),
		PythonScraperDir: filepath.Join(quotronRoot, "browser-scraper", "src"),
		ETLDir:           filepath.Join(quotronRoot, "etl"),

		// Log file paths
		DashboardLogFile:  "/tmp/dashboard.log",
		YFinanceLogFile:   "/tmp/yfinance_proxy.log",
		SchedulerLogFile:  "/tmp/scheduler.log",
		APIServiceLogFile: "/tmp/api_service.log",
		ETLServiceLogFile: "/tmp/etl_service.log",

		// PID files
		DashboardPIDFile:     filepath.Join(quotronRoot, "dashboard", ".dashboard.pid"),
		SchedulerPIDFile:     filepath.Join(quotronRoot, "scheduler", ".scheduler.pid"),
		YFinanceProxyPIDFile: filepath.Join(quotronRoot, "api-scraper", ".yfinance_proxy.pid"),
		APIServicePIDFile:    filepath.Join(quotronRoot, "api-service", ".api_service.pid"),
		ETLServicePIDFile:    filepath.Join(quotronRoot, "cli", ".etl_service.pid"),

		// External API keys
		AlphaVantageAPIKey: os.Getenv("ALPHA_VANTAGE_API_KEY"),
	}

	// Override with environment variables if available
	if envAPIHost := os.Getenv("API_HOST"); envAPIHost != "" {
		config.APIHost = envAPIHost
	}
	if envAPIPort := os.Getenv("API_PORT"); envAPIPort != "" {
		var port int
		if _, err := fmt.Sscanf(envAPIPort, "%d", &port); err == nil {
			config.APIPort = port
		}
	}
	// Add more environment variable overrides as needed

	return config
}

// LoadFromFile loads configuration from a JSON file
func (c *Config) LoadFromFile(filePath string) error {
	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse JSON
	err = json.Unmarshal(data, c)
	if err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	return nil
}

// SaveToFile saves the configuration to a JSON file
func (c *Config) SaveToFile(filePath string) error {
	// Convert to JSON
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}