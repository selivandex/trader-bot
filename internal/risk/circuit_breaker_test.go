package risk

import (
	"testing"
	"time"

	"github.com/selivandex/trader-bot/internal/adapters/config"
	"github.com/selivandex/trader-bot/pkg/logger"
)

func TestCircuitBreaker_RecordTrade(t *testing.T) {
	// Initialize logger for tests
	logger.Init("error", "")

	cfg := &config.RiskConfig{
		MaxConsecutiveLosses:   3,
		MaxDailyLossPercent:    5.0,
		CircuitBreakerCooldown: 1 * time.Hour,
	}

	// Use nil repository for testing (no DB persistence needed)
	cb := NewCircuitBreaker(cfg, nil, 1)
	initialBalance := 1000.0

	// Test consecutive losses
	t.Run("consecutive losses trigger", func(t *testing.T) {
		cb.Reset()

		// Record 2 losses - should not trigger
		cb.RecordTrade(-20, initialBalance)
		cb.RecordTrade(-20, initialBalance)

		if cb.IsOpen() {
			t.Error("Circuit breaker should not be open after 2 losses")
		}

		// 3rd loss should trigger
		err := cb.RecordTrade(-20, initialBalance)
		if err == nil {
			t.Error("Expected circuit breaker to open")
		}

		if !cb.IsOpen() {
			t.Error("Circuit breaker should be open after 3 consecutive losses")
		}
	})

	// Test daily loss limit
	t.Run("daily loss limit trigger", func(t *testing.T) {
		cb.Reset()

		// One big loss exceeding 5% of balance
		err := cb.RecordTrade(-60, initialBalance) // 6% loss
		if err == nil {
			t.Error("Expected circuit breaker to open")
		}

		if !cb.IsOpen() {
			t.Error("Circuit breaker should be open after exceeding daily loss")
		}
	})

	// Test win resets consecutive losses
	t.Run("win resets consecutive losses", func(t *testing.T) {
		cb.Reset()

		// Use smaller losses to avoid triggering daily loss limit (5%)
		// 3 losses of -10 = 30 total = 3% < 5%
		cb.RecordTrade(-10, initialBalance)
		cb.RecordTrade(-10, initialBalance)
		cb.RecordTrade(30, initialBalance)  // Win resets consecutive counter
		cb.RecordTrade(-10, initialBalance) // Only 1 consecutive loss after win

		if cb.IsOpen() {
			t.Error("Circuit breaker should not be open - counter was reset by win")
		}
	})
}

func TestCircuitBreaker_Cooldown(t *testing.T) {
	// Initialize logger for tests
	logger.Init("error", "")

	cfg := &config.RiskConfig{
		MaxConsecutiveLosses:   2,
		MaxDailyLossPercent:    10.0,
		CircuitBreakerCooldown: 100 * time.Millisecond,
	}

	// Use nil repository for testing (no DB persistence needed)
	cb := NewCircuitBreaker(cfg, nil, 1)

	// Trigger circuit breaker
	cb.RecordTrade(-10, 1000)
	cb.RecordTrade(-10, 1000)

	if !cb.IsOpen() {
		t.Fatal("Circuit breaker should be open")
	}

	// Wait for cooldown
	time.Sleep(150 * time.Millisecond)

	if cb.IsOpen() {
		t.Error("Circuit breaker should auto-close after cooldown")
	}
}
