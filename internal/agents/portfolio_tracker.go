package agents

import (
	"context"
	"sync"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/exchange"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// AgentPortfolioTracker tracks portfolio for a single agent
type AgentPortfolioTracker struct {
	exchange       exchange.Exchange
	repository     *Repository
	agentID        string
	symbol         string
	initialBalance decimal.Decimal
	currentBalance decimal.Decimal
	equity         decimal.Decimal
	peakEquity     decimal.Decimal
	totalPnL       decimal.Decimal
	mu             sync.RWMutex
}

// NewAgentPortfolioTracker creates new portfolio tracker for agent
func NewAgentPortfolioTracker(
	agentID string,
	symbol string,
	initialBalance float64,
	exchange exchange.Exchange,
	repository *Repository,
) *AgentPortfolioTracker {
	balance := decimal.NewFromFloat(initialBalance)

	return &AgentPortfolioTracker{
		agentID:        agentID,
		symbol:         symbol,
		exchange:       exchange,
		repository:     repository,
		initialBalance: balance,
		currentBalance: balance,
		equity:         balance,
		peakEquity:     balance,
		totalPnL:       decimal.Zero,
	}
}

// Initialize loads state from database or creates new
func (apt *AgentPortfolioTracker) Initialize(ctx context.Context) error {
	apt.mu.Lock()
	defer apt.mu.Unlock()

	// Try to load existing state
	state, err := apt.repository.GetAgentState(ctx, apt.agentID, apt.symbol)
	if err != nil {
		// Create new state
		state = &models.AgentState{
			AgentID:        apt.agentID,
			Symbol:         apt.symbol,
			Balance:        apt.initialBalance,
			InitialBalance: apt.initialBalance,
			Equity:         apt.initialBalance,
			PnL:            decimal.Zero,
			IsTrading:      true,
		}

		if err := apt.repository.CreateAgentState(ctx, state); err != nil {
			return err
		}

		logger.Info("initialized new agent portfolio",
			zap.String("agent_id", apt.agentID),
			zap.String("symbol", apt.symbol),
			zap.String("initial_balance", apt.initialBalance.String()),
		)
	} else {
		// Load from state
		apt.currentBalance = state.Balance
		apt.equity = state.Equity
		apt.totalPnL = state.PnL
		apt.peakEquity = state.Equity

		logger.Info("loaded agent portfolio from database",
			zap.String("agent_id", apt.agentID),
			zap.String("balance", apt.currentBalance.String()),
			zap.String("pnl", apt.totalPnL.String()),
		)
	}

	return nil
}

// GetBalance returns current balance
func (apt *AgentPortfolioTracker) GetBalance() float64 {
	apt.mu.RLock()
	defer apt.mu.RUnlock()

	balance, _ := apt.currentBalance.Float64()
	return balance
}

// GetEquity returns current equity
func (apt *AgentPortfolioTracker) GetEquity() float64 {
	apt.mu.RLock()
	defer apt.mu.RUnlock()

	equity, _ := apt.equity.Float64()
	return equity
}

// GetPnL returns total PnL
func (apt *AgentPortfolioTracker) GetPnL() float64 {
	apt.mu.RLock()
	defer apt.mu.RUnlock()

	pnl, _ := apt.totalPnL.Float64()
	return pnl
}

// GetPeakEquity returns peak equity
func (apt *AgentPortfolioTracker) GetPeakEquity() float64 {
	apt.mu.RLock()
	defer apt.mu.RUnlock()

	peak, _ := apt.peakEquity.Float64()
	return peak
}

// RecordTrade records a trade and updates balance
func (apt *AgentPortfolioTracker) RecordTrade(ctx context.Context, trade *AgentTrade) error {
	apt.mu.Lock()
	defer apt.mu.Unlock()

	// Update balance
	pnl := trade.PnL
	apt.currentBalance = apt.currentBalance.Add(pnl)
	apt.equity = apt.currentBalance
	apt.totalPnL = apt.totalPnL.Add(pnl)

	// Update peak equity
	if apt.equity.GreaterThan(apt.peakEquity) {
		apt.peakEquity = apt.equity
	}

	// Calculate win rate
	// TODO: Track wins/losses count in state

	logger.Info("trade recorded",
		zap.String("agent_id", apt.agentID),
		zap.String("symbol", apt.symbol),
		zap.String("side", trade.Side),
		zap.String("pnl", pnl.String()),
		zap.String("balance", apt.currentBalance.String()),
	)

	// Update state in database
	return apt.saveState(ctx)
}

// UpdateFromExchange updates portfolio from exchange state
func (apt *AgentPortfolioTracker) UpdateFromExchange(ctx context.Context) error {
	apt.mu.Lock()
	defer apt.mu.Unlock()

	// Fetch balance from exchange
	balance, err := apt.exchange.FetchBalance(ctx)
	if err != nil {
		return err
	}

	// Update equity based on exchange balance
	if usdtBalance, ok := balance.Currencies["USDT"]; ok {
		apt.currentBalance = usdtBalance.Free.Add(usdtBalance.Used)
		apt.equity = apt.currentBalance
		apt.totalPnL = apt.currentBalance.Sub(apt.initialBalance)

		// Update peak
		if apt.equity.GreaterThan(apt.peakEquity) {
			apt.peakEquity = apt.equity
		}
	}

	return apt.saveState(ctx)
}

// saveState saves current state to database
func (apt *AgentPortfolioTracker) saveState(ctx context.Context) error {
	state := &models.AgentState{
		AgentID:        apt.agentID,
		Symbol:         apt.symbol,
		Balance:        apt.currentBalance,
		InitialBalance: apt.initialBalance,
		Equity:         apt.equity,
		PnL:            apt.totalPnL,
		// TODO: Add wins/losses tracking
		IsTrading: true,
	}

	return apt.repository.CreateAgentState(ctx, state)
}

// AgentTrade represents a trade executed by agent
type AgentTrade struct {
	AgentID    string
	Symbol     string
	Side       string
	Size       decimal.Decimal
	EntryPrice decimal.Decimal
	ExitPrice  decimal.Decimal
	PnL        decimal.Decimal
	Fee        decimal.Decimal
}
