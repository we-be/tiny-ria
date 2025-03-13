package client

import (
	"context"
	"fmt"
	"time"
)

// HealthStatus represents the possible states for a data source
// This is kept for backward compatibility with the old system
// New code should use the unified health service
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
// This is kept for backward compatibility with the old system
type HealthMetadata struct {
	RateLimitRemaining int                    `json:"rate_limit_remaining,omitempty"`
	RateLimitReset     int64                  `json:"rate_limit_reset,omitempty"`
	CacheHitRatio      float64                `json:"cache_hit_ratio,omitempty"`
	Version            string                 `json:"version,omitempty"`
	AdditionalInfo     map[string]interface{} `json:"additional_info,omitempty"`
}

// HealthMonitor defines the interface for monitoring data source health
// This is kept for backward compatibility with the old system
type HealthMonitor interface {
	// CheckHealth performs a health check on the data source
	CheckHealth() (HealthStatus, error, int64)
	
	// GetSourceInfo returns information about this data source
	GetSourceInfo() (string, string, string)
	
	// GetMetadata returns additional metadata about the source
	GetMetadata() HealthMetadata
}

// HealthChecker is now just a compatibility wrapper that forwards to the unified health service
// DEPRECATED: Use the unified health service client directly
type HealthChecker struct {
	// No DB connection needed anymore as we use the unified health service
}

// NewHealthChecker creates a new health checker
// DEPRECATED: Use the unified health service client directly
func NewHealthChecker(_ interface{}) *HealthChecker {
	fmt.Println("WARNING: Using deprecated health checker. Please migrate to the unified health service.")
	return &HealthChecker{}
}

// RecordHealthStatus records a health check result using the unified health service
// DEPRECATED: Use the unified health service client directly
func (h *HealthChecker) RecordHealthStatus(monitor HealthMonitor) error {
	fmt.Println("WARNING: Using deprecated health monitoring. Please migrate to the unified health service.")
	
	// For backward compatibility, we'll create a UnifiedHealthMonitor and use that
	sourceType, sourceName, sourceDetail := monitor.GetSourceInfo()
	
	// Create a unified health monitor with the same parameters
	// This is a best-effort attempt at compatibility
	return fmt.Errorf("deprecated: use the unified health service instead. " +
		"Source: %s/%s", sourceType, sourceName)
}

// The old health monitor implementations have been removed
// Please use the UnifiedHealthMonitor from unified_health_monitor.go instead

// Example usage:
// 
// import healthClient "github.com/we-be/tiny-ria/quotron/health/client"
//
// func CreateClientWithHealthMonitoring() {
//     client := NewYahooFinanceClient()
//     monitor, _ := NewUnifiedHealthMonitor(
//         client, 
//         "api-scraper", 
//         "yahoo_finance", 
//         "Yahoo Finance API"
//     )
//     
//     // Use the monitor as needed
//     status, err, responseTime := monitor.CheckHealth()
// }