package users

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/alexanderselivanov/trader/internal/adapters/database"
	"github.com/alexanderselivanov/trader/pkg/models"
)

// Repository handles user data persistence
type Repository struct {
	db *database.DB
}

// NewRepository creates new user repository
func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// CreateUser creates new user
func (r *Repository) CreateUser(ctx context.Context, telegramID int64, username, firstName string) (*models.User, error) {
	var user models.User
	
	err := r.db.Conn().QueryRowContext(ctx, `
		INSERT INTO users (telegram_id, username, first_name, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, true, $4, $4)
		RETURNING id, telegram_id, username, first_name, is_active, created_at, updated_at
	`, telegramID, username, firstName, time.Now()).Scan(
		&user.ID, &user.TelegramID, &user.Username, &user.FirstName,
		&user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)
	
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	
	// Create default state
	if err := r.initializeUserState(ctx, user.ID); err != nil {
		return nil, fmt.Errorf("failed to initialize user state: %w", err)
	}
	
	return &user, nil
}

// GetUserByTelegramID finds user by Telegram ID
func (r *Repository) GetUserByTelegramID(ctx context.Context, telegramID int64) (*models.User, error) {
	var user models.User
	
	err := r.db.Conn().QueryRowContext(ctx, `
		SELECT id, telegram_id, username, first_name, is_active, created_at, updated_at
		FROM users
		WHERE telegram_id = $1
	`, telegramID).Scan(
		&user.ID, &user.TelegramID, &user.Username, &user.FirstName,
		&user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	
	return &user, nil
}

// SaveConfig saves user configuration
func (r *Repository) SaveConfig(ctx context.Context, config *models.UserConfig) error {
	_, err := r.db.Conn().ExecContext(ctx, `
		INSERT INTO user_configs (
			user_id, exchange, api_key, api_secret, testnet, symbol,
			initial_balance, max_position_percent, max_leverage,
			stop_loss_percent, take_profit_percent, is_trading,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13)
		ON CONFLICT (user_id) DO UPDATE SET
			exchange = $2,
			api_key = $3,
			api_secret = $4,
			testnet = $5,
			symbol = $6,
			initial_balance = $7,
			max_position_percent = $8,
			max_leverage = $9,
			stop_loss_percent = $10,
			take_profit_percent = $11,
			is_trading = $12,
			updated_at = $13
	`,
		config.UserID, config.Exchange, config.APIKey, config.APISecret,
		config.Testnet, config.Symbol, config.InitialBalance.String(),
		config.MaxPositionPercent.String(), config.MaxLeverage,
		config.StopLossPercent.String(), config.TakeProfitPercent.String(),
		config.IsTrading, time.Now(),
	)
	
	return err
}

// GetConfig gets user configuration
func (r *Repository) GetConfig(ctx context.Context, userID int64) (*models.UserConfig, error) {
	var config models.UserConfig
	var initialBalance, maxPositionPercent, stopLossPercent, takeProfitPercent string
	
	err := r.db.Conn().QueryRowContext(ctx, `
		SELECT id, user_id, exchange, api_key, api_secret, testnet, symbol,
			   initial_balance, max_position_percent, max_leverage,
			   stop_loss_percent, take_profit_percent, is_trading,
			   created_at, updated_at
		FROM user_configs
		WHERE user_id = $1
	`, userID).Scan(
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

// GetState gets user state
func (r *Repository) GetState(ctx context.Context, userID int64) (*models.UserState, error) {
	var state models.UserState
	var balance, equity, dailyPnL, peakEquity string
	
	err := r.db.Conn().QueryRowContext(ctx, `
		SELECT id, user_id, mode, status, balance, equity, daily_pnl, peak_equity, updated_at
		FROM user_states
		WHERE user_id = $1
	`, userID).Scan(
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

// UpdateState updates user state
func (r *Repository) UpdateState(ctx context.Context, state *models.UserState) error {
	_, err := r.db.Conn().ExecContext(ctx, `
		UPDATE user_states
		SET mode = $2, status = $3, balance = $4, equity = $5,
		    daily_pnl = $6, peak_equity = $7, updated_at = $8
		WHERE user_id = $1
	`,
		state.UserID, state.Mode, state.Status,
		state.Balance.String(), state.Equity.String(),
		state.DailyPnL.String(), state.PeakEquity.String(),
		time.Now(),
	)
	
	return err
}

// GetAllActiveUsers returns all active users
func (r *Repository) GetAllActiveUsers(ctx context.Context) ([]models.User, error) {
	rows, err := r.db.Conn().QueryContext(ctx, `
		SELECT id, telegram_id, username, first_name, is_active, created_at, updated_at
		FROM users
		WHERE is_active = true
	`)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get active users: %w", err)
	}
	defer rows.Close()
	
	users := make([]models.User, 0)
	for rows.Next() {
		var user models.User
		if err := rows.Scan(
			&user.ID, &user.TelegramID, &user.Username, &user.FirstName,
			&user.IsActive, &user.CreatedAt, &user.UpdatedAt,
		); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	
	return users, nil
}

// SetTradingStatus sets user trading status
func (r *Repository) SetTradingStatus(ctx context.Context, userID int64, isTrading bool) error {
	_, err := r.db.Conn().ExecContext(ctx, `
		UPDATE user_configs
		SET is_trading = $2, updated_at = $3
		WHERE user_id = $1
	`, userID, isTrading, time.Now())
	
	return err
}

// initializeUserState creates initial state for new user
func (r *Repository) initializeUserState(ctx context.Context, userID int64) error {
	_, err := r.db.Conn().ExecContext(ctx, `
		INSERT INTO user_states (user_id, mode, status, balance, equity, daily_pnl, peak_equity, updated_at)
		VALUES ($1, 'paper', 'stopped', 0, 0, 0, 0, $2)
	`, userID, time.Now())
	
	return err
}

