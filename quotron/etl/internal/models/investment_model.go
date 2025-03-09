package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// DetailLevel represents the level of detail for an investment model
type DetailLevel string

const (
	FullDetail    DetailLevel = "full"
	PartialDetail DetailLevel = "partial"
	MinimalDetail DetailLevel = "minimal"
)

// InvestmentModel represents an investment strategy model
type InvestmentModel struct {
	ID         string      `json:"id,omitempty" db:"id"`
	Provider   string      `json:"provider" db:"provider" validate:"required"`
	ModelName  string      `json:"model_name" db:"model_name" validate:"required"`
	DetailLevel DetailLevel `json:"detail_level" db:"detail_level" validate:"required"`
	Source     string      `json:"source,omitempty" db:"source"`
	FetchedAt  time.Time   `json:"fetched_at" db:"fetched_at"`
	Holdings   []ModelHolding      `json:"holdings,omitempty" db:"-"` // Not stored directly in this table
	Sectors    []SectorAllocation  `json:"sectors,omitempty" db:"-"`  // Not stored directly in this table
}

// ModelHolding represents a holding within an investment model
type ModelHolding struct {
	ID                int64           `json:"id,omitempty" db:"id"`
	ModelID           string          `json:"model_id" db:"model_id" validate:"required"`
	Ticker            string          `json:"ticker,omitempty" db:"ticker"`
	PositionName      string          `json:"position_name,omitempty" db:"position_name"`
	Allocation        float64         `json:"allocation" db:"allocation" validate:"required,gte=0"`
	AssetClass        string          `json:"asset_class,omitempty" db:"asset_class"`
	Sector            string          `json:"sector,omitempty" db:"sector"`
	AdditionalMetadata *JSONMap       `json:"additional_metadata,omitempty" db:"additional_metadata"`
}

// SectorAllocation represents a sector allocation within an investment model
type SectorAllocation struct {
	ID                int64   `json:"id,omitempty" db:"id"`
	ModelID           string  `json:"model_id" db:"model_id" validate:"required"`
	Sector            string  `json:"sector" db:"sector" validate:"required"`
	AllocationPercent float64 `json:"allocation_percent" db:"allocation_percent" validate:"required,gte=0"`
}

// JSONMap is a helper type for handling JSONB columns
type JSONMap map[string]interface{}

// Value implements the driver.Valuer interface for JSONMap
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface for JSONMap
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONMap)
		return nil
	}
	
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	
	return json.Unmarshal(bytes, j)
}

// InvestmentModelBatch represents a collection of investment models to be processed together
type InvestmentModelBatch struct {
	Models    []InvestmentModel `json:"models"`
	BatchID   string            `json:"batch_id" validate:"required"`
	CreatedAt time.Time         `json:"created_at"`
}