package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
)

// GetDetailedSchedulerStatus provides detailed information about the scheduler
// and Redis streams for monitoring purposes
func (sm *ServiceManager) GetDetailedSchedulerStatus() error {
	// First check if scheduler is running
	status, err := sm.GetServiceStatus()
	if err != nil {
		return fmt.Errorf("failed to get service status: %w", err)
	}

	fmt.Println("=== Scheduler Status ===")
	fmt.Printf("Scheduler: %s\n", formatStatusText(status.Scheduler))
	
	// If scheduler is not running, we can't provide detailed info
	if !status.Scheduler {
		return nil
	}

	// Show scheduled jobs info
	fmt.Println("\nScheduled jobs:")
	fmt.Println("  Job Name          Next Run Time           Last Run")
	fmt.Println("  ----------------- ----------------------- -----------------------")
	
	// Get job schedule information from scheduler config
	configFile := filepath.Join(sm.config.QuotronRoot, "scheduler-config.json")
	data, err := os.ReadFile(configFile)
	if err != nil {
		fmt.Printf("Error reading scheduler config: %v\n", err)
	} else {
		var config map[string]interface{}
		if err := json.Unmarshal(data, &config); err != nil {
			fmt.Printf("Error parsing scheduler config: %v\n", err)
		} else if schedules, ok := config["schedules"].(map[string]interface{}); ok {
			for jobName, jobData := range schedules {
				if job, ok := jobData.(map[string]interface{}); ok {
					cronExpr, _ := job["cron"].(string)
					enabled, _ := job["enabled"].(bool)
					
					if enabled {
						// Create a cron parser to calculate next run
						parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
						schedule, err := parser.Parse(cronExpr)
						if err != nil {
							fmt.Printf("  %-17s %-23s %s\n", jobName, "Error parsing cron", err.Error())
							continue
						}
						nextRun := schedule.Next(time.Now())
						
						// Get heartbeat file for last run estimation
						heartbeatFile := filepath.Join(sm.config.QuotronRoot, "scheduler", "scheduler_heartbeat")
						lastRunTime := "Never"
						if heartbeatData, err := os.ReadFile(heartbeatFile); err == nil {
							lastRunTime = strings.TrimSpace(string(heartbeatData))
						}
						
						fmt.Printf("  %-17s %-23s %s\n", jobName, nextRun.Format("2006-01-02 15:04:05"), lastRunTime)
					} else {
						fmt.Printf("  %-17s %-23s %s\n", jobName, "Disabled", "N/A")
					}
				}
			}
		}
	}
	
	// Show Redis stream info
	fmt.Println("\nRedis Streams:")
	fmt.Println("  Stream Name               Messages  Last Entry Time")
	fmt.Println("  ------------------------- --------- -----------------------")
	
	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", sm.config.RedisHost, sm.config.RedisPort),
	})
	defer redisClient.Close()
	
	ctx := context.Background()
	streamsList := []string{
		"quotron:stocks:stream",
		"quotron:crypto:stream",
		"quotron:indices:stream",
	}
	
	for _, streamName := range streamsList {
		info, err := redisClient.XInfoStream(ctx, streamName).Result()
		if err != nil {
			fmt.Printf("  %-25s %-9s %s\n", streamName, "N/A", "Stream does not exist")
			continue
		}
		
		// Convert timestamp from milliseconds to time
		lastEntryTime := "N/A"
		if info.LastEntry.ID != "" {
			// Extract timestamp from ID (format: timestamp-sequence)
			parts := strings.Split(info.LastEntry.ID, "-")
			if len(parts) > 0 {
				if ms, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
					lastEntryTime = time.Unix(0, ms*int64(time.Millisecond)).Format("2006-01-02 15:04:05")
				}
			}
		}
		
		fmt.Printf("  %-25s %-9d %s\n", streamName, info.Length, lastEntryTime)
	}
	
	// Check consumer groups
	fmt.Println("\nRedis Consumer Groups:")
	fmt.Println("  Stream Name               Consumer Group      Pending")
	fmt.Println("  ------------------------- ------------------ ---------")
	
	for _, streamName := range streamsList {
		// First check if stream exists
		exists, err := redisClient.Exists(ctx, streamName).Result()
		if err != nil || exists == 0 {
			continue
		}
		
		// Get consumer groups
		groups, err := redisClient.XInfoGroups(ctx, streamName).Result()
		if err != nil {
			continue
		}
		
		for _, group := range groups {
			fmt.Printf("  %-25s %-18s %-9d\n", 
				streamName, group.Name, group.Pending)
		}
	}

	return nil
}

// formatStatusText returns a colored status text for the terminal
func formatStatusText(running bool) string {
	if running {
		return "\033[0;32m✔ Running\033[0m"
	}
	return "\033[0;31m✘ Not running\033[0m"
}