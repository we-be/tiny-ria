package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SchemaField represents a field in a JSON schema
type SchemaField struct {
	Type        []string          `json:"type,omitempty"`
	Description string            `json:"description,omitempty"`
	Format      string            `json:"format,omitempty"`
	Enum        []string          `json:"enum,omitempty"`
	Properties  map[string]any    `json:"properties,omitempty"`
	Required    []string          `json:"required,omitempty"`
	Items       map[string]any    `json:"items,omitempty"`
}

// JSONSchema represents a JSON schema document
type JSONSchema struct {
	Schema      string            `json:"$schema"`
	Type        string            `json:"type"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Properties  map[string]any    `json:"properties"`
	Required    []string          `json:"required"`
}

// GeneratePythonSchema generates a Python Pydantic model from the Go types
func GeneratePythonSchema(outputPath string) error {
	schemas := map[string]JSONSchema{
		"DataSource": {
			Schema:      "http://json-schema.org/draft-07/schema#",
			Type:        "string",
			Title:       "DataSource",
			Description: "Data source for financial information",
			Properties:  map[string]any{},
			Required:    []string{},
		},
		"Exchange": {
			Schema:      "http://json-schema.org/draft-07/schema#",
			Type:        "string",
			Title:       "Exchange",
			Description: "Stock exchange",
			Properties:  map[string]any{},
			Required:    []string{},
		},
		"StockQuote": {
			Schema:      "http://json-schema.org/draft-07/schema#",
			Type:        "object",
			Title:       "StockQuote",
			Description: "Stock quote data",
			Properties: map[string]any{
				"id":             map[string]string{"type": "string", "description": "Unique identifier"},
				"symbol":         map[string]string{"type": "string", "description": "Stock ticker symbol"},
				"price":          map[string]string{"type": "number", "description": "Current price"},
				"change":         map[string]string{"type": "number", "description": "Absolute price change"},
				"change_percent": map[string]string{"type": "number", "description": "Percentage price change"},
				"volume":         map[string]string{"type": "integer", "description": "Trading volume"},
				"timestamp":      map[string]string{"type": "string", "format": "date-time", "description": "Quote timestamp"},
				"exchange":       map[string]string{"type": "string", "description": "Stock exchange"},
				"source":         map[string]string{"type": "string", "description": "Data source"},
				"created_at":     map[string]string{"type": "string", "format": "date-time", "description": "Record creation time"},
				"batch_id":       map[string]string{"type": "string", "description": "Batch identifier"},
			},
			Required: []string{"symbol", "price", "change", "change_percent", "volume", "timestamp", "exchange", "source"},
		},
		"MarketIndex": {
			Schema:      "http://json-schema.org/draft-07/schema#",
			Type:        "object",
			Title:       "MarketIndex",
			Description: "Market index data",
			Properties: map[string]any{
				"id":             map[string]string{"type": "string", "description": "Unique identifier"},
				"name":           map[string]string{"type": "string", "description": "Index name"},
				"symbol":         map[string]string{"type": "string", "description": "Index symbol"},
				"value":          map[string]string{"type": "number", "description": "Current value"},
				"change":         map[string]string{"type": "number", "description": "Absolute value change"},
				"change_percent": map[string]string{"type": "number", "description": "Percentage value change"},
				"timestamp":      map[string]string{"type": "string", "format": "date-time", "description": "Index timestamp"},
				"source":         map[string]string{"type": "string", "description": "Data source"},
				"created_at":     map[string]string{"type": "string", "format": "date-time", "description": "Record creation time"},
				"batch_id":       map[string]string{"type": "string", "description": "Batch identifier"},
			},
			Required: []string{"name", "value", "change", "change_percent", "timestamp", "source"},
		},
		"MarketBatch": {
			Schema:      "http://json-schema.org/draft-07/schema#",
			Type:        "object",
			Title:       "MarketBatch",
			Description: "Batch of market data",
			Properties: map[string]any{
				"quotes": map[string]any{
					"type":        "array",
					"description": "List of stock quotes",
					"items":       map[string]string{"$ref": "#/definitions/StockQuote"},
				},
				"indices": map[string]any{
					"type":        "array",
					"description": "List of market indices",
					"items":       map[string]string{"$ref": "#/definitions/MarketIndex"},
				},
				"batch_id":   map[string]string{"type": "string", "description": "Batch identifier"},
				"created_at": map[string]string{"type": "string", "format": "date-time", "description": "Batch creation time"},
			},
			Required: []string{"batch_id"},
		},
	}

	// Create output directory if it doesn't exist
	err := os.MkdirAll(filepath.Dir(outputPath), 0755)
	if err != nil {
		return err
	}

	// Generate JSON schema file
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write schemas to file
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(schemas)
}

// GenerateGoModels generates Go models from the JSON schema
func GenerateGoModels(schemaPath, outputPath string) error {
	// This is a placeholder. In a real implementation, we would:
	// 1. Read the JSON schema file
	// 2. Convert it to Go models
	// 3. Write the Go code to a file

	// For this example, we'll create a simple Go schema file
	goCode := `// Code generated by schema_gen.go. DO NOT EDIT.
package models

import (
	"encoding/json"
	"time"
)

// DataSource represents the source of financial data
type DataSource string

const (
	DataSourceAPIScraper      DataSource = "api-scraper"
	DataSourceBrowserScraper  DataSource = "browser-scraper"
	DataSourceManual          DataSource = "manual"
)

// Exchange represents a stock exchange
type Exchange string

const (
	ExchangeNYSE    Exchange = "NYSE"
	ExchangeNASDAQ  Exchange = "NASDAQ"
	ExchangeAMEX    Exchange = "AMEX"
	ExchangeOTC     Exchange = "OTC"
	ExchangeOTHER   Exchange = "OTHER"
	ExchangeCRYPTO  Exchange = "CRYPTO"
)

// StockQuote represents stock quote data
type StockQuote struct {
	ID            string      ` + "`json:\"id,omitempty\"`" + `
	Symbol        string      ` + "`json:\"symbol\" validate:\"required\"`" + `
	Price         float64     ` + "`json:\"price\" validate:\"required\"`" + `
	Change        float64     ` + "`json:\"change\" validate:\"required\"`" + `
	ChangePercent float64     ` + "`json:\"change_percent\" validate:\"required\"`" + `
	Volume        int64       ` + "`json:\"volume\" validate:\"required\"`" + `
	Timestamp     time.Time   ` + "`json:\"timestamp\" validate:\"required\"`" + `
	Exchange      Exchange    ` + "`json:\"exchange\" validate:\"required\"`" + `
	Source        DataSource  ` + "`json:\"source\" validate:\"required\"`" + `
	CreatedAt     *time.Time  ` + "`json:\"created_at,omitempty\"`" + `
	BatchID       string      ` + "`json:\"batch_id,omitempty\"`" + `
}

// MarketIndex represents market index data
type MarketIndex struct {
	ID            string      ` + "`json:\"id,omitempty\"`" + `
	Name          string      ` + "`json:\"name\" validate:\"required\"`" + `
	Symbol        string      ` + "`json:\"symbol,omitempty\"`" + `
	Value         float64     ` + "`json:\"value\" validate:\"required\"`" + `
	Change        float64     ` + "`json:\"change\" validate:\"required\"`" + `
	ChangePercent float64     ` + "`json:\"change_percent\" validate:\"required\"`" + `
	Timestamp     time.Time   ` + "`json:\"timestamp\" validate:\"required\"`" + `
	Source        DataSource  ` + "`json:\"source\" validate:\"required\"`" + `
	CreatedAt     *time.Time  ` + "`json:\"created_at,omitempty\"`" + `
	BatchID       string      ` + "`json:\"batch_id,omitempty\"`" + `
}

// MarketBatch represents a batch of market data
type MarketBatch struct {
	Quotes     []StockQuote  ` + "`json:\"quotes,omitempty\"`" + `
	Indices    []MarketIndex ` + "`json:\"indices,omitempty\"`" + `
	BatchID    string        ` + "`json:\"batch_id\" validate:\"required\"`" + `
	CreatedAt  time.Time     ` + "`json:\"created_at\"`" + `
}

// DataBatch represents data batch information
type DataBatch struct {
	ID          string      ` + "`json:\"id\" validate:\"required\"`" + `
	CreatedAt   time.Time   ` + "`json:\"created_at\" validate:\"required\"`" + `
	ProcessedAt *time.Time  ` + "`json:\"processed_at,omitempty\"`" + `
	Status      string      ` + "`json:\"status\" validate:\"required\"`" + `
	QuoteCount  int         ` + "`json:\"quote_count\"`" + `
	IndexCount  int         ` + "`json:\"index_count\"`" + `
	Source      DataSource  ` + "`json:\"source\" validate:\"required\"`" + `
	Metadata    interface{} ` + "`json:\"metadata,omitempty\"`" + `
}

// DataSourceHealth represents the health status of a data source
type DataSourceHealth struct {
	ID             string      ` + "`json:\"id,omitempty\"`" + `
	Source         DataSource  ` + "`json:\"source\" validate:\"required\"`" + `
	Status         string      ` + "`json:\"status\" validate:\"required\"`" + `
	LastChecked    time.Time   ` + "`json:\"last_checked\" validate:\"required\"`" + `
	ErrorCount     int         ` + "`json:\"error_count\"`" + `
	LastError      string      ` + "`json:\"last_error,omitempty\"`" + `
	LastErrorTime  *time.Time  ` + "`json:\"last_error_time,omitempty\"`" + `
	SuccessCount   int         ` + "`json:\"success_count\"`" + `
	ResponseTime   int         ` + "`json:\"response_time\"`" + `
	AverageLatency float64     ` + "`json:\"average_latency\"`" + `
	UpSince        *time.Time  ` + "`json:\"up_since,omitempty\"`" + `
	HealthScore    float64     ` + "`json:\"health_score\"`" + `
	Metadata       interface{} ` + "`json:\"metadata,omitempty\"`" + `
}
`

	// Create output directory if it doesn't exist
	err := os.MkdirAll(filepath.Dir(outputPath), 0755)
	if err != nil {
		return err
	}

	// Write the Go code to a file
	return os.WriteFile(outputPath, []byte(goCode), 0644)
}

// GenerateSchemas generates all schema files
func GenerateSchemas() error {
	// Create schemas directory if it doesn't exist
	err := os.MkdirAll("schemas", 0755)
	if err != nil {
		return fmt.Errorf("failed to create schemas directory: %w", err)
	}

	// Generate JSON schema
	schemaPath := "schemas/finance_schema.json"
	err = GeneratePythonSchema(schemaPath)
	if err != nil {
		return err
	}

	// Create ETL models directory
	etlDir := "../etl/internal/models"
	err = os.MkdirAll(etlDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create ETL models directory: %w", err)
	}

	// Generate models for ETL
	etlPath := filepath.Join(etlDir, "finance_schema.go")
	return GenerateGoModels(schemaPath, etlPath)
}

func main() {
	if err := GenerateSchemas(); err != nil {
		panic(err)
	}
}