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
	"syscall"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/rs/cors"
	"github.com/tiny-ria/quotron/api-service/pkg/client"
)

// Config holds configuration for the API service
type Config struct {
	Port         int
	DatabaseURL  string
	YahooEnabled bool
	AlphaKey     string
	YahooHost    string
	YahooPort    int
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

	router := mux.NewRouter()
	api := &API{
		config:        config,
		db:            db,
		router:        router,
		clientManager: clientManager,
	}

	// Set up routes
	api.setupRoutes()

	return api, nil
}

// setupRoutes configures the API routes
func (a *API) setupRoutes() {
	// Health check
	a.router.HandleFunc("/api/health", a.healthHandler).Methods("GET")

	// Stock quote endpoint
	a.router.HandleFunc("/api/quote/{symbol}", a.getQuoteHandler).Methods("GET")

	// Market index endpoint
	a.router.HandleFunc("/api/index/{index}", a.getIndexHandler).Methods("GET")

	// Data source health endpoint
	a.router.HandleFunc("/api/data-source/health", a.getDataSourceHealthHandler).Methods("GET")
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

// storeQuote stores quote data in the database
func (a *API) storeQuote(quote *client.StockQuote) error {
	if a.db == nil {
		log.Printf("Warning: Database not available, skipping quote storage")
		return nil
	}

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
		quote.Exchange,
		quote.Source,
	)
	return err
}

// storeMarketData stores market index data in the database
func (a *API) storeMarketData(data *client.MarketData) error {
	if a.db == nil {
		log.Printf("Warning: Database not available, skipping market data storage")
		return nil
	}

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
		data.Source,
	)
	return err
}

// updateDataSourceHealth updates the health status of a data source using the unified health service
func (a *API) updateDataSourceHealth(sourceName, status, message string) error {
	// ToDo: Replace this with calls to the unified health service
	// This is left as a stub for backward compatibility
	log.Printf("Health update for %s: %s - %s", sourceName, status, message)
	
	// In a future update, this should use the unified health client to report health
	// Example:
	// import healthClient "github.com/we-be/tiny-ria/quotron/health/client"
	// healthClient := healthClient.NewHealthClient("http://localhost:8085")
	// healthClient.ReportHealth(context.Background(), healthReport)
	
	return nil
}

// getDataSourceHealth retrieves health status for all data sources using the unified health service
func (a *API) getDataSourceHealth() ([]DataSourceHealth, error) {
	// In a future update, this should use the unified health client to get all health reports
	// For now, return mock data to maintain backward compatibility
	mockSources := []DataSourceHealth{
		{
			SourceName: "Yahoo Finance",
			Status:     "healthy",
			LastCheck:  time.Now(),
			Message:    "Transitioning to unified health service",
		},
		{
			SourceName: "Alpha Vantage",
			Status:     "unknown",
			LastCheck:  time.Now(),
			Message:    "Transitioning to unified health service",
		},
	}
	
	// Note that the unified health service provides a much richer API with more detailed health information
	// In a future update, this function should use the unified health client to get health reports
	// and convert them to the legacy DataSourceHealth format
	
	return mockSources, nil
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
	dbURL := flag.String("db", "postgres://quotron:quotron@localhost:5433/quotron?sslmode=disable", "Database connection URL")
	useYahoo := flag.Bool("yahoo", true, "Use Yahoo Finance as data source")
	alphaKey := flag.String("alpha-key", "", "Alpha Vantage API key")
	yahooHost := flag.String("yahoo-host", "localhost", "Yahoo Finance proxy host")
	yahooPort := flag.Int("yahoo-port", 5000, "Yahoo Finance proxy port")
	flag.Parse()

	config := Config{
		Port:         *port,
		DatabaseURL:  *dbURL,
		YahooEnabled: *useYahoo,
		AlphaKey:     *alphaKey,
		YahooHost:    *yahooHost,
		YahooPort:    *yahooPort,
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