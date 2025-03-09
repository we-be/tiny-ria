package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/we-be/tiny-ria/quotron/etl/internal/models"
)

// StoreInvestmentModel stores an investment model in the database
func (db *Database) StoreInvestmentModel(ctx context.Context, model *models.InvestmentModel) (string, error) {
	tx, err := db.db.Beginx()
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert the model
	var modelID string
	err = tx.QueryRowContext(
		ctx,
		`INSERT INTO investment_models 
		(provider, model_name, detail_level, source, fetched_at) 
		VALUES ($1, $2, $3, $4, $5) 
		RETURNING id`,
		model.Provider, model.ModelName, model.DetailLevel, model.Source, model.FetchedAt,
	).Scan(&modelID)
	if err != nil {
		return "", fmt.Errorf("failed to insert investment model: %w", err)
	}

	// Insert holdings if provided
	if len(model.Holdings) > 0 {
		stmt, err := tx.PrepareContext(
			ctx,
			`INSERT INTO model_holdings 
			(model_id, ticker, position_name, allocation, asset_class, sector, additional_metadata) 
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		)
		if err != nil {
			return "", fmt.Errorf("failed to prepare holdings statement: %w", err)
		}
		defer stmt.Close()

		for _, holding := range model.Holdings {
			_, err = stmt.ExecContext(
				ctx,
				modelID, holding.Ticker, holding.PositionName, 
				holding.Allocation, holding.AssetClass, holding.Sector, 
				holding.AdditionalMetadata,
			)
			if err != nil {
				return "", fmt.Errorf("failed to insert holding: %w", err)
			}
		}
	}

	// Insert sector allocations if provided
	if len(model.Sectors) > 0 {
		stmt, err := tx.PrepareContext(
			ctx,
			`INSERT INTO sector_allocations 
			(model_id, sector, allocation_percent) 
			VALUES ($1, $2, $3)`,
		)
		if err != nil {
			return "", fmt.Errorf("failed to prepare sector statement: %w", err)
		}
		defer stmt.Close()

		for _, sector := range model.Sectors {
			_, err = stmt.ExecContext(
				ctx,
				modelID, sector.Sector, sector.AllocationPercent,
			)
			if err != nil {
				return "", fmt.Errorf("failed to insert sector allocation: %w", err)
			}
		}
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return modelID, nil
}

// GetInvestmentModel retrieves an investment model by ID with its holdings and sectors
func (db *Database) GetInvestmentModel(ctx context.Context, modelID string) (*models.InvestmentModel, error) {
	// Get the model data
	model := &models.InvestmentModel{}
	err := db.db.QueryRowContext(
		ctx,
		`SELECT id, provider, model_name, detail_level, source, fetched_at 
		FROM investment_models 
		WHERE id = $1`,
		modelID,
	).Scan(
		&model.ID, &model.Provider, &model.ModelName, 
		&model.DetailLevel, &model.Source, &model.FetchedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("investment model not found: %s", modelID)
		}
		return nil, fmt.Errorf("failed to query investment model: %w", err)
	}

	// Get the holdings
	rows, err := db.db.QueryContext(
		ctx,
		`SELECT id, model_id, ticker, position_name, allocation, asset_class, sector, additional_metadata 
		FROM model_holdings 
		WHERE model_id = $1`,
		modelID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query holdings: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		holding := models.ModelHolding{}
		err := rows.Scan(
			&holding.ID, &holding.ModelID, &holding.Ticker, &holding.PositionName,
			&holding.Allocation, &holding.AssetClass, &holding.Sector, &holding.AdditionalMetadata,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan holding: %w", err)
		}
		model.Holdings = append(model.Holdings, holding)
	}

	// Get the sector allocations
	rows, err = db.db.QueryContext(
		ctx,
		`SELECT id, model_id, sector, allocation_percent 
		FROM sector_allocations 
		WHERE model_id = $1`,
		modelID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query sectors: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		sector := models.SectorAllocation{}
		err := rows.Scan(
			&sector.ID, &sector.ModelID, &sector.Sector, &sector.AllocationPercent,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sector: %w", err)
		}
		model.Sectors = append(model.Sectors, sector)
	}

	return model, nil
}

// ListInvestmentModels retrieves a list of investment models with optional filtering
func (db *Database) ListInvestmentModels(ctx context.Context, provider string, limit int) ([]models.InvestmentModel, error) {
	query := `SELECT id, provider, model_name, detail_level, source, fetched_at FROM investment_models`
	args := make([]interface{}, 0)
	
	if provider != "" {
		query += ` WHERE provider = $1`
		args = append(args, provider)
	}
	
	query += ` ORDER BY fetched_at DESC`
	
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, limit)
	}
	
	rows, err := db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query investment models: %w", err)
	}
	defer rows.Close()
	
	var modelList []models.InvestmentModel
	for rows.Next() {
		model := models.InvestmentModel{}
		err := rows.Scan(
			&model.ID, &model.Provider, &model.ModelName, 
			&model.DetailLevel, &model.Source, &model.FetchedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan investment model: %w", err)
		}
		modelList = append(modelList, model)
	}
	
	return modelList, nil
}