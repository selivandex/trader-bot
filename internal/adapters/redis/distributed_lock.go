package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/amyangfei/redlock-go/v3/redlock"
	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
)

// DistributedLock wraps redlock-go library for agent distribution across pods
type DistributedLock struct {
	lockManager *redlock.RedLock
	agentID     string
	lockName    string
	ttl         time.Duration
	locked      bool
}

// NewDistributedLock creates new distributed lock manager using redlock-go
func NewDistributedLock(lockManager *redlock.RedLock, agentID string) *DistributedLock {
	return &DistributedLock{
		lockManager: lockManager,
		agentID:     agentID,
		lockName:    fmt.Sprintf("agent:lock:%s", agentID),
		ttl:         30 * time.Second, // Lock TTL
		locked:      false,
	}
}

// TryAcquire attempts to acquire exclusive lock for agent using Redlock algorithm
// Returns true if lock was acquired, false if agent is already locked by another pod
func (dl *DistributedLock) TryAcquire(ctx context.Context) (bool, error) {
	// Try to acquire lock with TTL
	expiry, err := dl.lockManager.Lock(ctx, dl.lockName, dl.ttl)
	if err != nil {
		// Lock not acquired - another pod has it
		logger.Debug("agent lock already held by another pod",
			zap.String("agent_id", dl.agentID),
			zap.String("lock_name", dl.lockName),
		)
		return false, nil
	}

	if expiry <= 0 {
		// Lock acquisition failed
		return false, fmt.Errorf("failed to acquire lock: invalid expiry %v", expiry)
	}

	dl.locked = true

	logger.Info("agent lock acquired",
		zap.String("agent_id", dl.agentID),
		zap.String("lock_name", dl.lockName),
		zap.Duration("ttl", dl.ttl),
		zap.Duration("expiry", expiry),
	)

	// Start automatic lock renewal
	go dl.renewLock(ctx)

	return true, nil
}

// Release releases the Redis distributed lock
func (dl *DistributedLock) Release(ctx context.Context) error {
	if !dl.locked {
		return nil // No lock to release
	}

	err := dl.lockManager.UnLock(ctx, dl.lockName)
	if err != nil {
		logger.Warn("failed to release lock (may have already expired)",
			zap.String("agent_id", dl.agentID),
			zap.String("lock_name", dl.lockName),
			zap.Error(err),
		)
		// Don't return error - lock may have already expired naturally
	} else {
		logger.Info("agent lock released",
			zap.String("agent_id", dl.agentID),
			zap.String("lock_name", dl.lockName),
		)
	}

	dl.locked = false
	return nil
}

// renewLock automatically renews the lock before it expires
func (dl *DistributedLock) renewLock(ctx context.Context) {
	// Renew at 2/3 of TTL to have safety margin
	renewInterval := (dl.ttl * 2) / 3
	ticker := time.NewTicker(renewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Debug("lock renewal stopped (context cancelled)",
				zap.String("agent_id", dl.agentID),
			)
			return

		case <-ticker.C:
			if !dl.locked {
				return // Lock was released
			}

			// Release and re-acquire to extend TTL
			// Redlock-go doesn't have built-in renewal, so we do release+acquire
			err := dl.lockManager.UnLock(ctx, dl.lockName)
			if err != nil {
				logger.Error("lock renewal failed (unlock)",
					zap.String("agent_id", dl.agentID),
					zap.Error(err),
				)
				dl.locked = false
				return
			}

			expiry, err := dl.lockManager.Lock(ctx, dl.lockName, dl.ttl)
			if err != nil || expiry <= 0 {
				logger.Error("lock lost - another pod may have taken over!",
					zap.String("agent_id", dl.agentID),
					zap.String("lock_name", dl.lockName),
					zap.Error(err),
				)
				dl.locked = false
				return
			}

			logger.Debug("lock renewed successfully",
				zap.String("agent_id", dl.agentID),
				zap.Duration("expiry", expiry),
			)
		}
	}
}

// CheckLockHeld verifies if we still hold the lock
func (dl *DistributedLock) CheckLockHeld(ctx context.Context) (bool, error) {
	return dl.locked, nil
}

// GetAgentID returns the agent ID this lock is for
func (dl *DistributedLock) GetAgentID() string {
	return dl.agentID
}
