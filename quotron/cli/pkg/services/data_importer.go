package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DataImporter handles data import operations
type DataImporter struct {
	config *Config
}

// NewDataImporter creates a new DataImporter
func NewDataImporter(config *Config) *DataImporter {
	return &DataImporter{
		config: config,
	}
}

// ImportSP500Data imports S&P 500 data from the browser scraper
func (di *DataImporter) ImportSP500Data(ctx context.Context) error {
	fmt.Println("=== Starting S&P 500 Model Import Process ===")

	// Configure paths
	pythonScraperDir := di.config.PythonScraperDir
	etlDir := di.config.ETLDir
	outputDir := filepath.Join(di.config.QuotronRoot, "browser-scraper", "output")

	// Ensure output directory exists
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Step 1: Run the Python scraper to fetch S&P 500 data
	fmt.Println("Step 1: Scraping S&P 500 data from slickcharts.com...")

	// Check if Python scraper exists
	scraperScript := filepath.Join(pythonScraperDir, "sp500_scraper.py")
	if _, err := os.Stat(scraperScript); err != nil {
		return fmt.Errorf("S&P 500 scraper script not found at %s", scraperScript)
	}

	// Run the scraper
	cmd := exec.CommandContext(ctx, "python3", scraperScript)
	cmd.Dir = pythonScraperDir
	scraperOutput, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to scrape S&P 500 data: %w, output: %s", err, scraperOutput)
	}

	// Get the output file path from the Python script output
	outputLines := strings.Split(string(scraperOutput), "\n")
	var jsonFile string
	for i := len(outputLines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(outputLines[i])
		if strings.HasSuffix(line, ".json") {
			jsonFile = line
			break
		}
	}

	if jsonFile == "" {
		return fmt.Errorf("output file path not found in scraper output")
	}

	fmt.Printf("Successfully scraped S&P 500 data to %s\n", jsonFile)

	// Step 2: Import the data into the database using the Go CLI
	fmt.Println("Step 2: Importing S&P 500 model into database...")

	// Build the model CLI tool if needed
	modelCliPath := filepath.Join(etlDir, "modelcli")
	if _, err := os.Stat(modelCliPath); err != nil || !isExecutable(modelCliPath) {
		fmt.Println("Building model CLI tool...")
		buildCmd := exec.CommandContext(ctx, "go", "build", "-o", "modelcli", "./cmd/modelcli/main.go")
		buildCmd.Dir = etlDir
		buildOutput, err := buildCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to build model CLI tool: %w, output: %s", err, buildOutput)
		}
		fmt.Println("Model CLI tool built successfully")
	}

	// Import the data
	importCmd := exec.CommandContext(ctx, "go", "run", "cmd/modelcli/main.go", "-import", "-file="+jsonFile)
	importCmd.Dir = etlDir
	importOutput, err := importCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to import S&P 500 model: %w, output: %s", err, importOutput)
	}

	fmt.Println("=== S&P 500 Model Import Process Completed Successfully ===")
	return nil
}