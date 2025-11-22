# K8s Distributed Locking & Graceful Shutdown

This document describes how the trading system handles multi-pod deployments in Kubernetes with distributed agent locking and graceful shutdown.

## Problem

When running multiple pods in K8s, each pod would try to start the same agents from the database, leading to:
- ❌ Multiple instances of the same agent running simultaneously
- ❌ Duplicate trades
- ❌ Risk multiplication
- ❌ Balance corruption

## Solution

### 1. Redis Distributed Locking (Redlock)

We use the battle-tested [redlock-go](https://github.com/amyangfei/redlock-go) library implementing the Redlock algorithm.

**How it works:**
```
Pod 1 starts → TryAcquire(Agent A) → ✅ SUCCESS → Agent A runs in Pod 1
Pod 2 starts → TryAcquire(Agent A) → ❌ LOCKED   → Skip Agent A
Pod 3 starts → TryAcquire(Agent A) → ❌ LOCKED   → Skip Agent A

Result: Agent A runs ONLY in Pod 1 ✅
```

**Key features:**
- Lock TTL: 30 seconds
- Auto-renewal: Every 20 seconds (2/3 of TTL)
- Atomic operations using Lua scripts
- Automatic lock release on pod crash (TTL expiry)

### 2. Graceful Shutdown

When K8s sends SIGTERM (e.g., during deployment):

```go
1. Signal received → context.Cancel()
2. Stop all agents → CancelFunc()
3. Wait for goroutines → WaitGroup.Wait() (max 25s)
4. Save final state → database
5. Release locks → Redis
6. Close connections → DB, Redis
```

**Timeout:** 25 seconds (K8s gives 30s `terminationGracePeriodSeconds`)

If agents don't stop in time:
- ⚠️ Warning logged
- Locks expire naturally after 30s
- Other pods can take over

## Architecture

```
internal/adapters/redis/
  ├── client.go              # Redis connection with Redlock manager
  ├── distributed_lock.go     # Lock implementation
  └── lock_interface.go       # AgentLock interface

internal/agents/
  └── manager.go              # Uses locks via interface
```

## Configuration

### Environment Variables

```bash
# Redis (required for multi-pod deployments)
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
```

### Docker Compose

```yaml
redis:
  image: redis:7-alpine
  command: redis-server --appendonly yes
  ports:
    - "6379:6379"
  volumes:
    - redis_data:/data
```

### Kubernetes

For production, use Redis cluster:

```yaml
# values.yaml for redis helm chart
cluster:
  enabled: true
  slaveCount: 2

persistence:
  enabled: true
  size: 1Gi
```

Then configure multiple Redis nodes:
```bash
REDIS_ADDRESSES=tcp://redis-1:6379,tcp://redis-2:6379,tcp://redis-3:6379
```

## Usage

### Starting an Agent

```go
// Telegram bot: /start_agent AGENT_ID
// 1. Try to acquire lock
lock := redis.NewDistributedLock(lockManager, agentID)
acquired, err := lock.TryAcquire(ctx)

if !acquired {
    return fmt.Errorf("agent already running in another pod")
}

// 2. Start agent with lock
runner := &AgenticRunner{
    Lock: lock,
    // ...
}
```

### Stopping an Agent

```go
// Telegram bot: /stop_agent AGENT_ID
// 1. Cancel agent context
runner.CancelFunc()

// 2. Wait for agent to finish
// (automatic via WaitGroup in Shutdown)

// 3. Release lock
runner.Lock.Release(ctx)
```

### Agent Restoration (TODO)

For automatic agent recovery after pod restart, implement `RestoreRunningAgents()`:

```go
func (am *AgenticManager) RestoreRunningAgents(ctx context.Context) error {
    // 1. Load active agents from DB
    agents, err := am.repository.GetActiveAgents(ctx)
    
    // 2. Try to acquire lock for each
    for _, agent := range agents {
        lock := redis.NewDistributedLock(lockManager, agent.ID)
        acquired, _ := lock.TryAcquire(ctx)
        
        if !acquired {
            // Skip - another pod owns this agent
            continue
        }
        
        // 3. Start agent in this pod
        am.StartAgenticAgent(ctx, agent.ID, ...)
    }
}
```

Call in `main.go`:
```go
agenticManager := agents.NewAgenticManager(...)
agenticManager.RestoreRunningAgents(ctx) // Auto-restore agents
```

## Testing

### Local Testing

```bash
# Start Redis
docker-compose up redis

# Start multiple instances
go run cmd/bot/main.go &
go run cmd/bot/main.go &
go run cmd/bot/main.go &

# Create and start agent via Telegram
/create_agent conservative "Test Agent"
/assign_agent AGENT_ID BTC/USDT 500
/start_agent AGENT_ID

# Check logs - only ONE pod should run the agent
```

### K8s Testing

```bash
# Deploy with 3 replicas
kubectl scale deployment trader --replicas=3

# Start agent via Telegram
# Watch logs from all pods
kubectl logs -f -l app=trader

# Only one pod should have "agent lock acquired" message
```

### Graceful Shutdown Testing

```bash
# Start agent
/start_agent AGENT_ID

# Trigger pod restart
kubectl delete pod trader-xyz

# Check logs:
# ✅ "shutting down gracefully"
# ✅ "all agents stopped gracefully"
# ✅ "agent lock released"
```

## Monitoring

### Key Metrics to Track

1. **Lock acquisition failures**
   - High rate = Redis issues or too many pods
   
2. **Shutdown timeouts**
   - Agents taking > 25s to stop
   - May need to increase timeout
   
3. **Lock expiry without release**
   - Pod crashed before releasing
   - Lock TTL expired naturally

### Redis Commands

```bash
# Check active locks
redis-cli KEYS "agent:lock:*"

# Check specific agent lock
redis-cli GET "agent:lock:AGENT_ID_HERE"

# TTL remaining
redis-cli TTL "agent:lock:AGENT_ID_HERE"

# Force release (emergency only!)
redis-cli DEL "agent:lock:AGENT_ID_HERE"
```

## Trade-offs

### Redlock vs PostgreSQL Advisory Locks

| Feature | Redlock (Redis) | PG Advisory Locks |
|---------|----------------|-------------------|
| Performance | ⚡ Faster | 🐢 Slower |
| Fault tolerance | ✅ Independent | ❌ DB dependent |
| Lock expiry | ✅ Automatic TTL | ❌ Manual only |
| Complexity | 📦 +1 service | ✅ Uses existing DB |
| Scalability | ⚡ High | 🐢 Limited |

**Recommendation:** Redis Redlock for production

### Single Redis vs Redis Cluster

| | Single | Cluster |
|-|--------|---------|
| Availability | ❌ SPOF | ✅ HA |
| Consistency | ✅ Perfect | ⚠️ Eventually consistent |
| Cost | 💰 Cheap | 💰💰 Expensive |
| Setup | ✅ Simple | 📚 Complex |

**Recommendation:** 
- Development: Single Redis
- Production: Redis Cluster (3+ nodes)

## Troubleshooting

### Agent stuck "already running"

```bash
# Check if lock exists
redis-cli GET "agent:lock:AGENT_ID"

# Check TTL
redis-cli TTL "agent:lock:AGENT_ID"

# If stuck after pod crash, wait 30s for TTL expiry
# Or force release (use carefully!)
redis-cli DEL "agent:lock:AGENT_ID"
```

### Multiple pods running same agent

1. Check Redis connectivity from all pods
2. Verify Redis is reachable
3. Check for network partitions
4. Review lock acquisition logs

### Shutdown taking too long

1. Check agent decision interval (should be < 25s)
2. Review agent goroutines for blocking operations
3. Consider increasing termination grace period in K8s

## Future Improvements

1. **Leader Election** for control plane operations
2. **Health checks** with lock validation
3. **Metrics** for lock acquisition latency
4. **Auto-recovery** with `RestoreRunningAgents()`
5. **Lock priority** for critical agents

## References

- [Redlock Algorithm](https://redis.io/topics/distlock)
- [redlock-go Library](https://github.com/amyangfei/redlock-go)
- [K8s Graceful Shutdown](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination)

