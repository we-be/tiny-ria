package health

import (
	"context"
	"net/http"
	"time"
)

// HealthClientInterface defines the interface for a health client
type HealthClientInterface interface {
	ReportHealth(ctx context.Context, report HealthReport) error
}

// HealthReportingMiddleware creates middleware for automatic health reporting from HTTP handlers
func HealthReportingMiddleware(healthClient HealthClientInterface, sourceType, sourceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create a response recorder to capture status code
			rw := newResponseWriter(w)
			
			// Record start time
			start := time.Now()
			
			// Call the next handler
			next.ServeHTTP(rw, r)
			
			// Calculate elapsed time
			elapsed := time.Since(start)
			
			// Determine status based on response code
			status := StatusHealthy
			if rw.statusCode >= 500 {
				status = StatusFailed
			} else if rw.statusCode >= 400 {
				status = StatusDegraded
			}
			
			// Get path and method for metadata
			path := r.URL.Path
			method := r.Method
			
			// Create report
			report := HealthReport{
				SourceType:     sourceType,
				SourceName:     sourceName,
				Status:         status,
				LastCheck:      time.Now(),
				ResponseTimeMs: elapsed.Milliseconds(),
				Metadata: map[string]interface{}{
					"path":        path,
					"method":      method,
					"status_code": rw.statusCode,
				},
			}
			
			// Report health asynchronously
			go func() {
				// Create a new context for async operation
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				
				// Send the report
				err := healthClient.ReportHealth(ctx, report)
				if err != nil {
					// Just log error, don't affect the response
					// log.Printf("Error reporting health: %v", err)
				}
			}()
		})
	}
}

// responseWriter is a wrapper for http.ResponseWriter that captures the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// newResponseWriter creates a new responseWriter
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}

// WriteHeader captures the status code and calls the wrapped ResponseWriter's WriteHeader
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}