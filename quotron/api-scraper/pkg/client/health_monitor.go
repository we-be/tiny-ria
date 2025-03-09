package client

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// HealthStatus represents the possible states for a data source
type HealthStatus string

const (
	// Health status constants
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusFailed    HealthStatus = "failed"
	HealthStatusLimited   HealthStatus = "limited"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// HealthMetadata contains additional information about the health status
type HealthMetadata struct {
	RateLimitRemaining int    `json:"rate_limit_remaining,omitempty"`
	RateLimitReset     int64  `json:"rate_limit_reset,omitempty"`
	CacheHitRatio      float64 `json:"cache_hit_ratio,omitempty"`
	Version            string `json:"version,omitempty"`
	AdditionalInfo     map[string]interface{} `json:"additional_info,omitempty"`
}

// HealthMonitor defines the interface for monitoring data source health
type HealthMonitor interface {
	// CheckHealth performs a health check on the data source
	CheckHealth() (HealthStatus, error, int64)
	
	// GetSourceInfo returns information about this data source
	GetSourceInfo() (string, string, string)
	
	// GetMetadata returns additional metadata about the source
	GetMetadata() HealthMetadata
}

// HealthChecker provides functionality for checking and recording health status of data sources
type HealthChecker struct {
	db *sql.DB
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(db *sql.DB) *HealthChecker {
	return &HealthChecker{db: db}
}

// RecordHealthStatus records a health check result in the database
func (h *HealthChecker) RecordHealthStatus(monitor HealthMonitor) error {
	// Perform the health check
	status, err, responseTime := monitor.CheckHealth()
	
	// Get source information
	sourceType, sourceName, sourceDetail := monitor.GetSourceInfo()
	
	// Get additional metadata
	metadata := monitor.GetMetadata()
	
	// Convert metadata to JSON
	metadataJSON, jsonErr := json.Marshal(metadata)
	if jsonErr != nil {
		log.Printf("Error marshaling metadata to JSON: %v", jsonErr)
		metadataJSON = []byte("{}")
	}
	
	// Prepare error message if any
	var errorMessage string
	if err != nil {
		errorMessage = err.Error()
	}
	
	// Get the current time
	now := time.Now()
	
	// Check if the record exists
	var id string
	var errorCount int
	existsQuery := `
		SELECT id, error_count 
		FROM data_source_health 
		WHERE source_type = $1 AND source_name = $2
	`
	err = h.db.QueryRow(existsQuery, sourceType, sourceName).Scan(&id, &errorCount)
	
	if err == sql.ErrNoRows {
		// Insert new record
		insertQuery := `
			INSERT INTO data_source_health 
			(source_type, source_name, source_detail, status, last_check, 
			 error_message, response_time_ms, metadata, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`
		_, err = h.db.Exec(
			insertQuery,
			sourceType,
			sourceName,
			sourceDetail,
			status,
			now,
			errorMessage,
			responseTime,
			string(metadataJSON),
			now,
			now,
		)
		return err
	} else if err != nil {
		return fmt.Errorf("error checking for existing health record: %w", err)
	}
	
	// Update existing record
	updateQuery := `
		UPDATE data_source_health
		SET status = $1, 
		    last_check = $2,
		    updated_at = $2,
			error_message = $3,
			response_time_ms = $4,
			metadata = $5,
			error_count = CASE 
				WHEN $1 = 'failed' OR $1 = 'error' THEN error_count + 1 
				ELSE error_count 
			END,
			last_success = CASE 
				WHEN $1 = 'healthy' OR $1 = 'degraded' OR $1 = 'limited' THEN $2
				ELSE last_success 
			END
		WHERE id = $6
	`
	_, err = h.db.Exec(
		updateQuery,
		status,
		now,
		errorMessage,
		responseTime,
		string(metadataJSON),
		id,
	)
	return err
}

// Implementation for Alpha Vantage Health Monitoring
type AlphaVantageHealthMonitor struct {
	client *AlphaVantageClient
}

func NewAlphaVantageHealthMonitor(client *AlphaVantageClient) *AlphaVantageHealthMonitor {
	return &AlphaVantageHealthMonitor{client: client}
}

func (m *AlphaVantageHealthMonitor) CheckHealth() (HealthStatus, error, int64) {
	start := time.Now()
	
	// Use the API key status as a simple health check
	// In a real implementation, you would make a lightweight call to the API
	if m.client.apiKey == "" {
		return HealthStatusFailed, fmt.Errorf("no API key configured"), 0
	}
	
	// Perform a minimal API call to check health
	_, err := m.client.GetStockQuote("AAPL")
	responseTime := time.Since(start).Milliseconds()
	
	if err != nil {
		return HealthStatusFailed, err, responseTime
	}
	
	return HealthStatusHealthy, nil, responseTime
}

func (m *AlphaVantageHealthMonitor) GetSourceInfo() (string, string, string) {
	return "api-scraper", "alpha_vantage", "Alpha Vantage Financial API"
}

func (m *AlphaVantageHealthMonitor) GetMetadata() HealthMetadata {
	return HealthMetadata{
		AdditionalInfo: map[string]interface{}{
			"hasApiKey": m.client.apiKey != "",
		},
	}
}

// Implementation for Yahoo Finance Health Monitoring
type YahooFinanceHealthMonitor struct {
	client *YahooFinanceClient
}

func NewYahooFinanceHealthMonitor(client *YahooFinanceClient) *YahooFinanceHealthMonitor {
	return &YahooFinanceHealthMonitor{client: client}
}

func (m *YahooFinanceHealthMonitor) CheckHealth() (HealthStatus, error, int64) {
	start := time.Now()
	
	// Perform a minimal API call to check health
	_, err := m.client.GetStockQuote("AAPL")
	responseTime := time.Since(start).Milliseconds()
	
	if err != nil {
		return HealthStatusFailed, err, responseTime
	}
	
	return HealthStatusHealthy, nil, responseTime
}

func (m *YahooFinanceHealthMonitor) GetSourceInfo() (string, string, string) {
	return "api-scraper", "yahoo_finance", "Yahoo Finance Go Library"
}

func (m *YahooFinanceHealthMonitor) GetMetadata() HealthMetadata {
	return HealthMetadata{
		Version: "finance-go latest",
	}
}

// Implementation for Yahoo Finance REST Client Health Monitoring
type YahooFinanceRESTHealthMonitor struct {
	client *YahooFinanceRESTClient
}

func NewYahooFinanceRESTHealthMonitor(client *YahooFinanceRESTClient) *YahooFinanceRESTHealthMonitor {
	return &YahooFinanceRESTHealthMonitor{client: client}
}

func (m *YahooFinanceRESTHealthMonitor) CheckHealth() (HealthStatus, error, int64) {
	start := time.Now()
	
	// Perform a minimal API call to check health
	_, err := m.client.GetStockQuote("AAPL")
	responseTime := time.Since(start).Milliseconds()
	
	if err != nil {
		return HealthStatusFailed, err, responseTime
	}
	
	// Check if we're hitting rate limits
	if m.client.rateLimited {
		return HealthStatusLimited, nil, responseTime
	}
	
	return HealthStatusHealthy, nil, responseTime
}

func (m *YahooFinanceRESTHealthMonitor) GetSourceInfo() (string, string, string) {
	return "api-scraper", "yahoo_finance_rest", "Yahoo Finance REST API"
}

func (m *YahooFinanceRESTHealthMonitor) GetMetadata() HealthMetadata {
	// In a real implementation, you would track and return actual rate limit info
	return HealthMetadata{
		RateLimitRemaining: m.client.remainingCalls,
		RateLimitReset:     m.client.resetTime.Unix(),
		AdditionalInfo: map[string]interface{}{
			"rateLimited": m.client.rateLimited,
			"retryCount":  m.client.retryCount,
		},
	}
}

// Implementation for Yahoo Finance Proxy Client Health Monitoring
type YahooProxyHealthMonitor struct {
	client *YahooProxyClient
}

func NewYahooProxyHealthMonitor(client *YahooProxyClient) *YahooProxyHealthMonitor {
	return &YahooProxyHealthMonitor{client: client}
}

func (m *YahooProxyHealthMonitor) CheckHealth() (HealthStatus, error, int64) {
	start := time.Now()
	
	// Use the health endpoint
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	health, err := m.client.CheckProxyHealth(ctx)
	responseTime := time.Since(start).Milliseconds()
	
	if err != nil {
		return HealthStatusFailed, err, responseTime
	}
	
	if health.Status != "ok" {
		return HealthStatusDegraded, fmt.Errorf("proxy returned non-ok status: %s", health.Status), responseTime
	}
	
	return HealthStatusHealthy, nil, responseTime
}

func (m *YahooProxyHealthMonitor) GetSourceInfo() (string, string, string) {
	return "api-scraper", "yahoo_finance_proxy", "Yahoo Finance Python Proxy"
}

func (m *YahooProxyHealthMonitor) GetMetadata() HealthMetadata {
	hits, misses, ratio := m.client.GetCacheMetrics()
	
	return HealthMetadata{
		CacheHitRatio: ratio,
		AdditionalInfo: map[string]interface{}{
			"cacheHits":    hits,
			"cacheMisses":  misses,
			"requestCount": m.client.GetRequestCount(),
			"proxyURL":     m.client.proxyURL,
		},
	}
}