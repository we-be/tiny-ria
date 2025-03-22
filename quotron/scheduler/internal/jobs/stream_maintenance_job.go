package jobs

import (
	"context"
	"log"
	"time"

	"github.com/we-be/tiny-ria/quotron/scheduler/pkg/client"
)

// StreamMaintenanceJob handles Redis stream maintenance
type StreamMaintenanceJob struct {
	BaseJob
	redisAddr string
	lastRun   time.Time
}

// NewStreamMaintenanceJob creates a new stream maintenance job
func NewStreamMaintenanceJob(redisAddr string) *StreamMaintenanceJob {
	if redisAddr == "" {
		redisAddr = client.DefaultRedisAddr
	}
	
	return &StreamMaintenanceJob{
		BaseJob:   NewBaseJob("stream_maintenance", "Redis Stream Maintenance"),
		redisAddr: redisAddr,
	}
}

// Execute implements the Job interface
func (j *StreamMaintenanceJob) Execute(ctx context.Context, params map[string]string) error {
	err := j.run(ctx)
	if err == nil {
		j.lastRun = time.Now()
	}
	return err
}

// run executes the stream maintenance job
func (j *StreamMaintenanceJob) run(ctx context.Context) error {
	log.Printf("Starting Redis stream maintenance job")
	
	// Create Redis client
	redis := client.NewRedisClient(j.redisAddr)
	defer redis.Close()
	
	// Get stream info before trimming - handle errors gracefully
	var stockLen, cryptoLen, indexLen int64
	var err error
	
	// Get client directly to access XInfoStream
	rClient := redis.Client
	
	stockInfo, err := rClient.XInfoStream(ctx, client.StockQuoteStream).Result()
	if err == nil {
		stockLen = stockInfo.Length
	}
	
	cryptoInfo, err := rClient.XInfoStream(ctx, client.CryptoQuoteStream).Result()
	if err == nil {
		cryptoLen = cryptoInfo.Length
	}
	
	indexInfo, err := rClient.XInfoStream(ctx, client.MarketIndexStream).Result()
	if err == nil {
		indexLen = indexInfo.Length
	}
	
	log.Printf("Stream sizes before maintenance: Stocks: %d, Crypto: %d, Indices: %d",
		stockLen, cryptoLen, indexLen)
	
	// Trim all streams
	startTime := time.Now()
	err = redis.TrimStreams(ctx)
	if err != nil {
		log.Printf("Error trimming streams: %v", err)
		return err
	}
	
	// Get stream info after trimming - handle errors gracefully
	stockLen, cryptoLen, indexLen = 0, 0, 0
	
	stockInfo, err = rClient.XInfoStream(ctx, client.StockQuoteStream).Result()
	if err == nil {
		stockLen = stockInfo.Length
	}
	
	cryptoInfo, err = rClient.XInfoStream(ctx, client.CryptoQuoteStream).Result()
	if err == nil {
		cryptoLen = cryptoInfo.Length
	}
	
	indexInfo, err = rClient.XInfoStream(ctx, client.MarketIndexStream).Result()
	if err == nil {
		indexLen = indexInfo.Length
	}
	
	log.Printf("Stream sizes after maintenance: Stocks: %d, Crypto: %d, Indices: %d",
		stockLen, cryptoLen, indexLen)
	
	log.Printf("Redis stream maintenance completed in %v", time.Since(startTime))
	return nil
}