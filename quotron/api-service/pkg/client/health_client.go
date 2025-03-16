package client

import (
	"context"
	"time"

	healthClient "github.com/we-be/tiny-ria/quotron/health/client"
	"github.com/we-be/tiny-ria/quotron/health"
)

// HealthReporter provides an interface to report health status
type HealthReporter interface {
	ReportHealth(ctx context.Context, sourceType, sourceName string, status health.Status, message string) error
	GetServiceHealth(ctx context.Context, sourceType, sourceName string) (*health.HealthReport, error)
	GetAllHealth(ctx context.Context) ([]health.HealthReport, error)
}

// UnifiedHealthClient implements HealthReporter using the unified health service
type UnifiedHealthClient struct {
	client *healthClient.HealthClient
}

// NewUnifiedHealthClient creates a new health client
func NewUnifiedHealthClient(healthServiceURL string) *UnifiedHealthClient {
	return &UnifiedHealthClient{
		client: healthClient.NewHealthClient(healthServiceURL),
	}
}

// ReportHealth reports health status to the unified health service
func (c *UnifiedHealthClient) ReportHealth(ctx context.Context, sourceType, sourceName string, status health.Status, message string) error {
	report := health.HealthReport{
		SourceType:   sourceType,
		SourceName:   sourceName,
		Status:       status,
		LastCheck:    time.Now(),
		ErrorMessage: message,
	}

	// Attempt to report health, but don't fail if the health service is unavailable
	err := c.client.ReportHealth(ctx, report)
	if err != nil {
		// Log the error but don't propagate it to prevent service operation disruption
		// This makes the API service more resilient when the health service is unavailable
		return nil
	}
	return nil
}

// GetServiceHealth gets the health status of a specific service
func (c *UnifiedHealthClient) GetServiceHealth(ctx context.Context, sourceType, sourceName string) (*health.HealthReport, error) {
	report, err := c.client.GetServiceHealth(ctx, sourceType, sourceName)
	if err != nil {
		// Return default report if health service is unavailable
		return &health.HealthReport{
			SourceType:   sourceType,
			SourceName:   sourceName,
			Status:       health.StatusUnknown,
			LastCheck:    time.Now(),
			ErrorMessage: "Health service unavailable",
		}, nil
	}
	return report, nil
}

// GetAllHealth gets the health status of all services
func (c *UnifiedHealthClient) GetAllHealth(ctx context.Context) ([]health.HealthReport, error) {
	reports, err := c.client.GetAllHealth(ctx)
	if err != nil {
		// Return empty reports if health service is unavailable
		return []health.HealthReport{}, nil
	}
	return reports, nil
}

// NoopHealthClient implements HealthReporter with no-op methods for testing or when health service is unavailable
type NoopHealthClient struct{}

// NewNoopHealthClient creates a new no-op health client
func NewNoopHealthClient() *NoopHealthClient {
	return &NoopHealthClient{}
}

// ReportHealth is a no-op implementation
func (c *NoopHealthClient) ReportHealth(ctx context.Context, sourceType, sourceName string, status health.Status, message string) error {
	return nil
}

// GetServiceHealth is a no-op implementation
func (c *NoopHealthClient) GetServiceHealth(ctx context.Context, sourceType, sourceName string) (*health.HealthReport, error) {
	return &health.HealthReport{
		SourceType: sourceType,
		SourceName: sourceName,
		Status:     health.StatusUnknown,
		LastCheck:  time.Now(),
	}, nil
}

// GetAllHealth is a no-op implementation
func (c *NoopHealthClient) GetAllHealth(ctx context.Context) ([]health.HealthReport, error) {
	return []health.HealthReport{}, nil
}

// LegacyToUnifiedHealth converts legacy status strings to unified health status
func LegacyToUnifiedHealth(legacyStatus string) health.Status {
	switch legacyStatus {
	case "healthy":
		return health.StatusHealthy
	case "unhealthy":
		return health.StatusFailed
	default:
		return health.StatusUnknown
	}
}

// UnifiedToLegacyHealth converts unified health status to legacy status strings
func UnifiedToLegacyHealth(unifiedStatus health.Status) string {
	switch unifiedStatus {
	case health.StatusHealthy:
		return "healthy"
	case health.StatusDegraded, health.StatusLimited:
		return "degraded"
	case health.StatusFailed:
		return "unhealthy"
	default:
		return "unknown"
	}
}