package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/we-be/tiny-ria/quotron/health"
)

// HealthClient provides a client for the health monitoring service
type HealthClient struct {
	ServiceURL string
	HTTPClient *http.Client
}

// NewHealthClient creates a new health client with the given service URL
func NewHealthClient(serviceURL string) *HealthClient {
	return &HealthClient{
		ServiceURL: serviceURL,
		HTTPClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// ReportHealth reports a health status to the health service
func (c *HealthClient) ReportHealth(ctx context.Context, report health.HealthReport) error {
	url := fmt.Sprintf("%s/health", c.ServiceURL)
	
	// Marshal the report to JSON
	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("error marshaling health report: %w", err)
	}
	
	// Create a request with the given context
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	// Send the request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("error sending health report: %w", err)
	}
	defer resp.Body.Close()
	
	// Check for errors
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health service returned status code %d", resp.StatusCode)
	}
	
	return nil
}

// GetServiceHealth gets the health status of a specific service
func (c *HealthClient) GetServiceHealth(ctx context.Context, sourceType, sourceName string) (*health.HealthReport, error) {
	url := fmt.Sprintf("%s/health/%s/%s", c.ServiceURL, sourceType, sourceName)
	
	// Create a request with the given context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	
	// Send the request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error getting health status: %w", err)
	}
	defer resp.Body.Close()
	
	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("health service returned status code %d", resp.StatusCode)
	}
	
	// Decode the response
	var report health.HealthReport
	err = json.NewDecoder(resp.Body).Decode(&report)
	if err != nil {
		return nil, fmt.Errorf("error decoding health response: %w", err)
	}
	
	return &report, nil
}

// GetAllHealth gets the health status of all services
func (c *HealthClient) GetAllHealth(ctx context.Context) ([]health.HealthReport, error) {
	url := fmt.Sprintf("%s/health", c.ServiceURL)
	
	// Create a request with the given context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	
	// Send the request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error getting health statuses: %w", err)
	}
	defer resp.Body.Close()
	
	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("health service returned status code %d", resp.StatusCode)
	}
	
	// Decode the response
	var reports []health.HealthReport
	err = json.NewDecoder(resp.Body).Decode(&reports)
	if err != nil {
		return nil, fmt.Errorf("error decoding health response: %w", err)
	}
	
	return reports, nil
}

// GetSystemHealth gets the overall system health
func (c *HealthClient) GetSystemHealth(ctx context.Context) (*health.SystemHealth, error) {
	url := fmt.Sprintf("%s/health/system", c.ServiceURL)
	
	// Create a request with the given context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	
	// Send the request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error getting system health: %w", err)
	}
	defer resp.Body.Close()
	
	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("health service returned status code %d", resp.StatusCode)
	}
	
	// Decode the response
	var systemHealth health.SystemHealth
	err = json.NewDecoder(resp.Body).Decode(&systemHealth)
	if err != nil {
		return nil, fmt.Errorf("error decoding system health response: %w", err)
	}
	
	return &systemHealth, nil
}