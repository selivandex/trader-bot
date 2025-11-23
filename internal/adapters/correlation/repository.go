package correlation

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/selivandex/trader-bot/pkg/models"
)

// Repository handles database operations for correlations
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new correlation repository
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// SaveCorrelation stores a correlation calculation
func (r *Repository) SaveCorrelation(ctx context.Context, corr *models.AssetCorrelation) error {
	query := `
		INSERT INTO asset_correlations 
		(id, base_symbol, quote_symbol, period, correlation, sample_size, calculated_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.ExecContext(ctx, query,
		corr.ID,
		corr.BaseSymbol,
		corr.QuoteSymbol,
		corr.Period,
		corr.Correlation,
		corr.SampleSize,
		corr.CalculatedAt,
		corr.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save correlation: %w", err)
	}

	return nil
}

// GetLatestCorrelation retrieves the most recent correlation for a pair
func (r *Repository) GetLatestCorrelation(ctx context.Context, baseSymbol, quoteSymbol, period string) (*models.AssetCorrelation, error) {
	query := `
		SELECT id, base_symbol, quote_symbol, period, correlation, sample_size, calculated_at, created_at
		FROM asset_correlations
		WHERE base_symbol = $1 AND quote_symbol = $2 AND period = $3
		ORDER BY calculated_at DESC
		LIMIT 1
	`

	var corr models.AssetCorrelation
	err := r.db.QueryRowContext(ctx, query, baseSymbol, quoteSymbol, period).Scan(
		&corr.ID,
		&corr.BaseSymbol,
		&corr.QuoteSymbol,
		&corr.Period,
		&corr.Correlation,
		&corr.SampleSize,
		&corr.CalculatedAt,
		&corr.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no correlation found for %s/%s (%s)", baseSymbol, quoteSymbol, period)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get correlation: %w", err)
	}

	return &corr, nil
}

// GetAllCorrelations retrieves all recent correlations for a base symbol
func (r *Repository) GetAllCorrelations(ctx context.Context, baseSymbol, period string, limit int) ([]*models.AssetCorrelation, error) {
	query := `
		SELECT DISTINCT ON (quote_symbol) 
			id, base_symbol, quote_symbol, period, correlation, sample_size, calculated_at, created_at
		FROM asset_correlations
		WHERE base_symbol = $1 AND period = $2
		ORDER BY quote_symbol, calculated_at DESC
		LIMIT $3
	`

	rows, err := r.db.QueryContext(ctx, query, baseSymbol, period, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get correlations: %w", err)
	}
	defer rows.Close()

	var correlations []*models.AssetCorrelation
	for rows.Next() {
		var corr models.AssetCorrelation
		err := rows.Scan(
			&corr.ID,
			&corr.BaseSymbol,
			&corr.QuoteSymbol,
			&corr.Period,
			&corr.Correlation,
			&corr.SampleSize,
			&corr.CalculatedAt,
			&corr.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan correlation: %w", err)
		}
		correlations = append(correlations, &corr)
	}

	return correlations, nil
}

// SaveMarketRegime stores a market regime detection
func (r *Repository) SaveMarketRegime(ctx context.Context, regime *models.MarketRegime) error {
	query := `
		INSERT INTO market_regimes 
		(id, regime, btc_dominance, avg_correlation, volatility_level, confidence, detected_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.ExecContext(ctx, query,
		regime.ID,
		regime.Regime,
		regime.BTCDominance,
		regime.AvgCorrelation,
		regime.VolatilityLevel,
		regime.Confidence,
		regime.DetectedAt,
		regime.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save market regime: %w", err)
	}

	return nil
}

// GetLatestMarketRegime retrieves the most recent market regime
func (r *Repository) GetLatestMarketRegime(ctx context.Context) (*models.MarketRegime, error) {
	query := `
		SELECT id, regime, btc_dominance, avg_correlation, volatility_level, confidence, detected_at, created_at
		FROM market_regimes
		ORDER BY detected_at DESC
		LIMIT 1
	`

	var regime models.MarketRegime
	err := r.db.QueryRowContext(ctx, query).Scan(
		&regime.ID,
		&regime.Regime,
		&regime.BTCDominance,
		&regime.AvgCorrelation,
		&regime.VolatilityLevel,
		&regime.Confidence,
		&regime.DetectedAt,
		&regime.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no market regime found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get market regime: %w", err)
	}

	return &regime, nil
}

// CleanupOldCorrelations removes correlations older than specified duration
func (r *Repository) CleanupOldCorrelations(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `
		DELETE FROM asset_correlations
		WHERE calculated_at < $1
	`

	result, err := r.db.ExecContext(ctx, query, time.Now().Add(-olderThan))
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup correlations: %w", err)
	}

	count, _ := result.RowsAffected()
	return count, nil
}

// CleanupOldRegimes removes old market regime records
func (r *Repository) CleanupOldRegimes(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `
		DELETE FROM market_regimes
		WHERE detected_at < $1
	`

	result, err := r.db.ExecContext(ctx, query, time.Now().Add(-olderThan))
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup market regimes: %w", err)
	}

	count, _ := result.RowsAffected()
	return count, nil
}
