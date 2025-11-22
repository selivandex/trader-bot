package redis

import (
	"context"

	"github.com/amyangfei/redlock-go/v3/redlock"
)

// LockFactory creates distributed locks for agents
type LockFactory interface {
	CreateAgentLock(agentID string) AgentLock
}

// RedisLockFactory creates Redis-based distributed locks
type RedisLockFactory struct {
	lockManager *redlock.RedLock
}

// NewRedisLockFactory creates new Redis lock factory
func NewRedisLockFactory(lockManager *redlock.RedLock) *RedisLockFactory {
	return &RedisLockFactory{
		lockManager: lockManager,
	}
}

// CreateAgentLock creates a distributed lock for specific agent
func (f *RedisLockFactory) CreateAgentLock(agentID string) AgentLock {
	return NewDistributedLock(f.lockManager, agentID)
}

// MockLockFactory for testing (always succeeds)
type MockLockFactory struct{}

// NewMockLockFactory creates mock lock factory for tests
func NewMockLockFactory() *MockLockFactory {
	return &MockLockFactory{}
}

// CreateAgentLock creates a mock lock that always succeeds
func (f *MockLockFactory) CreateAgentLock(agentID string) AgentLock {
	return &MockLock{agentID: agentID}
}

// MockLock is a no-op lock for testing
type MockLock struct {
	agentID string
}

func (l *MockLock) TryAcquire(ctx context.Context) (bool, error) {
	return true, nil // Always succeeds
}

func (l *MockLock) Release(ctx context.Context) error {
	return nil // Always succeeds
}

func (l *MockLock) CheckLockHeld(ctx context.Context) (bool, error) {
	return true, nil // Always held
}

func (l *MockLock) GetAgentID() string {
	return l.agentID
}
