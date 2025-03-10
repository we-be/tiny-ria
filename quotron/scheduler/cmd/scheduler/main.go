package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tiny-ria/quotron/scheduler/internal/config"
	"github.com/tiny-ria/quotron/scheduler/pkg/scheduler"
)

func main() {
	// Parse command-line arguments
	configPath := flag.String("config", "", "Path to config file")
	apiScraperPath := flag.String("api-scraper", "", "Path to the API scraper binary")
	apiServiceHost := flag.String("api-host", "", "Hostname for the API service")
	apiServicePort := flag.Int("api-port", 0, "Port for the API service")
	useAPIService := flag.Bool("use-api-service", false, "Use API service instead of direct execution")
	genConfig := flag.Bool("gen-config", false, "Generate a default config file")
	runOnce := flag.Bool("run-once", false, "Run all jobs once and exit")
	runJob := flag.String("run-job", "", "Run a specific job once and exit")
	flag.Parse()

	// Generate default config if requested
	if *genConfig {
		defaultConfig := config.DefaultConfig()
		configJSON, err := os.Create("scheduler-config.json")
		if err != nil {
			log.Fatalf("Failed to create config file: %v", err)
		}
		defer configJSON.Close()

		err = json.NewEncoder(configJSON).Encode(defaultConfig)
		if err != nil {
			log.Fatalf("Failed to write config file: %v", err)
		}
		log.Println("Generated default config file: scheduler-config.json")
		return
	}

	// Load configuration
	var cfg *config.SchedulerConfig
	var err error
	if *configPath != "" {
		cfg, err = config.LoadConfig(*configPath)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
	} else {
		cfg = config.DefaultConfig()
		log.Println("Using default configuration")
	}

	// Use environment variable for API key if available
	if apiKey := os.Getenv("ALPHA_VANTAGE_API_KEY"); apiKey != "" {
		cfg.APIKey = apiKey
	}

	// Override API service settings from command line if provided
	if *apiServiceHost != "" {
		cfg.APIServiceHost = *apiServiceHost
	}
	if *apiServicePort != 0 {
		cfg.APIServicePort = *apiServicePort
	}
	if *useAPIService {
		cfg.UseAPIService = true
	}

	// Find API scraper binary if not using API service and path not specified
	if !cfg.UseAPIService {
		// Check API key only if not using API service
		if cfg.APIKey == "" {
			log.Fatalf("API key not provided. Set it in the config file or ALPHA_VANTAGE_API_KEY environment variable")
		}
		
		// Find API scraper binary if not specified
		if *apiScraperPath == "" {
			// Check common locations
			candidates := []string{
				"../api-scraper/api-scraper",
				"../../api-scraper/api-scraper",
				"/usr/local/bin/api-scraper",
			}

			for _, candidate := range candidates {
				absPath, err := filepath.Abs(candidate)
				if err == nil {
					if _, err := os.Stat(absPath); err == nil {
						*apiScraperPath = absPath
						log.Printf("Found API scraper at %s", *apiScraperPath)
						break
					}
				}
			}
		}

		if *apiScraperPath == "" {
			log.Fatalf("API scraper binary not found. Specify it with -api-scraper or use -use-api-service flag")
		}
		
		cfg.ApiScraper = *apiScraperPath
	} else {
		log.Printf("Using API service at %s:%d", cfg.APIServiceHost, cfg.APIServicePort)
	}

	// Create the scheduler
	s := scheduler.NewScheduler(cfg)

	// Register jobs
	if err := s.RegisterDefaultJobs(cfg.ToConfig()); err != nil {
		log.Fatalf("Failed to register jobs: %v", err)
	}

	// Handle run-once mode
	if *runOnce {
		for _, job := range s.ListJobs() {
			log.Printf("Running job '%s' once", job.Name())
			if err := s.RunJobNow(job.Name()); err != nil {
				log.Printf("Failed to run job '%s': %v", job.Name(), err)
			}
		}
		// Wait for all jobs to complete (simple approach)
		time.Sleep(5 * time.Second)
		return
	}

	// Handle run specific job
	if *runJob != "" {
		log.Printf("Running job '%s' once", *runJob)
		if err := s.RunJobNow(*runJob); err != nil {
			log.Fatalf("Failed to run job '%s': %v", *runJob, err)
		}
		// Wait for job to complete
		time.Sleep(5 * time.Second)
		return
	}

	// Start the scheduler
	if err := s.Start(); err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	// Print job schedule information
	log.Println("Scheduler started with the following jobs:")
	for _, job := range s.ListJobs() {
		nextRun, err := s.GetNextRun(job.Name())
		if err != nil {
			log.Printf("- %s: Not scheduled", job.Name())
		} else {
			log.Printf("- %s: Next run at %s", job.Name(), nextRun.Format(time.RFC3339))
		}
	}
	
	// Print mode information
	if cfg.UseAPIService {
		log.Printf("Using API service mode with host %s:%d", cfg.APIServiceHost, cfg.APIServicePort)
	} else {
		log.Printf("Using legacy mode with API scraper at %s", cfg.ApiScraper)
	}

	// Wait for termination signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	// Stop the scheduler
	log.Println("Shutting down scheduler...")
	s.Stop()
	log.Println("Scheduler stopped")
}