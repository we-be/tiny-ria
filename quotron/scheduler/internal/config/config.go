package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// SchedulerConfig holds the scheduler configuration
type SchedulerConfig struct {
	// API settings
	APIKey     string `json:"api_key"`
	APIBaseURL string `json:"api_base_url"`

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

	return &config, nil
}

// DefaultConfig returns a default configuration
func DefaultConfig() *SchedulerConfig {
	return &SchedulerConfig{
		APIBaseURL: "https://www.alphavantage.co/query",
		LogLevel:   "info",
		TimeZone:   "UTC",
		Retention:  24 * time.Hour * 7, // 7 days
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
					"indices": "^GSPC,^DJI,^IXIC",
				},
			},
		},
	}
}