package models

import (
	"encoding/json"
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

// GeneratePythonModels generates Python models from the JSON schema
func GeneratePythonModels(schemaPath, outputPath string) error {
	// This is a placeholder. In a real implementation, we would:
	// 1. Read the JSON schema file
	// 2. Convert it to Python Pydantic models
	// 3. Write the Python code to a file

	// For this example, we'll just create a simple Python model file
	pythonCode := `"""
Automatically generated finance models from Go schema.
Do not edit directly - changes will be overwritten.
"""

from datetime import datetime
from typing import Optional, List, Dict, Any
from enum import Enum
from pydantic import BaseModel, Field

class DataSource(str, Enum):
    API_SCRAPER = "api-scraper"
    BROWSER_SCRAPER = "browser-scraper"
    MANUAL = "manual"

class Exchange(str, Enum):
    NYSE = "NYSE"
    NASDAQ = "NASDAQ"
    AMEX = "AMEX"
    OTC = "OTC"
    OTHER = "OTHER"

class StockQuote(BaseModel):
    """Stock quote data"""
    id: Optional[str] = Field(None, description="Unique identifier")
    symbol: str = Field(..., description="Stock ticker symbol")
    price: float = Field(..., description="Current price")
    change: float = Field(..., description="Absolute price change")
    change_percent: float = Field(..., description="Percentage price change")
    volume: int = Field(..., description="Trading volume")
    timestamp: datetime = Field(..., description="Quote timestamp")
    exchange: Exchange = Field(..., description="Stock exchange")
    source: DataSource = Field(..., description="Data source")
    created_at: Optional[datetime] = Field(None, description="Record creation time")
    batch_id: Optional[str] = Field(None, description="Batch identifier")

class MarketIndex(BaseModel):
    """Market index data"""
    id: Optional[str] = Field(None, description="Unique identifier")
    name: str = Field(..., description="Index name")
    symbol: Optional[str] = Field(None, description="Index symbol")
    value: float = Field(..., description="Current value")
    change: float = Field(..., description="Absolute value change")
    change_percent: float = Field(..., description="Percentage value change")
    timestamp: datetime = Field(..., description="Index timestamp")
    source: DataSource = Field(..., description="Data source")
    created_at: Optional[datetime] = Field(None, description="Record creation time")
    batch_id: Optional[str] = Field(None, description="Batch identifier")

class MarketBatch(BaseModel):
    """Batch of market data"""
    quotes: List[StockQuote] = Field(default_factory=list, description="List of stock quotes")
    indices: List[MarketIndex] = Field(default_factory=list, description="List of market indices")
    batch_id: str = Field(..., description="Batch identifier")
    created_at: datetime = Field(default_factory=datetime.utcnow, description="Batch creation time")

class DataBatch(BaseModel):
    """Data batch information"""
    id: str = Field(..., description="Unique identifier")
    created_at: datetime = Field(..., description="Batch creation time")
    processed_at: Optional[datetime] = Field(None, description="Processing completion time")
    status: str = Field(..., description="Processing status")
    quote_count: int = Field(0, description="Number of quotes in batch")
    index_count: int = Field(0, description="Number of indices in batch")
    source: DataSource = Field(..., description="Data source")
    metadata: Optional[Dict[str, Any]] = Field(None, description="Additional batch metadata")

class DataSourceHealth(BaseModel):
    """Health status of a data source"""
    id: Optional[str] = Field(None, description="Unique identifier")
    source: DataSource = Field(..., description="Data source name")
    status: str = Field(..., description="Status: up, down, or degraded")
    last_checked: datetime = Field(..., description="Last health check time")
    error_count: int = Field(0, description="Number of consecutive errors")
    last_error: Optional[str] = Field(None, description="Last error message")
    last_error_time: Optional[datetime] = Field(None, description="Time of last error")
    success_count: int = Field(0, description="Number of consecutive successes")
    response_time: int = Field(0, description="Last response time in milliseconds")
    average_latency: float = Field(0.0, description="Average latency in milliseconds")
    up_since: Optional[datetime] = Field(None, description="Time since service has been up")
    health_score: float = Field(0.0, description="Health score from 0-100")
    metadata: Optional[Dict[str, Any]] = Field(None, description="Additional health metadata")
`

	// Create output directory if it doesn't exist
	err := os.MkdirAll(filepath.Dir(outputPath), 0755)
	if err != nil {
		return err
	}

	// Write the Python code to a file
	return os.WriteFile(outputPath, []byte(pythonCode), 0644)
}

// GenerateSchemas generates all schema files
func GenerateSchemas() error {
	// Generate JSON schema
	schemaPath := "schemas/finance_schema.json"
	err := GeneratePythonSchema(schemaPath)
	if err != nil {
		return err
	}

	// Generate Python models
	pythonPath := "../ingest-pipeline/schemas/finance_models.py"
	return GeneratePythonModels(schemaPath, pythonPath)
}

func main() {
	if err := GenerateSchemas(); err != nil {
		panic(err)
	}
}