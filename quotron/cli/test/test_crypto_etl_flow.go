package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Redis streams
	StockStream    = "quotron:stocks:stream"   // Stream name for stocks
	CryptoStream   = "quotron:crypto:stream"   // Stream name for crypto
	IndexStream    = "quotron:indices:stream"  // Stream name for market indices
	ConsumerGroup  = "quotron:etl"            // Consumer group name
	StreamMaxLen   = 1000                    // Maximum number of messages to keep in the stream
	RedisAddr      = "localhost:6379"
)

// CryptoQuote represents a cryptocurrency quote
type CryptoQuote struct {
	Symbol        string    `json:"symbol"`
	Price         float64   `json:"price"`
	Change        float64   `json:"change"`
	ChangePercent float64   `json:"changePercent"`
	Volume        int64     `json:"volume"`
	Timestamp     time.Time `json:"timestamp"`
	Exchange      string    `json:"exchange"` // Always "CRYPTO" for cryptocurrencies
	Source        string    `json:"source"`   // Source of the data (e.g., "yahoo-finance")
}

// PublisherClient publishes crypto quotes to Redis
type PublisherClient struct {
	redis *redis.Client
}

// NewPublisherClient creates a new Redis publisher client
func NewPublisherClient() (*PublisherClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr: RedisAddr,
	})

	// Verify connection
	ctx := context.Background()
	if _, err := client.Ping(ctx).Result(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &PublisherClient{
		redis: client,
	}, nil
}

// Close closes the Redis connection
func (p *PublisherClient) Close() {
	if p.redis != nil {
		p.redis.Close()
	}
}

// PublishCryptoQuote publishes a crypto quote to Redis Stream
func (p *PublisherClient) PublishCryptoQuote(ctx context.Context, quote *CryptoQuote) error {
	// Marshal quote to JSON
	data, err := json.Marshal(quote)
	if err != nil {
		return fmt.Errorf("error marshaling quote: %w", err)
	}

	// Add to stream
	result, err := p.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: CryptoStream,
		ID:     "*", // Auto-generate ID
		Values: map[string]interface{}{
			"data": string(data),
		},
		MaxLen: StreamMaxLen,
	}).Result()

	if err != nil {
		return fmt.Errorf("error publishing to Stream: %w", err)
	}
	log.Printf("Published to Redis Stream %s with ID %s", CryptoStream, result)

	return nil
}

// ConsumerClient receives and processes crypto quotes from Redis
type ConsumerClient struct {
	redis      *redis.Client
	consumerID string
}

// NewConsumerClient creates a new Redis consumer client
func NewConsumerClient(consumerID string) (*ConsumerClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr: RedisAddr,
	})

	// Verify connection
	ctx := context.Background()
	if _, err := client.Ping(ctx).Result(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &ConsumerClient{
		redis:      client,
		consumerID: consumerID,
	}, nil
}

// Close closes the Redis connection
func (c *ConsumerClient) Close() {
	if c.redis != nil {
		c.redis.Close()
	}
}

// InitializeStream initializes the Redis stream and consumer group
func (c *ConsumerClient) InitializeStream(ctx context.Context, streamName string) error {
	// Check if the stream exists
	streamInfo, err := c.redis.XInfoStream(ctx, streamName).Result()
	if err != nil {
		// Stream doesn't exist, create it with a dummy message
		_, err = c.redis.XAdd(ctx, &redis.XAddArgs{
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
	_, err = c.redis.XGroupCreateMkStream(ctx, streamName, ConsumerGroup, "0").Result()
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

// ConsumeStream consumes messages from a Redis stream
func (c *ConsumerClient) ConsumeStream(ctx context.Context, streamName string) error {
	// Read from stream with 1-second blocking timeout
	streams, err := c.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    ConsumerGroup,
		Consumer: c.consumerID,
		Streams:  []string{streamName, ">"}, // ">" means only undelivered messages
		Count:    1,                         // Process one message at a time
		Block:    1 * time.Second,
	}).Result()

	if err != nil {
		if err == redis.Nil || err.Error() == "redis: nil" {
			// No messages available, this is normal
			return nil
		}
		return fmt.Errorf("error reading from stream: %w", err)
	}

	// Process messages
	for _, stream := range streams {
		for _, message := range stream.Messages {
			// Extract payload
			data, ok := message.Values["data"].(string)
			if !ok {
				// Acknowledge invalid message to prevent redelivery
				c.redis.XAck(ctx, streamName, ConsumerGroup, message.ID)
				continue
			}

			// Parse the message
			var quote CryptoQuote
			err := json.Unmarshal([]byte(data), &quote)
			if err != nil {
				log.Printf("Error parsing message: %v", err)
				// Don't acknowledge on error - message will be redelivered
				continue
			}

			// In a real application, we would store this in the database
			log.Printf("Stream: Received crypto quote for %s, price: $%.2f", quote.Symbol, quote.Price)

			// Acknowledge message
			c.redis.XAck(ctx, streamName, ConsumerGroup, message.ID)
		}
	}

	return nil
}

// Main function to show the flow and relationship between scheduler and ETL
func main() {
	log.Println("Starting ETL flow test...")

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handler
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalCh
		log.Println("Received termination signal, shutting down...")
		cancel()
	}()

	// Create publisher (simulates the scheduler publishing quotes)
	publisher, err := NewPublisherClient()
	if err != nil {
		log.Fatalf("Failed to create publisher: %v", err)
	}
	defer publisher.Close()

	// Create consumer (simulates the ETL service)
	consumer, err := NewConsumerClient("etl-test-consumer")
	if err != nil {
		log.Fatalf("Failed to create consumer: %v", err)
	}
	defer consumer.Close()

	// Initialize stream
	if err := consumer.InitializeStream(ctx, CryptoStream); err != nil {
		log.Printf("Warning: Failed to initialize stream: %v", err)
	}

	// Generate some test quotes
	go func() {
		symbols := []string{"BTC-USD", "ETH-USD", "SOL-USD", "ADA-USD"}
		i := 0

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Generate a test quote
				symbol := symbols[i%len(symbols)]
				i++

				quote := &CryptoQuote{
					Symbol:        symbol,
					Price:         float64(20000 + i*100),
					Change:        float64(i * 5),
					ChangePercent: 0.5,
					Volume:        int64(1000000 + i),
					Timestamp:     time.Now(),
					Exchange:      "CRYPTO",
					Source:        "TEST",
				}

				// Publish the quote
				if err := publisher.PublishCryptoQuote(ctx, quote); err != nil {
					log.Printf("Error publishing quote: %v", err)
				} else {
					log.Printf("Published test quote for %s: $%.2f", symbol, quote.Price)
				}
			}
		}
	}()

	// Main loop for consuming from Stream
	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down...")
			return

		default:
			// Consume from stream
			if err := consumer.ConsumeStream(ctx, CryptoStream); err != nil {
				log.Printf("Error consuming from stream: %v", err)
			}

			// Brief pause to avoid CPU spinning
			time.Sleep(100 * time.Millisecond)
		}
	}
}