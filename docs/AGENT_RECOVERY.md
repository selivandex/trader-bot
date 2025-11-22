# Agent Recovery & Distributed Architecture

## Overview

This document describes how AI agents are recovered after pod restarts in a distributed Kubernetes environment.

## Problem Statement

When running in Kubernetes with multiple pods:
1. User creates agent via Telegram → Agent stored in database
2. User starts agent via `/start_agent` → Agent runs in Pod A
3. Pod A restarts/crashes → Agent stops trading ❌
4. **Without recovery**: Agent stays stopped until user manually restarts it

## Solution: Automatic Agent Recovery

### Architecture Components

1. **PostgreSQL Database** - Source of truth for agent state
   - `agent_configs.is_active` - Agent enabled/disabled (persistent)
   - `agent_states.is_trading` - Agent currently trading (runtime flag)

2. **Redis Distributed Locks (Redlock)** - Prevent duplicate agents
   - Each agent has a unique lock by `agent_id`
   - Lock held while agent is running
   - Automatically released on graceful shutdown
   - TTL-based expiration (lock expires if pod crashes)

3. **Agent Recovery Flow** - Automatic restoration on startup

### How It Works

#### 1. Pod Startup (Normal Start)

```
┌──────────────────┐
│  Pod Starts      │
└────────┬─────────┘
         │
         ▼
┌──────────────────────────────────┐
│ RestoreRunningAgents()           │
│ - Queries DB for active agents   │
│ - Filters: is_active=true        │
│           is_trading=true        │
└────────┬─────────────────────────┘
         │
         ▼
┌──────────────────────────────────┐
│ For each agent:                  │
│ 1. Try acquire Redis lock        │
│ 2. If locked → Skip (running)    │
│ 3. If free → Start agent         │
└──────────────────────────────────┘
```

#### 2. Agent Lifecycle

**Creation:**
```bash
/create_agent conservative "My Agent"
# ✅ Creates agent_configs entry
# ❌ Does NOT start agent
```

**Assignment:**
```bash
/assign_agent abc123 BTC/USDT 500
# ✅ Creates agent_symbol_assignments
# ❌ Still not running
```

**Starting:**
```bash
/start_agent abc123
# ✅ Acquires Redis lock
# ✅ Sets is_trading=true
# ✅ Starts agent loop in goroutine
# ✅ Agent actively trading
```

**Pod Restart:**
```
Pod crashes → Lock expires after TTL → New pod starts
                                            ↓
                           RestoreRunningAgents()
                                            ↓
                        Queries: is_trading=true
                                            ↓
                          Acquires lock → Starts agent
                                            ↓
                              ✅ Agent restored!
```

#### 3. Multi-Pod Scenario

**3 Pods, 2 Agents:**

```
┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│   Pod A     │  │   Pod B     │  │   Pod C     │
│             │  │             │  │             │
│ Agent-1 🔒  │  │ Agent-2 🔒  │  │   (empty)   │
└─────────────┘  └─────────────┘  └─────────────┘
      │                  │
      └──────┬───────────┘
             │
        Redis Locks:
        agent-1 → Pod A
        agent-2 → Pod B
```

**Pod A Crashes:**

```
┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│   Pod A     │  │   Pod B     │  │   Pod C     │
│   (crash)   │  │             │  │             │
│             │  │ Agent-2 🔒  │  │   (empty)   │
└─────────────┘  └─────────────┘  └─────────────┘
                         │
    Lock expires     Redis Locks:
    after 30s        agent-2 → Pod B
                     agent-1 → (expired)
```

**Pod A Restarts:**

```
┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│   Pod A     │  │   Pod B     │  │   Pod C     │
│ (starting)  │  │             │  │             │
│ Agent-1 🔒  │  │ Agent-2 🔒  │  │   (empty)   │
└─────────────┘  └─────────────┘  └─────────────┘
      │                  │
      └──────┬───────────┘
             │
        Redis Locks:
        agent-1 → Pod A (restored!)
        agent-2 → Pod B
```

### Code Flow

#### 1. Database Query

```go
// internal/agents/repository.go
func (r *Repository) GetAgentsToRestore(ctx context.Context) ([]AgentToRestore, error) {
    query := `
        SELECT 
            ac.id as agent_id,
            ac.user_id,
            ast.symbol,
            ast.balance,
            ue.exchange,
            ue.api_key,
            ue.api_secret,
            ue.testnet
        FROM agent_configs ac
        INNER JOIN agent_states ast ON ac.id = ast.agent_id
        INNER JOIN user_trading_pairs utp ON ast.symbol = utp.symbol
        INNER JOIN user_exchanges ue ON utp.exchange_id = ue.id
        WHERE ac.is_active = true
          AND ast.is_trading = true
    `
    // Returns list of agents that should be running
}
```

#### 2. Recovery Loop

```go
// internal/agents/manager.go
func (am *AgenticManager) RestoreRunningAgents(ctx context.Context, ...) error {
    agentsToRestore, err := am.repository.GetAgentsToRestore(ctx)
    
    for _, agentInfo := range agentsToRestore {
        // Try acquire lock
        lock := NewDistributedLock(agentInfo.AgentID)
        acquired, err := lock.TryAcquire(ctx)
        
        if !acquired {
            // Another pod already running this agent
            continue
        }
        
        // Start agent
        am.StartAgenticAgent(ctx, agentInfo.AgentID, ...)
    }
}
```

#### 3. Main Startup

```go
// cmd/bot/main.go
func run(ctx context.Context) error {
    // Initialize components
    agenticManager := agents.NewAgenticManager(...)
    
    // Restore agents
    exchangeFactory := createExchangeFactory(cfg)
    agenticManager.RestoreRunningAgents(ctx, exchangeFactory)
    
    // Continue with normal startup...
}
```

### Lock Management

**Redis Distributed Lock (Redlock)**

```go
type DistributedLock interface {
    TryAcquire(ctx context.Context) (bool, error)
    Release(ctx context.Context) error
    Refresh(ctx context.Context) error
}
```

**Lock Properties:**
- **Key**: `agent:lock:{agent_id}`
- **TTL**: 30 seconds (refreshed every 10s while agent running)
- **Value**: Pod name/ID (for debugging)

**Lock States:**

| State | Description | Recovery Action |
|-------|-------------|-----------------|
| 🔓 Free | No pod holds lock | **Acquire & Start** |
| 🔒 Locked | Another pod holds lock | **Skip** (agent running elsewhere) |
| ⏰ Expired | Pod crashed, TTL expired | **Acquire & Start** (automatic recovery) |

### Graceful Shutdown

When pod receives SIGTERM (Kubernetes termination):

```go
func (am *AgenticManager) Shutdown() error {
    // 1. Stop accepting new agents
    healthServer.SetReady(false)
    
    // 2. Cancel all agent contexts
    for _, runner := range am.runningAgents {
        runner.CancelFunc()
        runner.IsTrading = false
    }
    
    // 3. Wait for agents to finish (max 25s)
    am.wg.Wait()
    
    // 4. Save final states
    for _, runner := range am.runningAgents {
        runner.State.IsTrading = false
        am.repository.CreateAgentState(ctx, runner.State)
    }
    
    // 5. Release all locks
    for _, runner := range am.runningAgents {
        runner.Lock.Release(ctx)
    }
    
    // 6. Close connections
    db.Close()
    redisClient.Close()
}
```

**Timeline:**
```
t=0s   SIGTERM received
       ├─ Mark not ready (stop health checks)
       ├─ Cancel agent contexts
       
t=0-25s Agents finish current iteration
       ├─ Close positions
       ├─ Save memory
       ├─ Update state
       
t=25s  Shutdown timeout
       ├─ Force stop remaining agents
       ├─ Save final states
       ├─ Release locks
       
t=30s  Kubernetes kills pod (terminationGracePeriodSeconds)
```

## Configuration

### Environment Variables

```bash
# Redis Configuration (for distributed locking)
REDIS_HOST=redis-service
REDIS_PORT=6379
REDIS_PASSWORD=your-password
REDIS_DB=0

# Agent Recovery
AGENT_RECOVERY_ENABLED=true
AGENT_LOCK_TTL=30s
AGENT_LOCK_REFRESH_INTERVAL=10s
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: trader-bot
spec:
  replicas: 3  # Multiple pods
  template:
    spec:
      terminationGracePeriodSeconds: 30  # Time for graceful shutdown
      containers:
      - name: trader-bot
        image: trader-bot:latest
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

## Monitoring

### Metrics to Track

1. **Agent Recovery**
   - `agents_restored_total` - Total agents restored
   - `agents_restore_failed_total` - Failed restorations
   - `agent_restore_duration_seconds` - Time to restore

2. **Lock Status**
   - `agent_lock_acquire_total` - Lock acquisitions
   - `agent_lock_acquire_failed_total` - Failed lock acquisitions
   - `agent_lock_held_total` - Currently held locks

3. **Graceful Shutdown**
   - `shutdown_duration_seconds` - Shutdown time
   - `agents_saved_on_shutdown` - Agents gracefully stopped

### Logs to Monitor

```
🔄 restoring running agents from database...
found agents to restore count=5

✅ agent restored successfully agent_id=abc123 symbol=BTC/USDT
agent already running in another pod (lock held), skipping agent_id=def456

🎯 agent restoration complete total=5 restored=3 skipped=2 failed=0
```

## Edge Cases & Failure Modes

### 1. Redis Unavailable at Startup

**Problem:** Redis down, can't check locks

**Solution:** 
- Retry connection with exponential backoff
- If Redis unavailable after 5 retries → Start WITHOUT recovery
- Log warning: "⚠️ Redis unavailable, agent recovery skipped"
- User can manually restart agents via Telegram

### 2. Database Unavailable

**Problem:** Can't query agents to restore

**Solution:**
- Startup fails (database is critical)
- K8s restarts pod automatically
- Retry with backoff

### 3. Exchange API Unavailable

**Problem:** Agent recovered but can't connect to exchange

**Solution:**
- Agent starts but logs errors
- Circuit breaker prevents spam requests
- Agent retries with exponential backoff
- User notified via Telegram

### 4. Split-Brain (Network Partition)

**Problem:** Two pods think they can run same agent

**Solution:**
- Redis Redlock algorithm prevents this
- Lock refresh ensures only one holder
- If network partition → Lock expires → Re-election

### 5. Lock Leak (Pod crashes without releasing)

**Problem:** Pod crashes, lock never released

**Solution:**
- **TTL-based expiration** (30s default)
- Lock automatically expires
- Next pod acquires lock
- ✅ **Automatic recovery**

## Testing

### Unit Tests

```go
func TestAgentRecovery(t *testing.T) {
    // Test recovery flow
    // Test lock acquisition
    // Test skip if locked
}
```

### Integration Tests

```bash
# Start system with 2 agents
make start
curl -X POST /api/agents/start/agent-1
curl -X POST /api/agents/start/agent-2

# Kill pod
kubectl delete pod trader-bot-xxx

# Verify recovery
kubectl logs trader-bot-yyy | grep "agent restored"
# ✅ agent restored successfully agent_id=agent-1
# ✅ agent restored successfully agent_id=agent-2
```

## Best Practices

1. **Always use graceful shutdown** - Prevents state inconsistencies
2. **Monitor lock metrics** - Detect split-brain early
3. **Set appropriate TTLs** - Balance recovery speed vs false positives
4. **Test pod restarts** - Verify recovery works
5. **Alert on failed recovery** - Manual intervention may be needed

## Future Improvements

1. **Agent Heartbeat Worker** - Periodically check DB for new agents
2. **Leader Election** - One pod owns agent scheduling
3. **Agent Migration** - Move agents between pods for load balancing
4. **PostgreSQL Advisory Locks** - Alternative to Redis (simpler)
5. **Agent Auto-Scaling** - Start new pods when many agents active

## Summary

✅ **Agents automatically recover after pod restarts**
✅ **Distributed locking prevents duplicates**
✅ **Graceful shutdown preserves agent state**
✅ **Multi-pod deployment supported**
✅ **Zero manual intervention required**

When user creates and starts agent → It keeps running even if pod restarts! 🚀

