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

	"github.com/redis/go-redis/v9"
	_ "github.com/lib/pq"
)

const (
	// Redis streams configuration
	StockStream     = "quotron:stocks:stream"  // Stock quotes stream
	CryptoStream    = "quotron:crypto:stream"  // Crypto quotes stream
	IndexStream     = "quotron:indices:stream" // Market indices stream
	ConsumerGroup   = "quotron:etl"           // Consumer group name
	StreamMaxLen    = 1000                   // Maximum number of messages to keep in the stream
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

// MarketIndex represents a market index from various sources
type MarketIndex struct {
	IndexName     string    `json:"index_name"`
	Value         float64   `json:"value"`
	Change        float64   `json:"change"`
	ChangePercent float64   `json:"change_percent"`
	Timestamp     time.Time `json:"timestamp"`
	Source        string    `json:"source"`
}

// Service provides ETL functionality
type Service struct {
	// Configuration
	redisAddr   string
	dbConnStr   string
	numWorkers  int
	
	// State
	redis       *redis.Client
	db          *sql.DB
	
	// Control
	mutex       sync.RWMutex
	running     bool
	cancel      context.CancelFunc
	waitGroup   sync.WaitGroup
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
	
	// Initialize Redis Stream and consumer group
	err = s.initializeStream(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize Redis stream: %w", err)
	}
	
	// Start workers
	s.running = true
	
	// Start stream workers
	for i := 0; i < s.numWorkers; i++ {
		workerID := i
		s.waitGroup.Add(1)
		go func() {
			defer s.waitGroup.Done()
			s.runStreamWorker(ctx, workerID)
		}()
	}
	
	log.Printf("ETL service started with %d workers", s.numWorkers)
	return nil
}

// initializeStream initializes the Redis Stream and consumer group
func (s *Service) initializeStream(ctx context.Context) error {
	// Initialize stock stream
	if err := s.initializeSingleStream(ctx, StockStream); err != nil {
		return fmt.Errorf("failed to initialize stock stream: %w", err)
	}
	
	// Initialize crypto stream
	log.Printf("Initializing crypto stream: %s", CryptoStream)
	if err := s.initializeSingleStream(ctx, CryptoStream); err != nil {
		log.Printf("Warning: Could not initialize crypto stream: %v - continuing without it", err)
		// Continue without the crypto stream - don't fail the service
	} else {
		log.Printf("Successfully initialized crypto stream: %s", CryptoStream)
	}
	
	// Initialize market index stream
	if err := s.initializeSingleStream(ctx, IndexStream); err != nil {
		log.Printf("Warning: Could not initialize market index stream: %v - continuing without it", err)
		// Continue without the market index stream - don't fail the service
	}
	
	return nil
}

// initializeSingleStream initializes a single Redis Stream and consumer group
func (s *Service) initializeSingleStream(ctx context.Context, streamName string) error {
	// Check if the stream exists
	streamInfo, err := s.redis.XInfoStream(ctx, streamName).Result()
	if err != nil {
		// Stream doesn't exist, create it with a dummy message
		_, err = s.redis.XAdd(ctx, &redis.XAddArgs{
			Stream: streamName,
			ID:     "*", // Auto-generate ID
			Values: map[string]interface{}{
				"init": "true",
			},
			MaxLen: StreamMaxLen,
		}).Result()
		if err != nil {
			return fmt.Errorf("failed to create stream: %w", err)
		}
		log.Printf("Created Redis stream %s", streamName)
	} else {
		log.Printf("Using existing Redis stream %s, length: %d", streamName, streamInfo.Length)
	}
	
	// Create consumer group if it doesn't exist
	_, err = s.redis.XGroupCreateMkStream(ctx, streamName, ConsumerGroup, "0").Result()
	if err != nil {
		// If group already exists, this is fine
		if !redis.HasErrorPrefix(err, "BUSYGROUP") {
			return fmt.Errorf("failed to create consumer group: %w", err)
		}
		log.Printf("Using existing consumer group %s for stream %s", ConsumerGroup, streamName)
	} else {
		log.Printf("Created consumer group %s for stream %s", ConsumerGroup, streamName)
	}
	
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

// runStreamWorker processes messages from Redis Stream using consumer groups
func (s *Service) runStreamWorker(ctx context.Context, workerID int) {
	log.Printf("Worker %d: Starting", workerID)
	
	// Create a unique consumer name for this worker
	consumerName := fmt.Sprintf("worker-%d", workerID)
	
	// Create a new Redis client for this worker
	client := redis.NewClient(&redis.Options{
		Addr: s.redisAddr,
	})
	defer client.Close()
	
	// Main processing loop
	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d: Shutting down", workerID)
			return
			
		default:
			// Read from stock stream
			if err := s.processStream(ctx, workerID, consumerName, client, StockStream); err != nil {
				log.Printf("Worker %d: Error processing stock stream: %v", workerID, err)
				time.Sleep(1 * time.Second) // Avoid tight loop on persistent errors
			}
			
			// Read from crypto stream
			if err := s.processStream(ctx, workerID, consumerName, client, CryptoStream); err != nil {
				log.Printf("Worker %d: Error processing crypto stream: %v", workerID, err)
				time.Sleep(1 * time.Second) // Avoid tight loop on persistent errors
			}
			
			// Read from market index stream
			if err := s.processStream(ctx, workerID, consumerName, client, IndexStream); err != nil {
				log.Printf("Worker %d: Error processing market index stream: %v", workerID, err)
				time.Sleep(1 * time.Second) // Avoid tight loop on persistent errors
			}
			
			// Brief pause to avoid CPU spinning if no messages
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// processStream processes messages from a single Redis stream
func (s *Service) processStream(ctx context.Context, workerID int, consumerName string, client *redis.Client, streamName string) error {
	// Log attempt to read from stream
	log.Printf("Worker %d: Attempting to read from stream %s", workerID, streamName)
	
	// Read from stream with 2-second blocking timeout
	streams, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    ConsumerGroup,
		Consumer: consumerName,
		Streams:  []string{streamName, ">"}, // ">" means only undelivered messages
		Count:    1,  // Process one message at a time
		Block:    2 * time.Second,
	}).Result()
	
	if err != nil {
		if err == redis.Nil || err.Error() == "redis: nil" {
			// No messages available, just continue
			return nil
		}
		
		// Real error
		return fmt.Errorf("error reading from stream: %w", err)
	}
	
	// Process messages
	log.Printf("Worker %d: Found %d streams from %s", workerID, len(streams), streamName)
	for _, stream := range streams {
		log.Printf("Worker %d: Found %d messages in stream %s", workerID, len(stream.Messages), stream.Stream)
		for _, message := range stream.Messages {
			// Extract payload
			data, ok := message.Values["data"].(string)
			if !ok {
				log.Printf("Worker %d: Invalid message format, no data field", workerID)
				// Acknowledge message even if invalid
				client.XAck(ctx, streamName, ConsumerGroup, message.ID)
				continue
			}
			
			var err error
			// Process based on stream type
			switch streamName {
			case StockStream, CryptoStream:
				// Parse and process stock/crypto quote
				var quote StockQuote
				if err = json.Unmarshal([]byte(data), &quote); err != nil {
					log.Printf("Worker %d: Failed to parse quote message: %v", workerID, err)
				} else {
					err = s.processQuote(ctx, &quote)
					if err == nil {
						log.Printf("Worker %d: Processed quote for %s from %s: $%.2f", 
							workerID, quote.Symbol, streamName, quote.Price)
					}
				}
			case IndexStream:
				// Parse and process market index
				var index MarketIndex
				if err = json.Unmarshal([]byte(data), &index); err != nil {
					log.Printf("Worker %d: Failed to parse market index message: %v", workerID, err)
				} else {
					err = s.processMarketIndex(ctx, &index)
					if err == nil {
						log.Printf("Worker %d: Processed market index %s: %.2f", 
							workerID, index.IndexName, index.Value)
					}
				}
			default:
				log.Printf("Worker %d: Unknown stream type %s", workerID, streamName)
				err = fmt.Errorf("unknown stream type")
			}
			
			if err != nil {
				log.Printf("Worker %d: Error processing message: %v", workerID, err)
				// Don't acknowledge on error - it will be redelivered
			} else {
				// Acknowledge successful processing
				client.XAck(ctx, streamName, ConsumerGroup, message.ID)
			}
		}
	}
	
	return nil
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

// processMarketIndex handles a single market index
func (s *Service) processMarketIndex(ctx context.Context, index *MarketIndex) error {
	// Map source to valid enum value
	source := mapSourceToEnum(index.Source)
	
	// Store in database
	query := `
		INSERT INTO market_indices (
			index_name, value, change, change_percent, timestamp, source
		) VALUES ($1, $2, $3, $4, $5, $6)
	`
	
	_, err := s.db.ExecContext(
		ctx,
		query,
		index.IndexName,
		index.Value,
		index.Change,
		index.ChangePercent,
		index.Timestamp,
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

// ETLPackage provides compatibility with the service_manager.go implementation
type ETLPackage struct {}

// NewService creates a new ETL service (compatibility method for service_manager.go)
func (p *ETLPackage) NewService(redisAddr, dbConnStr string, numWorkers int) *Service {
	return NewService(redisAddr, dbConnStr, numWorkers)
}