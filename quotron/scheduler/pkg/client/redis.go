package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	StockQuoteChannel = "quotron:stocks"
	DefaultRedisAddr  = "localhost:6379"
)

// StockQuote represents a stock quote
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

// RedisClient provides methods for interacting with Redis
type RedisClient struct {
	client *redis.Client
}

// NewRedisClient creates a new Redis client
func NewRedisClient(addr string) *RedisClient {
	if addr == "" {
		addr = DefaultRedisAddr
	}
	
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	
	return &RedisClient{
		client: client,
	}
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// PublishStockQuote publishes a stock quote to Redis
func (r *RedisClient) PublishStockQuote(ctx context.Context, quote *StockQuote) error {
	// Convert to JSON
	data, err := json.Marshal(quote)
	if err != nil {
		return fmt.Errorf("failed to marshal stock quote: %w", err)
	}
	
	// Publish to Redis
	err = r.client.Publish(ctx, StockQuoteChannel, string(data)).Err()
	if err != nil {
		return fmt.Errorf("failed to publish to Redis: %w", err)
	}
	
	return nil
}

// GetSubscriberCount returns the number of subscribers for a channel
func (r *RedisClient) GetSubscriberCount(ctx context.Context, channel string) (int64, error) {
	result, err := r.client.PubSubNumSub(ctx, channel).Result()
	if err != nil {
		return 0, err
	}
	
	return result[channel], nil
}