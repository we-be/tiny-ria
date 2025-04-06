// Package etlservice provides the ETL service interface for the Quotron CLI
package etlservice

import (
	"context"
	"time"

	"github.com/we-be/tiny-ria/quotron/etl/internal/db"
	"github.com/we-be/tiny-ria/quotron/etl/internal/models"
	"github.com/we-be/tiny-ria/quotron/etl/internal/pipeline"
)

// ETLService is the interface for the ETL service
type ETLService interface {
	// ProcessQuotesFile processes a quotes file and stores data in the database
	ProcessQuotesFile(ctx context.Context, filePath string, source string) (string, []string, error)

	// ProcessIndicesFile processes an indices file and stores data in the database
	ProcessIndicesFile(ctx context.Context, filePath string, source string) (string, []string, error)

	// ProcessMixedFile processes a file containing both quotes and indices
	ProcessMixedFile(ctx context.Context, filePath string, source string) (string, []string, []string, error)
	
	// SetupDatabase sets up the database schema
	SetupDatabase(ctx context.Context) error
	
	// ListLatestData lists the latest quotes and indices
	ListLatestData(ctx context.Context, limit int, symbols, indices []string) ([]models.StockQuote, []models.MarketIndex, error)
	
	// Close closes the service
	Close() error
}

// ETLServiceImpl implements the ETLService interface
type ETLServiceImpl struct {
	db       *db.Database
	pipeline *pipeline.Pipeline
}

// ETLOptions contains options for the ETL service
type ETLOptions struct {
	DBHost           string
	DBPort           int
	DBName           string
	DBUser           string
	DBPassword       string
	ConcurrentBatches int
	MaxRetries       int
	AllowHistoricalData bool
}

// DefaultOptions returns default ETL options
func DefaultOptions() *ETLOptions {
	return &ETLOptions{
		ConcurrentBatches: 4,
		MaxRetries:       3,
		AllowHistoricalData: false,
	}
}

// NewETLService creates a new ETL service
func NewETLService(opts *ETLOptions) (ETLService, error) {
	// Create database config
	dbConfig := db.DefaultConfig()
	
	// Override with options if provided
	if opts.DBHost != "" {
		dbConfig.Host = opts.DBHost
	}
	if opts.DBPort != 0 {
		dbConfig.Port = opts.DBPort
	}
	if opts.DBName != "" {
		dbConfig.DBName = opts.DBName
	}
	if opts.DBUser != "" {
		dbConfig.User = opts.DBUser
	}
	if opts.DBPassword != "" {
		dbConfig.Password = opts.DBPassword
	}
	
	// Create database connection
	database, err := db.NewDatabase(dbConfig)
	if err != nil {
		return nil, err
	}
	
	// Create pipeline options
	pipelineOpts := pipeline.DefaultPipelineOptions()
	pipelineOpts.ConcurrentBatches = opts.ConcurrentBatches
	pipelineOpts.MaxRetries = opts.MaxRetries
	pipelineOpts.AllowHistoricalData = opts.AllowHistoricalData
	
	// Create pipeline
	p := pipeline.NewPipeline(database, pipelineOpts)
	
	return &ETLServiceImpl{
		db:       database,
		pipeline: p,
	}, nil
}

// ProcessQuotesFile implements ETLService.ProcessQuotesFile
func (s *ETLServiceImpl) ProcessQuotesFile(ctx context.Context, filePath string, source string) (string, []string, error) {
	// Parse data source
	dataSource := models.DataSource(source)
	
	// Load quotes from file
	quotes, err := pipeline.LoadQuotesFromFile(filePath)
	if err != nil {
		return "", nil, err
	}
	
	// Process quotes
	return s.pipeline.ProcessStockQuotes(ctx, quotes, dataSource)
}

// ProcessIndicesFile implements ETLService.ProcessIndicesFile
func (s *ETLServiceImpl) ProcessIndicesFile(ctx context.Context, filePath string, source string) (string, []string, error) {
	// Parse data source
	dataSource := models.DataSource(source)
	
	// Load indices from file
	indices, err := pipeline.LoadIndicesFromFile(filePath)
	if err != nil {
		return "", nil, err
	}
	
	// Process indices
	return s.pipeline.ProcessMarketIndices(ctx, indices, dataSource)
}

// ProcessMixedFile implements ETLService.ProcessMixedFile
func (s *ETLServiceImpl) ProcessMixedFile(ctx context.Context, filePath string, source string) (string, []string, []string, error) {
	// Parse data source
	dataSource := models.DataSource(source)
	
	// Load mixed data from file
	quotes, indices, err := pipeline.LoadMixedDataFromFile(filePath)
	if err != nil {
		return "", nil, nil, err
	}
	
	// Process mixed data
	return s.pipeline.ProcessMixedBatch(ctx, quotes, indices, dataSource)
}

// SetupDatabase implements ETLService.SetupDatabase
func (s *ETLServiceImpl) SetupDatabase(ctx context.Context) error {
	return pipeline.SetupDatabaseSchema(s.db)
}

// ListLatestData implements ETLService.ListLatestData
func (s *ETLServiceImpl) ListLatestData(ctx context.Context, limit int, symbols, indices []string) ([]models.StockQuote, []models.MarketIndex, error) {
	// Get latest quotes
	quotes, err := s.db.GetLatestQuotes(ctx, symbols, limit)
	if err != nil {
		return nil, nil, err
	}
	
	// Get latest indices
	idxs, err := s.db.GetLatestIndices(ctx, indices, limit)
	if err != nil {
		return quotes, nil, err
	}
	
	return quotes, idxs, nil
}

// Close implements ETLService.Close
func (s *ETLServiceImpl) Close() error {
	return s.db.Close()
}