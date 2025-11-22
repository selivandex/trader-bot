package users

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/alexanderselivanov/trader/pkg/models"
)

// GetAllConfigs returns all trading pairs for a user
func (r *Repository) GetAllConfigs(ctx context.Context, userID int64) ([]models.UserConfig, error) {
	rows, err := r.db.Conn().QueryContext(ctx, `
		SELECT id, user_id, exchange, api_key, api_secret, testnet, symbol,
			   initial_balance, max_position_percent, max_leverage,
			   stop_loss_percent, take_profit_percent, is_trading,
			   created_at, updated_at
		FROM user_configs
		WHERE user_id = $1
		ORDER BY symbol
	`, userID)

	if err != nil {
		return nil, fmt.Errorf("failed to get configs: %w", err)
	}
	defer rows.Close()

	configs := make([]models.UserConfig, 0)
	for rows.Next() {
		var config models.UserConfig
		var initialBalance, maxPositionPercent, stopLossPercent, takeProfitPercent string

		if err := rows.Scan(
			&config.ID, &config.UserID, &config.Exchange, &config.APIKey,
			&config.APISecret, &config.Testnet, &config.Symbol,
			&initialBalance, &maxPositionPercent, &config.MaxLeverage,
			&stopLossPercent, &takeProfitPercent, &config.IsTrading,
			&config.CreatedAt, &config.UpdatedAt,
		); err != nil {
			return nil, err
		}

		// Parse decimals
		config.InitialBalance, _ = models.NewDecimal(0).SetString(initialBalance)
		config.MaxPositionPercent, _ = models.NewDecimal(0).SetString(maxPositionPercent)
		config.StopLossPercent, _ = models.NewDecimal(0).SetString(stopLossPercent)
		config.TakeProfitPercent, _ = models.NewDecimal(0).SetString(takeProfitPercent)

		configs = append(configs, config)
	}

	return configs, nil
}

// GetConfigBySymbol gets configuration for specific symbol
func (r *Repository) GetConfigBySymbol(ctx context.Context, userID int64, symbol string) (*models.UserConfig, error) {
	var config models.UserConfig
	var initialBalance, maxPositionPercent, stopLossPercent, takeProfitPercent string

	err := r.db.Conn().QueryRowContext(ctx, `
		SELECT id, user_id, exchange, api_key, api_secret, testnet, symbol,
			   initial_balance, max_position_percent, max_leverage,
			   stop_loss_percent, take_profit_percent, is_trading,
			   created_at, updated_at
		FROM user_configs
		WHERE user_id = $1 AND symbol = $2
	`, userID, symbol).Scan(
		&config.ID, &config.UserID, &config.Exchange, &config.APIKey,
		&config.APISecret, &config.Testnet, &config.Symbol,
		&initialBalance, &maxPositionPercent, &config.MaxLeverage,
		&stopLossPercent, &takeProfitPercent, &config.IsTrading,
		&config.CreatedAt, &config.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	// Parse decimals
	config.InitialBalance, _ = models.NewDecimal(0).SetString(initialBalance)
	config.MaxPositionPercent, _ = models.NewDecimal(0).SetString(maxPositionPercent)
	config.StopLossPercent, _ = models.NewDecimal(0).SetString(stopLossPercent)
	config.TakeProfitPercent, _ = models.NewDecimal(0).SetString(takeProfitPercent)

	return &config, nil
}

// AddPairConfig adds new trading pair configuration for user
func (r *Repository) AddPairConfig(ctx context.Context, config *models.UserConfig) error {
	_, err := r.db.Conn().ExecContext(ctx, `
		INSERT INTO user_configs (
			user_id, exchange, api_key, api_secret, testnet, symbol,
			initial_balance, max_position_percent, max_leverage,
			stop_loss_percent, take_profit_percent, is_trading,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13)
	`,
		config.UserID, config.Exchange, config.APIKey, config.APISecret,
		config.Testnet, config.Symbol, config.InitialBalance.String(),
		config.MaxPositionPercent.String(), config.MaxLeverage,
		config.StopLossPercent.String(), config.TakeProfitPercent.String(),
		config.IsTrading, time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to add pair config: %w", err)
	}

	// Create initial state for this pair
	_, err = r.db.Conn().ExecContext(ctx, `
		INSERT INTO user_states (user_id, symbol, mode, status, balance, equity, daily_pnl, peak_equity, updated_at)
		VALUES ($1, $2, 'paper', 'stopped', $3, $3, 0, $3, $4)
	`, config.UserID, config.Symbol, config.InitialBalance.String(), time.Now())

	return err
}

// RemovePairConfig removes trading pair configuration
func (r *Repository) RemovePairConfig(ctx context.Context, userID int64, symbol string) error {
	// First check if bot is trading
	var isTrading bool
	err := r.db.Conn().QueryRowContext(ctx, `
		SELECT is_trading FROM user_configs WHERE user_id = $1 AND symbol = $2
	`, userID, symbol).Scan(&isTrading)

	if err == sql.ErrNoRows {
		return fmt.Errorf("pair config not found")
	}

	if err != nil {
		return err
	}

	if isTrading {
		return fmt.Errorf("cannot remove trading pair while bot is active - stop trading first")
	}

	// Delete config (cascade will handle related data)
	_, err = r.db.Conn().ExecContext(ctx, `
		DELETE FROM user_configs WHERE user_id = $1 AND symbol = $2
	`, userID, symbol)

	if err != nil {
		return fmt.Errorf("failed to remove pair config: %w", err)
	}

	// Delete state
	_, err = r.db.Conn().ExecContext(ctx, `
		DELETE FROM user_states WHERE user_id = $1 AND symbol = $2
	`, userID, symbol)

	return err
}

// SetPairTradingStatus sets trading status for specific pair
func (r *Repository) SetPairTradingStatus(ctx context.Context, userID int64, symbol string, isTrading bool) error {
	_, err := r.db.Conn().ExecContext(ctx, `
		UPDATE user_configs
		SET is_trading = $3, updated_at = $4
		WHERE user_id = $1 AND symbol = $2
	`, userID, symbol, isTrading, time.Now())

	return err
}

// GetStateBySymbol gets state for specific symbol
func (r *Repository) GetStateBySymbol(ctx context.Context, userID int64, symbol string) (*models.UserState, error) {
	var state models.UserState
	var balance, equity, dailyPnL, peakEquity string

	err := r.db.Conn().QueryRowContext(ctx, `
		SELECT id, user_id, mode, status, balance, equity, daily_pnl, peak_equity, updated_at
		FROM user_states
		WHERE user_id = $1 AND symbol = $2
	`, userID, symbol).Scan(
		&state.ID, &state.UserID, &state.Mode, &state.Status,
		&balance, &equity, &dailyPnL, &peakEquity, &state.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get state: %w", err)
	}

	// Parse decimals
	state.Balance, _ = models.NewDecimal(0).SetString(balance)
	state.Equity, _ = models.NewDecimal(0).SetString(equity)
	state.DailyPnL, _ = models.NewDecimal(0).SetString(dailyPnL)
	state.PeakEquity, _ = models.NewDecimal(0).SetString(peakEquity)

	return &state, nil
}

// UpdateStateBySymbol updates state for specific symbol
func (r *Repository) UpdateStateBySymbol(ctx context.Context, userID int64, symbol string, state *models.UserState) error {
	_, err := r.db.Conn().ExecContext(ctx, `
		UPDATE user_states
		SET mode = $3, status = $4, balance = $5, equity = $6,
		    daily_pnl = $7, peak_equity = $8, updated_at = $9
		WHERE user_id = $1 AND symbol = $2
	`,
		userID, symbol,
		state.Mode, state.Status,
		state.Balance.String(), state.Equity.String(),
		state.DailyPnL.String(), state.PeakEquity.String(),
		time.Now(),
	)

	return err
}

// GetAllTradingPairs returns all pairs where trading is enabled
func (r *Repository) GetAllTradingPairs(ctx context.Context) ([]struct {
	UserID int64
	Symbol string
}, error) {
	rows, err := r.db.Conn().QueryContext(ctx, `
		SELECT user_id, symbol
		FROM user_configs
		WHERE is_trading = true
	`)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pairs []struct {
		UserID int64
		Symbol string
	}

	for rows.Next() {
		var pair struct {
			UserID int64
			Symbol string
		}
		if err := rows.Scan(&pair.UserID, &pair.Symbol); err != nil {
			return nil, err
		}
		pairs = append(pairs, pair)
	}

	return pairs, nil
}
