package economics

import (
	"encoding/json"
	"net/http"
	"time"
	
	"github.com/gorilla/mux"
)

// Handler provides API endpoints for US economic factors data
type Handler struct {
	client EconomicFactorsClient
}

// EconomicFactorsClient defines the interface for an economic factors data client
type EconomicFactorsClient interface {
	GetIndicator(indicator, period string) (interface{}, error)
	GetAllIndicators() (interface{}, error)
	GetSummary() (interface{}, error)
}

// NewHandler creates a new handler for economic factors endpoints
func NewHandler(client EconomicFactorsClient) *Handler {
	return &Handler{
		client: client,
	}
}

// RegisterRoutes registers the API routes for this handler
func (h *Handler) RegisterRoutes(router *mux.Router) {
	// Create API v1 routes for economics
	prefix := "/api/v1/economics"
	router.HandleFunc(prefix+"/indicators", h.GetAllIndicators).Methods("GET")
	router.HandleFunc(prefix+"/indicators/{indicator}", h.GetIndicator).Methods("GET")
	router.HandleFunc(prefix+"/summary", h.GetSummary).Methods("GET")
	router.HandleFunc(prefix+"/dashboard", h.Dashboard).Methods("GET")
}

// GetIndicator returns data for a specific economic indicator
func (h *Handler) GetIndicator(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	indicator := vars["indicator"]
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "5y"
	}

	// Validate indicator
	if indicator == "" {
		http.Error(w, "Indicator parameter is required", http.StatusBadRequest)
		return
	}

	// Validate period
	validPeriods := map[string]bool{
		"1y":  true,
		"5y":  true,
		"10y": true,
		"max": true,
	}

	if _, valid := validPeriods[period]; !valid {
		http.Error(w, "Invalid period parameter. Must be one of: 1y, 5y, 10y, max", http.StatusBadRequest)
		return
	}

	// Get indicator data
	data, err := h.client.GetIndicator(indicator, period)
	if err != nil {
		http.Error(w, "Failed to fetch indicator data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Set content type and return JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	// Convert data to JSON using http.ResponseWriter
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Error encoding response: "+err.Error(), http.StatusInternalServerError)
	}
}

// GetAllIndicators returns list of all available economic indicators
func (h *Handler) GetAllIndicators(w http.ResponseWriter, r *http.Request) {
	// Get indicators list
	data, err := h.client.GetAllIndicators()
	if err != nil {
		errorResponse := map[string]interface{}{
			"error":     "Failed to fetch indicators list",
			"message":   err.Error(),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

// GetSummary returns a summary of key economic indicators
func (h *Handler) GetSummary(w http.ResponseWriter, r *http.Request) {
	// Get economic summary
	data, err := h.client.GetSummary()
	if err != nil {
		errorResponse := map[string]interface{}{
			"error":     "Failed to fetch economic summary",
			"message":   err.Error(),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

// Dashboard redirects to the economic factors dashboard UI
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	// Redirect to the dedicated dashboard UI
	http.Redirect(w, r, "http://localhost:5002/dashboard", http.StatusFound)
}