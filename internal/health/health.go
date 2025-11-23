package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/database"
	redisAdapter "github.com/selivandex/trader-bot/internal/adapters/redis"
	"github.com/selivandex/trader-bot/internal/agents"
	"github.com/selivandex/trader-bot/pkg/logger"
)

// Server provides health check HTTP endpoints for K8s
type Server struct {
	server       *http.Server
	db           *database.DB
	redis        *redisAdapter.Client
	agentManager *agents.AgenticManager
	ready        bool
	readyMu      sync.RWMutex
	startTime    time.Time
}

// HealthStatus represents system health
type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Uptime    string            `json:"uptime"`
	Checks    map[string]string `json:"checks,omitempty"`
}

// ReadinessStatus represents system readiness
type ReadinessStatus struct {
	Ready     bool              `json:"ready"`
	Timestamp string            `json:"timestamp"`
	Checks    map[string]string `json:"checks"`
	Agents    AgentsStatus      `json:"agents"`
}

// AgentsStatus shows agent stats
type AgentsStatus struct {
	Running int `json:"running"`
	Total   int `json:"total"`
}

// NewServer creates new health check server
func NewServer(
	port string,
	db *database.DB,
	redis *redisAdapter.Client,
	agentManager *agents.AgenticManager,
) *Server {
	mux := http.NewServeMux()

	s := &Server{
		server: &http.Server{
			Addr:         ":" + port,
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		db:           db,
		redis:        redis,
		agentManager: agentManager,
		ready:        false,
		startTime:    time.Now(),
	}

	// Health endpoints for K8s probes only
	mux.HandleFunc("/health", s.handleHealth)    // Liveness probe
	mux.HandleFunc("/ready", s.handleReadiness)  // Readiness probe
	mux.HandleFunc("/healthz", s.handleHealth)   // Alias
	mux.HandleFunc("/readyz", s.handleReadiness) // Alias

	return s
}

// Start starts the health check server
func (s *Server) Start() error {
	logger.Info("health check server starting",
		zap.String("addr", s.server.Addr),
	)

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

// Stop gracefully stops the server
func (s *Server) Stop(ctx context.Context) error {
	logger.Info("stopping health check server...")
	return s.server.Shutdown(ctx)
}

// SetReady marks the service as ready
func (s *Server) SetReady(ready bool) {
	s.readyMu.Lock()
	defer s.readyMu.Unlock()
	s.ready = ready

	if ready {
		logger.Info("✅ service marked as READY")
	} else {
		logger.Warn("⚠️ service marked as NOT READY")
	}
}

// handleHealth handles liveness probe - /health
// Returns 200 if process is alive (even if dependencies are down)
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Uptime:    time.Since(s.startTime).Round(time.Second).String(),
	}

	// Optional: include dependency checks (for debugging)
	if r.URL.Query().Get("verbose") == "true" {
		checks := make(map[string]string)

		// Check DB (non-blocking)
		if err := s.db.Health(); err != nil {
			checks["database"] = "unhealthy: " + err.Error()
		} else {
			checks["database"] = "healthy"
		}

		// Check Redis (non-blocking)
		if err := s.redis.Health(); err != nil {
			checks["redis"] = "unhealthy: " + err.Error()
		} else {
			checks["redis"] = "healthy"
		}

		status.Checks = checks
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// handleReadiness handles readiness probe - /ready
// Returns 200 only if service is ready to accept traffic
func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	s.readyMu.RLock()
	ready := s.ready
	s.readyMu.RUnlock()

	checks := make(map[string]string)
	allHealthy := true

	// Check Database
	if err := s.db.Health(); err != nil {
		checks["database"] = "unhealthy: " + err.Error()
		allHealthy = false
	} else {
		checks["database"] = "healthy"
	}

	// Check Redis
	if err := s.redis.Health(); err != nil {
		checks["redis"] = "unhealthy: " + err.Error()
		allHealthy = false
	} else {
		checks["redis"] = "healthy"
	}

	// Agent stats
	runningAgents := s.agentManager.GetRunningAgents()
	agentsStatus := AgentsStatus{
		Running: len(runningAgents),
		Total:   len(runningAgents), // TODO: Get total from DB
	}

	// Service is ready if:
	// 1. Marked as ready (startup complete)
	// 2. Dependencies are healthy
	isReady := ready && allHealthy

	status := ReadinessStatus{
		Ready:     isReady,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
		Agents:    agentsStatus,
	}

	w.Header().Set("Content-Type", "application/json")

	if isReady {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(status)
}
