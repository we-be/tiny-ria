package main

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

// Example implementation of a health check CLI command
// This has been integrated into the main Quotron CLI

func main() {
	// Parse command line arguments
	serviceURL := flag.String("service-url", "http://localhost:8085", "Health service URL")
	action := flag.String("action", "all", "Action to perform: all, get, system, or service <type> <name>")
	flag.Parse()

	// Get remaining arguments
	args := flag.Args()

	// Create health client
	healthClient := client.NewHealthClient(*serviceURL)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Perform the specified action
	switch *action {
	case "all":
		// Get all service health statuses
		reports, err := healthClient.GetAllHealth(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting health statuses: %v\n", err)
			os.Exit(1)
		}

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

	case "system":
		// Get system health
		systemHealth, err := healthClient.GetSystemHealth(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting system health: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("=== System Health ===")
		fmt.Printf("Health Score: %.2f%%\n", systemHealth.HealthScore)
		fmt.Printf("Total Services: %d\n", systemHealth.TotalServices)
		fmt.Printf("Healthy: %d\n", systemHealth.HealthyCount)
		fmt.Printf("Degraded: %d\n", systemHealth.DegradedCount)
		fmt.Printf("Failed: %d\n", systemHealth.FailedCount)
		fmt.Printf("Last Check: %s\n", systemHealth.LastCheck.Format(time.RFC3339))

	case "service":
		// Check if we have enough arguments
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Error: service action requires type and name arguments\n")
			os.Exit(1)
		}

		sourceType := args[0]
		sourceName := args[1]

		// Get service health
		report, err := healthClient.GetServiceHealth(ctx, sourceType, sourceName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting service health: %v\n", err)
			os.Exit(1)
		}

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

	default:
		// Check if it's a service name
		parts := strings.Split(*action, "/")
		if len(parts) == 2 {
			sourceType := parts[0]
			sourceName := parts[1]

			// Get service health
			report, err := healthClient.GetServiceHealth(ctx, sourceType, sourceName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting service health: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("=== Health Status for %s/%s ===\n", sourceType, sourceName)
			fmt.Printf("Status: %s\n", string(report.Status))
		} else {
			fmt.Fprintf(os.Stderr, "Unknown action: %s\n", *action)
			os.Exit(1)
		}
	}
}