package server

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
	"strings"
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

// APIHealthReport represents a simplified health status report for the API
type APIHealthReport struct {
	SourceType   string    `json:"source_type"`
	SourceName   string    `json:"source_name"`
	Status       string    `json:"status"`
	LastCheck    time.Time `json:"last_check"`
	ErrorMessage string    `json:"error_message"`
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
	
	// Crypto endpoints
	a.router.HandleFunc("/api/crypto/{symbol}", a.getCryptoQuoteHandler).Methods("GET")
	
	// Root handler for Dashboard UI
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

// getCryptoQuoteHandler returns crypto quote data for a given symbol
func (a *API) getCryptoQuoteHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	symbol := vars["symbol"]

	if symbol == "" {
		respondWithError(w, http.StatusBadRequest, "Missing symbol parameter")
		return
	}

	// Add USD suffix if not already present (BTC -> BTC-USD)
	if !strings.Contains(symbol, "-") {
		symbol = symbol + "-USD"
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Get quote from client manager - we'll use stock quote implementation
	// since the underlying data structure is the same
	quote, err := a.clientManager.GetStockQuote(ctx, symbol)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Error fetching crypto quote: %v", err))
		return
	}

	// Ensure exchange is set to CRYPTO
	quote.Exchange = "CRYPTO"
	
	// Store the result in database
	if err := a.storeQuote(quote); err != nil {
		log.Printf("Error storing crypto quote: %v", err)
		// Continue anyway to return the quote
	}

	// Update data source health
	if err := a.updateDataSourceHealth(quote.Source, "healthy", "Successfully fetched crypto quote"); err != nil {
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
	reports, err := a.getDataSourceHealth()
	if err != nil {
		// Return a more descriptive error message with proper HTTP status code
		respondWithError(w, http.StatusServiceUnavailable, fmt.Sprintf("Error retrieving data source health: %v", err))
		return
	}

	// Even if the list is empty, return a 200 OK with empty array instead of fake data
	respondWithJSON(w, http.StatusOK, reports)
}

// rootHandler serves the dashboard UI directly
func (a *API) rootHandler(w http.ResponseWriter, r *http.Request) {
	// If the request specifically asks for JSON (API info), provide it
	if r.Header.Get("Accept") == "application/json" {
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
				{"path": "/api/crypto/{symbol}", "method": "GET", "description": "Get cryptocurrency quote for a symbol"},
			},
		}
		respondWithJSON(w, http.StatusOK, apiInfo)
		return
	}

	// Otherwise, serve the dashboard UI
	dashboard := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Quotron Dashboard</title>
    <style>
        :root {
            --bg-color: #0d1117;
            --text-color: #c9d1d9;
            --accent-color: #58a6ff;
            --secondary-bg: #161b22;
            --border-color: #30363d;
            --success-color: #3fb950;
            --warning-color: #d29922;
            --error-color: #f85149;
            --font-mono: ui-monospace, SFMono-Regular, SF Mono, Menlo, Consolas, monospace;
        }
        
        body {
            background-color: var(--bg-color);
            color: var(--text-color);
            font-family: var(--font-mono);
            line-height: 1.5;
            margin: 0;
            padding: 20px;
        }
        
        .container {
            max-width: 1200px;
            margin: 0 auto;
        }
        
        header {
            border-bottom: 1px solid var(--border-color);
            padding-bottom: 10px;
            margin-bottom: 20px;
        }
        
        h1, h2, h3 {
            margin-top: 0;
            font-weight: 600;
        }
        
        h1 {
            color: var(--accent-color);
        }
        
        .status-box {
            background-color: var(--secondary-bg);
            border: 1px solid var(--border-color);
            border-radius: 6px;
            padding: 15px;
            margin-bottom: 20px;
        }
        
        .card-section {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
            gap: 25px;
            margin-bottom: 30px;
        }
        
        .card {
            background-color: var(--secondary-bg);
            border: 1px solid var(--border-color);
            border-radius: 6px;
            padding: 20px;
            min-height: 200px;
        }
        
        .card h3 {
            margin-top: 0;
            border-bottom: 1px solid var(--border-color);
            padding-bottom: 10px;
            font-size: 1.1em;
        }
        
        .search-form {
            display: flex;
            margin-bottom: 20px;
        }
        
        input[type="text"] {
            flex-grow: 1;
            background-color: var(--secondary-bg);
            border: 1px solid var(--border-color);
            border-radius: 6px 0 0 6px;
            padding: 8px 12px;
            color: var(--text-color);
            font-family: var(--font-mono);
            margin: 0;
        }
        
        button {
            background-color: var(--accent-color);
            color: black;
            border: none;
            border-radius: 0 6px 6px 0;
            padding: 8px 15px;
            font-family: var(--font-mono);
            cursor: pointer;
            font-weight: 600;
        }
        
        button:hover {
            opacity: 0.9;
        }
        
        pre {
            background-color: var(--bg-color);
            border: 1px solid var(--border-color);
            border-radius: 6px;
            padding: 15px;
            overflow-x: auto;
            margin: 0;
        }
        
        .tag {
            display: inline-block;
            padding: 3px 8px;
            border-radius: 12px;
            font-size: 0.8em;
            margin-right: 5px;
        }
        
        .tag-success {
            background-color: rgba(63, 185, 80, 0.2);
            color: var(--success-color);
            border: 1px solid rgba(63, 185, 80, 0.4);
        }
        
        .tag-warning {
            background-color: rgba(210, 153, 34, 0.2);
            color: var(--warning-color);
            border: 1px solid rgba(210, 153, 34, 0.4);
        }
        
        .tag-error {
            background-color: rgba(248, 81, 73, 0.2);
            color: var(--error-color);
            border: 1px solid rgba(248, 81, 73, 0.4);
        }
        
        table {
            width: 100%;
            border-collapse: collapse;
            margin: 15px 0;
        }
        
        th, td {
            padding: 8px 12px;
            text-align: left;
            border-bottom: 1px solid var(--border-color);
        }
        
        th {
            background-color: var(--bg-color);
            font-weight: 600;
        }
        
        .positive {
            color: var(--success-color);
        }
        
        .negative {
            color: var(--error-color);
        }
        
        .hidden {
            display: none !important;
        }
        
        @media (max-width: 600px) {
            .card-section {
                grid-template-columns: 1fr;
            }
        }
        
        .loader {
            display: inline-block;
            border: 3px solid var(--secondary-bg);
            border-radius: 50%;
            border-top: 3px solid var(--accent-color);
            width: 20px;
            height: 20px;
            margin-left: 10px;
            animation: spin 1s linear infinite;
        }
        
        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }
        
        a {
            text-decoration: none;
            color: var(--accent-color);
        }
        
        a:hover {
            text-decoration: underline;
        }
        
        .symbol-link {
            cursor: pointer;
            display: inline-block;
            margin: 0 2px;
        }

        .price-badge {
            display: inline-block;
            padding: 5px 10px;
            border-radius: 4px;
            font-weight: bold;
            margin-right: 10px;
        }

        .crypto-section {
            margin-top: 30px;
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>Quotron Dashboard</h1>
            <p>Financial Data Monitoring Platform</p>
        </header>
        
        <div class="status-box">
            <div id="service-status">
                <h3>Service Status</h3>
                <div id="status-loading">Loading services status... <div class="loader"></div></div>
                <div id="status-content" class="hidden">
                    <table id="services-table">
                        <thead>
                            <tr>
                                <th>Service</th>
                                <th>Status</th>
                            </tr>
                        </thead>
                        <tbody></tbody>
                    </table>
                </div>
            </div>
        </div>
        
        <div class="card-section">
            <div class="card">
                <h3>Stock Quote Lookup</h3>
                <div class="search-form">
                    <input type="text" id="stock-symbol" placeholder="Enter stock symbol (e.g., AAPL)">
                    <button id="get-quote">Get Quote</button>
                </div>
                <div style="margin-top: 10px; font-size: 0.85em;">
                    Popular symbols: 
                    <a href="#" class="symbol-link" data-type="stock" data-symbol="AAPL">AAPL</a> | 
                    <a href="#" class="symbol-link" data-type="stock" data-symbol="MSFT">MSFT</a> | 
                    <a href="#" class="symbol-link" data-type="stock" data-symbol="GOOGL">GOOGL</a> | 
                    <a href="#" class="symbol-link" data-type="stock" data-symbol="AMZN">AMZN</a> | 
                    <a href="#" class="symbol-link" data-type="stock" data-symbol="TSLA">TSLA</a>
                </div>
                <div id="quote-result" class="hidden">
                    <div id="quote-loading">
                        <div class="loader"></div> Loading data...
                    </div>
                    <div id="quote-content" class="hidden"></div>
                    <pre id="quote-data" class="hidden"></pre>
                </div>
            </div>
            
            <div class="card">
                <h3>Market Indices</h3>
                <div id="indices-loading">
                    <div class="loader"></div> Loading market indices...
                </div>
                <div id="indices-content" class="hidden">
                    <table id="indices-table">
                        <thead>
                            <tr>
                                <th>Index</th>
                                <th>Value</th>
                                <th>Change</th>
                            </tr>
                        </thead>
                        <tbody></tbody>
                    </table>
                </div>
            </div>
        </div>
        
        <div class="crypto-section">
            <h2>Cryptocurrency Quotes</h2>
            <div id="crypto-loading">
                <div class="loader"></div> Loading cryptocurrency data...
            </div>
            <div class="card-section" id="crypto-cards"></div>
        </div>
    </div>

    <script>
        // Utility function to format numbers and changes
        function formatNumber(num) {
            return num.toLocaleString(undefined, {
                minimumFractionDigits: 2,
                maximumFractionDigits: 2
            });
        }
        
        function formatPercent(num) {
            return num.toLocaleString(undefined, {
                minimumFractionDigits: 2,
                maximumFractionDigits: 2
            }) + '%';
        }
        
        function formatChangeClass(num) {
            return num >= 0 ? 'positive' : 'negative';
        }
        
        function formatChangeSymbol(num) {
            return num >= 0 ? '+' : '';
        }
        
        function formatTimestamp(timestamp) {
            const date = new Date(timestamp);
            return date.toLocaleString();
        }
        
        // Function to render a stock quote
        function renderQuote(quote) {
            const content = document.getElementById('quote-content');
            
            // Format quote card
            const changeClass = formatChangeClass(quote.change);
            const changeSymbol = formatChangeSymbol(quote.change);
            
            content.innerHTML = ''+
                '<h4>' + quote.symbol + '</h4>' +
                '<div class="price-badge">' + formatNumber(quote.price) + '</div>' +
                '<span class="' + changeClass + '">' +
                    changeSymbol + formatNumber(quote.change) + ' (' + changeSymbol + formatPercent(quote.changePercent) + ')' +
                '</span>' +
                '<div>' +
                    '<small>Volume: ' + quote.volume.toLocaleString() + '</small>' +
                '</div>' +
                '<div>' +
                    '<small>Exchange: ' + quote.exchange + '</small>' +
                '</div>' +
                '<div>' +
                    '<small>Last updated: ' + formatTimestamp(quote.timestamp) + '</small>' +
                '</div>' +
                '<div>' +
                    '<small>Source: ' + quote.source + '</small>' +
                '</div>' +
            '';
            
            // Format JSON data
            document.getElementById('quote-data').textContent = JSON.stringify(quote, null, 2);
        }
        
        // Function to load and display a stock quote
        function loadQuote(symbol) {
            const resultContainer = document.getElementById('quote-result');
            const loadingElement = document.getElementById('quote-loading');
            const contentElement = document.getElementById('quote-content');
            const dataElement = document.getElementById('quote-data');
            
            // Show loading, hide results
            resultContainer.classList.remove('hidden');
            loadingElement.classList.remove('hidden');
            contentElement.classList.add('hidden');
            dataElement.classList.add('hidden');
            
            fetch('/api/quote/' + symbol)
                .then(function(response) {
                    if (!response.ok) {
                        throw new Error('Error ' + response.status + ': ' + response.statusText);
                    }
                    return response.json();
                })
                .then(function(data) {
                    renderQuote(data);
                    loadingElement.classList.add('hidden');
                    contentElement.classList.remove('hidden');
                    
                    // Uncomment to show raw JSON data
                    // dataElement.classList.remove('hidden');
                })
                .catch(function(err) {
                    loadingElement.classList.add('hidden');
                    contentElement.innerHTML = '<div class="error">Error: ' + err.message + '</div>';
                    contentElement.classList.remove('hidden');
                });
        }
        
        // Function to load and display market indices
        function loadIndices() {
            const indicesLoading = document.getElementById('indices-loading');
            const indicesContent = document.getElementById('indices-content');
            const indicesTable = document.getElementById('indices-table').querySelector('tbody');
            
            indicesLoading.classList.remove('hidden');
            indicesContent.classList.add('hidden');
            
            // List of indices to fetch
            const indices = [
                { symbol: '^GSPC', name: 'S&P 500' },
                { symbol: '^DJI', name: 'Dow Jones' },
                { symbol: '^IXIC', name: 'NASDAQ' },
                { symbol: '^RUT', name: 'Russell 2000' }
            ];
            
            // Fetch all indices in parallel using Promise.all
            Promise.all(indices.map(function(index) {
                return fetch('/api/index/' + index.symbol)
                    .then(function(response) {
                        if (!response.ok) throw new Error('Error fetching ' + index.name);
                        return response.json();
                    })
                    .then(function(data) {
                        data.displayName = index.name;
                        return data;
                    })
                    .catch(function(err) {
                        return { error: err.message, displayName: index.name };
                    });
            }))
            .then(function(results) {
                indicesTable.innerHTML = '';
                
                results.forEach(function(data) {
                    if (data.error) {
                        indicesTable.innerHTML += 
                            '<tr>' +
                                '<td>' + data.displayName + '</td>' +
                                '<td colspan="2" class="error">Error: ' + data.error + '</td>' +
                            '</tr>';
                    } else {
                        const changeClass = formatChangeClass(data.change);
                        const changeSymbol = formatChangeSymbol(data.change);
                        
                        indicesTable.innerHTML += 
                            '<tr>' +
                                '<td>' + data.displayName + '</td>' +
                                '<td>' + formatNumber(data.value) + '</td>' +
                                '<td class="' + changeClass + '">' +
                                    changeSymbol + formatNumber(data.change) + ' (' + changeSymbol + formatPercent(data.changePercent) + ')' +
                                '</td>' +
                            '</tr>';
                    }
                });
                
                indicesLoading.classList.add('hidden');
                indicesContent.classList.remove('hidden');
            })
            .catch(function(err) {
                indicesTable.innerHTML = 
                    '<tr>' +
                        '<td colspan="3" class="error">Error loading indices: ' + err.message + '</td>' +
                    '</tr>';
                indicesLoading.classList.add('hidden');
                indicesContent.classList.remove('hidden');
            });
        }
        
        // Function to load cryptocurrency data
        function loadCryptoData() {
            const cryptoLoading = document.getElementById('crypto-loading');
            const cryptoCards = document.getElementById('crypto-cards');
            
            cryptoLoading.classList.remove('hidden');
            
            // List of cryptocurrencies to display
            const cryptos = [
                { symbol: 'BTC-USD', name: 'Bitcoin' },
                { symbol: 'ETH-USD', name: 'Ethereum' },
                { symbol: 'SOL-USD', name: 'Solana' },
                { symbol: 'DOGE-USD', name: 'Dogecoin' },
                { symbol: 'XRP-USD', name: 'Ripple' }
            ];
            
            // Fetch all cryptocurrencies in parallel
            Promise.all(cryptos.map(function(crypto) {
                return fetch('/api/crypto/' + crypto.symbol)
                    .then(function(response) {
                        if (!response.ok) throw new Error('Error fetching ' + crypto.name);
                        return response.json();
                    })
                    .then(function(data) {
                        data.displayName = crypto.name;
                        return data;
                    })
                    .catch(function(err) {
                        return { 
                            error: err.message, 
                            symbol: crypto.symbol, 
                            displayName: crypto.name 
                        };
                    });
            }))
            .then(function(results) {
                cryptoCards.innerHTML = '';
                
                results.forEach(function(data) {
                    var cardHTML = '<div class="card">';
                    
                    if (data.error) {
                        cardHTML += 
                            '<h3>' + data.displayName + ' (' + data.symbol + ')</h3>' +
                            '<div class="error">Error: ' + data.error + '</div>';
                    } else {
                        const changeClass = formatChangeClass(data.change);
                        const changeSymbol = formatChangeSymbol(data.change);
                        
                        cardHTML += 
                            '<h3>' + data.displayName + ' (' + data.symbol + ')</h3>' +
                            '<div class="price-badge">' + formatNumber(data.price) + '</div>' +
                            '<span class="' + changeClass + '">' +
                                changeSymbol + formatNumber(data.change) + ' (' + changeSymbol + formatPercent(data.changePercent) + ')' +
                            '</span>' +
                            '<div>' +
                                '<small>Volume: ' + (data.volume ? data.volume.toLocaleString() : 'N/A') + '</small>' +
                            '</div>' +
                            '<div>' +
                                '<small>Last updated: ' + formatTimestamp(data.timestamp) + '</small>' +
                            '</div>';
                    }
                    
                    cardHTML += '</div>';
                    cryptoCards.innerHTML += cardHTML;
                });
                
                cryptoLoading.classList.add('hidden');
            })
            .catch(function(err) {
                cryptoCards.innerHTML = 
                    '<div class="card">' +
                        '<h3>Error</h3>' +
                        '<div class="error">Failed to load cryptocurrency data: ' + err.message + '</div>' +
                    '</div>';
                cryptoLoading.classList.add('hidden');
            });
        }
        
        // Function to load service status
        function loadServiceStatus() {
            const statusLoading = document.getElementById('status-loading');
            const statusContent = document.getElementById('status-content');
            const servicesTable = document.getElementById('services-table').querySelector('tbody');
            
            statusLoading.classList.remove('hidden');
            statusContent.classList.add('hidden');
            
            fetch('/api/health')
                .then(function(response) {
                    if (!response.ok) throw new Error('Error ' + response.status);
                    return response.json();
                })
                .then(function(data) {
                    // Prepare services table
                    servicesTable.innerHTML = '';
                    
                    // Add API service status
                    servicesTable.innerHTML += 
                        '<tr>' +
                            '<td>API Service</td>' +
                            '<td><span class="tag tag-success">RUNNING</span></td>' +
                        '</tr>';
                    
                    // Add data sources health
                    if (data.data_sources) {
                        Object.keys(data.data_sources).forEach(function(name) {
                            var status = data.data_sources[name];
                            var statusClass = status === 'healthy' ? 'tag-success' : 'tag-error';
                            var statusText = status === 'healthy' ? 'HEALTHY' : 'UNHEALTHY';
                            
                            servicesTable.innerHTML += 
                                '<tr>' +
                                    '<td>' + name + '</td>' +
                                    '<td><span class="tag ' + statusClass + '">' + statusText + '</span></td>' +
                                '</tr>';
                        });
                    }
                    
                    statusLoading.classList.add('hidden');
                    statusContent.classList.remove('hidden');
                })
                .catch(function(err) {
                    servicesTable.innerHTML = 
                        '<tr>' +
                            '<td colspan="2" class="error">Error loading service status: ' + err.message + '</td>' +
                        '</tr>';
                    statusLoading.classList.add('hidden');
                    statusContent.classList.remove('hidden');
                });
        }
        
        // Initialize the dashboard
        document.addEventListener('DOMContentLoaded', function() {
            // Load all data sections
            loadServiceStatus();
            loadIndices();
            loadCryptoData();
            
            // Set up stock quote lookup
            document.getElementById('get-quote').addEventListener('click', function() {
                var symbol = document.getElementById('stock-symbol').value.trim();
                if (symbol) {
                    loadQuote(symbol);
                }
            });
            
            // Enter key in the symbol input
            document.getElementById('stock-symbol').addEventListener('keypress', function(e) {
                if (e.key === 'Enter') {
                    document.getElementById('get-quote').click();
                }
            });
            
            // Symbol quick links
            document.querySelectorAll('.symbol-link').forEach(function(link) {
                link.addEventListener('click', function(e) {
                    e.preventDefault();
                    var symbol = this.getAttribute('data-symbol');
                    var type = this.getAttribute('data-type');
                    
                    if (type === 'stock') {
                        document.getElementById('stock-symbol').value = symbol;
                        loadQuote(symbol);
                    }
                });
            });
            
            // Set up auto-refresh timer (every 5 minutes)
            setInterval(function() {
                loadIndices();
                loadCryptoData();
                loadServiceStatus();
            }, 300000);
        });
    </script>
</body>
</html>
`
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(dashboard))
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
	case "CRYPTO":
		return "CRYPTO"
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
func (a *API) getDataSourceHealth() ([]APIHealthReport, error) {
	// Use the unified health client to get all data source health reports
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Get health reports from the unified health service
	reports, err := a.healthClient.GetAllHealth(ctx)
	if err != nil {
		log.Printf("Error getting health reports from unified service: %v", err)
		// Return actual error instead of falling back to mock data
		return nil, fmt.Errorf("failed to retrieve health data: %w", err)
	}

	// Filter reports by type and convert to API format
	var sources []APIHealthReport
	for _, report := range reports {
		if report.SourceType == "data_source" {
			sources = append(sources, APIHealthReport{
				SourceType:   report.SourceType,
				SourceName:   report.SourceName,
				Status:       string(report.Status),
				LastCheck:    report.LastCheck,
				ErrorMessage: report.ErrorMessage,
			})
		}
	}
	
	// If no reports were found, return empty list with no error
	// Rather than returning fake data, the API caller will see an empty array
	
	return sources, nil
}

// Removed getMockDataSourceHealth function as part of issue #19
// We now return proper errors instead of mock data

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

	// Initialize a channel to signal when server is stopped
	serverStopped := make(chan struct{})
	
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
	
	// Set up graceful shutdown handler
	go func() {
		// Set up graceful shutdown
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		
		// Block until signal is received
		<-stop
		
		// Perform shutdown
		a.shutdown(server)
		
		close(serverStopped)
	}()
	
	// Report service is running
	log.Printf("Starting API server on port %d", a.config.Port)
	
	// Start the server (this blocks until server is shut down)
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("error starting server: %w", err)
	}
	
	// Wait for server to be fully stopped
	<-serverStopped
	
	return nil
}

// shutdown gracefully stops the server
func (a *API) shutdown(server *http.Server) {
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
		log.Printf("Error during server shutdown: %v", err)
	}
	
	log.Println("Server gracefully stopped")
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

// RunAPIService starts the API service with the given configuration
// This function can be called directly by the CLI for in-process execution
func RunAPIService(config Config, stopChan chan struct{}) error {
	// Create API
	api, err := NewAPI(config)
	if err != nil {
		return fmt.Errorf("error creating API: %w", err)
	}
	
	// Create a goroutine to handle cleanup
	go func() {
		// Wait for stop signal
		<-stopChan
		
		// Close the API resources
		api.Close()
	}()
	
	// Start API (blocking until stopped)
	return api.Start()
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

	// Create stop channel for clean shutdown
	stopChan := make(chan struct{})
	
	// Handle OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	
	// Start signal handler
	go func() {
		<-sigChan
		close(stopChan)
	}()
	
	// Run API service
	if err := RunAPIService(config, stopChan); err != nil {
		log.Fatalf("Error running API: %v", err)
	}
}