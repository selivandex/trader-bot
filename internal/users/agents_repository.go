package users

import (
	"context"
	"fmt"

	"github.com/alexanderselivanov/trader/internal/adapters/database"
	"github.com/alexanderselivanov/trader/pkg/crypto"
	"github.com/alexanderselivanov/trader/pkg/models"
)

// AgentsRepository handles user operations for agent system
type AgentsRepository struct {
	db *database.DB
}

// NewAgentsRepository creates new agents repository
func NewAgentsRepository(db *database.DB) *AgentsRepository {
	return &AgentsRepository{db: db}
}

// CreateUser creates new user from Telegram
func (r *AgentsRepository) CreateUser(ctx context.Context, telegramID int64, username, firstName string) (*models.User, error) {
	query := `
		INSERT INTO users (telegram_id, username, first_name, is_active)
		VALUES ($1, $2, $3, true)
		RETURNING id, created_at, updated_at
	`

	var user models.User
	user.TelegramID = telegramID
	user.Username = username
	user.FirstName = firstName
	user.IsActive = true

	err := r.db.DB().QueryRowContext(ctx, query, telegramID, username, firstName).
		Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &user, nil
}

// GetUserByTelegramID finds user by Telegram ID
func (r *AgentsRepository) GetUserByTelegramID(ctx context.Context, telegramID int64) (*models.User, error) {
	query := `
		SELECT id, telegram_id, username, first_name, is_active, created_at, updated_at
		FROM users
		WHERE telegram_id = $1
	`

	var user models.User
	err := r.db.DB().GetContext(ctx, &user, query, telegramID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// AddExchange adds exchange connection for user with encrypted credentials
func (r *AgentsRepository) AddExchange(ctx context.Context, userID, exchange, apiKey, apiSecret string, testnet bool) (*models.UserExchange, error) {
	// Encrypt API credentials
	encryptedKey, err := crypto.Encrypt(apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt API key: %w", err)
	}

	encryptedSecret, err := crypto.Encrypt(apiSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt API secret: %w", err)
	}

	query := `
		INSERT INTO user_exchanges (user_id, exchange, api_key_encrypted, api_secret_encrypted, testnet, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
		RETURNING id, created_at, updated_at
	`

	var ex models.UserExchange
	ex.UserID = userID
	ex.Exchange = exchange
	ex.APIKey = apiKey // Store decrypted in memory (never persisted)
	ex.APISecret = apiSecret
	ex.Testnet = testnet
	ex.IsActive = true

	err = r.db.DB().QueryRowContext(ctx, query, userID, exchange, encryptedKey, encryptedSecret, testnet).
		Scan(&ex.ID, &ex.CreatedAt, &ex.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to add exchange: %w", err)
	}

	return &ex, nil
}

// GetUserExchange gets user's exchange connection with decrypted credentials
func (r *AgentsRepository) GetUserExchange(ctx context.Context, userID, exchange string) (*models.UserExchange, error) {
	query := `
		SELECT id, user_id, exchange, api_key_encrypted, api_secret_encrypted, testnet, is_active, created_at, updated_at
		FROM user_exchanges
		WHERE user_id = $1 AND exchange = $2
	`

	var ex models.UserExchange
	var encryptedKey, encryptedSecret string

	err := r.db.DB().QueryRowContext(ctx, query, userID, exchange).Scan(
		&ex.ID,
		&ex.UserID,
		&ex.Exchange,
		&encryptedKey,
		&encryptedSecret,
		&ex.Testnet,
		&ex.IsActive,
		&ex.CreatedAt,
		&ex.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange: %w", err)
	}

	// Decrypt credentials
	ex.APIKey, err = crypto.Decrypt(encryptedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt API key: %w", err)
	}

	ex.APISecret, err = crypto.Decrypt(encryptedSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt API secret: %w", err)
	}

	return &ex, nil
}

// AddTradingPair adds trading pair for user
func (r *AgentsRepository) AddTradingPair(ctx context.Context, userID, exchangeID, symbol string, budget float64) (*models.UserTradingPair, error) {
	query := `
		INSERT INTO user_trading_pairs (user_id, exchange_id, symbol, budget, is_active)
		VALUES ($1, $2, $3, $4, true)
		RETURNING id, created_at, updated_at
	`

	var pair models.UserTradingPair
	pair.UserID = userID
	pair.ExchangeID = exchangeID
	pair.Symbol = symbol
	pair.Budget = models.NewDecimal(budget)
	pair.IsActive = true

	err := r.db.DB().QueryRowContext(ctx, query, userID, exchangeID, symbol, budget).
		Scan(&pair.ID, &pair.CreatedAt, &pair.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to add trading pair: %w", err)
	}

	return &pair, nil
}

// GetUserTradingPairs gets all trading pairs for user
func (r *AgentsRepository) GetUserTradingPairs(ctx context.Context, userID string) ([]models.UserTradingPair, error) {
	query := `
		SELECT id, user_id, exchange_id, symbol, budget, is_active, created_at, updated_at
		FROM user_trading_pairs
		WHERE user_id = $1 AND is_active = true
		ORDER BY created_at DESC
	`
	
	var pairs []models.UserTradingPair
	err := r.db.DB().SelectContext(ctx, &pairs, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get trading pairs: %w", err)
	}
	
	return pairs, nil
}

// GetAllUserExchanges gets all exchanges for user
func (r *AgentsRepository) GetAllUserExchanges(ctx context.Context, userID string) ([]models.UserExchange, error) {
	query := `
		SELECT id, user_id, exchange, api_key_encrypted, api_secret_encrypted, testnet, is_active, created_at, updated_at
		FROM user_exchanges
		WHERE user_id = $1 AND is_active = true
		ORDER BY created_at DESC
	`
	
	rows, err := r.db.DB().QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchanges: %w", err)
	}
	defer rows.Close()
	
	exchanges := []models.UserExchange{}
	for rows.Next() {
		var ex models.UserExchange
		var encryptedKey, encryptedSecret string
		
		err := rows.Scan(
			&ex.ID, &ex.UserID, &ex.Exchange,
			&encryptedKey, &encryptedSecret,
			&ex.Testnet, &ex.IsActive, &ex.CreatedAt, &ex.UpdatedAt,
		)
		if err != nil {
			continue
		}
		
		// Decrypt credentials
		ex.APIKey, _ = crypto.Decrypt(encryptedKey)
		ex.APISecret, _ = crypto.Decrypt(encryptedSecret)
		
		exchanges = append(exchanges, ex)
	}
	
	return exchanges, nil
}

// GetTradingPairWithExchange gets trading pair with exchange info
func (r *AgentsRepository) GetTradingPairWithExchange(ctx context.Context, pairID string) (*models.UserTradingPair, *models.UserExchange, error) {
	// Get trading pair
	var pair models.UserTradingPair
	pairQuery := `
		SELECT id, user_id, exchange_id, symbol, budget, is_active, created_at, updated_at
		FROM user_trading_pairs
		WHERE id = $1
	`
	err := r.db.DB().GetContext(ctx, &pair, pairQuery, pairID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get trading pair: %w", err)
	}
	
	// Get exchange
	var ex models.UserExchange
	var encryptedKey, encryptedSecret string
	
	exQuery := `
		SELECT id, user_id, exchange, api_key_encrypted, api_secret_encrypted, testnet, is_active, created_at, updated_at
		FROM user_exchanges
		WHERE id = $1
	`
	err = r.db.DB().QueryRowContext(ctx, exQuery, pair.ExchangeID).Scan(
		&ex.ID, &ex.UserID, &ex.Exchange,
		&encryptedKey, &encryptedSecret,
		&ex.Testnet, &ex.IsActive, &ex.CreatedAt, &ex.UpdatedAt,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get exchange: %w", err)
	}
	
	// Decrypt
	ex.APIKey, _ = crypto.Decrypt(encryptedKey)
	ex.APISecret, _ = crypto.Decrypt(encryptedSecret)
	
	return &pair, &ex, nil
}

// AssignAgentToSymbol assigns agent to trade specific symbol
func (r *AgentsRepository) AssignAgentToSymbol(ctx context.Context, userID, agentID, tradingPairID string, budget float64) (*models.AgentSymbolAssignment, error) {
	query := `
		INSERT INTO agent_symbol_assignments (user_id, agent_id, trading_pair_id, budget, is_active)
		VALUES ($1, $2, $3, $4, true)
		RETURNING id, created_at
	`

	var assignment models.AgentSymbolAssignment
	assignment.UserID = userID
	assignment.AgentID = agentID
	assignment.TradingPairID = tradingPairID
	assignment.Budget = models.NewDecimal(budget)
	assignment.IsActive = true

	err := r.db.DB().QueryRowContext(ctx, query, userID, agentID, tradingPairID, budget).
		Scan(&assignment.ID, &assignment.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to assign agent: %w", err)
	}

	return &assignment, nil
}

// GetAgentAssignments gets all agent-symbol assignments for user
func (r *AgentsRepository) GetAgentAssignments(ctx context.Context, userID string) ([]models.AgentSymbolAssignment, error) {
	query := `
		SELECT id, user_id, agent_id, trading_pair_id, budget, is_active, created_at
		FROM agent_symbol_assignments
		WHERE user_id = $1 AND is_active = true
		ORDER BY created_at DESC
	`

	var assignments []models.AgentSymbolAssignment
	err := r.db.DB().SelectContext(ctx, &assignments, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get assignments: %w", err)
	}

	return assignments, nil
}
