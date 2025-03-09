package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/we-be/tiny-ria/quotron/etl/internal/db"
	"github.com/we-be/tiny-ria/quotron/etl/internal/models"
)

func main() {
	// Define command-line flags
	var (
		// Commands
		importCmd    = flag.Bool("import", false, "Import an investment model from a JSON file")
		listCmd      = flag.Bool("list", false, "List investment models")
		getCmd       = flag.Bool("get", false, "Get a single investment model by ID")
		
		// File options
		filePath     = flag.String("file", "", "Path to the JSON file containing model data")
		
		// List options
		provider     = flag.String("provider", "", "Filter models by provider")
		limit        = flag.Int("limit", 10, "Number of models to list")
		
		// Get options
		modelID      = flag.String("id", "", "Investment model ID to retrieve")
		
		// Database options
		dbHost       = flag.String("db-host", "", "Database hostname")
		dbPort       = flag.Int("db-port", 0, "Database port")
		dbName       = flag.String("db-name", "", "Database name")
		dbUser       = flag.String("db-user", "", "Database username")
		dbPass       = flag.String("db-pass", "", "Database password")
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

	ctx := context.Background()

	// Handle commands
	if *importCmd {
		if *filePath == "" {
			log.Fatal("File path is required for import command")
		}

		// Load model from file
		model, err := loadInvestmentModel(*filePath)
		if err != nil {
			log.Fatalf("Failed to load investment model: %v", err)
		}

		// Store model in database
		modelID, err := database.StoreInvestmentModel(ctx, model)
		if err != nil {
			log.Fatalf("Failed to store investment model: %v", err)
		}

		fmt.Printf("Investment model imported successfully with ID: %s\n", modelID)
		fmt.Printf("Model has %d holdings and %d sector allocations\n", 
			len(model.Holdings), len(model.Sectors))
	}

	if *listCmd {
		// List investment models
		models, err := database.ListInvestmentModels(ctx, *provider, *limit)
		if err != nil {
			log.Fatalf("Failed to list investment models: %v", err)
		}

		fmt.Printf("Found %d investment models:\n", len(models))
		for _, model := range models {
			fmt.Printf("ID: %s, Provider: %s, Model: %s, Detail Level: %s, Fetched: %s\n",
				model.ID, model.Provider, model.ModelName, model.DetailLevel, 
				model.FetchedAt.Format(time.RFC3339))
		}
	}

	if *getCmd {
		if *modelID == "" {
			log.Fatal("Model ID is required for get command")
		}

		// Get investment model
		model, err := database.GetInvestmentModel(ctx, *modelID)
		if err != nil {
			log.Fatalf("Failed to get investment model: %v", err)
		}

		// Print model details
		fmt.Printf("Investment Model: %s\n", model.ModelName)
		fmt.Printf("Provider: %s\n", model.Provider)
		fmt.Printf("Detail Level: %s\n", model.DetailLevel)
		fmt.Printf("Source: %s\n", model.Source)
		fmt.Printf("Fetched At: %s\n", model.FetchedAt.Format(time.RFC3339))
		fmt.Printf("Holdings: %d\n", len(model.Holdings))
		fmt.Printf("Sector Allocations: %d\n", len(model.Sectors))
		
		// Print top 10 holdings
		if len(model.Holdings) > 0 {
			fmt.Println("\nTop Holdings:")
			limit := 10
			if len(model.Holdings) < limit {
				limit = len(model.Holdings)
			}
			for i := 0; i < limit; i++ {
				fmt.Printf("  %s (%s): %.4f%%\n", 
					model.Holdings[i].PositionName, 
					model.Holdings[i].Ticker,
					model.Holdings[i].Allocation)
			}
		}
		
		// Print sector allocations
		if len(model.Sectors) > 0 {
			fmt.Println("\nSector Allocations:")
			for _, sector := range model.Sectors {
				fmt.Printf("  %s: %.4f%%\n", sector.Sector, sector.AllocationPercent)
			}
		}
	}

	// If no command is specified, show usage
	if !(*importCmd || *listCmd || *getCmd) {
		flag.Usage()
	}
}

// Load investment model from a JSON file
func loadInvestmentModel(filePath string) (*models.InvestmentModel, error) {
	// Read the file
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse JSON into a model
	var model models.InvestmentModel
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate the model
	if model.Provider == "" {
		return nil, fmt.Errorf("provider is required")
	}
	if model.ModelName == "" {
		return nil, fmt.Errorf("model name is required")
	}
	if model.DetailLevel == "" {
		model.DetailLevel = models.FullDetail // Default to full detail
	}
	
	// Set fetched_at if not provided
	if model.FetchedAt.IsZero() {
		model.FetchedAt = time.Now()
	}

	// Process holdings
	for i := range model.Holdings {
		// For holdings without a sector, set to "Unknown"
		if model.Holdings[i].Sector == "" {
			model.Holdings[i].Sector = "Unknown"
		}
		
		// For holdings without an asset class, default to "Equity" for the S&P 500
		if model.Holdings[i].AssetClass == "" {
			model.Holdings[i].AssetClass = "Equity"
		}
	}

	// If we have holdings but no sectors, we can generate sector allocations
	if len(model.Holdings) > 0 && len(model.Sectors) == 0 {
		// Group by sector and calculate allocations
		sectorMap := make(map[string]float64)
		for _, holding := range model.Holdings {
			sectorMap[holding.Sector] += holding.Allocation
		}
		
		// Convert to sector allocations
		for sector, allocation := range sectorMap {
			model.Sectors = append(model.Sectors, models.SectorAllocation{
				Sector:            sector,
				AllocationPercent: allocation,
			})
		}
	}

	return &model, nil
}