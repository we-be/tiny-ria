package middleware

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"log"
	"net/http"
	"time"
)

// TracingMiddleware adds distributed tracing capabilities to HTTP handlers
type TracingMiddleware struct {
	DB        *sql.DB
	ServiceName string
}

// Trace is a middleware that tracks request timing and dependencies
func (t *TracingMiddleware) Trace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Generate trace ID or use from header if it exists
		traceID := r.Header.Get("X-Trace-ID")
		if traceID == "" {
			traceID = uuid.New().String()
		}

		// Get parent span ID if it exists
		parentSpanID := r.Header.Get("X-Span-ID")

		// Generate span ID for this request
		spanID := uuid.New().String()

		// Add trace headers for downstream services
		r = r.WithContext(r.Context())
		r.Header.Set("X-Trace-ID", traceID)
		r.Header.Set("X-Span-ID", spanID)

		// Create a wrapper for the ResponseWriter to capture status code
		wrapper := NewResponseWriter(w)

		// Record start time
		startTime := time.Now()

		// Call the next handler
		next.ServeHTTP(wrapper, r)

		// Calculate duration
		duration := time.Since(startTime)

		// Determine status based on response code
		status := "success"
		var errorMessage string
		if wrapper.Status() >= 400 {
			status = "error"
			errorMessage = fmt.Sprintf("HTTP %d", wrapper.Status())
		}

		// Create metadata
		metadata := map[string]interface{}{
			"http_method": r.Method,
			"http_path":   r.URL.Path,
			"http_status": wrapper.Status(),
			"user_agent":  r.UserAgent(),
		}

		// Add query parameters if any, but remove sensitive data
		if len(r.URL.Query()) > 0 {
			queries := make(map[string]string)
			for k, v := range r.URL.Query() {
				// Skip sensitive parameters
				if k == "api_key" || k == "key" || k == "token" || k == "password" {
					queries[k] = "[REDACTED]"
				} else if len(v) > 0 {
					queries[k] = v[0]
				}
			}
			metadata["query_params"] = queries
		}

		// Convert metadata to JSON
		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			log.Printf("Error marshaling trace metadata: %v", err)
		}

		// Store the trace in the database
		if t.DB != nil {
			_, err = t.DB.Exec(
				`INSERT INTO service_traces 
				(trace_id, parent_id, name, service, start_time, end_time, duration_ms, status, error_message, metadata) 
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
				traceID, parentSpanID, r.URL.Path, t.ServiceName, startTime, time.Now(), duration.Milliseconds(), status, errorMessage, metadataJSON,
			)
			if err != nil {
				log.Printf("Error storing trace: %v", err)
			}
		}
	})
}

// ResponseWriter is a wrapper for http.ResponseWriter that captures the status code
type ResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// NewResponseWriter creates a new ResponseWriter
func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{w, http.StatusOK}
}

// WriteHeader captures the status code and passes it to the underlying ResponseWriter
func (rw *ResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Status returns the status code
func (rw *ResponseWriter) Status() int {
	return rw.statusCode
}