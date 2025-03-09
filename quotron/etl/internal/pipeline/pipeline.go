package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
	"uuid"

	"github.com/we-be/tiny-ria/quotron/etl/internal/db"
	"github.com/we-be/tiny-ria/quotron/etl/internal/enrichment"
	"github.com/we-be/tiny-ria/quotron/etl/internal/models"
	"github.com/we-be/tiny-ria/quotron/etl/internal/validation"
)

// PipelineOptions contains configuration options for the pipeline
type PipelineOptions struct {
	AllowHistoricalData bool
	ConcurrentBatches   int
	MaxRetries          int
	RetryDelay          time.Duration
}

// DefaultPipelineOptions returns the default pipeline options
func DefaultPipelineOptions() PipelineOptions {
	return PipelineOptions{
		AllowHistoricalData: false,
		ConcurrentBatches:   4,
		MaxRetries:          3,
		RetryDelay:          time.Second * 2,
	}
}

// Pipeline defines the core ETL pipeline
type Pipeline struct {
	options   PipelineOptions
	validator *validation.DataValidator
	enricher  *enrichment.DataEnricher
	database  *db.Database
	mu        sync.Mutex
	batches   map[string]*models.MarketBatch
}

// NewPipeline creates a new ETL pipeline
func NewPipeline(database *db.Database, options PipelineOptions) *Pipeline {
	return &Pipeline{
		options:   options,
		validator: validation.NewDataValidator(options.AllowHistoricalData),
		enricher:  enrichment.NewDataEnricher(),
		database:  database,
		batches:   make(map[string]*models.MarketBatch),
	}
}

// ProcessStockQuotes processes a batch of stock quotes
func (p *Pipeline) ProcessStockQuotes(ctx context.Context, quotes []models.StockQuote, source models.DataSource) (string, []string, error) {
	batchID := generateBatchID("quotes")
	log.Printf("Processing batch %s with %d quotes from %s", batchID, len(quotes), source)

	// Set batch ID for all quotes
	for i := range quotes {
		quotes[i].Source = source
		quotes[i].BatchID = batchID
	}

	// Validate quotes (concurrent processing happens inside Validate)
	validQuotes, _ := p.validator.ValidateBatch(quotes, nil)
	log.Printf("Validated %d/%d quotes for batch %s", len(validQuotes), len(quotes), batchID)

	// Create batch object
	batch := &models.MarketBatch{
		Quotes:    validQuotes,
		BatchID:   batchID,
		CreatedAt: time.Now(),
	}

	// Store batch for future reference
	p.mu.Lock()
	p.batches[batchID] = batch
	p.mu.Unlock()

	// Store batch in database
	dbBatch := &models.DataBatch{
		ID:         batchID,
		CreatedAt:  batch.CreatedAt,
		Status:     "processing",
		QuoteCount: len(validQuotes),
		IndexCount: 0,
		Source:     source,
	}

	// Add metadata to the batch
	metadataMap := map[string]interface{}{
		"original_count": len(quotes),
		"valid_count":    len(validQuotes),
	}
	metadataJSON, err := json.Marshal(metadataMap)
	if err == nil {
		dbBatch.Metadata = metadataJSON
	}

	// Start a transaction
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Store batch record
	_, err = p.database.InsertDataBatch(ctx, dbBatch)
	if err != nil {
		log.Printf("Error inserting batch record: %v", err)
		return batchID, nil, fmt.Errorf("failed to insert batch record: %w", err)
	}

	// Process quotes in parallel with a worker pool
	quoteCh := make(chan models.StockQuote, len(validQuotes))
	resultCh := make(chan string, len(validQuotes))
	errorCh := make(chan error, len(validQuotes))
	
	var wg sync.WaitGroup
	
	// Start worker goroutines
	workerCount := min(p.options.ConcurrentBatches, len(validQuotes))
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for quote := range quoteCh {
				// Insert quote with retries
				var quoteID string
				var quoteErr error
				
				for retry := 0; retry < p.options.MaxRetries; retry++ {
					quoteID, quoteErr = p.database.InsertStockQuote(ctx, &quote)
					if quoteErr == nil {
						resultCh <- quoteID
						break
					}
					
					log.Printf("Error inserting quote (retry %d/%d): %v", 
						retry+1, p.options.MaxRetries, quoteErr)
					
					// Wait before retrying
					select {
					case <-ctx.Done():
						errorCh <- ctx.Err()
						return
					case <-time.After(p.options.RetryDelay * time.Duration(retry+1)):
						// Exponential backoff
					}
				}
				
				if quoteErr != nil {
					errorCh <- quoteErr
				}
			}
		}()
	}
	
	// Send quotes to workers
	for _, quote := range validQuotes {
		quoteCh <- quote
	}
	close(quoteCh)
	
	// Wait for all workers to complete
	wg.Wait()
	close(resultCh)
	close(errorCh)
	
	// Collect results and errors
	quoteIDs := make([]string, 0, len(validQuotes))
	var errs []error
	
	for id := range resultCh {
		quoteIDs = append(quoteIDs, id)
	}
	
	for err := range errorCh {
		errs = append(errs, err)
	}

	// Compute and store statistics if we have valid quotes
	if len(validQuotes) > 0 {
		// Enrich the batch
		enrichedBatch := p.enricher.EnrichBatch(batch)
		
		// Compute statistics
		stats, statsJSON := p.enricher.ComputeStatistics(enrichedBatch)
		
		if stats != nil {
			// Convert statistics to database format
			meanPrice := &stats.MeanPrice
			medianPrice := &stats.MedianPrice
			meanChangePercent := &stats.MeanChangePercent
			positiveCount := &stats.PositiveChangeCount
			negativeCount := &stats.NegativeChangeCount
			unchangedCount := &stats.UnchangedCount
			totalVolume := &stats.TotalVolume
			
			// Insert statistics
			_, err = p.database.InsertBatchStatistics(ctx, batchID, 
				meanPrice, medianPrice, meanChangePercent,
				positiveCount, negativeCount, unchangedCount, 
				totalVolume, statsJSON)
			
			if err != nil {
				log.Printf("Error inserting statistics: %v", err)
				// Non-critical error, continue
			}
		}
	}

	// Update batch status
	now := time.Now()
	status := "completed"
	if len(errs) > 0 {
		status = "partial"
		if len(quoteIDs) == 0 {
			status = "failed"
		}
	}
	
	err = p.database.UpdateBatchStatus(ctx, batchID, status, &now)
	if err != nil {
		log.Printf("Error updating batch status: %v", err)
		// Non-critical error, continue
	}

	log.Printf("Processed batch %s: %d quotes stored, %d errors", 
		batchID, len(quoteIDs), len(errs))
	
	if len(errs) > 0 {
		return batchID, quoteIDs, fmt.Errorf("encountered %d errors during processing", len(errs))
	}
	
	return batchID, quoteIDs, nil
}

// ProcessMarketIndices processes a batch of market indices
func (p *Pipeline) ProcessMarketIndices(ctx context.Context, indices []models.MarketIndex, source models.DataSource) (string, []string, error) {
	batchID := generateBatchID("indices")
	log.Printf("Processing batch %s with %d indices from %s", batchID, len(indices), source)

	// Set batch ID for all indices
	for i := range indices {
		indices[i].Source = source
		indices[i].BatchID = batchID
	}

	// Validate indices (concurrent processing happens inside Validate)
	_, validIndices := p.validator.ValidateBatch(nil, indices)
	log.Printf("Validated %d/%d indices for batch %s", len(validIndices), len(indices), batchID)

	// Create batch object
	batch := &models.MarketBatch{
		Indices:   validIndices,
		BatchID:   batchID,
		CreatedAt: time.Now(),
	}

	// Store batch for future reference
	p.mu.Lock()
	p.batches[batchID] = batch
	p.mu.Unlock()

	// Store batch in database
	dbBatch := &models.DataBatch{
		ID:         batchID,
		CreatedAt:  batch.CreatedAt,
		Status:     "processing",
		QuoteCount: 0,
		IndexCount: len(validIndices),
		Source:     source,
	}

	// Add metadata to the batch
	metadataMap := map[string]interface{}{
		"original_count": len(indices),
		"valid_count":    len(validIndices),
	}
	metadataJSON, err := json.Marshal(metadataMap)
	if err == nil {
		dbBatch.Metadata = metadataJSON
	}

	// Start a transaction
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Store batch record
	_, err = p.database.InsertDataBatch(ctx, dbBatch)
	if err != nil {
		log.Printf("Error inserting batch record: %v", err)
		return batchID, nil, fmt.Errorf("failed to insert batch record: %w", err)
	}

	// Process indices in parallel with a worker pool
	indexCh := make(chan models.MarketIndex, len(validIndices))
	resultCh := make(chan string, len(validIndices))
	errorCh := make(chan error, len(validIndices))
	
	var wg sync.WaitGroup
	
	// Start worker goroutines
	workerCount := min(p.options.ConcurrentBatches, len(validIndices))
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range indexCh {
				// Insert index with retries
				var indexID string
				var indexErr error
				
				for retry := 0; retry < p.options.MaxRetries; retry++ {
					indexID, indexErr = p.database.InsertMarketIndex(ctx, &index)
					if indexErr == nil {
						resultCh <- indexID
						break
					}
					
					log.Printf("Error inserting index (retry %d/%d): %v", 
						retry+1, p.options.MaxRetries, indexErr)
					
					// Wait before retrying
					select {
					case <-ctx.Done():
						errorCh <- ctx.Err()
						return
					case <-time.After(p.options.RetryDelay * time.Duration(retry+1)):
						// Exponential backoff
					}
				}
				
				if indexErr != nil {
					errorCh <- indexErr
				}
			}
		}()
	}
	
	// Send indices to workers
	for _, index := range validIndices {
		indexCh <- index
	}
	close(indexCh)
	
	// Wait for all workers to complete
	wg.Wait()
	close(resultCh)
	close(errorCh)
	
	// Collect results and errors
	indexIDs := make([]string, 0, len(validIndices))
	var errs []error
	
	for id := range resultCh {
		indexIDs = append(indexIDs, id)
	}
	
	for err := range errorCh {
		errs = append(errs, err)
	}

	// Update batch status
	now := time.Now()
	status := "completed"
	if len(errs) > 0 {
		status = "partial"
		if len(indexIDs) == 0 {
			status = "failed"
		}
	}
	
	err = p.database.UpdateBatchStatus(ctx, batchID, status, &now)
	if err != nil {
		log.Printf("Error updating batch status: %v", err)
		// Non-critical error, continue
	}

	log.Printf("Processed batch %s: %d indices stored, %d errors", 
		batchID, len(indexIDs), len(errs))
	
	if len(errs) > 0 {
		return batchID, indexIDs, fmt.Errorf("encountered %d errors during processing", len(errs))
	}
	
	return batchID, indexIDs, nil
}

// ProcessMixedBatch processes a mixed batch of stock quotes and market indices
func (p *Pipeline) ProcessMixedBatch(ctx context.Context, quotes []models.StockQuote, indices []models.MarketIndex, source models.DataSource) (string, []string, []string, error) {
	batchID := generateBatchID("mixed")
	log.Printf("Processing mixed batch %s with %d quotes and %d indices from %s", 
		batchID, len(quotes), len(indices), source)
	
	// Set batch ID for all data
	for i := range quotes {
		quotes[i].Source = source
		quotes[i].BatchID = batchID
	}
	
	for i := range indices {
		indices[i].Source = source
		indices[i].BatchID = batchID
	}
	
	// Validate data (concurrent processing happens inside Validate)
	validQuotes, validIndices := p.validator.ValidateBatch(quotes, indices)
	log.Printf("Validated %d/%d quotes and %d/%d indices for batch %s", 
		len(validQuotes), len(quotes), 
		len(validIndices), len(indices), batchID)
	
	// Create batch object
	batch := &models.MarketBatch{
		Quotes:    validQuotes,
		Indices:   validIndices,
		BatchID:   batchID,
		CreatedAt: time.Now(),
	}
	
	// Store batch for future reference
	p.mu.Lock()
	p.batches[batchID] = batch
	p.mu.Unlock()
	
	// Store batch in database
	dbBatch := &models.DataBatch{
		ID:         batchID,
		CreatedAt:  batch.CreatedAt,
		Status:     "processing",
		QuoteCount: len(validQuotes),
		IndexCount: len(validIndices),
		Source:     source,
	}
	
	// Add metadata to the batch
	metadataMap := map[string]interface{}{
		"original_quote_count": len(quotes),
		"valid_quote_count":    len(validQuotes),
		"original_index_count": len(indices),
		"valid_index_count":    len(validIndices),
	}
	metadataJSON, err := json.Marshal(metadataMap)
	if err == nil {
		dbBatch.Metadata = metadataJSON
	}
	
	// Start a context with timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	// Store batch record
	_, err = p.database.InsertDataBatch(ctx, dbBatch)
	if err != nil {
		log.Printf("Error inserting batch record: %v", err)
		return batchID, nil, nil, fmt.Errorf("failed to insert batch record: %w", err)
	}
	
	// Process quotes and indices in parallel
	var quoteWg, indexWg sync.WaitGroup
	quoteCh := make(chan models.StockQuote, len(validQuotes))
	indexCh := make(chan models.MarketIndex, len(validIndices))
	quoteResultCh := make(chan string, len(validQuotes))
	indexResultCh := make(chan string, len(validIndices))
	quoteErrorCh := make(chan error, len(validQuotes))
	indexErrorCh := make(chan error, len(validIndices))
	
	// Quote worker goroutines
	quoteWorkerCount := min(p.options.ConcurrentBatches, len(validQuotes))
	for i := 0; i < quoteWorkerCount; i++ {
		quoteWg.Add(1)
		go func() {
			defer quoteWg.Done()
			for quote := range quoteCh {
				// Insert quote with retries
				var quoteID string
				var quoteErr error
				
				for retry := 0; retry < p.options.MaxRetries; retry++ {
					quoteID, quoteErr = p.database.InsertStockQuote(ctx, &quote)
					if quoteErr == nil {
						quoteResultCh <- quoteID
						break
					}
					
					log.Printf("Error inserting quote (retry %d/%d): %v", 
						retry+1, p.options.MaxRetries, quoteErr)
					
					select {
					case <-ctx.Done():
						quoteErrorCh <- ctx.Err()
						return
					case <-time.After(p.options.RetryDelay * time.Duration(retry+1)):
						// Exponential backoff
					}
				}
				
				if quoteErr != nil {
					quoteErrorCh <- quoteErr
				}
			}
		}()
	}
	
	// Index worker goroutines
	indexWorkerCount := min(p.options.ConcurrentBatches, len(validIndices))
	for i := 0; i < indexWorkerCount; i++ {
		indexWg.Add(1)
		go func() {
			defer indexWg.Done()
			for index := range indexCh {
				// Insert index with retries
				var indexID string
				var indexErr error
				
				for retry := 0; retry < p.options.MaxRetries; retry++ {
					indexID, indexErr = p.database.InsertMarketIndex(ctx, &index)
					if indexErr == nil {
						indexResultCh <- indexID
						break
					}
					
					log.Printf("Error inserting index (retry %d/%d): %v", 
						retry+1, p.options.MaxRetries, indexErr)
					
					select {
					case <-ctx.Done():
						indexErrorCh <- ctx.Err()
						return
					case <-time.After(p.options.RetryDelay * time.Duration(retry+1)):
						// Exponential backoff
					}
				}
				
				if indexErr != nil {
					indexErrorCh <- indexErr
				}
			}
		}()
	}
	
	// Send data to workers
	for _, quote := range validQuotes {
		quoteCh <- quote
	}
	close(quoteCh)
	
	for _, index := range validIndices {
		indexCh <- index
	}
	close(indexCh)
	
	// Wait for all workers to complete
	quoteWg.Wait()
	indexWg.Wait()
	close(quoteResultCh)
	close(indexResultCh)
	close(quoteErrorCh)
	close(indexErrorCh)
	
	// Collect results and errors
	quoteIDs := make([]string, 0, len(validQuotes))
	indexIDs := make([]string, 0, len(validIndices))
	var errs []error
	
	for id := range quoteResultCh {
		quoteIDs = append(quoteIDs, id)
	}
	
	for id := range indexResultCh {
		indexIDs = append(indexIDs, id)
	}
	
	for err := range quoteErrorCh {
		errs = append(errs, err)
	}
	
	for err := range indexErrorCh {
		errs = append(errs, err)
	}
	
	// Compute and store statistics if we have valid quotes
	if len(validQuotes) > 0 {
		// Enrich the batch
		enrichedBatch := p.enricher.EnrichBatch(batch)
		
		// Compute statistics
		stats, statsJSON := p.enricher.ComputeStatistics(enrichedBatch)
		
		if stats != nil {
			// Convert statistics to database format
			meanPrice := &stats.MeanPrice
			medianPrice := &stats.MedianPrice
			meanChangePercent := &stats.MeanChangePercent
			positiveCount := &stats.PositiveChangeCount
			negativeCount := &stats.NegativeChangeCount
			unchangedCount := &stats.UnchangedCount
			totalVolume := &stats.TotalVolume
			
			// Insert statistics
			_, err = p.database.InsertBatchStatistics(ctx, batchID, 
				meanPrice, medianPrice, meanChangePercent,
				positiveCount, negativeCount, unchangedCount, 
				totalVolume, statsJSON)
			
			if err != nil {
				log.Printf("Error inserting statistics: %v", err)
				// Non-critical error, continue
			}
		}
	}
	
	// Update batch status
	now := time.Now()
	status := "completed"
	if len(errs) > 0 {
		status = "partial"
		if len(quoteIDs) == 0 && len(indexIDs) == 0 {
			status = "failed"
		}
	}
	
	err = p.database.UpdateBatchStatus(ctx, batchID, status, &now)
	if err != nil {
		log.Printf("Error updating batch status: %v", err)
		// Non-critical error, continue
	}
	
	log.Printf("Processed mixed batch %s: %d quotes and %d indices stored, %d errors", 
		batchID, len(quoteIDs), len(indexIDs), len(errs))
	
	if len(errs) > 0 {
		return batchID, quoteIDs, indexIDs, fmt.Errorf("encountered %d errors during processing", len(errs))
	}
	
	return batchID, quoteIDs, indexIDs, nil
}

// Helper functions

// generateBatchID generates a unique batch ID
func generateBatchID(prefix string) string {
	timestamp := time.Now().UTC().Format("20060102150405")
	uniqueID := uuid.NewString()[:8]
	return fmt.Sprintf("%s_%s_%s", prefix, timestamp, uniqueID)
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}