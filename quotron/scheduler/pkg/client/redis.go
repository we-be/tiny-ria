package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	StockQuoteStream     = "quotron:stocks:stream"
	CryptoQuoteStream    = "quotron:crypto:stream"
	MarketIndexStream    = "quotron:indices:stream"
	DefaultRedisAddr     = "localhost:6379"
	StreamMaxLen         = 1000                   // Maximum entries to keep in streams
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
	Client *redis.Client // Exported for advanced usage
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
		Client: client,
	}
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	return r.Client.Close()
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
	
	// Add to stream with max length to prevent unbounded growth
	err = r.Client.XAdd(ctx, &redis.XAddArgs{
		Stream: StockQuoteStream,
		ID:     "*", // Auto-generate ID
		Values: values,
		MaxLen: StreamMaxLen, // Limit stream length
		Approx: true,        // Use ~ for better performance
	}).Err()
	
	if err != nil {
		return fmt.Errorf("failed to add to Redis stream: %w", err)
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
	
	// Add to stream with max length to prevent unbounded growth
	err = r.Client.XAdd(ctx, &redis.XAddArgs{
		Stream: CryptoQuoteStream,
		ID:     "*", // Auto-generate ID
		Values: values,
		MaxLen: StreamMaxLen, // Limit stream length
		Approx: true,        // Use ~ for better performance
	}).Err()
	
	if err != nil {
		return fmt.Errorf("failed to add to Redis stream: %w", err)
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
	
	// Add to stream with max length to prevent unbounded growth
	err = r.Client.XAdd(ctx, &redis.XAddArgs{
		Stream: MarketIndexStream,
		ID:     "*", // Auto-generate ID
		Values: values,
		MaxLen: StreamMaxLen, // Limit stream length
		Approx: true,        // Use ~ for better performance
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

// TrimStreams trims all streams to specified length
func (r *RedisClient) TrimStreams(ctx context.Context) error {
	// Trim stock quotes stream
	if err := r.TrimStream(ctx, StockQuoteStream, StreamMaxLen); err != nil {
		return fmt.Errorf("failed to trim stock stream: %w", err)
	}
	
	// Trim crypto quotes stream
	if err := r.TrimStream(ctx, CryptoQuoteStream, StreamMaxLen); err != nil {
		return fmt.Errorf("failed to trim crypto stream: %w", err)
	}
	
	// Trim market index stream
	if err := r.TrimStream(ctx, MarketIndexStream, StreamMaxLen); err != nil {
		return fmt.Errorf("failed to trim index stream: %w", err) 
	}
	
	return nil
}

// TrimStream trims a single stream to the specified length
func (r *RedisClient) TrimStream(ctx context.Context, stream string, maxLen int64) error {
	// First check if stream exists to avoid errors
	_, err := r.Client.XInfoStream(ctx, stream).Result()
	if err != nil {
		if err == redis.Nil {
			// Stream doesn't exist, nothing to trim
			return nil
		}
		return fmt.Errorf("failed to get stream info: %w", err)
	}
	
	// Trim the stream
	err = r.Client.XTrimApprox(ctx, stream, maxLen).Err()
	if err != nil {
		return fmt.Errorf("failed to trim stream: %w", err)
	}
	
	return nil
}