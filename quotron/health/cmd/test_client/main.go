package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/we-be/tiny-ria/quotron/health"
	"github.com/we-be/tiny-ria/quotron/health/client"
)

func main() {
	// Parse command line flags
	serviceURL := flag.String("url", "http://localhost:8085", "Health service URL")
	sourceType := flag.String("type", "test-client", "Source type")
	sourceName := flag.String("name", "go-client", "Source name")
	action := flag.String("action", "report", "Action to perform (report, get, all, system)")
	status := flag.String("status", "healthy", "Health status (for report action)")
	flag.Parse()

	// Create a client
	healthClient := client.NewHealthClient(*serviceURL)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Perform the requested action
	switch *action {
	case "report":
		// Create a health report
		report := health.HealthReport{
			SourceType:     *sourceType,
			SourceName:     *sourceName,
			SourceDetail:   "Test client for health monitoring",
			Status:         health.Status(*status),
			LastCheck:      time.Now(),
			ResponseTimeMs: 100,
			Metadata: map[string]interface{}{
				"test":      true,
				"timestamp": time.Now().Unix(),
			},
		}

		// Report health
		err := healthClient.ReportHealth(ctx, report)
		if err != nil {
			log.Fatalf("Error reporting health: %v", err)
		}
		fmt.Println("Health reported successfully")

	case "get":
		// Get health for a specific service
		report, err := healthClient.GetServiceHealth(ctx, *sourceType, *sourceName)
		if err != nil {
			log.Fatalf("Error getting health: %v", err)
		}
		fmt.Printf("Health for %s/%s: %s\n", *sourceType, *sourceName, report.Status)
		fmt.Printf("Last check: %s\n", report.LastCheck.Format(time.RFC3339))
		if report.LastSuccess != nil {
			fmt.Printf("Last success: %s\n", report.LastSuccess.Format(time.RFC3339))
		}
		fmt.Printf("Response time: %d ms\n", report.ResponseTimeMs)
		fmt.Printf("Error count: %d\n", report.ErrorCount)
		if report.ErrorMessage != "" {
			fmt.Printf("Error message: %s\n", report.ErrorMessage)
		}
		fmt.Println("Metadata:", report.Metadata)

	case "all":
		// Get all health statuses
		reports, err := healthClient.GetAllHealth(ctx)
		if err != nil {
			log.Fatalf("Error getting all health statuses: %v", err)
		}
		fmt.Printf("Found %d health reports:\n", len(reports))
		for _, report := range reports {
			fmt.Printf("- %s/%s: %s\n", report.SourceType, report.SourceName, report.Status)
		}

	case "system":
		// Get system health
		systemHealth, err := healthClient.GetSystemHealth(ctx)
		if err != nil {
			log.Fatalf("Error getting system health: %v", err)
		}
		fmt.Printf("System health score: %.2f%%\n", systemHealth.HealthScore)
		fmt.Printf("Services: %d total, %d healthy, %d degraded, %d failed\n",
			systemHealth.TotalServices,
			systemHealth.HealthyCount,
			systemHealth.DegradedCount,
			systemHealth.FailedCount,
		)

	default:
		fmt.Printf("Unknown action: %s\n", *action)
	}
}