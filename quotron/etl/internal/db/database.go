package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/we-be/tiny-ria/quotron/etl/internal/models"
)

// Database connection configuration
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// DefaultConfig returns a default database configuration
func DefaultConfig() Config {
	return Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnvInt("DB_PORT", 5432),
		User:     getEnv("DB_USER", "quotron"),
		Password: getEnv("DB_PASSWORD", "quotron"),
		DBName:   getEnv("DB_NAME", "quotron"),
		SSLMode:  getEnv("DB_SSL_MODE", "disable"),
	}
}

// Helper function to get environment variables with defaults
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// Helper function to get integer environment variables with defaults
func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// Database provides methods for interacting with the PostgreSQL database
type Database struct {
	db     *sqlx.DB
	config Config
	mu     sync.Mutex // for thread safety
}

// NewDatabase creates a new database connection with the given configuration
func NewDatabase(config Config) (*Database, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Password, config.DBName, config.SSLMode,
	)

	db, err := sqlx.Connect("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Printf("Connected to database at %s:%d/%s", config.Host, config.Port, config.DBName)
	return &Database{db: db, config: config}, nil
}

// Close closes the database connection pool
func (d *Database) Close() error {
	if d.db != nil {
		log.Printf("Closing database connection")
		return d.db.Close()
	}
	return nil
}

// InsertStockQuote inserts a stock quote into the database
func (d *Database) InsertStockQuote(ctx context.Context, quote *models.StockQuote) (string, error) {
	const query = `
		INSERT INTO stock_quotes (
			symbol, price, change, change_percent, volume, 
			timestamp, exchange, source, batch_id, created_at
		) VALUES (
			:symbol, :price, :change, :change_percent, :volume,
			:timestamp, :exchange, :source, :batch_id, NOW()
		) RETURNING id
	`

	// Initialize a map for the SQL named parameters
	params := map[string]interface{}{
		"symbol":          quote.Symbol,
		"price":           quote.Price,
		"change":          quote.Change,
		"change_percent":  quote.ChangePercent,
		"volume":          quote.Volume,
		"timestamp":       quote.Timestamp,
		"exchange":        quote.Exchange,
		"source":          quote.Source,
		"batch_id":        quote.BatchID,
	}

	var id string
	rows, err := d.db.NamedQueryContext(ctx, query, params)
	if err != nil {
		return "", fmt.Errorf("failed to insert stock quote: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		if err := rows.Scan(&id); err != nil {
			return "", fmt.Errorf("failed to scan returned ID: %w", err)
		}
	}

	log.Printf("Inserted stock quote %s with ID %s", quote.Symbol, id)
	return id, nil
}

// InsertMarketIndex inserts a market index into the database
func (d *Database) InsertMarketIndex(ctx context.Context, index *models.MarketIndex) (string, error) {
	const query = `
		INSERT INTO market_indices (
			name, value, change, change_percent, timestamp, source, batch_id, created_at
		) VALUES (
			:name, :value, :change, :change_percent, :timestamp, :source, :batch_id, NOW()
		) RETURNING id
	`

	// Initialize a map for the SQL named parameters
	params := map[string]interface{}{
		"name":            index.Name,
		"value":           index.Value,
		"change":          index.Change,
		"change_percent":  index.ChangePercent,
		"timestamp":       index.Timestamp,
		"source":          index.Source,
		"batch_id":        index.BatchID,
	}

	var id string
	rows, err := d.db.NamedQueryContext(ctx, query, params)
	if err != nil {
		return "", fmt.Errorf("failed to insert market index: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		if err := rows.Scan(&id); err != nil {
			return "", fmt.Errorf("failed to scan returned ID: %w", err)
		}
	}

	log.Printf("Inserted market index %s with ID %s", index.Name, id)
	return id, nil
}

// InsertDataBatch inserts a data batch into the database
func (d *Database) InsertDataBatch(ctx context.Context, batch *models.DataBatch) (string, error) {
	const query = `
		INSERT INTO data_batches (
			id, created_at, status, quote_count, index_count, source, metadata
		) VALUES (
			:id, :created_at, :status, :quote_count, :index_count, :source, :metadata
		) RETURNING id
	`

	// Initialize a map for the SQL named parameters
	params := map[string]interface{}{
		"id":          batch.ID,
		"created_at":  batch.CreatedAt,
		"status":      batch.Status,
		"quote_count": batch.QuoteCount,
		"index_count": batch.IndexCount,
		"source":      batch.Source,
		"metadata":    batch.Metadata,
	}

	rows, err := d.db.NamedQueryContext(ctx, query, params)
	if err != nil {
		return "", fmt.Errorf("failed to insert data batch: %w", err)
	}
	defer rows.Close()

	var id string
	if rows.Next() {
		if err := rows.Scan(&id); err != nil {
			return "", fmt.Errorf("failed to scan returned ID: %w", err)
		}
	}

	log.Printf("Inserted data batch with ID %s", id)
	return id, nil
}

// UpdateBatchStatus updates the status of a data batch
func (d *Database) UpdateBatchStatus(ctx context.Context, batchID string, status string, processedAt *time.Time) error {
	query := `
		UPDATE data_batches 
		SET status = $1, processed_at = COALESCE($2, processed_at)
		WHERE id = $3
	`

	result, err := d.db.ExecContext(ctx, query, status, processedAt, batchID)
	if err != nil {
		return fmt.Errorf("failed to update batch status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("no batch found with ID %s", batchID)
	}

	log.Printf("Updated status of batch %s to %s", batchID, status)
	return nil
}

// InsertBatchStatistics inserts batch statistics into the database
func (d *Database) InsertBatchStatistics(ctx context.Context, batchID string, meanPrice, medianPrice, meanChangePercent *float64,
	positiveCount, negativeCount, unchangedCount *int, totalVolume *int64, statsJSON []byte) (string, error) {
	const query = `
		INSERT INTO batch_statistics (
			batch_id, mean_price, median_price, mean_change_percent,
			positive_change_count, negative_change_count, unchanged_count,
			total_volume, statistics_json, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, NOW()
		) RETURNING id
	`

	var id string
	err := d.db.QueryRowContext(
		ctx, query, batchID, meanPrice, medianPrice, meanChangePercent,
		positiveCount, negativeCount, unchangedCount, totalVolume, statsJSON,
	).Scan(&id)
	
	if err != nil {
		return "", fmt.Errorf("failed to insert batch statistics: %w", err)
	}

	log.Printf("Inserted batch statistics for batch %s with ID %s", batchID, id)
	return id, nil
}

// GetLatestQuotes returns the latest stock quotes, optionally filtered by symbols
func (d *Database) GetLatestQuotes(ctx context.Context, symbols []string, limit int) ([]models.StockQuote, error) {
	var query string
	var args []interface{}

	if len(symbols) > 0 {
		// Build a query with a symbol filter
		query = "SELECT * FROM latest_stock_prices WHERE symbol IN ("
		for i, symbol := range symbols {
			if i > 0 {
				query += ", "
			}
			query += fmt.Sprintf("$%d", i+1)
			args = append(args, symbol)
		}
		query += ") ORDER BY timestamp DESC"
	} else {
		// Query all symbols
		query = "SELECT * FROM latest_stock_prices ORDER BY timestamp DESC"
	}

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	var quotes []models.StockQuote
	err := d.db.SelectContext(ctx, &quotes, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest quotes: %w", err)
	}

	return quotes, nil
}

// GetLatestIndices returns the latest market indices, optionally filtered by names
func (d *Database) GetLatestIndices(ctx context.Context, names []string, limit int) ([]models.MarketIndex, error) {
	var query string
	var args []interface{}

	if len(names) > 0 {
		// Build a query with a name filter
		query = "SELECT * FROM latest_market_indices WHERE name IN ("
		for i, name := range names {
			if i > 0 {
				query += ", "
			}
			query += fmt.Sprintf("$%d", i+1)
			args = append(args, name)
		}
		query += ") ORDER BY timestamp DESC"
	} else {
		// Query all indices
		query = "SELECT * FROM latest_market_indices ORDER BY timestamp DESC"
	}

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	var indices []models.MarketIndex
	err := d.db.SelectContext(ctx, &indices, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest indices: %w", err)
	}

	return indices, nil
}

// GetQuoteHistory returns historical quotes for a specific symbol
func (d *Database) GetQuoteHistory(ctx context.Context, symbol string, limit int) ([]models.StockQuote, error) {
	query := `
		SELECT * FROM stock_quotes
		WHERE symbol = $1
		ORDER BY timestamp DESC
	`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	var quotes []models.StockQuote
	err := d.db.SelectContext(ctx, &quotes, query, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get quote history: %w", err)
	}

	return quotes, nil
}

// GetIndexHistory returns historical values for a specific market index
func (d *Database) GetIndexHistory(ctx context.Context, name string, limit int) ([]models.MarketIndex, error) {
	query := `
		SELECT * FROM market_indices
		WHERE name = $1
		ORDER BY timestamp DESC
	`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	var indices []models.MarketIndex
	err := d.db.SelectContext(ctx, &indices, query, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get index history: %w", err)
	}

	return indices, nil
}

// GetBatch retrieves a specific data batch by ID
func (d *Database) GetBatch(ctx context.Context, batchID string) (*models.DataBatch, error) {
	query := "SELECT * FROM data_batches WHERE id = $1"

	var batch models.DataBatch
	err := d.db.GetContext(ctx, &batch, query, batchID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No batch found
		}
		return nil, fmt.Errorf("failed to get batch: %w", err)
	}

	return &batch, nil
}

// GetBatchQuotes retrieves all stock quotes for a specific batch
func (d *Database) GetBatchQuotes(ctx context.Context, batchID string) ([]models.StockQuote, error) {
	query := "SELECT * FROM stock_quotes WHERE batch_id = $1"

	var quotes []models.StockQuote
	err := d.db.SelectContext(ctx, &quotes, query, batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get batch quotes: %w", err)
	}

	return quotes, nil
}

// GetBatchIndices retrieves all market indices for a specific batch
func (d *Database) GetBatchIndices(ctx context.Context, batchID string) ([]models.MarketIndex, error) {
	query := "SELECT * FROM market_indices WHERE batch_id = $1"

	var indices []models.MarketIndex
	err := d.db.SelectContext(ctx, &indices, query, batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get batch indices: %w", err)
	}

	return indices, nil
}

// GetBatchStatistics retrieves statistics for a specific batch
func (d *Database) GetBatchStatistics(ctx context.Context, batchID string) (*models.BatchStatistics, error) {
	query := "SELECT * FROM batch_statistics WHERE batch_id = $1"

	var stats models.BatchStatistics
	err := d.db.GetContext(ctx, &stats, query, batchID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No statistics found
		}
		return nil, fmt.Errorf("failed to get batch statistics: %w", err)
	}

	return &stats, nil
}