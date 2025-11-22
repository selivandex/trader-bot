package workers

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/alexanderselivanov/trader/pkg/models"
)

// Repository handles database operations for workers
type Repository struct {
	db *sqlx.DB
}

// NewRepository creates new workers repository
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// ========== Sentiment Operations ==========

// SaveSentimentSnapshot saves sentiment snapshot to database
func (r *Repository) SaveSentimentSnapshot(ctx context.Context, sentiment *models.AggregatedSentiment) error {
	query := `
		INSERT INTO sentiment_snapshots (
			timestamp, bullish_score, bearish_score, net_sentiment,
			news_count, high_impact_count, average_sentiment, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.ExecContext(ctx, query,
		sentiment.Timestamp,
		sentiment.BullishScore,
		sentiment.BearishScore,
		sentiment.NetSentiment,
		sentiment.NewsCount,
		sentiment.HighImpactCount,
		sentiment.AverageSentiment,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to save sentiment snapshot: %w", err)
	}

	return nil
}

// GetRecentSentimentSnapshots retrieves recent sentiment snapshots
func (r *Repository) GetRecentSentimentSnapshots(ctx context.Context, limit int) ([]float64, error) {
	query := `
		SELECT net_sentiment
		FROM sentiment_snapshots
		ORDER BY timestamp DESC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query sentiment snapshots: %w", err)
	}
	defer rows.Close()

	datapoints := make([]float64, 0)
	for rows.Next() {
		var netSentiment float64
		if err := rows.Scan(&netSentiment); err == nil {
			datapoints = append(datapoints, netSentiment)
		}
	}

	return datapoints, nil
}

// ========== On-Chain Operations ==========

// SaveWhaleTransaction saves whale transaction to database
func (r *Repository) SaveWhaleTransaction(ctx context.Context, tx *models.WhaleTransaction) error {
	query := `
		INSERT INTO whale_transactions (
			tx_hash, blockchain, symbol, amount, amount_usd,
			from_address, to_address, from_owner, to_owner,
			transaction_type, timestamp, impact_score, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (tx_hash) DO NOTHING
	`

	_, err := r.db.ExecContext(ctx, query,
		tx.TxHash, tx.Blockchain, tx.Symbol,
		tx.Amount.String(), tx.AmountUSD.String(),
		tx.FromAddress, tx.ToAddress,
		tx.FromOwner, tx.ToOwner,
		tx.TransactionType, tx.Timestamp, tx.ImpactScore,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to save whale transaction: %w", err)
	}

	return nil
}

// GetRecentWhaleTransactions retrieves recent whale transactions for a symbol
func (r *Repository) GetRecentWhaleTransactions(ctx context.Context, symbol string, since time.Duration, limit int) ([]models.WhaleTransaction, error) {
	cutoff := time.Now().Add(-since)

	query := `
		SELECT tx_hash, symbol, amount, amount_usd, from_owner, to_owner,
		       transaction_type, timestamp, impact_score
		FROM whale_transactions
		WHERE symbol = $1 AND timestamp > $2
		ORDER BY timestamp DESC
		LIMIT $3
	`

	rows, err := r.db.QueryContext(ctx, query, symbol, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query whale transactions: %w", err)
	}
	defer rows.Close()

	transactions := make([]models.WhaleTransaction, 0)
	for rows.Next() {
		var tx models.WhaleTransaction
		var amount, amountUSD string

		if err := rows.Scan(
			&tx.TxHash, &tx.Symbol, &amount, &amountUSD,
			&tx.FromOwner, &tx.ToOwner, &tx.TransactionType,
			&tx.Timestamp, &tx.ImpactScore,
		); err == nil {
			tx.Amount = models.DecimalFromString(amount)
			tx.AmountUSD = models.DecimalFromString(amountUSD)
			transactions = append(transactions, tx)
		}
	}

	return transactions, nil
}

// GetExchangeNetFlow gets net exchange flow for a symbol
func (r *Repository) GetExchangeNetFlow(ctx context.Context, symbol string, since time.Duration) (float64, error) {
	cutoff := time.Now().Add(-since)

	query := `
		SELECT COALESCE(SUM(net_flow), 0)
		FROM exchange_flows
		WHERE symbol = $1 AND timestamp > $2
	`

	var netFlow float64
	err := r.db.QueryRowContext(ctx, query, symbol, cutoff).Scan(&netFlow)
	if err != nil {
		return 0, fmt.Errorf("failed to get net flow: %w", err)
	}

	return netFlow, nil
}

// ========== Exchange Flow Operations ==========

// AggregateExchangeFlows calculates and saves exchange flows from whale transactions
func (r *Repository) AggregateExchangeFlows(ctx context.Context, timestamp time.Time) error {
	query := `
		INSERT INTO exchange_flows (exchange, symbol, timestamp, inflow, outflow, net_flow, created_at)
		SELECT 
			CASE 
				WHEN transaction_type = 'exchange_inflow' THEN to_owner
				WHEN transaction_type = 'exchange_outflow' THEN from_owner
			END as exchange,
			symbol,
			$1 as timestamp,
			COALESCE(SUM(CASE WHEN transaction_type = 'exchange_inflow' THEN amount ELSE 0 END), 0) as inflow,
			COALESCE(SUM(CASE WHEN transaction_type = 'exchange_outflow' THEN amount ELSE 0 END), 0) as outflow,
			COALESCE(SUM(CASE WHEN transaction_type = 'exchange_inflow' THEN amount ELSE 0 END), 0) -
			COALESCE(SUM(CASE WHEN transaction_type = 'exchange_outflow' THEN amount ELSE 0 END), 0) as net_flow,
			NOW() as created_at
		FROM whale_transactions
		WHERE 
			timestamp >= $1 
			AND timestamp < $1 + INTERVAL '1 hour'
			AND transaction_type IN ('exchange_inflow', 'exchange_outflow')
		GROUP BY 
			CASE 
				WHEN transaction_type = 'exchange_inflow' THEN to_owner
				WHEN transaction_type = 'exchange_outflow' THEN from_owner
			END,
			symbol
		HAVING COUNT(*) > 0
		ON CONFLICT (exchange, symbol, timestamp) DO UPDATE SET
			inflow = EXCLUDED.inflow,
			outflow = EXCLUDED.outflow,
			net_flow = EXCLUDED.net_flow
	`

	_, err := r.db.ExecContext(ctx, query, timestamp)
	if err != nil {
		return fmt.Errorf("failed to aggregate exchange flows: %w", err)
	}

	return nil
}

// ========== Daily Metrics Operations ==========

// GetActiveUserIDs retrieves all user IDs from user_configs
func (r *Repository) GetActiveUserIDs(ctx context.Context) ([]int64, error) {
	query := `
		SELECT DISTINCT user_id FROM user_configs
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active users: %w", err)
	}
	defer rows.Close()

	userIDs := make([]int64, 0)
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err == nil {
			userIDs = append(userIDs, userID)
		}
	}

	return userIDs, nil
}

// CalculateDailyMetrics calls database function to calculate daily metrics for a user
func (r *Repository) CalculateDailyMetrics(ctx context.Context, userID int64, date time.Time) error {
	query := `
		SELECT calculate_daily_metrics($1, $2)
	`

	_, err := r.db.ExecContext(ctx, query, userID, date)
	if err != nil {
		return fmt.Errorf("failed to calculate daily metrics: %w", err)
	}

	return nil
}

// ========== Cleanup Operations ==========

// CleanupOldSentimentSnapshots removes old sentiment snapshots
func (r *Repository) CleanupOldSentimentSnapshots(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	query := `
		DELETE FROM sentiment_snapshots
		WHERE timestamp < $1
	`

	result, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup sentiment snapshots: %w", err)
	}

	deleted, _ := result.RowsAffected()
	return deleted, nil
}

// CleanupOldWhaleTransactions removes old whale transactions
func (r *Repository) CleanupOldWhaleTransactions(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	query := `
		DELETE FROM whale_transactions
		WHERE timestamp < $1
	`

	result, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup whale transactions: %w", err)
	}

	deleted, _ := result.RowsAffected()
	return deleted, nil
}

// CleanupOldExchangeFlows removes old exchange flow records
func (r *Repository) CleanupOldExchangeFlows(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	query := `
		DELETE FROM exchange_flows
		WHERE timestamp < $1
	`

	result, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup exchange flows: %w", err)
	}

	deleted, _ := result.RowsAffected()
	return deleted, nil
}

