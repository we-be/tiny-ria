package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/we-be/tiny-ria/quotron/etl/internal/db"
	"github.com/we-be/tiny-ria/quotron/etl/internal/models"
	"github.com/we-be/tiny-ria/quotron/etl/internal/pipeline"
)

func main() {
	// Define command-line flags
	var (
		// Commands
		setupCmd      = flag.Bool("setup", false, "Set up the database schema")
		quotesCmd     = flag.Bool("quotes", false, "Process a file of stock quotes")
		indicesCmd    = flag.Bool("indices", false, "Process a file of market indices")
		mixedCmd      = flag.Bool("mixed", false, "Process a file containing both quotes and indices")
		realtimeCmd   = flag.Bool("realtime", false, "Process simulated real-time data")
		listCmd       = flag.Bool("list", false, "List the latest data")
		
		// File options
		filePath      = flag.String("file", "", "Path to the JSON file containing financial data")
		source        = flag.String("source", "manual", "Source of the data (api-scraper, browser-scraper, manual)")
		allowOldData  = flag.Bool("allow-old-data", false, "Allow processing of historical data")
		
		// Processing options
		concurrency   = flag.Int("concurrency", 4, "Number of concurrent workers")
		retries       = flag.Int("retries", 3, "Number of retry attempts for database operations")
		
		// Database options
		dbHost        = flag.String("db-host", "", "Database hostname")
		dbPort        = flag.Int("db-port", 0, "Database port")
		dbName        = flag.String("db-name", "", "Database name")
		dbUser        = flag.String("db-user", "", "Database username")
		dbPass        = flag.String("db-pass", "", "Database password")
		
		// List options
		limit         = flag.Int("limit", 10, "Number of items to list")
		symbols       = flag.String("symbols", "", "Comma-separated list of symbols to filter")
		indexList     = flag.String("index-list", "", "Comma-separated list of indices to filter")
		
		// Realtime options
		duration      = flag.Int("duration", 60, "Duration in seconds to run the real-time processing")
	)

	// Parse command-line flags
	flag.Parse()

	// Create database configuration
	dbConfig := db.DefaultConfig()

	// Override with command-line options if provided
	if *dbHost != "" {
		dbConfig.Host = *dbHost
	}
	if *dbPort != 0 {
		dbConfig.Port = *dbPort
	}
	if *dbName != "" {
		dbConfig.DBName = *dbName
	}
	if *dbUser != "" {
		dbConfig.User = *dbUser
	}
	if *dbPass != "" {
		dbConfig.Password = *dbPass
	}

	// Create database connection
	database, err := db.NewDatabase(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Create pipeline options
	pipelineOpts := pipeline.DefaultPipelineOptions()
	pipelineOpts.AllowHistoricalData = *allowOldData
	pipelineOpts.ConcurrentBatches = *concurrency
	pipelineOpts.MaxRetries = *retries

	// Create pipeline
	ctx := context.Background()
	p := pipeline.NewPipeline(database, pipelineOpts)

	// Handle commands
	if *setupCmd {
		fmt.Println("Setting up database schema...")
		err := setupDatabaseSchema(database)
		if err != nil {
			log.Fatalf("Failed to setup database schema: %v", err)
		}
		fmt.Println("Database schema setup complete")
		return
	}

	if *quotesCmd {
		if *filePath == "" {
			log.Fatal("File path is required for quotes command")
		}

		// Parse data source
		dataSource := models.DataSource(*source)

		// Load quotes from file
		quotes, err := loadQuotes(*filePath)
		if err != nil {
			log.Fatalf("Failed to load quotes: %v", err)
		}

		// Process quotes
		batchID, quoteIDs, err := p.ProcessStockQuotes(ctx, quotes, dataSource)
		if err != nil {
			log.Printf("Warning: processing completed with errors: %v", err)
		}

		fmt.Printf("Processed batch %s with %d quotes\n", batchID, len(quoteIDs))
	}

	if *indicesCmd {
		if *filePath == "" {
			log.Fatal("File path is required for indices command")
		}

		// Parse data source
		dataSource := models.DataSource(*source)

		// Load indices from file
		indices, err := loadIndices(*filePath)
		if err != nil {
			log.Fatalf("Failed to load indices: %v", err)
		}

		// Process indices
		batchID, indexIDs, err := p.ProcessMarketIndices(ctx, indices, dataSource)
		if err != nil {
			log.Printf("Warning: processing completed with errors: %v", err)
		}

		fmt.Printf("Processed batch %s with %d indices\n", batchID, len(indexIDs))
	}

	if *mixedCmd {
		if *filePath == "" {
			log.Fatal("File path is required for mixed command")
		}

		// Parse data source
		dataSource := models.DataSource(*source)

		// Load mixed data from file
		quotes, indices, err := loadMixedData(*filePath)
		if err != nil {
			log.Fatalf("Failed to load mixed data: %v", err)
		}

		// Process mixed data
		batchID, quoteIDs, indexIDs, err := p.ProcessMixedBatch(ctx, quotes, indices, dataSource)
		if err != nil {
			log.Printf("Warning: processing completed with errors: %v", err)
		}

		fmt.Printf("Processed batch %s with %d quotes and %d indices\n", batchID, len(quoteIDs), len(indexIDs))
	}

	if *realtimeCmd {
		// Parse data source
		dataSource := models.DataSource(*source)

		// Process simulated real-time data
		processRealtimeData(ctx, p, dataSource, *duration)
	}

	if *listCmd {
		// List latest data
		listLatestData(ctx, database, *limit, *symbols, *indexList)
	}

	// If no command is specified, show usage
	if !(*setupCmd || *quotesCmd || *indicesCmd || *mixedCmd || *realtimeCmd || *listCmd) {
		flag.Usage()
	}
}

// Load stock quotes from a JSON file
func loadQuotes(filePath string) ([]models.StockQuote, error) {
	// Read the file
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Try to unmarshal as a list of quotes
	var quotes []models.StockQuote
	err = json.Unmarshal(data, &quotes)
	if err == nil && len(quotes) > 0 {
		return quotes, nil
	}

	// Try to unmarshal as a map with a quotes key
	var quotesMap map[string]json.RawMessage
	err = json.Unmarshal(data, &quotesMap)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	quotesData, ok := quotesMap["quotes"]
	if !ok {
		return nil, fmt.Errorf("no quotes found in file")
	}

	err = json.Unmarshal(quotesData, &quotes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse quotes: %w", err)
	}

	return quotes, nil
}

// Load market indices from a JSON file
func loadIndices(filePath string) ([]models.MarketIndex, error) {
	// Read the file
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Try to unmarshal as a list of indices
	var indices []models.MarketIndex
	err = json.Unmarshal(data, &indices)
	if err == nil && len(indices) > 0 {
		return indices, nil
	}

	// Try to unmarshal as a map with an indices key
	var indicesMap map[string]json.RawMessage
	err = json.Unmarshal(data, &indicesMap)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	indicesData, ok := indicesMap["indices"]
	if !ok {
		return nil, fmt.Errorf("no indices found in file")
	}

	err = json.Unmarshal(indicesData, &indices)
	if err != nil {
		return nil, fmt.Errorf("failed to parse indices: %w", err)
	}

	return indices, nil
}

// Load mixed data (quotes and indices) from a JSON file
func loadMixedData(filePath string) ([]models.StockQuote, []models.MarketIndex, error) {
	// Read the file
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Unmarshal as a map
	var mixedMap map[string]json.RawMessage
	err = json.Unmarshal(data, &mixedMap)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Extract quotes
	var quotes []models.StockQuote
	quotesData, ok := mixedMap["quotes"]
	if ok {
		err = json.Unmarshal(quotesData, &quotes)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse quotes: %w", err)
		}
	}

	// Extract indices
	var indices []models.MarketIndex
	indicesData, ok := mixedMap["indices"]
	if ok {
		err = json.Unmarshal(indicesData, &indices)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse indices: %w", err)
		}
	}

	if len(quotes) == 0 && len(indices) == 0 {
		return nil, nil, fmt.Errorf("no quotes or indices found in file")
	}

	return quotes, indices, nil
}

// Process simulated real-time data
func processRealtimeData(ctx context.Context, p *pipeline.Pipeline, source models.DataSource, durationSeconds int) {
	fmt.Printf("Starting real-time processing from %s for %d seconds\n", source, durationSeconds)

	startTime := time.Now()
	count := 0

	for time.Since(startTime).Seconds() < float64(durationSeconds) {
		// Simulate receiving a batch of quotes every few seconds
		batchSize := 5 // Small batch size for real-time processing

		// Generate simulated data
		quotes := make([]models.StockQuote, batchSize)
		for i := 0; i < batchSize; i++ {
			quotes[i] = models.StockQuote{
				Symbol:        fmt.Sprintf("SIM%d", i),
				Price:         100.0 + float64(i),
				Change:        float64(i),
				ChangePercent: float64(i),
				Volume:        1000 * int64(i),
				Timestamp:     time.Now(),
				Exchange:      models.NYSE,
			}
		}

		// Process the batch
		batchID, quoteIDs, err := p.ProcessStockQuotes(ctx, quotes, source)
		count++

		if err != nil {
			fmt.Printf("Batch %d (%s) processed with errors: %v\n", count, batchID, err)
		} else {
			fmt.Printf("Batch %d (%s) processed successfully with %d quotes\n", count, batchID, len(quoteIDs))
		}

		// Sleep for a short time to simulate data arriving at intervals
		time.Sleep(5 * time.Second)
	}

	fmt.Printf("Completed real-time processing: %d batches processed\n", count)
}

// List the latest data from the database
func listLatestData(ctx context.Context, database *db.Database, limit int, symbolsStr, indicesStr string) {
	// Parse symbols and indices
	var symbols, indexNames []string
	if symbolsStr != "" {
		symbols = parseCommaList(symbolsStr)
	}
	if indicesStr != "" {
		indexNames = parseCommaList(indicesStr)
	}

	// Get latest quotes
	quotes, err := database.GetLatestQuotes(ctx, symbols, limit)
	if err != nil {
		fmt.Printf("Error getting latest quotes: %v\n", err)
	} else {
		fmt.Printf("Latest %d stock quotes:\n", len(quotes))
		for _, quote := range quotes {
			fmt.Printf("Quote: %s - $%.4f (%.4f%%)\n", quote.Symbol, quote.Price, quote.ChangePercent)
		}
	}

	// Get latest indices
	indices, err := database.GetLatestIndices(ctx, indexNames, limit)
	if err != nil {
		fmt.Printf("Error getting latest indices: %v\n", err)
	} else {
		fmt.Printf("\nLatest %d market indices:\n", len(indices))
		for _, index := range indices {
			fmt.Printf("Index: %s - %.4f (%.4f%%)\n", index.Name, index.Value, index.ChangePercent)
		}
	}
}

// Parse a comma-separated list into a slice of strings
func parseCommaList(list string) []string {
	if list == "" {
		return nil
	}

	// Split by comma
	var result []string
	var current string
	for i := 0; i < len(list); i++ {
		if list[i] == ',' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else if list[i] != ' ' { // Skip spaces
			current += string(list[i])
		}
	}
	if current != "" {
		result = append(result, current)
	}

	return result
}

// setupDatabaseSchema applies the migration SQL files to the database
func setupDatabaseSchema(database *db.Database) error {
	// First check if tables already exist
	tablesExist, err := checkIfTablesExist(database)
	if err != nil {
		return fmt.Errorf("failed to check existing tables: %w", err)
	}
	
	if tablesExist {
		log.Println("Database schema already exists, skipping migration")
		return nil
	}

	// Path to the migration directory
	migrationsPath := findMigrationsPath()
	if migrationsPath == "" {
		return fmt.Errorf("migrations directory not found")
	}

	log.Printf("Using migrations from: %s", migrationsPath)

	// Get all SQL migration files
	files, err := os.ReadDir(migrationsPath)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Filter and sort migration files (only the 'up' migrations, not the 'down' ones)
	var migrationFiles []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".sql") && !strings.Contains(file.Name(), "_down.sql") {
			migrationFiles = append(migrationFiles, filepath.Join(migrationsPath, file.Name()))
		}
	}

	// Sort migration files by name to ensure proper order
	// This assumes files are named with numeric prefixes like 001_, 002_, etc.
	sortMigrationFiles(migrationFiles)

	// Execute each migration file
	for _, file := range migrationFiles {
		log.Printf("Applying migration: %s", filepath.Base(file))
		
		// Read the SQL content
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, err)
		}
		
		// Modify SQL to handle existing objects
		// For each CREATE statement, add IF NOT EXISTS
		safeSQL := makeCreateStatementsIdempotent(string(content))
		
		// Execute the SQL
		_, err = database.ExecuteSQL(safeSQL)
		if err != nil {
			log.Printf("Warning: Error executing migration %s: %v", file, err)
			// Continue with next migration instead of failing completely
			continue
		}
		
		log.Printf("Successfully applied: %s", filepath.Base(file))
	}

	return nil
}

// checkIfTablesExist checks if the primary tables already exist in the database
func checkIfTablesExist(database *db.Database) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'stock_quotes'
		)
	`
	
	var exists bool
	err := database.QueryRow(query, &exists)
	if err != nil {
		return false, err
	}
	
	return exists, nil
}

// makeCreateStatementsIdempotent modifies SQL to handle existing objects
func makeCreateStatementsIdempotent(sql string) string {
	// Add IF NOT EXISTS to CREATE TABLE statements
	sql = strings.ReplaceAll(sql, "CREATE TABLE ", "CREATE TABLE IF NOT EXISTS ")
	
	// Add IF NOT EXISTS to CREATE INDEX statements
	sql = strings.ReplaceAll(sql, "CREATE INDEX ", "CREATE INDEX IF NOT EXISTS ")
	
	// Add OR REPLACE to CREATE VIEW statements
	sql = strings.ReplaceAll(sql, "CREATE VIEW ", "CREATE OR REPLACE VIEW ")
	
	// Handle enums and extensions more carefully - these often cause errors on re-run
	// We can't easily modify them, so we'll just comment them out for debugging
	
	return sql
}

// findMigrationsPath attempts to locate the migrations directory
func findMigrationsPath() string {
	// Check common relative paths
	possiblePaths := []string{
		"../../storage/migrations",           // From etl/cmd/etlcli to quotron/storage/migrations
		"../../../storage/migrations",        // Alternative path
		"../../../../storage/migrations",     // Another alternative
		"../../../../../quotron/storage/migrations", // From a deeper directory
	}

	// Working directory as base
	workDir, err := os.Getwd()
	if err == nil {
		for _, relPath := range possiblePaths {
			path := filepath.Join(workDir, relPath)
			if dirExists(path) {
				return path
			}
		}
	}

	// Try to find from GOPATH or in the repository root
	gopath := os.Getenv("GOPATH")
	if gopath != "" {
		path := filepath.Join(gopath, "src", "github.com", "we-be", "tiny-ria", "quotron", "storage", "migrations")
		if dirExists(path) {
			return path
		}
	}

	// As a last resort, try to find relative to the executable path
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		for i := 0; i < 5; i++ { // Look up to 5 directories up
			path := filepath.Join(exeDir, "storage", "migrations")
			if dirExists(path) {
				return path
			}
			exeDir = filepath.Dir(exeDir)
		}
	}

	return ""
}

// sortMigrationFiles sorts migration files by their numeric prefix
func sortMigrationFiles(files []string) {
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			fileI := filepath.Base(files[i])
			fileJ := filepath.Base(files[j])
			
			// Extract the numeric prefix from the filename (e.g., "001" from "001_migration.sql")
			prefixI := extractPrefix(fileI)
			prefixJ := extractPrefix(fileJ)
			
			if prefixI > prefixJ {
				files[i], files[j] = files[j], files[i]
			}
		}
	}
}

// extractPrefix extracts the numeric prefix from a filename
func extractPrefix(filename string) string {
	parts := strings.Split(filename, "_")
	if len(parts) > 0 {
		return parts[0]
	}
	return filename
}

// dirExists checks if a directory exists
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}