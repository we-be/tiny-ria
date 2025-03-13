package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/we-be/tiny-ria/quotron/health"
)

// HealthAPI provides the HTTP API for health monitoring
type HealthAPI struct {
	Service *health.HealthService
	Router  *mux.Router
}

// NewHealthAPI creates a new health API with the given service
func NewHealthAPI(service *health.HealthService) *HealthAPI {
	api := &HealthAPI{Service: service}
	
	router := mux.NewRouter()
	router.HandleFunc("/health", api.GetAllHealth).Methods("GET")
	router.HandleFunc("/health/{type}/{name}", api.GetServiceHealth).Methods("GET")
	router.HandleFunc("/health", api.ReportHealth).Methods("POST")
	router.HandleFunc("/health/system", api.GetSystemHealth).Methods("GET")
	
	api.Router = router
	return api
}

// ServeHTTP implements the http.Handler interface
func (api *HealthAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	api.Router.ServeHTTP(w, r)
}

// GetAllHealth returns the health status of all services
func (api *HealthAPI) GetAllHealth(w http.ResponseWriter, r *http.Request) {
	reports, err := api.Service.GetAllHealthStatuses()
	if err != nil {
		log.Printf("Error getting all health statuses: %v", err)
		http.Error(w, "Error getting health statuses", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reports)
}

// GetServiceHealth returns the health status of a specific service
func (api *HealthAPI) GetServiceHealth(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sourceType := vars["type"]
	sourceName := vars["name"]
	
	if sourceType == "" || sourceName == "" {
		http.Error(w, "Source type and name are required", http.StatusBadRequest)
		return
	}
	
	report, err := api.Service.GetHealthForService(sourceType, sourceName)
	if err != nil {
		log.Printf("Error getting health for %s/%s: %v", sourceType, sourceName, err)
		http.Error(w, fmt.Sprintf("Error getting health for %s/%s", sourceType, sourceName), http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

// ReportHealth records a health report from a service
func (api *HealthAPI) ReportHealth(w http.ResponseWriter, r *http.Request) {
	var report health.HealthReport
	
	err := json.NewDecoder(r.Body).Decode(&report)
	if err != nil {
		log.Printf("Error decoding health report: %v", err)
		http.Error(w, "Invalid health report format", http.StatusBadRequest)
		return
	}
	
	// Set the check time to now if not provided
	if report.LastCheck.IsZero() {
		report.LastCheck = time.Now()
	}
	
	err = api.Service.ReportHealth(report)
	if err != nil {
		log.Printf("Error saving health report: %v", err)
		http.Error(w, "Error saving health report", http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusAccepted)
}

// GetSystemHealth returns the overall system health
func (api *HealthAPI) GetSystemHealth(w http.ResponseWriter, r *http.Request) {
	systemHealth, err := api.Service.GetSystemHealth()
	if err != nil {
		log.Printf("Error calculating system health: %v", err)
		http.Error(w, "Error calculating system health", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(systemHealth)
}