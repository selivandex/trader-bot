package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/amyangfei/redlock-go/v3/redlock"
	redis "github.com/go-redis/redis/v8"
	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/config"
	"github.com/selivandex/trader-bot/pkg/logger"
)

// Client wraps RedLock manager for distributed locking + standard Redis for caching
type Client struct {
	lockManager *redlock.RedLock
	cache       *redis.Client
	redisAddrs  []string
}

// New creates new Redis client with RedLock support + caching
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

	// Create standard Redis client for caching (embeddings, etc)
	cacheClient := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
	})

	// Test cache connection
	if err := cacheClient.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis cache: %w", err)
	}

	logger.Info("redis cache client initialized",
		zap.String("address", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)),
		zap.Int("db", cfg.DB),
	)

	return &Client{
		lockManager: lockManager,
		redisAddrs:  redisAddrs,
		cache:       cacheClient,
	}, nil
}

// GetLockManager returns RedLock manager for distributed locking
func (c *Client) GetLockManager() *redlock.RedLock {
	return c.lockManager
}

// GetLockFactory returns a lock factory for creating agent locks
func (c *Client) GetLockFactory() LockFactory {
	return NewRedisLockFactory(c.lockManager)
}

// Close closes redis connections
func (c *Client) Close() error {
	if c.lockManager != nil {
		logger.Info("closing redis redlock connections")
		// RedLock manager doesn't have explicit Close, connections close automatically
	}

	if c.cache != nil {
		logger.Info("closing redis cache client")
		if err := c.cache.Close(); err != nil {
			return fmt.Errorf("failed to close redis cache: %w", err)
		}
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

// ============ CACHING METHODS ============

// Get retrieves value from Redis cache
func (c *Client) Get(ctx context.Context, key string) *redis.StringCmd {
	return c.cache.Get(ctx, key)
}

// Set stores value in Redis cache with TTL
func (c *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	return c.cache.Set(ctx, key, value, expiration)
}

// Del deletes keys from Redis cache
func (c *Client) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	return c.cache.Del(ctx, keys...)
}

// Exists checks if key exists in Redis cache
func (c *Client) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	return c.cache.Exists(ctx, keys...)
}
