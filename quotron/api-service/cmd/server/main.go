package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/rs/cors"
	"github.com/tiny-ria/quotron/api-service/pkg/client"
)

// Config holds configuration for the API service
type Config struct {
	Port           int
	DatabaseURL    string
	YahooEnabled   bool
	AlphaKey       string
	YahooHost      string
	YahooPort      int
	HealthEnabled  bool
	HealthService  string
	ServiceName    string
}

// DataSourceHealth represents health status of data sources
// This is kept for compatibility, but we're transitioning to the unified health service
type DataSourceHealth struct {
	SourceName string    `json:"source_name"`
	Status     string    `json:"status"`
	LastCheck  time.Time `json:"last_check"`
	Message    string    `json:"message"`
}

// API handles the HTTP server and routes
type API struct {
	config        Config
	db            *sql.DB
	router        *mux.Router
	clientManager *client.ClientManager
	healthClient  client.HealthReporter
}

// NewAPI creates a new API instance
func NewAPI(config Config) (*API, error) {
	var db *sql.DB
	var err error
	
	// Try to connect to database, but continue with warnings if it fails
	db, err = sql.Open("postgres", config.DatabaseURL)
	if err != nil {
		log.Printf("Warning: Error connecting to database: %v", err)
		log.Printf("Warning: Continuing without database support")
		// Continue without database
	} else {
		// Verify connection
		if err := db.Ping(); err != nil {
			log.Printf("Warning: Error pinging database: %v", err)
			log.Printf("Warning: Continuing without database support")
			// Continue without database
			db = nil
		}
	}

	// Create data clients
	var primaryClient, secondaryClient client.DataClient
	
	if config.YahooEnabled {
		// Use Yahoo Finance as primary
		primaryClient = client.NewYahooProxyClient(config.YahooHost, config.YahooPort, "api-service", false)
		if config.AlphaKey != "" {
			secondaryClient = client.NewAlphaVantageClient(config.AlphaKey)
		}
	} else if config.AlphaKey != "" {
		// Use Alpha Vantage as primary
		primaryClient = client.NewAlphaVantageClient(config.AlphaKey)
		secondaryClient = client.NewYahooProxyClient(config.YahooHost, config.YahooPort, "api-service", false)
	} else {
		// Default to Yahoo Finance
		primaryClient = client.NewYahooProxyClient(config.YahooHost, config.YahooPort, "api-service", false)
	}

	// If secondary client is nil, use the primary client
	if secondaryClient == nil {
		secondaryClient = primaryClient
	}

	clientManager := client.NewClientManager(primaryClient, secondaryClient)

	// Create health client
	var healthClient client.HealthReporter
	if config.HealthEnabled && config.HealthService != "" {
		healthClient = client.NewUnifiedHealthClient(config.HealthService)
	} else {
		healthClient = client.NewNoopHealthClient()
	}

	router := mux.NewRouter()
	api := &API{
		config:        config,
		db:            db,
		router:        router,
		clientManager: clientManager,
		healthClient:  healthClient,
	}

	// Set up routes
	api.setupRoutes()

	return api, nil
}

// setupRoutes configures the API routes
func (a *API) setupRoutes() {
	// Health check
	a.router.HandleFunc("/api/health", a.healthHandler).Methods("GET")

	// Stock quote endpoints
	a.router.HandleFunc("/api/quote/{symbol}", a.getQuoteHandler).Methods("GET")
	a.router.HandleFunc("/api/quotes/batch", a.getBatchQuotesHandler).Methods("POST")
	a.router.HandleFunc("/api/quotes/history/{symbol}", a.getQuoteHistoryHandler).Methods("GET")

	// Market index endpoints
	a.router.HandleFunc("/api/index/{index}", a.getIndexHandler).Methods("GET")
	a.router.HandleFunc("/api/indices/batch", a.getBatchIndicesHandler).Methods("POST")

	// Data source health endpoint
	a.router.HandleFunc("/api/data-source/health", a.getDataSourceHealthHandler).Methods("GET")
	
	// Root handler for OpenAPI or documentation
	a.router.HandleFunc("/", a.rootHandler).Methods("GET")
}

// healthHandler returns API health status
func (a *API) healthHandler(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":      "ok",
		"timestamp":   time.Now().Format(time.RFC3339),
		"data_sources": a.clientManager.GetClientHealth(),
	}
	respondWithJSON(w, http.StatusOK, health)
}

// getQuoteHandler returns stock quote data for a given symbol
func (a *API) getQuoteHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	symbol := vars["symbol"]

	if symbol == "" {
		respondWithError(w, http.StatusBadRequest, "Missing symbol parameter")
		return
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Get quote from client manager
	quote, err := a.clientManager.GetStockQuote(ctx, symbol)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Error fetching quote: %v", err))
		return
	}

	// Store the result in database
	if err := a.storeQuote(quote); err != nil {
		log.Printf("Error storing quote: %v", err)
		// Continue anyway to return the quote
	}

	// Update data source health
	if err := a.updateDataSourceHealth(quote.Source, "healthy", "Successfully fetched quote"); err != nil {
		log.Printf("Error updating data source health: %v", err)
	}

	respondWithJSON(w, http.StatusOK, quote)
}

// getIndexHandler returns market index data for a given index
func (a *API) getIndexHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	index := vars["index"]

	if index == "" {
		respondWithError(w, http.StatusBadRequest, "Missing index parameter")
		return
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Get market data from client manager
	marketData, err := a.clientManager.GetMarketData(ctx, index)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Error fetching market data: %v", err))
		return
	}

	// Store the result in database
	if err := a.storeMarketData(marketData); err != nil {
		log.Printf("Error storing market data: %v", err)
		// Continue anyway to return the data
	}

	// Update data source health
	if err := a.updateDataSourceHealth(marketData.Source, "healthy", "Successfully fetched market data"); err != nil {
		log.Printf("Error updating data source health: %v", err)
	}

	respondWithJSON(w, http.StatusOK, marketData)
}

// getDataSourceHealthHandler returns health status of all data sources
func (a *API) getDataSourceHealthHandler(w http.ResponseWriter, r *http.Request) {
	sources, err := a.getDataSourceHealth()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error retrieving data source health")
		return
	}

	respondWithJSON(w, http.StatusOK, sources)
}

// rootHandler serves the API documentation or redirects to the documentation
func (a *API) rootHandler(w http.ResponseWriter, r *http.Request) {
	apiInfo := map[string]interface{}{
		"name":        "Quotron API Service",
		"version":     "1.0.0",
		"description": "Financial data API service providing stock quotes and market indices",
		"endpoints": []map[string]string{
			{"path": "/api/health", "method": "GET", "description": "Get API health status"},
			{"path": "/api/quote/{symbol}", "method": "GET", "description": "Get stock quote for a symbol"},
			{"path": "/api/quotes/batch", "method": "POST", "description": "Get batch stock quotes"},
			{"path": "/api/quotes/history/{symbol}", "method": "GET", "description": "Get quote history for a symbol"},
			{"path": "/api/index/{index}", "method": "GET", "description": "Get market index data"},
			{"path": "/api/indices/batch", "method": "POST", "description": "Get batch market indices"},
			{"path": "/api/data-source/health", "method": "GET", "description": "Get data source health"},
		},
	}
	respondWithJSON(w, http.StatusOK, apiInfo)
}

// BatchQuoteRequest represents a request for batch quotes
type BatchQuoteRequest struct {
	Symbols []string `json:"symbols"`
}

// BatchQuoteResponse represents the response for batch quotes
type BatchQuoteResponse struct {
	Quotes []*client.StockQuote `json:"quotes"`
	Errors map[string]string    `json:"errors,omitempty"`
}

// getBatchQuotesHandler returns stock quotes for multiple symbols
func (a *API) getBatchQuotesHandler(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var request BatchQuoteRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request format")
		return
	}
	
	if len(request.Symbols) == 0 {
		respondWithError(w, http.StatusBadRequest, "No symbols provided")
		return
	}
	
	// Limit number of symbols to prevent abuse
	if len(request.Symbols) > 20 {
		respondWithError(w, http.StatusBadRequest, "Too many symbols (maximum 20)")
		return
	}
	
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	
	// Process quotes concurrently
	var quotes []*client.StockQuote
	errors := make(map[string]string)
	
	// Use a wait group to wait for all goroutines to complete
	var wg sync.WaitGroup
	var mutex sync.Mutex
	
	for _, symbol := range request.Symbols {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			
			quote, err := a.clientManager.GetStockQuote(ctx, sym)
			
			mutex.Lock()
			defer mutex.Unlock()
			
			if err != nil {
				errors[sym] = err.Error()
			} else {
				quotes = append(quotes, quote)
				
				// Store the result in database (non-blocking)
				go func(q *client.StockQuote) {
					if err := a.storeQuote(q); err != nil {
						log.Printf("Error storing quote for %s: %v", q.Symbol, err)
					}
				}(quote)
				
				// Update data source health (non-blocking)
				go func(q *client.StockQuote) {
					if err := a.updateDataSourceHealth(q.Source, "healthy", "Successfully fetched quote"); err != nil {
						log.Printf("Error updating data source health: %v", err)
					}
				}(quote)
			}
		}(symbol)
	}
	
	// Wait for all goroutines to complete
	wg.Wait()
	
	response := BatchQuoteResponse{
		Quotes: quotes,
	}
	
	if len(errors) > 0 {
		response.Errors = errors
	}
	
	respondWithJSON(w, http.StatusOK, response)
}

// getQuoteHistoryHandler returns historical data for a symbol
func (a *API) getQuoteHistoryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	symbol := vars["symbol"]
	
	if symbol == "" {
		respondWithError(w, http.StatusBadRequest, "Missing symbol parameter")
		return
	}
	
	// Get query parameters
	days := 7 // Default to 7 days
	if daysParam := r.URL.Query().Get("days"); daysParam != "" {
		if d, err := strconv.Atoi(daysParam); err == nil && d > 0 && d <= 30 {
			days = d
		}
	}
	
	// Check if database is available
	if a.db == nil {
		respondWithError(w, http.StatusServiceUnavailable, "Database not available for historical data")
		return
	}
	
	// Query the database for historical data
	query := `
		SELECT symbol, price, change, change_percent, volume, timestamp, exchange, source
		FROM stock_quotes
		WHERE symbol = $1 AND timestamp > NOW() - INTERVAL '1 day' * $2
		ORDER BY timestamp DESC
	`
	
	rows, err := a.db.Query(query, symbol, days)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Error querying database: %v", err))
		return
	}
	defer rows.Close()
	
	var quotes []*client.StockQuote
	
	for rows.Next() {
		quote := &client.StockQuote{}
		err := rows.Scan(
			&quote.Symbol,
			&quote.Price,
			&quote.Change,
			&quote.ChangePercent,
			&quote.Volume,
			&quote.Timestamp,
			&quote.Exchange,
			&quote.Source,
		)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Error scanning row: %v", err))
			return
		}
		
		quotes = append(quotes, quote)
	}
	
	if err := rows.Err(); err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Error iterating rows: %v", err))
		return
	}
	
	if len(quotes) == 0 {
		// If no historical data, try to get current quote
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		
		quote, err := a.clientManager.GetStockQuote(ctx, symbol)
		if err != nil {
			respondWithError(w, http.StatusNotFound, fmt.Sprintf("No historical data found for symbol: %s", symbol))
			return
		}
		
		quotes = append(quotes, quote)
	}
	
	respondWithJSON(w, http.StatusOK, quotes)
}

// BatchIndicesRequest represents a request for batch market indices
type BatchIndicesRequest struct {
	Indices []string `json:"indices"`
}

// BatchIndicesResponse represents the response for batch indices
type BatchIndicesResponse struct {
	Indices []*client.MarketData `json:"indices"`
	Errors  map[string]string    `json:"errors,omitempty"`
}

// getBatchIndicesHandler returns data for multiple market indices
func (a *API) getBatchIndicesHandler(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var request BatchIndicesRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request format")
		return
	}
	
	if len(request.Indices) == 0 {
		respondWithError(w, http.StatusBadRequest, "No indices provided")
		return
	}
	
	// Limit number of indices to prevent abuse
	if len(request.Indices) > 10 {
		respondWithError(w, http.StatusBadRequest, "Too many indices (maximum 10)")
		return
	}
	
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	
	// Process indices concurrently
	var indices []*client.MarketData
	errors := make(map[string]string)
	
	// Use a wait group to wait for all goroutines to complete
	var wg sync.WaitGroup
	var mutex sync.Mutex
	
	for _, indexName := range request.Indices {
		wg.Add(1)
		go func(idx string) {
			defer wg.Done()
			
			data, err := a.clientManager.GetMarketData(ctx, idx)
			
			mutex.Lock()
			defer mutex.Unlock()
			
			if err != nil {
				errors[idx] = err.Error()
			} else {
				indices = append(indices, data)
				
				// Store the result in database (non-blocking)
				go func(d *client.MarketData) {
					if err := a.storeMarketData(d); err != nil {
						log.Printf("Error storing market data for %s: %v", d.IndexName, err)
					}
				}(data)
				
				// Update data source health (non-blocking)
				go func(d *client.MarketData) {
					if err := a.updateDataSourceHealth(d.Source, "healthy", "Successfully fetched market data"); err != nil {
						log.Printf("Error updating data source health: %v", err)
					}
				}(data)
			}
		}(indexName)
	}
	
	// Wait for all goroutines to complete
	wg.Wait()
	
	response := BatchIndicesResponse{
		Indices: indices,
	}
	
	if len(errors) > 0 {
		response.Errors = errors
	}
	
	respondWithJSON(w, http.StatusOK, response)
}

// storeQuote stores quote data in the database
func (a *API) storeQuote(quote *client.StockQuote) error {
	if a.db == nil {
		log.Printf("Warning: Database not available, skipping quote storage")
		return nil
	}

	// Map exchange to match database enum values
	// Exchange enum is ('NYSE', 'NASDAQ', 'AMEX', 'OTC', 'OTHER')
	exchange := mapExchangeToEnum(quote.Exchange)

	// Map source to match database enum values
	source := mapSourceToEnum(quote.Source)

	query := `
		INSERT INTO stock_quotes (
			symbol, price, change, change_percent, volume, timestamp, exchange, source
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := a.db.Exec(
		query,
		quote.Symbol,
		quote.Price,
		quote.Change,
		quote.ChangePercent,
		quote.Volume,
		quote.Timestamp,
		exchange,
		source,
	)
	return err
}

// mapExchangeToEnum maps various exchange codes to the database enum values
func mapExchangeToEnum(exchange string) string {
	switch exchange {
	case "NYSE":
		return "NYSE"
	case "NASDAQ", "NMS", "NGS", "NAS", "NCM":
		return "NASDAQ"
	case "AMEX", "ASE", "CBOE":
		return "AMEX"
	case "OTC", "OTCBB", "OTC PINK":
		return "OTC"
	default:
		return "OTHER"
	}
}

// storeMarketData stores market index data in the database
func (a *API) storeMarketData(data *client.MarketData) error {
	if a.db == nil {
		log.Printf("Warning: Database not available, skipping market data storage")
		return nil
	}

	// Map source to match database enum values
	// data_source enum is ('api-scraper', 'browser-scraper', 'manual')
	source := mapSourceToEnum(data.Source)

	query := `
		INSERT INTO market_indices (
			index_name, value, change, change_percent, timestamp, source
		) VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := a.db.Exec(
		query,
		data.IndexName,
		data.Value,
		data.Change,
		data.ChangePercent,
		data.Timestamp,
		source,
	)
	return err
}

// mapSourceToEnum maps various data sources to the database enum values
func mapSourceToEnum(source string) string {
	switch source {
	case "Alpha Vantage", "Yahoo Finance", "IEX Cloud":
		return "api-scraper"
	case "Browser Scraper", "Selenium", "Playwright":
		return "browser-scraper"
	default:
		return "manual"
	}
}

// updateDataSourceHealth updates the health status of a data source using the unified health service
func (a *API) updateDataSourceHealth(sourceName, status, message string) error {
	// Convert legacy status to unified health status
	healthStatus := client.LegacyToUnifiedHealth(status)
	
	// Use the health client to report the health status
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err := a.healthClient.ReportHealth(ctx, "data_source", sourceName, healthStatus, message)
	if err != nil {
		log.Printf("Error reporting health status: %v", err)
	}
	
	// For backward compatibility, log the health update
	log.Printf("Health update for %s: %s - %s", sourceName, status, message)
	
	return err
}

// getDataSourceHealth retrieves health status for all data sources using the unified health service
func (a *API) getDataSourceHealth() ([]DataSourceHealth, error) {
	// Use the unified health client to get all data source health reports
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Try to get health reports from the unified health service
	reports, err := a.healthClient.GetAllHealth(ctx)
	if err != nil {
		log.Printf("Error getting health reports from unified service: %v", err)
		// Fall back to mock data on error
		return a.getMockDataSourceHealth(), nil
	}

	// Filter reports by type and convert to legacy format
	var sources []DataSourceHealth
	for _, report := range reports {
		if report.SourceType == "data_source" {
			sources = append(sources, DataSourceHealth{
				SourceName: report.SourceName,
				Status:     client.UnifiedToLegacyHealth(report.Status),
				LastCheck:  report.LastCheck,
				Message:    report.ErrorMessage,
			})
		}
	}
	
	// If no reports were found, return mock data
	if len(sources) == 0 {
		return a.getMockDataSourceHealth(), nil
	}
	
	return sources, nil
}

// getMockDataSourceHealth returns mock health data for backward compatibility
func (a *API) getMockDataSourceHealth() []DataSourceHealth {
	// Use client health data if available
	clientHealth := a.clientManager.GetClientHealth()
	
	sources := []DataSourceHealth{}
	for name, status := range clientHealth {
		sources = append(sources, DataSourceHealth{
			SourceName: name,
			Status:     status,
			LastCheck:  time.Now(),
			Message:    "Using local client health data",
		})
	}
	
	// If no clients were found, return default mock data
	if len(sources) == 0 {
		sources = []DataSourceHealth{
			{
				SourceName: "Yahoo Finance",
				Status:     "unknown",
				LastCheck:  time.Now(),
				Message:    "No health data available",
			},
			{
				SourceName: "Alpha Vantage",
				Status:     "unknown",
				LastCheck:  time.Now(),
				Message:    "No health data available",
			},
		}
	}
	
	return sources
}

// Start starts the HTTP server
func (a *API) Start() error {
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})

	handlerWithCORS := c.Handler(a.router)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", a.config.Port),
		Handler:      handlerWithCORS,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting API server on port %d", a.config.Port)
		
		// Report service startup to health service
		if a.config.HealthEnabled {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			err := a.healthClient.ReportHealth(
				ctx, 
				"service", 
				a.config.ServiceName, 
				client.LegacyToUnifiedHealth("healthy"),
				"Service started successfully",
			)
			if err != nil {
				log.Printf("Warning: Failed to report service startup to health service: %v", err)
			}
		}
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error starting server: %v", err)
		}
	}()

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Block until signal is received
	<-stop

	log.Println("Shutting down server...")
	
	// Report service shutdown to health service
	if a.config.HealthEnabled {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := a.healthClient.ReportHealth(
			ctx, 
			"service", 
			a.config.ServiceName, 
			client.LegacyToUnifiedHealth("unhealthy"),
			"Service shutting down",
		)
		cancel()
		if err != nil {
			log.Printf("Warning: Failed to report service shutdown to health service: %v", err)
		}
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("error during server shutdown: %w", err)
	}

	log.Println("Server gracefully stopped")
	return nil
}

// Close closes the database connection
func (a *API) Close() error {
	if a.db != nil {
		return a.db.Close()
	}
	return nil
}

// Helper function to respond with JSON
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error marshaling JSON"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

// Helper function to respond with error
func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func main() {
	// Parse command line flags
	port := flag.Int("port", 8080, "API server port")
	dbURL := flag.String("db", "postgres://postgres:postgres@localhost:5432/quotron?sslmode=disable", "Database connection URL")
	useYahoo := flag.Bool("yahoo", true, "Use Yahoo Finance as data source")
	alphaKey := flag.String("alpha-key", "", "Alpha Vantage API key")
	yahooHost := flag.String("yahoo-host", "localhost", "Yahoo Finance proxy host")
	yahooPort := flag.Int("yahoo-port", 5000, "Yahoo Finance proxy port")
	useHealth := flag.Bool("health", false, "Enable unified health reporting")
	healthSvc := flag.String("health-service", "", "Unified health service URL (empty to disable)")
	svcName := flag.String("name", "api-service", "Service name for health reporting")
	flag.Parse()

	config := Config{
		Port:           *port,
		DatabaseURL:    *dbURL,
		YahooEnabled:   *useYahoo,
		AlphaKey:       *alphaKey,
		YahooHost:      *yahooHost,
		YahooPort:      *yahooPort,
		HealthEnabled:  *useHealth,
		HealthService:  *healthSvc,
		ServiceName:    *svcName,
	}

	// Create and start API
	api, err := NewAPI(config)
	if err != nil {
		log.Fatalf("Error creating API: %v", err)
	}
	defer api.Close()

	if err := api.Start(); err != nil {
		log.Fatalf("Error running API: %v", err)
	}
}