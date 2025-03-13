package health

import (
	"database/sql/driver"
	"errors"
	"time"
)

// Status represents the health status of a service
type Status string

const (
	// Status constants
	StatusHealthy  Status = "healthy"
	StatusDegraded Status = "degraded"
	StatusFailed   Status = "failed"
	StatusLimited  Status = "limited"
	StatusUnknown  Status = "unknown"
)

// Scan implements the sql.Scanner interface for Status
func (s *Status) Scan(value interface{}) error {
	if value == nil {
		*s = StatusUnknown
		return nil
	}
	
	strVal, ok := value.(string)
	if !ok {
		return errors.New("cannot convert status to string")
	}
	
	*s = Status(strVal)
	return nil
}

// Value implements the driver.Valuer interface for Status
func (s Status) Value() (driver.Value, error) {
	return string(s), nil
}

// HealthReport represents a health status report for a service
type HealthReport struct {
	ID             string                 `json:"id,omitempty"`
	SourceType     string                 `json:"source_type"`
	SourceName     string                 `json:"source_name"`
	SourceDetail   string                 `json:"source_detail,omitempty"`
	Status         Status                 `json:"status"`
	LastCheck      time.Time              `json:"last_check"`
	LastSuccess    *time.Time             `json:"last_success,omitempty"`
	ResponseTimeMs int64                  `json:"response_time_ms,omitempty"`
	ErrorMessage   string                 `json:"error_message,omitempty"`
	ErrorCount     int                    `json:"error_count,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// SystemHealth represents the overall health of the system
type SystemHealth struct {
	HealthScore   float64        `json:"health_score"`
	HealthReports []HealthReport `json:"health_reports"`
	LastCheck     time.Time      `json:"last_check"`
	TotalServices int            `json:"total_services"`
	HealthyCount  int            `json:"healthy_count"`
	DegradedCount int            `json:"degraded_count"`
	FailedCount   int            `json:"failed_count"`
}