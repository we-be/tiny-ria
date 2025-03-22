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
	HealthServiceURL string `json:"health_service_url"`

	// Paths
	QuotronRoot        string `json:"quotron_root"`
	APIScraperPath     string `json:"api_scraper_path"`
	SchedulerPath      string `json:"scheduler_path"`
	SchedulerConfigFile string `json:"scheduler_config_file"`
	PythonScraperDir   string `json:"python_scraper_dir"`
	ETLDir             string `json:"etl_dir"`

	// Log file paths
	YFinanceLogFile     string `json:"yfinance_log_file"`
	SchedulerLogFile    string `json:"scheduler_log_file"`
	APIServiceLogFile   string `json:"api_service_log_file"`
	ETLServiceLogFile   string `json:"etl_service_log_file"`

	// PID files
	SchedulerPIDFile     string `json:"scheduler_pid_file"`
	YFinanceProxyPIDFile string `json:"yfinance_proxy_pid_file"`
	APIServicePIDFile    string `json:"api_service_pid_file"`
	ETLServicePIDFile    string `json:"etl_service_pid_file"`

	// External API keys
	AlphaVantageAPIKey string `json:"alpha_vantage_api_key"`
}

// GetQuotronTempDir returns the platform-appropriate temp directory for Quotron files
func GetQuotronTempDir() string {
	// Get the base temp directory for the current platform
	tempDir := os.TempDir()
	
	// Create a Quotron-specific subdirectory to avoid conflicts
	quotronTempDir := filepath.Join(tempDir, "quotron")
	
	// Create the directory if it doesn't exist
	if err := os.MkdirAll(quotronTempDir, 0755); err != nil {
		fmt.Printf("Warning: Failed to create Quotron temp directory: %v\n", err)
		// Fall back to using the system temp dir directly
		return tempDir
	}
	
	return quotronTempDir
}

// DefaultConfig creates a default configuration
func DefaultConfig() *Config {
	// Get the project root directory by finding the "quotron" directory
	// Start with the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		// Fall back to executable path if we can't get current directory
		ex, err := os.Executable()
		if err != nil {
			// Last resort, use relative path to what might be quotron directory
			cwd = "."
		} else {
			cwd = filepath.Dir(ex)
		}
	}
	
	// Try to find the quotron root by walking up the directory tree
	quotronRoot := findQuotronRoot(cwd)
	
	// Get the platform-specific temp directory
	quotronTempDir := GetQuotronTempDir()
	
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
		HealthServiceURL:  "http://localhost:8085",

		// Paths
		QuotronRoot:         quotronRoot,
		APIScraperPath:      filepath.Join(quotronRoot, "api-scraper", "api-scraper"),
		SchedulerPath:       filepath.Join(quotronRoot, "scheduler"),
		SchedulerConfigFile: filepath.Join(quotronRoot, "scheduler-config.json"),
		PythonScraperDir:    filepath.Join(quotronRoot, "browser-scraper", "src"),
		ETLDir:              filepath.Join(quotronRoot, "etl"),

		// Log file paths
		YFinanceLogFile:   filepath.Join(quotronTempDir, "yfinance_proxy.log"),
		SchedulerLogFile:  filepath.Join(quotronTempDir, "scheduler.log"),
		APIServiceLogFile: filepath.Join(quotronTempDir, "api_service.log"),
		ETLServiceLogFile: filepath.Join(quotronTempDir, "etl_service.log"),

		// PID files
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

// findQuotronRoot attempts to locate the quotron root directory by walking up the directory tree
// from the given starting path until it finds a directory that looks like the quotron root
func findQuotronRoot(startPath string) string {
	// First, check environment variable
	if envRoot := os.Getenv("QUOTRON_ROOT"); envRoot != "" {
		if _, err := os.Stat(envRoot); err == nil {
			return envRoot
		}
		// Even if the env var is invalid, try using it
		return envRoot
	}

	// Define max depth to prevent infinite loops with symlinks
	maxDepth := 10
	currPath := startPath
	
	for i := 0; i < maxDepth; i++ {
		// Check for markers that indicate we're in the quotron root
		// Look for common files/directories that should be in the root
		markersFound := 0
		markers := []string{
			"api-scraper",        // Check for api-scraper directory
			"dashboard",          // Check for dashboard directory
			"scheduler",          // Check for scheduler directory
			"scheduler-config.json", // Check for config file
			".env",               // Check for env file
		}
		
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(currPath, marker)); err == nil {
				markersFound++
			}
		}
		
		// If we found at least 3 markers, this is likely the quotron root
		if markersFound >= 3 {
			return currPath
		}
		
		// Not found - go up one directory
		parent := filepath.Dir(currPath)
		if parent == currPath {
			// We've reached the filesystem root without finding quotron
			break
		}
		currPath = parent
	}
	
	// If we couldn't find the root, use the current directory as a fallback
	fmt.Println("Warning: Could not determine Quotron root directory. Using current directory.")
	return startPath
}