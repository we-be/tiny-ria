package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// TestManager handles running tests for services
type TestManager struct {
	config *Config
}

// NewTestManager creates a new TestManager
func NewTestManager(config *Config) *TestManager {
	return &TestManager{
		config: config,
	}
}

// RunAllTests runs all test suites
func (tm *TestManager) RunAllTests(ctx context.Context) error {
	// Start services needed for tests
	serviceManager := NewServiceManager(tm.config)

	// Check if YFinance proxy is running
	status, _ := serviceManager.GetServiceStatus()
	if !status.YFinanceProxy {
		fmt.Println("YFinance Proxy is not running. Starting it...")
		err := serviceManager.StartServices(ctx, ServiceList{YFinanceProxy: true}, false)
		if err != nil {
			return fmt.Errorf("failed to start YFinance Proxy: %w", err)
		}
	}

	// Check if API service is running
	if !status.APIService {
		fmt.Println("API Service is not running. Starting it...")
		err := serviceManager.StartServices(ctx, ServiceList{APIService: true}, false)
		if err != nil {
			return fmt.Errorf("failed to start API Service: %w", err)
		}
	}

	// Run API service tests
	fmt.Println("Running API service tests...")
	err := tm.TestAPIService(ctx)
	if err != nil {
		return fmt.Errorf("API service tests failed: %w", err)
	}
	fmt.Println("API service tests passed!")

	// Test database connectivity if PostgreSQL is installed
	fmt.Println("Testing database connectivity...")
	err = tm.TestDatabaseConnectivity(ctx)
	if err != nil {
		fmt.Printf("Warning: Database connectivity test failed: %v\n", err)
		fmt.Println("This is expected if you haven't set up the database yet.")
	} else {
		fmt.Println("Database connectivity test passed!")
	}

	// Run integration tests
	fmt.Println("Running integration tests...")
	err = tm.RunIntegrationTests(ctx)
	if err != nil {
		return fmt.Errorf("integration tests failed: %w", err)
	}
	fmt.Println("Integration tests passed!")

	// Run scheduler job tests
	fmt.Println("Running scheduler job tests...")
	err = tm.RunSchedulerJob(ctx, "stock_quotes")
	if err != nil {
		return fmt.Errorf("scheduler job test failed: %w", err)
	}
	fmt.Println("Scheduler job tests passed!")

	return nil
}

// TestAPIService tests the API service functionality
func (tm *TestManager) TestAPIService(ctx context.Context) error {
	// Check if API service is running
	serviceManager := NewServiceManager(tm.config)
	status, _ := serviceManager.GetServiceStatus()
	if !status.APIService {
		return fmt.Errorf("API service is not running")
	}

	// Check health endpoint
	fmt.Println("Checking API service health...")
	healthURL := fmt.Sprintf("http://%s:%d/api/health", tm.config.APIHost, tm.config.APIPort)
	resp, err := http.Get(healthURL)
	if err != nil {
		return fmt.Errorf("failed to connect to API service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API service health check failed with status: %d", resp.StatusCode)
	}

	// Test stock quote endpoint
	fmt.Println("Testing stock quote endpoint...")
	quoteURL := fmt.Sprintf("http://%s:%d/api/quote/AAPL", tm.config.APIHost, tm.config.APIPort)
	resp, err = http.Get(quoteURL)
	if err != nil {
		return fmt.Errorf("failed to connect to quote endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("quote endpoint failed with status: %d", resp.StatusCode)
	}

	// Check response is valid JSON with expected fields
	quoteData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var quoteResponse map[string]interface{}
	err = json.Unmarshal(quoteData, &quoteResponse)
	if err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Check for expected fields
	if _, ok := quoteResponse["symbol"]; !ok {
		return fmt.Errorf("quote response missing symbol field")
	}

	// Test market index endpoint
	fmt.Println("Testing market index endpoint...")
	indexURL := fmt.Sprintf("http://%s:%d/api/index/SPY", tm.config.APIHost, tm.config.APIPort)
	resp, err = http.Get(indexURL)
	if err != nil {
		return fmt.Errorf("failed to connect to index endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("index endpoint failed with status: %d", resp.StatusCode)
	}

	// Check response is valid JSON with expected fields
	indexData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var indexResponse map[string]interface{}
	err = json.Unmarshal(indexData, &indexResponse)
	if err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Check for expected fields
	if _, ok := indexResponse["symbol"]; !ok {
		return fmt.Errorf("index response missing symbol field")
	}

	return nil
}

// TestDatabaseConnectivity tests the database connectivity
func (tm *TestManager) TestDatabaseConnectivity(ctx context.Context) error {
	// Check if PostgreSQL client is installed
	psqlCmd := exec.CommandContext(ctx, "which", "psql")
	if err := psqlCmd.Run(); err != nil {
		return fmt.Errorf("PostgreSQL client not found")
	}

	// Try to connect to the database
	cmd := exec.CommandContext(ctx, "psql",
		"-h", tm.config.DBHost,
		"-p", fmt.Sprintf("%d", tm.config.DBPort),
		"-U", tm.config.DBUser,
		"-d", tm.config.DBName,
		"-c", "\\dt")

	// Set environment variable for PostgreSQL password
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", tm.config.DBPassword))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("database connection failed: %w, output: %s", err, output)
	}

	// Check if stock_quotes table exists
	if !strings.Contains(string(output), "stock_quotes") {
		return fmt.Errorf("stock_quotes table not found in database")
	}

	return nil
}

// RunIntegrationTests runs integration tests
func (tm *TestManager) RunIntegrationTests(ctx context.Context) error {
	// Check if YFinance proxy is running
	serviceManager := NewServiceManager(tm.config)
	status, _ := serviceManager.GetServiceStatus()
	if !status.YFinanceProxy {
		return fmt.Errorf("YFinance proxy is not running")
	}

	// Run Yahoo Finance integration test
	testsDir := filepath.Join(tm.config.QuotronRoot, "tests")
	testScript := filepath.Join(testsDir, "yahoo_finance_test.py")

	// Check if test script exists
	if _, err := os.Stat(testScript); err != nil {
		return fmt.Errorf("test script not found: %s", testScript)
	}

	// Run the test
	fmt.Println("Running Yahoo Finance integration test...")
	cmd := exec.CommandContext(ctx, "python", "yahoo_finance_test.py")
	cmd.Dir = testsDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Yahoo Finance integration test failed: %w, output: %s", err, output)
	}

	fmt.Println("Yahoo Finance integration test passed!")

	// Run scheduler integration test if API service is running
	if status.APIService {
		fmt.Println("Running scheduler integration test with API service...")
		err = tm.RunSchedulerJob(ctx, "stock_quotes")
		if err != nil {
			return fmt.Errorf("scheduler integration test failed: %w", err)
		}
		fmt.Println("Scheduler integration test passed!")
	}

	return nil
}

// RunSchedulerJob runs a scheduler job
func (tm *TestManager) RunSchedulerJob(ctx context.Context, jobName string) error {
	// Validate job name
	if jobName != "stock_quotes" && jobName != "market_indices" {
		return fmt.Errorf("invalid job name: %s (valid options: stock_quotes, market_indices)", jobName)
	}

	// Check if API service is running
	serviceManager := NewServiceManager(tm.config)
	status, _ := serviceManager.GetServiceStatus()
	useAPIService := status.APIService

	// Prepare command
	schedulerDir := tm.config.SchedulerPath
	args := []string{
		"run", "cmd/scheduler/main.go",
	}

	// Add config file if available
	configFile := filepath.Join(tm.config.QuotronRoot, "scheduler-config.json")
	if _, err := os.Stat(configFile); err == nil {
		args = append(args, "--config", configFile)
	}

	// Set API service mode if available
	if useAPIService {
		fmt.Println("Using API service mode for scheduler job")
		args = append(args, "--use-api-service",
			"--api-host", tm.config.APIHost,
			"--api-port", fmt.Sprintf("%d", tm.config.APIPort))
	} else {
		// Use API scraper directly
		args = append(args, "--api-scraper", tm.config.APIScraperPath)
	}

	// Add job name
	args = append(args, "--run-job", jobName)

	// Set API key if available
	if tm.config.AlphaVantageAPIKey != "" {
		os.Setenv("ALPHA_VANTAGE_API_KEY", tm.config.AlphaVantageAPIKey)
	}

	// Run the command
	fmt.Printf("Running scheduler job: %s\n", jobName)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = schedulerDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("job %s failed: %w, output: %s", jobName, err, output)
	}

	fmt.Printf("Job %s completed successfully!\n", jobName)
	return nil
}