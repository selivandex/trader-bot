package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/amyangfei/redlock-go/v3/redlock"
	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/config"
	"github.com/selivandex/trader-bot/pkg/logger"
)

// Client wraps RedLock manager for distributed locking
type Client struct {
	lockManager *redlock.RedLock
	redisAddrs  []string
}

// New creates new Redis client with RedLock support
func New(cfg *config.RedisConfig) (*Client, error) {
	// Build Redis address
	addr := fmt.Sprintf("tcp://%s:%d", cfg.Host, cfg.Port)

	// For production with Redis cluster, you would provide multiple addresses:
	// []string{"tcp://redis1:6379", "tcp://redis2:6379", "tcp://redis3:6379"}
	// For now, using single instance (works but less fault-tolerant)
	redisAddrs := []string{addr}

	// Create RedLock manager
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	lockManager, err := redlock.NewRedLock(ctx, redisAddrs)
	if err != nil {
		return nil, fmt.Errorf("failed to create redlock manager: %w", err)
	}

	logger.Info("redis redlock manager initialized",
		zap.Strings("addresses", redisAddrs),
	)

	return &Client{
		lockManager: lockManager,
		redisAddrs:  redisAddrs,
	}, nil
}

// GetLockManager returns RedLock manager for distributed locking
func (c *Client) GetLockManager() *redlock.RedLock {
	return c.lockManager
}

// Close closes redis connections
func (c *Client) Close() error {
	if c.lockManager != nil {
		logger.Info("closing redis redlock connections")
		// RedLock manager doesn't have explicit Close, connections close automatically
	}
	return nil
}

// Health checks redis health
func (c *Client) Health() error {
	// Try to acquire and release a test lock
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	testLock := "health:check"
	expiry, err := c.lockManager.Lock(ctx, testLock, 1*time.Second)
	if err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}

	if expiry <= 0 {
		return fmt.Errorf("redis health check failed: invalid expiry")
	}

	// Release test lock
	_ = c.lockManager.UnLock(ctx, testLock)

	return nil
}
