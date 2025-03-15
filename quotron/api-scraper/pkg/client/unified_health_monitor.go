package client

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/we-be/tiny-ria/quotron/health"
	healthClient "github.com/we-be/tiny-ria/quotron/health/client"
)

// UnifiedHealthMonitor is an implementation of HealthMonitor that uses the unified health service
type UnifiedHealthMonitor struct {
	client       Client
	healthClient *healthClient.HealthClient
	sourceType   string
	sourceName   string
	sourceDetail string
}

// NewUnifiedHealthMonitor creates a new unified health monitor
func NewUnifiedHealthMonitor(client Client, sourceType, sourceName, sourceDetail string) (*UnifiedHealthMonitor, error) {
	// Get health service URL from environment or use default
	healthServiceURL := os.Getenv("HEALTH_SERVICE_URL")
	if healthServiceURL == "" {
		healthServiceURL = "http://localhost:8085"
	}

	// Create health client
	hc := healthClient.NewHealthClient(healthServiceURL)

	return &UnifiedHealthMonitor{
		client:       client,
		healthClient: hc,
		sourceType:   sourceType,
		sourceName:   sourceName,
		sourceDetail: sourceDetail,
	}, nil
}

// CheckHealth performs a health check and reports to the unified health service
func (m *UnifiedHealthMonitor) CheckHealth() (HealthStatus, error, int64) {
	start := time.Now()

	// Perform a minimal API call to check health
	ctx := context.Background()
	_, err := m.client.GetStockQuote(ctx, "AAPL")
	responseTime := time.Since(start).Milliseconds()

	// Determine status based on error
	status := HealthStatusHealthy
	if err != nil {
		status = HealthStatusFailed
	}

	// Report to unified health service
	healthStatus := health.StatusHealthy
	if status == HealthStatusFailed {
		healthStatus = health.StatusFailed
	}

	// Create metadata
	metadata := map[string]interface{}{
		"response_time_ms": responseTime,
		"timestamp":        time.Now().Unix(),
	}

	// Include error message if any
	errorMessage := ""
	if err != nil {
		errorMessage = err.Error()
	}

	// Report health asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		report := health.HealthReport{
			SourceType:     m.sourceType,
			SourceName:     m.sourceName,
			SourceDetail:   m.sourceDetail,
			Status:         healthStatus,
			LastCheck:      time.Now(),
			ResponseTimeMs: responseTime,
			ErrorMessage:   errorMessage,
			Metadata:       metadata,
		}

		err := m.healthClient.ReportHealth(ctx, report)
		if err != nil {
			fmt.Printf("Error reporting health: %v\n", err)
		}
	}()

	if err != nil {
		return HealthStatusFailed, err, responseTime
	}

	return HealthStatusHealthy, nil, responseTime
}

// GetSourceInfo returns information about this data source
func (m *UnifiedHealthMonitor) GetSourceInfo() (string, string, string) {
	return m.sourceType, m.sourceName, m.sourceDetail
}

// GetMetadata returns additional metadata about the source
func (m *UnifiedHealthMonitor) GetMetadata() HealthMetadata {
	return HealthMetadata{
		AdditionalInfo: map[string]interface{}{
			"source": m.sourceName,
		},
	}
}
