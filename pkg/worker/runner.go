package worker

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
)

// Worker interface that background workers should implement
type Worker interface {
	// Name returns worker name for logging
	Name() string
	// Run executes one iteration of work
	Run(ctx context.Context) error
}

// PeriodicWorker wraps a Worker with periodic execution
type PeriodicWorker struct {
	worker   Worker
	interval time.Duration
	wg       *sync.WaitGroup
	name     string
}

// NewPeriodicWorker creates new periodic worker
func NewPeriodicWorker(worker Worker, interval time.Duration) *PeriodicWorker {
	return &PeriodicWorker{
		worker:   worker,
		interval: interval,
		wg:       &sync.WaitGroup{},
		name:     worker.Name(),
	}
}

// Start starts the worker with graceful shutdown support
func (pw *PeriodicWorker) Start(ctx context.Context) {
	pw.wg.Add(1)
	go pw.run(ctx)
}

// Stop waits for graceful shutdown
func (pw *PeriodicWorker) Stop(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		pw.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("✅ Worker stopped gracefully",
			zap.String("worker", pw.name),
		)
	case <-time.After(timeout):
		logger.Warn("⚠️ Worker stop timeout",
			zap.String("worker", pw.name),
		)
	}
}

// run executes worker periodically
func (pw *PeriodicWorker) run(ctx context.Context) {
	defer pw.wg.Done()

	logger.Info("🚀 Worker started",
		zap.String("worker", pw.name),
		zap.Duration("interval", pw.interval),
	)

	// Run immediately on start
	if err := pw.worker.Run(ctx); err != nil {
		logger.Error("worker execution failed",
			zap.String("worker", pw.name),
			zap.Error(err),
		)
	}

	ticker := time.NewTicker(pw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("🛑 Worker stopping",
				zap.String("worker", pw.name),
			)
			return

		case <-ticker.C:
			if err := pw.worker.Run(ctx); err != nil {
				logger.Error("worker execution failed",
					zap.String("worker", pw.name),
					zap.Error(err),
				)
				// Continue despite error - don't crash worker
			}
		}
	}
}

// WorkerGroup manages multiple workers with graceful shutdown
type WorkerGroup struct {
	workers []*PeriodicWorker
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
}

// NewWorkerGroup creates new worker group
func NewWorkerGroup(ctx context.Context) *WorkerGroup {
	ctx, cancel := context.WithCancel(ctx)
	return &WorkerGroup{
		workers: make([]*PeriodicWorker, 0),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Add adds worker to group
func (wg *WorkerGroup) Add(worker Worker, interval time.Duration) {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	pw := NewPeriodicWorker(worker, interval)
	wg.workers = append(wg.workers, pw)
}

// Start starts all workers
func (wg *WorkerGroup) Start() {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	for _, worker := range wg.workers {
		worker.Start(wg.ctx)
	}

	logger.Info("🚀 Worker group started",
		zap.Int("workers", len(wg.workers)),
	)
}

// Stop stops all workers gracefully
func (wg *WorkerGroup) Stop(timeout time.Duration) {
	logger.Info("🛑 Stopping worker group...",
		zap.Int("workers", len(wg.workers)),
	)

	// Cancel context first
	wg.cancel()

	// Wait for all workers with timeout
	wg.mu.Lock()
	defer wg.mu.Unlock()

	for _, worker := range wg.workers {
		worker.Stop(timeout)
	}

	logger.Info("✅ Worker group stopped")
}

// RunBackground is a convenience function to run single worker
// Usage: worker.RunBackground(ctx, myWorker, 30*time.Second)
func RunBackground(ctx context.Context, worker Worker, interval time.Duration) *PeriodicWorker {
	pw := NewPeriodicWorker(worker, interval)
	pw.Start(ctx)
	return pw
}
