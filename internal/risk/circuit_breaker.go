package risk

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/internal/adapters/config"
	"github.com/alexanderselivanov/trader/pkg/logger"
)

// CircuitBreaker prevents trading when certain risk thresholds are exceeded
type CircuitBreaker struct {
	mu                   sync.RWMutex
	repo                 *Repository
	userID               int64
	isOpen               bool
	consecutiveLosses    int
	maxConsecutiveLosses int
	dailyLoss            float64
	maxDailyLoss         float64
	cooldownDuration     time.Duration
	openedAt             time.Time
	lastResetDate        time.Time
}

// NewCircuitBreaker creates new circuit breaker
func NewCircuitBreaker(cfg *config.RiskConfig, repo *Repository, userID int64) *CircuitBreaker {
	return &CircuitBreaker{
		repo:                 repo,
		userID:               userID,
		isOpen:               false,
		consecutiveLosses:    0,
		maxConsecutiveLosses: cfg.MaxConsecutiveLosses,
		dailyLoss:            0,
		maxDailyLoss:         cfg.MaxDailyLossPercent,
		cooldownDuration:     cfg.CircuitBreakerCooldown,
		lastResetDate:        time.Now(),
	}
}

// IsOpen returns true if circuit breaker is open (trading disabled)
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	// Check if cooldown period has passed
	if cb.isOpen && time.Since(cb.openedAt) >= cb.cooldownDuration {
		return false // Cooldown expired, can trade again
	}

	return cb.isOpen
}

// RecordTrade records trade result and updates circuit breaker state
func (cb *CircuitBreaker) RecordTrade(pnl, initialBalance float64) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Reset daily counters if new day
	if !isSameDay(cb.lastResetDate, time.Now()) {
		cb.dailyLoss = 0
		cb.lastResetDate = time.Now()
		logger.Info("circuit breaker: daily counters reset")
	}

	// Update consecutive losses
	if pnl < 0 {
		cb.consecutiveLosses++
		cb.dailyLoss += abs(pnl)

		logger.Warn("trade loss recorded",
			zap.Float64("pnl", pnl),
			zap.Int("consecutive_losses", cb.consecutiveLosses),
			zap.Float64("daily_loss", cb.dailyLoss),
		)
	} else {
		cb.consecutiveLosses = 0
	}

	// Check if circuit breaker should open
	dailyLossPercent := (cb.dailyLoss / initialBalance) * 100

	if cb.consecutiveLosses >= cb.maxConsecutiveLosses {
		return cb.open(fmt.Sprintf("max consecutive losses reached (%d)", cb.consecutiveLosses))
	}

	if dailyLossPercent >= cb.maxDailyLoss {
		return cb.open(fmt.Sprintf("max daily loss reached (%.2f%%)", dailyLossPercent))
	}

	return nil
}

// open opens the circuit breaker
func (cb *CircuitBreaker) open(reason string) error {
	if cb.isOpen {
		return nil // Already open
	}

	cb.isOpen = true
	cb.openedAt = time.Now()

	logger.Error("CIRCUIT BREAKER OPENED",
		zap.String("reason", reason),
		zap.Time("opened_at", cb.openedAt),
		zap.Duration("cooldown", cb.cooldownDuration),
	)

	// Log to risk_events table
	if cb.repo != nil {
		_ = cb.repo.LogRiskEvent(context.Background(), cb.userID, "CIRCUIT_BREAKER_OPEN", reason, map[string]interface{}{
			"consecutive_losses": cb.consecutiveLosses,
			"daily_loss":         cb.dailyLoss,
			"cooldown_minutes":   cb.cooldownDuration.Minutes(),
		})
	}

	return fmt.Errorf("circuit breaker opened: %s", reason)
}

// Close manually closes the circuit breaker
func (cb *CircuitBreaker) Close() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if !cb.isOpen {
		return
	}

	cb.isOpen = false
	cb.consecutiveLosses = 0

	logger.Info("circuit breaker manually closed")
}

// Reset resets all counters
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.isOpen = false
	cb.consecutiveLosses = 0
	cb.dailyLoss = 0
	cb.lastResetDate = time.Now()

	logger.Info("circuit breaker reset")
}

// GetStatus returns current circuit breaker status
func (cb *CircuitBreaker) GetStatus() CircuitBreakerStatus {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	status := CircuitBreakerStatus{
		IsOpen:            cb.isOpen,
		ConsecutiveLosses: cb.consecutiveLosses,
		DailyLoss:         cb.dailyLoss,
		OpenedAt:          cb.openedAt,
	}

	if cb.isOpen {
		remaining := cb.cooldownDuration - time.Since(cb.openedAt)
		if remaining > 0 {
			status.CooldownRemaining = remaining
		}
	}

	return status
}

// CircuitBreakerStatus represents current status
type CircuitBreakerStatus struct {
	IsOpen            bool          `json:"is_open"`
	ConsecutiveLosses int           `json:"consecutive_losses"`
	DailyLoss         float64       `json:"daily_loss"`
	OpenedAt          time.Time     `json:"opened_at,omitempty"`
	CooldownRemaining time.Duration `json:"cooldown_remaining,omitempty"`
}
