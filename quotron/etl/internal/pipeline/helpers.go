package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/we-be/tiny-ria/quotron/etl/internal/db"
	"github.com/we-be/tiny-ria/quotron/etl/internal/models"
)

// LoadQuotesFromFile loads stock quotes from a JSON file
func LoadQuotesFromFile(filePath string) ([]models.StockQuote, error) {
	// Read the file
	data, err := os.ReadFile(filePath)
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

// LoadIndicesFromFile loads market indices from a JSON file
func LoadIndicesFromFile(filePath string) ([]models.MarketIndex, error) {
	// Read the file
	data, err := os.ReadFile(filePath)
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

// LoadMixedDataFromFile loads mixed data (quotes and indices) from a JSON file
func LoadMixedDataFromFile(filePath string) ([]models.StockQuote, []models.MarketIndex, error) {
	// Read the file
	data, err := os.ReadFile(filePath)
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

// SetupDatabaseSchema applies the migration SQL files to the database
func SetupDatabaseSchema(database *db.Database) error {
	// First check if tables already exist
	tablesExist, err := checkIfTablesExist(database)
	if err != nil {
		return fmt.Errorf("failed to check existing tables: %w", err)
	}
	
	if tablesExist {
		return nil
	}

	// Path to the migration directory
	migrationsPath := findMigrationsPath()
	if migrationsPath == "" {
		return fmt.Errorf("migrations directory not found")
	}

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
	sortMigrationFiles(migrationFiles)

	// Execute each migration file
	for _, file := range migrationFiles {
		// Read the SQL content
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, err)
		}
		
		// Modify SQL to handle existing objects
		safeSQL := makeCreateStatementsIdempotent(string(content))
		
		// Execute the SQL
		_, err = database.ExecuteSQL(safeSQL)
		if err != nil {
			return fmt.Errorf("error executing migration %s: %w", file, err)
		}
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
	
	return sql
}

// findMigrationsPath attempts to locate the migrations directory
func findMigrationsPath() string {
	// Check common relative paths
	possiblePaths := []string{
		"../storage/migrations",           // From etl/internal/pipeline to quotron/storage/migrations
		"../../storage/migrations",        // From etl dir to quotron/storage/migrations
		"../../../storage/migrations",     // From deeper directories
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