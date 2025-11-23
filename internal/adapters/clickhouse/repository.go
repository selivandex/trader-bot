package clickhouse

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// Repository handles ClickHouse data operations
type Repository struct {
	db *sqlx.DB
}

// NewRepository creates new ClickHouse repository
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// SaveCandles saves OHLCV candles to ClickHouse
func (r *Repository) SaveCandles(ctx context.Context, symbol, timeframe string, candles []models.Candle) error {
	if len(candles) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	stmt, err := tx.Preparex(`
		INSERT INTO market_ohlcv 
		(timestamp, symbol, timeframe, open, high, low, close, volume, quote_volume, trades)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, candle := range candles {
		_, err = stmt.ExecContext(ctx,
			candle.Timestamp,
			symbol,
			timeframe,
			candle.Open.InexactFloat64(),
			candle.High.InexactFloat64(),
			candle.Low.InexactFloat64(),
			candle.Close.InexactFloat64(),
			candle.Volume.InexactFloat64(),
			candle.QuoteVolume.InexactFloat64(),
			candle.Trades,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to insert candle: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Debug("saved candles to ClickHouse",
		zap.Int("count", len(candles)),
	)

	return nil
}

// SaveTrades saves trades to ClickHouse history
func (r *Repository) SaveTrades(ctx context.Context, trades []models.Trade) error {
	if len(trades) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	stmt, err := tx.Preparex(`
		INSERT INTO trades_history 
		(id, agent_id, user_id, symbol, side, entry_price, exit_price, size, 
		 leverage, pnl, pnl_percent, fee, realized_pnl, opened_at, closed_at, 
		 duration, entry_reason, exit_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, trade := range trades {
		_, err = stmt.ExecContext(ctx,
			trade.ID,
			trade.AgentID,
			trade.UserID,
			trade.Symbol,
			string(trade.Side),
			trade.EntryPrice.InexactFloat64(),
			trade.ExitPrice.InexactFloat64(),
			trade.Size.InexactFloat64(),
			trade.Leverage,
			trade.PnL.InexactFloat64(),
			trade.PnLPercent,
			trade.Fee.InexactFloat64(),
			trade.RealizedPnL.InexactFloat64(),
			trade.OpenedAt,
			trade.ClosedAt,
			int(trade.ClosedAt.Sub(trade.OpenedAt).Seconds()),
			trade.EntryReason,
			trade.ExitReason,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to insert trade: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Debug("saved trades to ClickHouse",
		zap.Int("count", len(trades)),
	)

	return nil
}

// SaveNews saves news articles to ClickHouse
func (r *Repository) SaveNews(ctx context.Context, articles []models.NewsItem) error {
	if len(articles) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	stmt, err := tx.Preparex(`
		INSERT INTO news_articles 
		(id, source, title, content, url, author, sentiment, impact, symbols, published_at, processed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, article := range articles {
		symbols := article.Symbols
		if symbols == nil {
			symbols = []string{}
		}

		_, err = stmt.ExecContext(ctx,
			article.ID,
			article.Source,
			article.Title,
			article.Content,
			article.URL,
			article.Author,
			article.Sentiment,
			article.Impact,
			symbols,
			article.PublishedAt,
			article.ProcessedAt,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to insert news: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Debug("saved news to ClickHouse",
		zap.Int("count", len(articles)),
	)

	return nil
}

// SaveOnChainTransactions saves on-chain transactions to ClickHouse
func (r *Repository) SaveOnChainTransactions(ctx context.Context, transactions []models.WhaleTransaction) error {
	if len(transactions) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	stmt, err := tx.Preparex(`
		INSERT INTO onchain_transactions 
		(id, transaction_hash, transaction_type, symbol, from_address, to_address, 
		 amount, amount_usd, impact_score, blockchain, exchange_name, detected_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, txn := range transactions {
		txHash := txn.TxHash
		if txHash == "" {
			txHash = txn.TransactionHash
		}

		_, err = stmt.ExecContext(ctx,
			txn.ID,
			txHash,
			txn.TransactionType,
			txn.Symbol,
			txn.FromAddress,
			txn.ToAddress,
			txn.Amount.InexactFloat64(),
			txn.AmountUSD.InexactFloat64(),
			txn.ImpactScore,
			txn.Blockchain,
			txn.ExchangeName,
			txn.DetectedAt,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to insert transaction: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Debug("saved on-chain transactions to ClickHouse",
		zap.Int("count", len(transactions)),
	)

	return nil
}

// SaveAgentMetrics saves agent performance snapshots
func (r *Repository) SaveAgentMetrics(ctx context.Context, metrics []models.AgentMetric) error {
	if len(metrics) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	stmt, err := tx.Preparex(`
		INSERT INTO agent_performance 
		(agent_id, timestamp, symbol, balance, equity, pnl, pnl_percent, 
		 total_trades, winning_trades, losing_trades, win_rate, sharpe_ratio, 
		 max_drawdown, current_drawdown)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, metric := range metrics {
		_, err = stmt.ExecContext(ctx,
			metric.AgentID,
			metric.Timestamp,
			metric.Symbol,
			metric.Balance.InexactFloat64(),
			metric.Equity.InexactFloat64(),
			metric.PnL.InexactFloat64(),
			metric.PnLPercent,
			metric.TotalTrades,
			metric.WinningTrades,
			metric.LosingTrades,
			metric.WinRate,
			metric.SharpeRatio,
			metric.MaxDrawdown,
			metric.CurrentDrawdown,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to insert metric: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Debug("saved agent metrics to ClickHouse",
		zap.Int("count", len(metrics)),
	)

	return nil
}
