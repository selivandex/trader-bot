package redis

import "context"

// AgentLock defines interface for distributed agent locking
// This allows swapping implementations (Redis, PostgreSQL, etcd, etc.)
type AgentLock interface {
	// TryAcquire attempts to acquire exclusive lock for agent
	// Returns true if lock was acquired, false if already locked
	TryAcquire(ctx context.Context) (bool, error)

	// Release releases the lock
	Release(ctx context.Context) error

	// CheckLockHeld verifies if we still hold the lock
	CheckLockHeld(ctx context.Context) (bool, error)

	// GetAgentID returns the agent ID this lock is for
	GetAgentID() string
}
