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
	
	// Generate Python models
	pythonPath := "../../../ingest-pipeline/schemas/finance_models.py"
	err = models.GeneratePythonModels(schemaPath, pythonPath)
	if err != nil {
		fmt.Printf("Error generating Python models: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("Schema generation complete!")
}