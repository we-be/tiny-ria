package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/we-be/tiny-ria/quotron/cli/pkg/services"
)

var (
	// Global configuration
	config = services.DefaultConfig()

	// Command flags
	logLevel    = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	configFile  = flag.String("config", "", "Path to config file")
	force       = flag.Bool("force", false, "Force operations even if conflicts are detected")
	genConfig   = flag.Bool("gen-config", false, "Generate default config file")
	monitorMode = flag.Bool("monitor", false, "Monitor mode - watch services and restart if they fail")
)

// getAvailableCommands returns all available commands
func getAvailableCommands(ctx context.Context) map[string]services.Command {
	commands := make(map[string]services.Command)
	
	// Register built-in commands
	commands["health"] = services.NewHealthCommand()
	
	return commands
}

func main() {
	// Define command-line structure
	flag.Usage = usage
	flag.Parse()

	// Load configuration from file if specified
	if *configFile != "" {
		err := config.LoadFromFile(*configFile)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
	}

	// Generate default config if requested
	if *genConfig {
		err := config.SaveToFile("quotron.json")
		if err != nil {
			log.Fatalf("Failed to generate config file: %v", err)
		}
		fmt.Println("Generated default config file: quotron.json")
		return
	}

	// Initialize context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handler for graceful shutdown
	setupSignalHandler(cancel)

	// Process commands
	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(1)
	}

	// Get available commands
	commands := getAvailableCommands(ctx)

	// Parse command and execute
	command := args[0]
	commandArgs := args[1:]

	// Check for registered commands first
	if cmd, exists := commands[command]; exists {
		if err := cmd.Run(commandArgs); err != nil {
			log.Fatalf("Command failed: %v", err)
		}
		return
	}

	// Handle built-in commands
	switch command {
	case "start":
		handleStartCommand(ctx, commandArgs)
	case "stop":
		handleStopCommand(commandArgs)
	case "status":
		handleStatusCommand()
	case "test":
		handleTestCommand(ctx, commandArgs)
	case "import-sp500":
		handleImportSP500Command(ctx)
	case "scheduler":
		handleSchedulerCommand(ctx, commandArgs)
	case "help":
		usage()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		usage()
		os.Exit(1)
	}
}

// usage prints the help message
func usage() {
	fmt.Println("Quotron - Financial data system CLI")
	fmt.Println()
	fmt.Println("Usage: quotron [OPTIONS] COMMAND [ARGS]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --config FILE      Path to config file")
	fmt.Println("  --log-level LEVEL  Set log level (debug, info, warn, error)")
	fmt.Println("  --force            Force operations even if conflicts detected")
	fmt.Println("  --gen-config       Generate default config file")
	fmt.Println("  --monitor          Monitor services and restart if they fail")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  start [SERVICE...]  Start services (all or specified services)")
	fmt.Println("  stop [SERVICE...]   Stop services (all or specified services)")
	fmt.Println("  status              Show status of all services")
	fmt.Println("  test [TEST]         Run tests (all or specified test)")
	fmt.Println("  import-sp500        Import S&P 500 data")
	fmt.Println("  scheduler <ACTION>  Manage or interact with the scheduler")
	fmt.Println("  health              Check health of services")
	fmt.Println("  help                Show this help message")
	fmt.Println()
	fmt.Println("Services:")
	fmt.Println("  all                 All services (default)")
	fmt.Println("  proxy               YFinance proxy only")
	fmt.Println("  api                 API service only")
	fmt.Println("  dashboard           Dashboard only")
	fmt.Println("  scheduler           Scheduler only")
	fmt.Println()
	fmt.Println("Tests:")
	fmt.Println("  all                 All tests (default)")
	fmt.Println("  api                 API service tests")
	fmt.Println("  integration         Integration tests")
	fmt.Println("  job JOBNAME         Run a specific job test")
}

// setupSignalHandler sets up a handler for SIGINT and SIGTERM
func setupSignalHandler(cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		fmt.Printf("\nReceived signal: %s\n", sig)
		fmt.Println("Shutting down gracefully...")
		cancel()
		
		// Give processes a chance to clean up
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}()
}

// handleStartCommand processes the 'start' command
func handleStartCommand(ctx context.Context, args []string) {
	// Find which services to start
	serviceList := services.ServiceList{
		YFinanceProxy: false,
		APIService:    false,
		Scheduler:     false,
		Dashboard:     false,
	}

	if len(args) == 0 || contains(args, "all") {
		// Start all services
		serviceList = services.ServiceList{
			YFinanceProxy: true,
			APIService:    true,
			Scheduler:     true,
			Dashboard:     true,
		}
	} else {
		// Start selected services
		for _, arg := range args {
			switch arg {
			case "proxy":
				serviceList.YFinanceProxy = true
			case "api":
				serviceList.APIService = true
			case "scheduler":
				serviceList.Scheduler = true
			case "dashboard":
				serviceList.Dashboard = true
			default:
				log.Printf("Unknown service: %s", arg)
			}
		}
	}

	// Create service manager and start services
	manager := services.NewServiceManager(config)
	err := manager.StartServices(ctx, serviceList, *monitorMode)
	if err != nil {
		log.Fatalf("Failed to start services: %v", err)
	}

	if *monitorMode {
		fmt.Println("Started in monitor mode. Press Ctrl+C to stop.")
		// Keep running until context is cancelled
		<-ctx.Done()
	}
}

// handleStopCommand processes the 'stop' command
func handleStopCommand(args []string) {
	// Find which services to stop
	serviceList := services.ServiceList{
		YFinanceProxy: false,
		APIService:    false,
		Scheduler:     false,
		Dashboard:     false,
	}

	if len(args) == 0 || contains(args, "all") {
		// Stop all services
		serviceList = services.ServiceList{
			YFinanceProxy: true,
			APIService:    true,
			Scheduler:     true,
			Dashboard:     true,
		}
	} else {
		// Stop selected services
		for _, arg := range args {
			switch arg {
			case "proxy":
				serviceList.YFinanceProxy = true
			case "api":
				serviceList.APIService = true
			case "scheduler":
				serviceList.Scheduler = true
			case "dashboard":
				serviceList.Dashboard = true
			default:
				log.Printf("Unknown service: %s", arg)
			}
		}
	}

	// Create service manager and stop services
	manager := services.NewServiceManager(config)
	err := manager.StopServices(serviceList)
	if err != nil {
		log.Fatalf("Failed to stop services: %v", err)
	}
}

// handleStatusCommand processes the 'status' command
func handleStatusCommand() {
	// Create service manager and check status
	manager := services.NewServiceManager(config)
	status, err := manager.GetServiceStatus()
	if err != nil {
		log.Fatalf("Failed to get service status: %v", err)
	}

	// Display status
	fmt.Println("=== Quotron Services Status ===")
	fmt.Printf("YFinance Proxy: %s\n", formatStatus(status.YFinanceProxy))
	fmt.Printf("API Service: %s\n", formatStatus(status.APIService))
	fmt.Printf("Scheduler: %s\n", formatStatus(status.Scheduler))
	fmt.Printf("Dashboard: %s\n", formatStatus(status.Dashboard))
}

// handleTestCommand processes the 'test' command
func handleTestCommand(ctx context.Context, args []string) {
	testManager := services.NewTestManager(config)

	if len(args) == 0 || contains(args, "all") {
		// Run all tests
		err := testManager.RunAllTests(ctx)
		if err != nil {
			log.Fatalf("Tests failed: %v", err)
		}
		fmt.Println("All tests passed!")
		return
	}

	// Run specific tests
	testType := args[0]
	switch testType {
	case "api":
		err := testManager.TestAPIService(ctx)
		if err != nil {
			log.Fatalf("API service tests failed: %v", err)
		}
		fmt.Println("API service tests passed!")
	case "integration":
		err := testManager.RunIntegrationTests(ctx)
		if err != nil {
			log.Fatalf("Integration tests failed: %v", err)
		}
		fmt.Println("Integration tests passed!")
	case "job":
		if len(args) < 2 {
			log.Fatalf("Job name required for job tests")
		}
		jobName := args[1]
		err := testManager.RunSchedulerJob(ctx, jobName)
		if err != nil {
			log.Fatalf("Job test failed: %v", err)
		}
		fmt.Printf("Job %s completed successfully!\n", jobName)
	default:
		log.Fatalf("Unknown test type: %s", testType)
	}
}

// handleImportSP500Command processes the 'import-sp500' command
func handleImportSP500Command(ctx context.Context) {
	importer := services.NewDataImporter(config)
	err := importer.ImportSP500Data(ctx)
	if err != nil {
		log.Fatalf("Failed to import S&P 500 data: %v", err)
	}
	fmt.Println("S&P 500 data imported successfully!")
}

// handleSchedulerCommand processes the 'scheduler' command which allows interaction with the running scheduler
func handleSchedulerCommand(ctx context.Context, args []string) {
	if len(args) == 0 {
		fmt.Println("Missing scheduler sub-command. Available commands: jobs, run-job, status")
		return
	}

	manager := services.NewServiceManager(config)
	subCommand := args[0]

	switch subCommand {
	case "jobs":
		// List all jobs in the scheduler
		jobs, err := manager.ListSchedulerJobs()
		if err != nil {
			log.Fatalf("Failed to list scheduler jobs: %v", err)
		}

		fmt.Println("=== Scheduler Jobs ===")
		for _, job := range jobs {
			enabled, _ := job["enabled"].(bool)
			cron, _ := job["cron"].(string)
			desc, _ := job["description"].(string)
			
			enabledStatus := "⚪ disabled"
			if enabled {
				enabledStatus = "🟢 enabled"
			}
			
			fmt.Printf("%s [%s] %s - %s\n", job["name"], enabledStatus, cron, desc)
		}

	case "next-runs":
		// Show when jobs will run next
		nextRuns, err := manager.GetSchedulerNextRuns()
		if err != nil {
			log.Fatalf("Failed to get next run times: %v", err)
		}

		fmt.Println("=== Scheduler Next Run Times ===")
		if len(nextRuns) == 0 {
			fmt.Println("No scheduled jobs found or scheduler just started")
		} else {
			for job, nextRun := range nextRuns {
				fmt.Printf("%s: %s\n", job, nextRun)
			}
		}

	case "run-job":
		// Run a specific job immediately
		if len(args) < 2 {
			log.Fatalf("Job name required")
		}
		jobName := args[1]
		
		err := manager.RunSchedulerJob(jobName)
		if err != nil {
			log.Fatalf("Failed to run job '%s': %v", jobName, err)
		}

	case "status":
		// Show scheduler status
		status, err := manager.GetServiceStatus()
		if err != nil {
			log.Fatalf("Failed to get service status: %v", err)
		}

		fmt.Println("=== Scheduler Status ===")
		fmt.Printf("Scheduler: %s\n", formatStatus(status.Scheduler))
		
		// If scheduler is running, get job information
		if status.Scheduler {
			// Get the number of jobs
			jobs, err := manager.ListSchedulerJobs()
			if err == nil {
				fmt.Printf("Jobs configured: %d\n", len(jobs))
			}
			
			// Get next run times
			nextRuns, err := manager.GetSchedulerNextRuns()
			if err == nil && len(nextRuns) > 0 {
				fmt.Println("\nNext scheduled runs:")
				for job, nextRun := range nextRuns {
					fmt.Printf("- %s: %s\n", job, nextRun)
				}
			}
		}

	default:
		fmt.Printf("Unknown scheduler sub-command: %s\n", subCommand)
		fmt.Println("Available commands: jobs, run-job, next-runs, status")
	}
}

// formatStatus formats a service status as a colored string
func formatStatus(running bool) string {
	if running {
		return "\033[0;32m✔ Running\033[0m"
	}
	return "\033[0;31m✘ Not running\033[0m"
}

// contains checks if a string slice contains a string
func contains(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}