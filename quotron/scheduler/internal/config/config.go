package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// Config holds global configuration settings
type Config struct {
	// Legacy API scraper settings
	ApiScraper string
	ApiKey     string
	OutputDir  string
	
	// API Service settings
	ApiHost       string
	ApiPort       int
	UseAPIService bool
}

// SchedulerConfig holds the scheduler configuration
type SchedulerConfig struct {
	// API settings
	APIKey     string `json:"api_key"`
	APIBaseURL string `json:"api_base_url"`
	
	// API Service settings
	APIServiceHost string `json:"api_service_host"`
	APIServicePort int    `json:"api_service_port"`
	UseAPIService  bool   `json:"use_api_service"`
	
	// API Scraper settings
	ApiScraper string `json:"api_scraper"`
	OutputDir  string `json:"output_dir"`

	// Job schedules
	Schedules map[string]JobSchedule `json:"schedules"`

	// Global settings
	LogLevel  string        `json:"log_level"`
	TimeZone  string        `json:"timezone"`
	Retention time.Duration `json:"retention"`
}

// JobSchedule defines the schedule for a specific job
type JobSchedule struct {
	Cron        string            `json:"cron"`
	Enabled     bool              `json:"enabled"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters"`
}

// LoadConfig loads the configuration from a file
func LoadConfig(configPath string) (*SchedulerConfig, error) {
	if configPath == "" {
		return nil, errors.New("config path not specified")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config SchedulerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Use environment variables if available
	if envAPIKey := os.Getenv("ALPHA_VANTAGE_API_KEY"); envAPIKey != "" {
		config.APIKey = envAPIKey
	}
	
	// Check environment variables for API service settings
	if envAPIHost := os.Getenv("API_SERVICE_HOST"); envAPIHost != "" {
		config.APIServiceHost = envAPIHost
	}
	
	if envAPIServiceEnabled := os.Getenv("USE_API_SERVICE"); envAPIServiceEnabled == "true" {
		config.UseAPIService = true
	}

	return &config, nil
}

// ToConfig converts SchedulerConfig to the more streamlined Config type
func (sc *SchedulerConfig) ToConfig() *Config {
	return &Config{
		ApiScraper:    sc.ApiScraper,
		ApiKey:        sc.APIKey,
		OutputDir:     sc.OutputDir,
		ApiHost:       sc.APIServiceHost,
		ApiPort:       sc.APIServicePort,
		UseAPIService: sc.UseAPIService,
	}
}

// DefaultConfig returns a default configuration
func DefaultConfig() *SchedulerConfig {
	return &SchedulerConfig{
		APIBaseURL:     "https://www.alphavantage.co/query",
		APIServiceHost: "localhost",
		APIServicePort: 8080,
		UseAPIService:  false, // Default to legacy mode
		ApiScraper:     "api-scraper", // Default path if not specified
		OutputDir:      "data", // Default output directory
		LogLevel:       "info",
		TimeZone:       "UTC",
		Retention:      24 * time.Hour * 7, // 7 days
		Schedules: map[string]JobSchedule{
			"stock_quotes": {
				Cron:        "*/30 * * * *", // Every 30 minutes
				Enabled:     true,
				Description: "Fetch stock quotes for tracked symbols",
				Parameters: map[string]string{
					"symbols": "AAPL,MSFT,GOOG,AMZN",
				},
			},
			"market_indices": {
				Cron:        "0 * * * *", // Every hour
				Enabled:     true,
				Description: "Fetch market indices data",
				Parameters: map[string]string{
					"indices": "SPY,QQQ,DIA", // ETFs that track major indices, more likely to be available
				},
			},
			"crypto_quotes": {
				Cron:        "*/15 * * * *", // Every 15 minutes
				Enabled:     true,
				Description: "Fetch cryptocurrency quotes for tracked symbols",
				Parameters: map[string]string{
					"symbols": "BTC-USD,ETH-USD,SOL-USD,DOGE-USD,XRP-USD",
				},
			},
		},
	}
}