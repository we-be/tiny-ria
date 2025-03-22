package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	StockQuoteChannel    = "quotron:stocks"
	CryptoQuoteChannel   = "quotron:crypto"
	MarketIndexChannel   = "quotron:indices"
	StockQuoteStream     = "quotron:stocks:stream"
	CryptoQuoteStream    = "quotron:crypto:stream"
	MarketIndexStream    = "quotron:indices:stream"
	DefaultRedisAddr     = "localhost:6379"
)

// QuoteData represents a quote data structure used in Redis
type QuoteData struct {
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

// PublishStockQuote publishes a stock quote to Redis PubSub
func (r *RedisClient) PublishStockQuote(ctx context.Context, quote *QuoteData) error {
	// Convert to JSON
	data, err := json.Marshal(quote)
	if err != nil {
		return fmt.Errorf("failed to marshal stock quote: %w", err)
	}
	
	// Publish to Redis PubSub
	err = r.client.Publish(ctx, StockQuoteChannel, string(data)).Err()
	if err != nil {
		return fmt.Errorf("failed to publish to Redis: %w", err)
	}
	
	return nil
}

// PublishToStockStream publishes a stock quote to Redis Stream
func (r *RedisClient) PublishToStockStream(ctx context.Context, quote *QuoteData) error {
	// Convert to JSON
	data, err := json.Marshal(quote)
	if err != nil {
		return fmt.Errorf("failed to marshal stock quote for stream: %w", err)
	}
	
	// Create values map for XAdd
	values := map[string]interface{}{
		"data": string(data),
	}
	
	// Add to stream
	err = r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: StockQuoteStream,
		ID:     "*", // Auto-generate ID
		Values: values,
	}).Err()
	
	if err != nil {
		return fmt.Errorf("failed to add to Redis stream: %w", err)
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

// PublishCryptoQuote publishes a cryptocurrency quote to Redis PubSub
func (r *RedisClient) PublishCryptoQuote(ctx context.Context, quote *QuoteData) error {
	// Convert to JSON
	data, err := json.Marshal(quote)
	if err != nil {
		return fmt.Errorf("failed to marshal crypto quote: %w", err)
	}
	
	// Publish to Redis PubSub
	err = r.client.Publish(ctx, CryptoQuoteChannel, string(data)).Err()
	if err != nil {
		return fmt.Errorf("failed to publish to Redis PubSub: %w", err)
	}
	
	return nil
}

// PublishToCryptoStream publishes a cryptocurrency quote to Redis Stream
func (r *RedisClient) PublishToCryptoStream(ctx context.Context, quote *QuoteData) error {
	// Convert to JSON
	data, err := json.Marshal(quote)
	if err != nil {
		return fmt.Errorf("failed to marshal crypto quote for stream: %w", err)
	}
	
	// Create values map for XAdd
	values := map[string]interface{}{
		"data": string(data),
	}
	
	// Add to stream
	err = r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: CryptoQuoteStream,
		ID:     "*", // Auto-generate ID
		Values: values,
	}).Err()
	
	if err != nil {
		return fmt.Errorf("failed to add to Redis stream: %w", err)
	}
	
	return nil
}

// PublishMarketIndex publishes a market index to Redis PubSub
func (r *RedisClient) PublishMarketIndex(ctx context.Context, marketData *MarketData) error {
	// Convert to JSON
	data, err := json.Marshal(marketData)
	if err != nil {
		return fmt.Errorf("failed to marshal market index data: %w", err)
	}
	
	// Publish to Redis PubSub
	err = r.client.Publish(ctx, MarketIndexChannel, string(data)).Err()
	if err != nil {
		return fmt.Errorf("failed to publish to Redis PubSub: %w", err)
	}
	
	return nil
}

// PublishToMarketIndexStream publishes a market index to Redis Stream
func (r *RedisClient) PublishToMarketIndexStream(ctx context.Context, marketData *MarketData) error {
	// Convert to JSON
	data, err := json.Marshal(marketData)
	if err != nil {
		return fmt.Errorf("failed to marshal market index data for stream: %w", err)
	}
	
	// Create values map for XAdd
	values := map[string]interface{}{
		"data": string(data),
	}
	
	// Add to stream
	err = r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: MarketIndexStream,
		ID:     "*", // Auto-generate ID
		Values: values,
	}).Err()
	
	if err != nil {
		return fmt.Errorf("failed to add to Redis stream: %w", err)
	}
	
	return nil
}

// StockQuoteToQuoteData converts a StockQuote to QuoteData
func StockQuoteToQuoteData(quote *StockQuote) *QuoteData {
	return &QuoteData{
		Symbol:        quote.Symbol,
		Price:         quote.Price,
		Change:        quote.Change,
		ChangePercent: quote.ChangePercent,
		Volume:        quote.Volume,
		Timestamp:     quote.Timestamp,
		Exchange:      quote.Exchange,
		Source:        quote.Source,
	}
}