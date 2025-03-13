package services

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/we-be/tiny-ria/quotron/health"
	"github.com/we-be/tiny-ria/quotron/health/client"
)

// HealthCommand implements the health check command
type HealthCommand struct {
	flags       *flag.FlagSet
	serviceURL  string
	action      string
	format      string
	serviceType string
	serviceName string
}

// NewHealthCommand creates a new health check command
func NewHealthCommand() *HealthCommand {
	hc := &HealthCommand{
		flags: flag.NewFlagSet("health", flag.ExitOnError),
	}
	
	// Define flags
	hc.flags.StringVar(&hc.serviceURL, "service-url", "http://localhost:8085", "Health service URL")
	hc.flags.StringVar(&hc.action, "action", "all", "Action to perform: all, get, system, or specific service")
	hc.flags.StringVar(&hc.format, "format", "text", "Output format: text or json")
	
	return hc
}

// Name returns the name of the command
func (hc *HealthCommand) Name() string {
	return "health"
}

// Description returns a description of the command
func (hc *HealthCommand) Description() string {
	return "Check health of services"
}

// Run executes the health check command
func (hc *HealthCommand) Run(args []string) error {
	// Parse flags
	if err := hc.flags.Parse(args); err != nil {
		return err
	}
	
	// Get remaining arguments
	remainingArgs := hc.flags.Args()
	if len(remainingArgs) > 0 {
		// If first arg is a known action, use it
		switch remainingArgs[0] {
		case "all", "system":
			hc.action = remainingArgs[0]
			remainingArgs = remainingArgs[1:]
		case "service":
			hc.action = "service"
			remainingArgs = remainingArgs[1:]
			if len(remainingArgs) >= 2 {
				hc.serviceType = remainingArgs[0]
				hc.serviceName = remainingArgs[1]
			}
		default:
			// Check if it's a service name with type/name format
			parts := strings.Split(remainingArgs[0], "/")
			if len(parts) == 2 {
				hc.action = "service"
				hc.serviceType = parts[0]
				hc.serviceName = parts[1]
			}
		}
	}
	
	// Create health client
	healthClient := client.NewHealthClient(hc.serviceURL)
	
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Perform the specified action
	switch hc.action {
	case "all":
		return hc.showAllHealth(ctx, healthClient)
	case "system":
		return hc.showSystemHealth(ctx, healthClient)
	case "service":
		if hc.serviceType == "" || hc.serviceName == "" {
			return fmt.Errorf("service action requires type and name arguments")
		}
		return hc.showServiceHealth(ctx, healthClient, hc.serviceType, hc.serviceName)
	default:
		return fmt.Errorf("unknown action: %s", hc.action)
	}
}

func (hc *HealthCommand) showAllHealth(ctx context.Context, healthClient *client.HealthClient) error {
	// Get all service health statuses
	reports, err := healthClient.GetAllHealth(ctx)
	if err != nil {
		return err
	}
	
	if hc.format == "json" {
		// JSON output
		outputJSON(reports)
		return nil
	}
	
	// Text output
	fmt.Println("=== Health Status for All Services ===")
	fmt.Printf("Found %d services\n\n", len(reports))
	
	for i, report := range reports {
		fmt.Printf("%d. %s/%s: %s\n", i+1, report.SourceType, report.SourceName, report.Status)
		if report.LastCheck.IsZero() {
			fmt.Println("   Last Check: Never")
		} else {
			fmt.Printf("   Last Check: %s\n", report.LastCheck.Format(time.RFC3339))
		}
		if report.ErrorCount > 0 {
			fmt.Printf("   Error Count: %d\n", report.ErrorCount)
			if report.ErrorMessage != "" {
				fmt.Printf("   Last Error: %s\n", report.ErrorMessage)
			}
		}
		fmt.Println()
	}
	
	return nil
}

func (hc *HealthCommand) showSystemHealth(ctx context.Context, healthClient *client.HealthClient) error {
	// Get system health
	systemHealth, err := healthClient.GetSystemHealth(ctx)
	if err != nil {
		return err
	}
	
	if hc.format == "json" {
		// JSON output
		outputJSON(systemHealth)
		return nil
	}
	
	// Text output
	fmt.Println("=== System Health ===")
	fmt.Printf("Health Score: %.2f%%\n", systemHealth.HealthScore)
	fmt.Printf("Total Services: %d\n", systemHealth.TotalServices)
	fmt.Printf("Healthy: %d\n", systemHealth.HealthyCount)
	fmt.Printf("Degraded: %d\n", systemHealth.DegradedCount)
	fmt.Printf("Failed: %d\n", systemHealth.FailedCount)
	fmt.Printf("Last Check: %s\n", systemHealth.LastCheck.Format(time.RFC3339))
	
	return nil
}

func (hc *HealthCommand) showServiceHealth(ctx context.Context, healthClient *client.HealthClient, sourceType, sourceName string) error {
	// Get service health
	report, err := healthClient.GetServiceHealth(ctx, sourceType, sourceName)
	if err != nil {
		return err
	}
	
	if hc.format == "json" {
		// JSON output
		outputJSON(report)
		return nil
	}
	
	// Text output
	fmt.Printf("=== Health Status for %s/%s ===\n", sourceType, sourceName)
	fmt.Printf("Status: %s\n", report.Status)
	if report.LastCheck.IsZero() {
		fmt.Println("Last Check: Never")
	} else {
		fmt.Printf("Last Check: %s\n", report.LastCheck.Format(time.RFC3339))
	}
	if report.LastSuccess != nil && !report.LastSuccess.IsZero() {
		fmt.Printf("Last Success: %s\n", report.LastSuccess.Format(time.RFC3339))
	}
	if report.ErrorCount > 0 {
		fmt.Printf("Error Count: %d\n", report.ErrorCount)
		if report.ErrorMessage != "" {
			fmt.Printf("Last Error: %s\n", report.ErrorMessage)
		}
	}
	if report.ResponseTimeMs > 0 {
		fmt.Printf("Response Time: %dms\n", report.ResponseTimeMs)
	}
	if report.SourceDetail != "" {
		fmt.Printf("Description: %s\n", report.SourceDetail)
	}
	if len(report.Metadata) > 0 {
		fmt.Println("\nMetadata:")
		for key, value := range report.Metadata {
			fmt.Printf("  %s: %v\n", key, value)
		}
	}
	
	return nil
}

// Helper for JSON output
func outputJSON(data interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}