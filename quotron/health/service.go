package health

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

// HealthService handles the reporting and querying of service health
type HealthService struct {
	DB *sql.DB
}

// NewHealthService creates a new health service with a database connection
func NewHealthService(db *sql.DB) *HealthService {
	return &HealthService{DB: db}
}

// ReportHealth records a health status in the database
func (h *HealthService) ReportHealth(report HealthReport) error {
	if h.DB == nil {
		return errors.New("database connection not initialized")
	}

	if report.SourceType == "" || report.SourceName == "" {
		return errors.New("source type and name are required")
	}

	// Default status if not set
	if report.Status == "" {
		report.Status = StatusUnknown
	}

	// Set the check time to now if not provided
	if report.LastCheck.IsZero() {
		report.LastCheck = time.Now()
	}

	// Convert metadata to JSON
	var metadataJSON []byte
	var err error
	if report.Metadata != nil {
		metadataJSON, err = json.Marshal(report.Metadata)
		if err != nil {
			log.Printf("Error marshaling metadata to JSON: %v", err)
			metadataJSON = []byte("{}")
		}
	} else {
		metadataJSON = []byte("{}")
	}

	// Check if record exists
	var id string
	var errorCount int
	existsQuery := `
		SELECT id, error_count 
		FROM data_source_health 
		WHERE source_type = $1 AND source_name = $2
	`
	err = h.DB.QueryRow(existsQuery, report.SourceType, report.SourceName).Scan(&id, &errorCount)

	if err == sql.ErrNoRows {
		// Insert new record
		insertQuery := `
			INSERT INTO data_source_health 
			(source_type, source_name, source_detail, status, last_check, 
			 error_message, response_time_ms, metadata, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`
		_, err = h.DB.Exec(
			insertQuery,
			report.SourceType,
			report.SourceName,
			report.SourceDetail,
			report.Status,
			report.LastCheck,
			report.ErrorMessage,
			report.ResponseTimeMs,
			string(metadataJSON),
			time.Now(),
			time.Now(),
		)
		return err
	} else if err != nil {
		return fmt.Errorf("error checking for existing health record: %w", err)
	}

	// Update existing record
	updateQuery := `
		UPDATE data_source_health
		SET status = $1, 
		    last_check = $2,
		    updated_at = $2,
			error_message = $3,
			response_time_ms = $4,
			metadata = $5,
			error_count = CASE 
				WHEN $1 = 'failed' OR $1 = 'error' THEN error_count + 1 
				ELSE error_count 
			END,
			last_success = CASE 
				WHEN $1 = 'healthy' OR $1 = 'degraded' OR $1 = 'limited' THEN $2
				ELSE last_success 
			END
		WHERE id = $6
	`
	_, err = h.DB.Exec(
		updateQuery,
		report.Status,
		report.LastCheck,
		report.ErrorMessage,
		report.ResponseTimeMs,
		string(metadataJSON),
		id,
	)
	return err
}

// GetHealthForService gets the health status for a specific service
func (h *HealthService) GetHealthForService(sourceType, sourceName string) (*HealthReport, error) {
	if h.DB == nil {
		return nil, errors.New("database connection not initialized")
	}

	query := `
		SELECT id, source_type, source_name, source_detail, status, 
		       last_check, last_success, error_message, response_time_ms, 
			   error_count, metadata
		FROM data_source_health
		WHERE source_type = $1 AND source_name = $2
	`

	row := h.DB.QueryRow(query, sourceType, sourceName)
	
	var report HealthReport
	var metadataJSON string
	var lastSuccess sql.NullTime
	
	err := row.Scan(
		&report.ID,
		&report.SourceType,
		&report.SourceName,
		&report.SourceDetail,
		&report.Status,
		&report.LastCheck,
		&lastSuccess,
		&report.ErrorMessage,
		&report.ResponseTimeMs,
		&report.ErrorCount,
		&metadataJSON,
	)
	
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no health record found for %s/%s", sourceType, sourceName)
	} else if err != nil {
		return nil, err
	}
	
	// Handle nullable last success
	if lastSuccess.Valid {
		report.LastSuccess = &lastSuccess.Time
	}
	
	// Parse metadata JSON
	if metadataJSON != "" {
		err = json.Unmarshal([]byte(metadataJSON), &report.Metadata)
		if err != nil {
			log.Printf("Error unmarshaling metadata JSON: %v", err)
			report.Metadata = make(map[string]interface{})
		}
	} else {
		report.Metadata = make(map[string]interface{})
	}
	
	return &report, nil
}

// GetAllHealthStatuses gets the health status for all services
func (h *HealthService) GetAllHealthStatuses() ([]HealthReport, error) {
	if h.DB == nil {
		return nil, errors.New("database connection not initialized")
	}

	query := `
		SELECT id, source_type, source_name, source_detail, status, 
		       last_check, last_success, error_message, response_time_ms, 
			   error_count, metadata
		FROM data_source_health
		ORDER BY source_type, source_name
	`

	rows, err := h.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var reports []HealthReport
	
	for rows.Next() {
		var report HealthReport
		var metadataJSON string
		var lastSuccess sql.NullTime
		
		err := rows.Scan(
			&report.ID,
			&report.SourceType,
			&report.SourceName,
			&report.SourceDetail,
			&report.Status,
			&report.LastCheck,
			&lastSuccess,
			&report.ErrorMessage,
			&report.ResponseTimeMs,
			&report.ErrorCount,
			&metadataJSON,
		)
		
		if err != nil {
			return nil, err
		}
		
		// Handle nullable last success
		if lastSuccess.Valid {
			report.LastSuccess = &lastSuccess.Time
		}
		
		// Parse metadata JSON
		if metadataJSON != "" {
			err = json.Unmarshal([]byte(metadataJSON), &report.Metadata)
			if err != nil {
				log.Printf("Error unmarshaling metadata JSON: %v", err)
				report.Metadata = make(map[string]interface{})
			}
		} else {
			report.Metadata = make(map[string]interface{})
		}
		
		reports = append(reports, report)
	}
	
	if err = rows.Err(); err != nil {
		return nil, err
	}
	
	return reports, nil
}

// GetSystemHealth calculates the overall system health
func (h *HealthService) GetSystemHealth() (*SystemHealth, error) {
	reports, err := h.GetAllHealthStatuses()
	if err != nil {
		return nil, err
	}
	
	systemHealth := &SystemHealth{
		HealthReports: reports,
		LastCheck:     time.Now(),
		TotalServices: len(reports),
	}
	
	// Count service status
	healthScore := 0.0
	for _, report := range reports {
		switch report.Status {
		case StatusHealthy:
			systemHealth.HealthyCount++
			healthScore += 1.0
		case StatusDegraded, StatusLimited:
			systemHealth.DegradedCount++
			healthScore += 0.5
		case StatusFailed:
			systemHealth.FailedCount++
			// Failed services don't add to health score
		}
	}
	
	// Calculate overall health score (0-100%)
	if systemHealth.TotalServices > 0 {
		systemHealth.HealthScore = (healthScore / float64(systemHealth.TotalServices)) * 100.0
	}
	
	return systemHealth, nil
}