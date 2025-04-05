package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	// AlertStream is the Redis stream name for price alerts
	AlertStream = "quotron:alerts:stream"
	// StreamMaxLen is the maximum number of messages to keep in the stream
	StreamMaxLen = 1000
	// ConsumerGroup for alert consumers
	AlertConsumerGroup = "quotron:agent:alerts"
)

// AlertMessage represents a price movement alert
type AlertMessage struct {
	Symbol        string    `json:"symbol"`
	Price         float64   `json:"price"`
	PreviousPrice float64   `json:"previousPrice"`
	PercentChange float64   `json:"percentChange"`
	Volume        int64     `json:"volume"`
	Timestamp     time.Time `json:"timestamp"`
	Direction     string    `json:"direction"` // "increased" or "decreased"
}

// QueuePublisher handles publishing messages to Redis streams
type QueuePublisher struct {
	client *redis.Client
	logger *log.Logger
}

// NewQueuePublisher creates a new Redis publisher
func NewQueuePublisher(redisAddr string, logger *log.Logger) (*QueuePublisher, error) {
	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w", redisAddr, err)
	}

	logger.Printf("Connected to Redis at %s", redisAddr)
	return &QueuePublisher{
		client: client,
		logger: logger,
	}, nil
}

// PublishAlert publishes a price alert to the Redis stream
func (p *QueuePublisher) PublishAlert(ctx context.Context, alert AlertMessage) error {
	// Convert to JSON
	data, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("error marshaling alert: %w", err)
	}

	// Create values map for XAdd
	values := map[string]interface{}{
		"data": string(data),
	}

	// Add to stream
	result, err := p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: AlertStream,
		ID:     "*", // Auto-generate ID
		Values: values,
		MaxLen: StreamMaxLen,
	}).Result()

	if err != nil {
		return fmt.Errorf("error publishing to stream: %w", err)
	}

	p.logger.Printf("Published alert for %s with ID: %s", alert.Symbol, result)
	return nil
}

// Close closes the Redis connection
func (p *QueuePublisher) Close() error {
	return p.client.Close()
}

// AlertHandler is a function that processes alert messages
type AlertHandler func(alert AlertMessage) error

// QueueConsumer handles consuming messages from Redis streams
type QueueConsumer struct {
	client  *redis.Client
	logger  *log.Logger
	handler AlertHandler
}

// NewQueueConsumer creates a new Redis consumer
func NewQueueConsumer(redisAddr string, logger *log.Logger, handler AlertHandler) (*QueueConsumer, error) {
	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w", redisAddr, err)
	}

	logger.Printf("Connected to Redis at %s", redisAddr)

	// Create consumer group if it doesn't exist
	_, err = client.XGroupCreateMkStream(ctx, AlertStream, AlertConsumerGroup, "0").Result()
	if err != nil {
		// If group already exists, this is fine
		if err.Error() != "BUSYGROUP Consumer Group name already exists" {
			logger.Printf("Failed to create consumer group: %v", err)
		} else {
			logger.Printf("Using existing consumer group %s for stream %s",
				AlertConsumerGroup, AlertStream)
		}
	} else {
		logger.Printf("Created consumer group %s for stream %s",
			AlertConsumerGroup, AlertStream)
	}

	return &QueueConsumer{
		client:  client,
		logger:  logger,
		handler: handler,
	}, nil
}

// StartConsuming starts consuming messages from the stream
func (c *QueueConsumer) StartConsuming(ctx context.Context, consumerID string) error {
	c.logger.Printf("Starting to consume alerts from Redis Stream %s...", AlertStream)

	// Process incoming messages in a loop
	for {
		select {
		case <-ctx.Done():
			return nil

		default:
			// Read from stream with 2-second blocking timeout
			streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    AlertConsumerGroup,
				Consumer: consumerID,
				Streams:  []string{AlertStream, ">"}, // ">" means only undelivered messages
				Count:    10,                         // Process up to 10 messages at a time
				Block:    2 * time.Second,
			}).Result()

			if err != nil {
				if err == redis.Nil || err.Error() == "redis: nil" {
					// No messages available, just continue
					continue
				}

				// Real error
				c.logger.Printf("Error reading from stream: %v", err)
				time.Sleep(1 * time.Second) // Avoid tight loops on errors
				continue
			}

			// Process messages
			for _, stream := range streams {
				for _, message := range stream.Messages {
					// Extract payload
					data, ok := message.Values["data"].(string)
					if !ok {
						c.logger.Printf("Invalid message format, no data field: %v", message.Values)
						// Acknowledge message even if invalid
						c.client.XAck(ctx, AlertStream, AlertConsumerGroup, message.ID)
						continue
					}

					// Try to parse the message as an alert
					var alert AlertMessage
					err := json.Unmarshal([]byte(data), &alert)
					if err != nil {
						c.logger.Printf("Error parsing message: %v", err)
						c.logger.Printf("Raw message: %s", data)
						// Acknowledge message
						c.client.XAck(ctx, AlertStream, AlertConsumerGroup, message.ID)
						continue
					}

					// Process the alert
					err = c.handler(alert)
					if err != nil {
						c.logger.Printf("Error processing alert: %v", err)
					}

					// Acknowledge message after processing attempt
					c.client.XAck(ctx, AlertStream, AlertConsumerGroup, message.ID)
				}
			}

			// Brief pause to avoid CPU spinning if no messages
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// Close closes the Redis connection
func (c *QueueConsumer) Close() error {
	return c.client.Close()
}
