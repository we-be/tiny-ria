package trends

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/tiny-ria/quotron/api-scraper/pkg/client"
)

// Handler handles Google Trends related API requests
type Handler struct {
	trendsClient *client.GoogleTrendsClient
}

// NewHandler creates a new Google Trends API handler
func NewHandler() (*Handler, error) {
	// Create a new Google Trends client with 30 second timeout
	trendsClient, err := client.NewGoogleTrendsClient(30 * time.Second)
	if err != nil {
		return nil, err
	}

	return &Handler{
		trendsClient: trendsClient,
	}, nil
}

// RegisterRoutes registers the Google Trends API routes
func (h *Handler) RegisterRoutes(r *mux.Router) {
	// Register the interest over time endpoint
	r.HandleFunc("/api/trends/interest/{keyword}", h.GetInterestOverTime).Methods("GET")
	
	// Register the related queries endpoint
	r.HandleFunc("/api/trends/queries/{keyword}", h.GetRelatedQueries).Methods("GET")
	
	// Register the related topics endpoint
	r.HandleFunc("/api/trends/topics/{keyword}", h.GetRelatedTopics).Methods("GET")
	
	// Register the trends URL endpoint
	r.HandleFunc("/api/trends/url/{keyword}", h.GetTrendsURL).Methods("GET")
	
	// Register the health endpoint
	r.HandleFunc("/api/trends/health", h.GetHealth).Methods("GET")
}

// GetInterestOverTime handles requests for interest over time data
func (h *Handler) GetInterestOverTime(w http.ResponseWriter, r *http.Request) {
	// Get the keyword from the URL
	vars := mux.Vars(r)
	keyword := vars["keyword"]
	
	// Get the timeframe from the query parameters (optional)
	timeframe := r.URL.Query().Get("timeframe")
	
	// Default timeframe if not provided
	if timeframe == "" {
		timeframe = "today 5-y"
	}
	
	// Format parameter to control output format (optional: json, html, csv)
	format := r.URL.Query().Get("format")
	
	// Get the data from the Google Trends client
	data, err := h.trendsClient.GetInterestOverTime(r.Context(), keyword, timeframe)
	if err != nil {
		http.Error(w, "Failed to get interest over time data: "+err.Error(), http.StatusInternalServerError)
		return
	}
	
	// If HTML format is requested, render a visualization
	if format == "html" {
		renderTrendsHtml(w, data, keyword, timeframe)
		return
	} else if format == "csv" {
		renderTrendsCsv(w, data, keyword)
		return
	}
	
	// Default: Set content type for JSON
	w.Header().Set("Content-Type", "application/json")
	
	// Encode the data as JSON
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode interest over time data: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// renderTrendsHtml renders Google Trends interest data as an HTML chart
func renderTrendsHtml(w http.ResponseWriter, data interface{}, keyword string, timeframe string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	// Cast the data to access its fields (assuming it matches Google Trends API structure)
	trendsData, ok := data.(map[string]interface{})
	if !ok {
		http.Error(w, "Failed to parse trends data", http.StatusInternalServerError)
		return
	}
	
	// Extract the timeline data
	timeline, hasTimeline := trendsData["timeline"].([]interface{})
	
	// Extract the interest values
	values, hasValues := trendsData["values"].([]interface{})
	
	// Prepare data for the chart
	var labels []string
	var dataPoints []float64
	
	if hasTimeline && hasValues && len(timeline) == len(values) {
		for i, t := range timeline {
			// Format date to readable string
			dateStr, _ := t.(string)
			labels = append(labels, dateStr)
			
			// Get value and convert to float
			val, _ := values[i].(float64)
			dataPoints = append(dataPoints, val)
		}
	}
	
	// Convert data to JSON for JavaScript
	labelsJson, _ := json.Marshal(labels)
	dataJson, _ := json.Marshal(dataPoints)
	
	// HTML template with embedded Chart.js
	html := `
	<!DOCTYPE html>
	<html>
	<head>
		<title>Google Trends: ` + keyword + `</title>
		<meta charset="utf-8">
		<meta name="viewport" content="width=device-width, initial-scale=1">
		<script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
		<style>
			body {
				font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
				margin: 0;
				padding: 20px;
				background-color: #f8f9fa;
				color: #333;
			}
			.container {
				max-width: 1200px;
				margin: 0 auto;
				background-color: #fff;
				padding: 20px;
				border-radius: 8px;
				box-shadow: 0 2px 10px rgba(0,0,0,0.05);
			}
			h1 {
				color: #1a73e8;
				margin-top: 0;
			}
			.chart-container {
				position: relative;
				height: 60vh;
				width: 100%;
			}
			.metadata {
				margin-top: 20px;
				padding: 10px;
				background-color: #f5f5f5;
				border-radius: 4px;
				font-size: 0.9em;
			}
			.back-link {
				display: block;
				margin-top: 20px;
				color: #1a73e8;
				text-decoration: none;
			}
			.back-link:hover {
				text-decoration: underline;
			}
			.export-links {
				margin-top: 15px;
			}
			.export-links a {
				margin-right: 15px;
				color: #1a73e8;
				text-decoration: none;
			}
			.export-links a:hover {
				text-decoration: underline;
			}
		</style>
	</head>
	<body>
		<div class="container">
			<h1>Google Trends: "` + keyword + `"</h1>
			<div class="chart-container">
				<canvas id="trendsChart"></canvas>
			</div>
			
			<div class="metadata">
				<p><strong>Timeframe:</strong> ` + timeframe + `</p>
				<p><strong>Data points:</strong> ` + fmt.Sprintf("%d", len(labels)) + `</p>
				<p><strong>Generated:</strong> ` + time.Now().Format(time.RFC1123) + `</p>
			</div>
			
			<div class="export-links">
				<a href="/api/trends/url/` + keyword + `?timeframe=` + timeframe + `" target="_blank">View on Google Trends</a>
				<a href="/api/trends/interest/` + keyword + `?format=csv&timeframe=` + timeframe + `">Download CSV</a>
				<a href="/api/trends/interest/` + keyword + `?timeframe=` + timeframe + `">Raw JSON</a>
			</div>
			
			<a href="#" class="back-link" onclick="window.history.back(); return false;">← Back to previous page</a>
		</div>
		
		<script>
			// Create the chart
			const ctx = document.getElementById('trendsChart').getContext('2d');
			const chart = new Chart(ctx, {
				type: 'line',
				data: {
					labels: ` + string(labelsJson) + `,
					datasets: [{
						label: 'Interest Over Time',
						data: ` + string(dataJson) + `,
						fill: false,
						borderColor: '#1a73e8',
						backgroundColor: 'rgba(26, 115, 232, 0.1)',
						pointBackgroundColor: '#1a73e8',
						pointBorderColor: '#fff',
						pointHoverBackgroundColor: '#fff',
						pointHoverBorderColor: '#1a73e8',
						tension: 0.1,
						fill: true
					}]
				},
				options: {
					responsive: true,
					maintainAspectRatio: false,
					scales: {
						x: {
							grid: {
								display: false
							},
							ticks: {
								maxTicksLimit: 10,
								maxRotation: 0
							}
						},
						y: {
							beginAtZero: true,
							grid: {
								color: 'rgba(0, 0, 0, 0.05)'
							},
							ticks: {
								precision: 0
							}
						}
					},
					plugins: {
						title: {
							display: true,
							text: 'Interest Over Time for "` + keyword + `"',
							font: {
								size: 16
							}
						},
						tooltip: {
							mode: 'index',
							intersect: false
						},
						legend: {
							display: true,
							position: 'top'
						}
					},
					interaction: {
						mode: 'nearest',
						axis: 'x',
						intersect: false
					}
				}
			});
		</script>
	</body>
	</html>
	`
	
	w.Write([]byte(html))
}

// renderTrendsCsv renders Google Trends interest data as a CSV file
func renderTrendsCsv(w http.ResponseWriter, data interface{}, keyword string) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=trends_%s.csv", keyword))
	
	// Cast the data to access its fields
	trendsData, ok := data.(map[string]interface{})
	if !ok {
		http.Error(w, "Failed to parse trends data", http.StatusInternalServerError)
		return
	}
	
	// Extract the timeline data
	timeline, hasTimeline := trendsData["timeline"].([]interface{})
	
	// Extract the interest values
	values, hasValues := trendsData["values"].([]interface{})
	
	// Write the CSV header
	fmt.Fprintln(w, "Date,Interest")
	
	// Write the data rows
	if hasTimeline && hasValues && len(timeline) == len(values) {
		for i, t := range timeline {
			// Get date string
			dateStr, _ := t.(string)
			
			// Get value
			val, _ := values[i].(float64)
			
			// Write CSV row
			fmt.Fprintf(w, "%s,%.1f\n", dateStr, val)
		}
	}
}

// GetRelatedQueries handles requests for related queries data
func (h *Handler) GetRelatedQueries(w http.ResponseWriter, r *http.Request) {
	// Get the keyword from the URL
	vars := mux.Vars(r)
	keyword := vars["keyword"]
	
	// Get the data from the Google Trends client
	data, err := h.trendsClient.GetRelatedQueries(r.Context(), keyword)
	if err != nil {
		http.Error(w, "Failed to get related queries data: "+err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Set the content type
	w.Header().Set("Content-Type", "application/json")
	
	// Encode the data as JSON
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode related queries data: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetRelatedTopics handles requests for related topics data
func (h *Handler) GetRelatedTopics(w http.ResponseWriter, r *http.Request) {
	// Get the keyword from the URL
	vars := mux.Vars(r)
	keyword := vars["keyword"]
	
	// Get the data from the Google Trends client
	data, err := h.trendsClient.GetRelatedTopics(r.Context(), keyword)
	if err != nil {
		http.Error(w, "Failed to get related topics data: "+err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Set the content type
	w.Header().Set("Content-Type", "application/json")
	
	// Encode the data as JSON
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode related topics data: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetTrendsURL handles requests for Google Trends URLs
func (h *Handler) GetTrendsURL(w http.ResponseWriter, r *http.Request) {
	// Get the keyword from the URL
	vars := mux.Vars(r)
	keyword := vars["keyword"]
	
	// Get the timeframe from the query parameters (optional)
	timeframe := r.URL.Query().Get("timeframe")
	
	// Generate the Google Trends URL
	url := client.FormatTrendsURL(keyword, timeframe)
	
	// Create the response
	response := map[string]string{
		"keyword":   keyword,
		"timeframe": timeframe,
		"url":       url,
	}
	
	// Set the content type
	w.Header().Set("Content-Type", "application/json")
	
	// Encode the response as JSON
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode URL response: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetHealth handles requests for health status
func (h *Handler) GetHealth(w http.ResponseWriter, r *http.Request) {
	// Check the health of the Google Trends client
	health, err := h.trendsClient.CheckProxyHealth(r.Context())
	if err != nil {
		// If the health check fails, return a 500 error with the error message
		http.Error(w, "Google Trends proxy health check failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Create a health status response
	status := "healthy"
	if health.Status != "ok" {
		status = "unhealthy"
	}
	
	// Get cache metrics
	hits, misses, ratio := h.trendsClient.GetCacheMetrics()
	requests := h.trendsClient.GetRequestCount()
	
	// Create the response
	response := map[string]interface{}{
		"status":       status,
		"proxyStatus":  health.Status,
		"cacheHits":    hits,
		"cacheMisses":  misses,
		"cacheRatio":   ratio,
		"requestCount": requests,
		"timestamp":    time.Now().Format(time.RFC3339),
	}
	
	// Set the content type
	w.Header().Set("Content-Type", "application/json")
	
	// Encode the response as JSON
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode health response: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// Close closes the Google Trends client
func (h *Handler) Close() {
	if h.trendsClient != nil {
		h.trendsClient.Stop()
	}
}