package main

import (
	"fmt"
	"os"

	"github.com/we-be/tiny-ria/quotron/models"
)

func main() {
	fmt.Println("Generating schema files...")
	
	// Create schemas directory if it doesn't exist
	os.MkdirAll("../../schemas", 0755)
	
	// Generate JSON schema
	schemaPath := "../../schemas/finance_schema.json"
	err := models.GeneratePythonSchema(schemaPath)
	if err != nil {
		fmt.Printf("Error generating JSON schema: %v\n", err)
		os.Exit(1)
	}
	
	// Generate Go models for ETL
	etlPath := "../../../etl/internal/models/finance_schema.go"
	err = models.GenerateGoModels(schemaPath, etlPath)
	if err != nil {
		fmt.Printf("Error generating Python models: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("Schema generation complete!")
}