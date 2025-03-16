package etl

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
)

const (
	StockChannel = "quotron:stocks"
)

// StockQuote represents a stock quote from various sources
type StockQuote struct {
	Symbol        string    `json:"symbol"`
	Price         float64   `json:"price"`
	Change        float64   `json:"change"`
	ChangePercent float64   `json:"changePercent"`
	Volume        int64     `json:"volume"`
	Timestamp     time.Time `json:"timestamp"`
	Exchange      string    `json:"exchange"`
	Source        string    `json:"source"`
}

// Service provides ETL functionality
type Service struct {
	// Configuration
	redisAddr string
	dbConnStr string
	numWorkers int
	
	// State
	redis *redis.Client
	db    *sql.DB
	
	// Control
	mutex    sync.RWMutex
	running  bool
	cancel   context.CancelFunc
	waitGroup sync.WaitGroup
}

// NewService creates a new ETL service
func NewService(redisAddr, dbConnStr string, numWorkers int) *Service {
	return &Service{
		redisAddr:  redisAddr,
		dbConnStr:  dbConnStr,
		numWorkers: numWorkers,
	}
}

// Start begins the ETL service
func (s *Service) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	if s.running {
		return fmt.Errorf("ETL service is already running")
	}
	
	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	
	// Set up signal handler
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalCh
		log.Println("Received termination signal, shutting down...")
		s.Stop()
	}()
	
	// Connect to Redis
	s.redis = redis.NewClient(&redis.Options{
		Addr: s.redisAddr,
	})
	
	// Test Redis connection
	if _, err := s.redis.Ping(ctx).Result(); err != nil {
		s.redis.Close()
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}
	log.Printf("Connected to Redis at %s", s.redisAddr)
	
	// Connect to database
	var err error
	s.db, err = sql.Open("postgres", s.dbConnStr)
	if err != nil {
		s.redis.Close()
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	
	// Test database connection
	if err := s.db.PingContext(ctx); err != nil {
		s.redis.Close()
		s.db.Close()
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	log.Printf("Connected to database")
	
	// Start workers
	s.running = true
	for i := 0; i < s.numWorkers; i++ {
		workerID := i
		s.waitGroup.Add(1)
		go func() {
			defer s.waitGroup.Done()
			s.runWorker(ctx, workerID)
		}()
	}
	
	log.Printf("ETL service started with %d workers", s.numWorkers)
	return nil
}

// Stop halts the ETL service
func (s *Service) Stop() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	if !s.running {
		return
	}
	
	// Signal workers to stop
	if s.cancel != nil {
		s.cancel()
	}
	
	// Wait for workers to finish
	s.waitGroup.Wait()
	
	// Close connections
	if s.redis != nil {
		s.redis.Close()
		s.redis = nil
	}
	
	if s.db != nil {
		s.db.Close()
		s.db = nil
	}
	
	s.running = false
	log.Printf("ETL service stopped")
}

// IsRunning returns the current service state
func (s *Service) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.running
}

// runWorker processes messages from Redis
func (s *Service) runWorker(ctx context.Context, workerID int) {
	log.Printf("Worker %d: Starting", workerID)
	
	// Create a new Redis client for this worker
	client := redis.NewClient(&redis.Options{
		Addr: s.redisAddr,
	})
	defer client.Close()
	
	// Subscribe to the stock channel
	pubsub := client.Subscribe(ctx, StockChannel)
	defer pubsub.Close()
	
	// Wait for confirmation of subscription
	_, err := pubsub.Receive(ctx)
	if err != nil {
		log.Printf("Worker %d: Failed to subscribe: %v", workerID, err)
		return
	}
	log.Printf("Worker %d: Subscribed to channel %s", workerID, StockChannel)
	
	// Process messages
	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d: Shutting down", workerID)
			return
			
		case msg, ok := <-ch:
			if !ok {
				log.Printf("Worker %d: Channel closed", workerID)
				return
			}
			
			// Parse the message
			var quote StockQuote
			if err := json.Unmarshal([]byte(msg.Payload), &quote); err != nil {
				log.Printf("Worker %d: Failed to parse message: %v", workerID, err)
				continue
			}
			
			// Process the stock quote
			if err := s.processQuote(ctx, &quote); err != nil {
				log.Printf("Worker %d: Error processing quote: %v", workerID, err)
			} else {
				log.Printf("Worker %d: Processed quote for %s: $%.2f", 
					workerID, quote.Symbol, quote.Price)
			}
		}
	}
}

// processQuote handles a single stock quote
func (s *Service) processQuote(ctx context.Context, quote *StockQuote) error {
	// Map exchange to standard format
	exchange := mapExchangeToEnum(quote.Exchange)
	
	// Map source to valid enum value
	source := mapSourceToEnum(quote.Source)
	
	// Store in database
	query := `
		INSERT INTO stock_quotes (
			symbol, price, change, change_percent, volume, timestamp, exchange, source
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	
	_, err := s.db.ExecContext(
		ctx,
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
	
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}
	
	return nil
}

// mapExchangeToEnum standardizes exchange codes
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

// mapSourceToEnum maps source names to valid enum values
func mapSourceToEnum(source string) string {
	switch source {
	case "Yahoo", "Yahoo Finance", "YAHOO":
		return "yahoo-finance"
	case "Alpha Vantage", "ALPHA":
		return "alpha-vantage"
	case "IEX", "IEX Cloud":
		return "iex-cloud"
	case "TEST", "Test":
		return "manual" // Map test data to manual source
	default:
		return "api-scraper" // Default source
	}
}