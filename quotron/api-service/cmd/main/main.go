package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/tiny-ria/quotron/api-service/cmd/server"
)

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

	config := server.Config{
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
	if err := server.RunAPIService(config, stopChan); err != nil {
		log.Fatalf("Error running API: %v", err)
	}
}