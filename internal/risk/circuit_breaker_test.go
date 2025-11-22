package risk

import (
	"testing"
	"time"

	"github.com/alexanderselivanov/trader/internal/adapters/config"
)

func TestCircuitBreaker_RecordTrade(t *testing.T) {
	cfg := &config.RiskConfig{
		MaxConsecutiveLosses:   3,
		MaxDailyLossPercent:    5.0,
		CircuitBreakerCooldown: 1 * time.Hour,
	}
	
	cb := NewCircuitBreaker(cfg)
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
		
		cb.RecordTrade(-20, initialBalance)
		cb.RecordTrade(-20, initialBalance)
		cb.RecordTrade(30, initialBalance) // Win resets counter
		cb.RecordTrade(-20, initialBalance)
		
		if cb.IsOpen() {
			t.Error("Circuit breaker should not be open - counter was reset by win")
		}
	})
}

func TestCircuitBreaker_Cooldown(t *testing.T) {
	cfg := &config.RiskConfig{
		MaxConsecutiveLosses:   2,
		MaxDailyLossPercent:    10.0,
		CircuitBreakerCooldown: 100 * time.Millisecond,
	}
	
	cb := NewCircuitBreaker(cfg)
	
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

