package workers

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/agents"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// PositionMonitorWorker monitors open positions and triggers reflection when closed
type PositionMonitorWorker struct {
	agentManager *agents.AgenticManager
	repository   *agents.Repository
}

// NewPositionMonitorWorker creates new position monitor worker
func NewPositionMonitorWorker(
	agentManager *agents.AgenticManager,
	repository *agents.Repository,
) *PositionMonitorWorker {
	return &PositionMonitorWorker{
		agentManager: agentManager,
		repository:   repository,
	}
}

// Name returns worker name
func (w *PositionMonitorWorker) Name() string {
	return "position_monitor"
}

// Run executes ONE iteration of position monitoring
// Called periodically by PeriodicWorker from pkg/worker
func (w *PositionMonitorWorker) Run(ctx context.Context) error {
	runners := w.agentManager.GetRunningAgents()

	if len(runners) == 0 {
		logger.Debug("no running agents to monitor")
		return nil
	}

	logger.Debug("🔍 Monitoring positions",
		zap.Int("agents", len(runners)),
	)

	closedCount := 0

	for _, runner := range runners {
		if err := w.checkAgentPosition(ctx, runner); err != nil {
			logger.Error("failed to check agent position",
				zap.String("agent_id", runner.Config.ID),
				zap.String("agent_name", runner.Config.Name),
				zap.Error(err),
			)
			continue
		}

		// Check if position was closed in this iteration
		if runner.State.PositionJustClosed {
			closedCount++
			runner.State.PositionJustClosed = false // Reset flag
		}
	}

	if closedCount > 0 {
		logger.Info("📊 Position closures detected",
			zap.Int("closed_positions", closedCount),
		)
	}

	return nil
}

// checkAgentPosition checks single agent's position for closure
func (w *PositionMonitorWorker) checkAgentPosition(ctx context.Context, runner *agents.AgenticRunner) error {
	symbol := runner.State.Symbol

	// 1. Fetch current position from exchange
	currentPos, err := runner.Exchange.FetchPosition(ctx, symbol)
	if err != nil {
		// Position not found is OK - might mean it's closed
		if err.Error() != fmt.Sprintf("no position found for %s", symbol) {
			return fmt.Errorf("failed to fetch position: %w", err)
		}
		currentPos = nil
	}

	// 2. Get last known position from agent state
	lastPos := runner.State.LastKnownPosition

	// 3. Check if position was open and is now closed
	wasOpen := lastPos != nil && lastPos.Side != models.PositionNone
	isClosedNow := currentPos == nil || currentPos.Side == models.PositionNone

	if wasOpen && isClosedNow {
		logger.Info("🔔 Position closed detected",
			zap.String("agent", runner.Config.Name),
			zap.String("personality", string(runner.Config.Personality)),
			zap.String("symbol", symbol),
			zap.String("side", string(lastPos.Side)),
		)

		// 4. Get closure information (SL/TP/Manual)
		closure, err := w.getClosureInfo(ctx, runner, lastPos)
		if err != nil {
			logger.Warn("failed to get closure info, using defaults",
				zap.Error(err),
			)
			// Use default closure info
			closure = &PositionClosure{
				Type:      "unknown",
				ExitPrice: lastPos.CurrentPrice,
				PnL:       lastPos.UnrealizedPnL,
				Timestamp: time.Now(),
			}
		}

		// 5. Trigger reflection (async to not block monitoring)
		go func() {
			reflectCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := w.triggerReflection(reflectCtx, runner, lastPos, closure); err != nil {
				logger.Error("reflection failed after position closure",
					zap.String("agent", runner.Config.Name),
					zap.Error(err),
				)
			}
		}()

		// 6. Update agent state
		pnl, _ := closure.PnL.Float64()
		runner.State.Balance = runner.State.Balance.Add(closure.PnL)
		runner.State.PnL = runner.State.PnL.Add(closure.PnL)
		runner.State.LastKnownPosition = nil
		runner.State.PositionJustClosed = true

		logger.Info("💰 Agent balance updated after closure",
			zap.String("agent", runner.Config.Name),
			zap.Float64("pnl", pnl),
			zap.Float64("new_balance", runner.State.Balance.InexactFloat64()),
		)

		// 7. Save updated state to DB
		if err := w.repository.CreateAgentState(ctx, runner.State); err != nil {
			logger.Error("failed to save agent state after position closure", zap.Error(err))
		}

	} else if currentPos != nil && currentPos.Side != models.PositionNone {
		// Position still open - update last known position
		runner.State.LastKnownPosition = currentPos
	}

	return nil
}

// PositionClosure contains info about how position was closed
type PositionClosure struct {
	Type      string          // "stop_loss", "take_profit", "manual", "unknown"
	OrderID   string          // Order ID that closed position
	ExitPrice decimal.Decimal // Exit price
	PnL       decimal.Decimal // Realized PnL
	Timestamp time.Time       // Closure time
}

// getClosureInfo determines how position was closed
func (w *PositionMonitorWorker) getClosureInfo(
	ctx context.Context,
	runner *agents.AgenticRunner,
	lastPos *models.Position,
) (*PositionClosure, error) {
	// Get last decision that opened this position
	decision, err := w.repository.GetLastOpenDecisionForSymbol(ctx, runner.Config.ID, runner.State.Symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get last decision: %w", err)
	}

	closure := &PositionClosure{
		Type:      "unknown",
		ExitPrice: lastPos.CurrentPrice,
		PnL:       lastPos.UnrealizedPnL,
		Timestamp: time.Now(),
	}

	// Check if Stop Loss order was filled
	if decision.StopLossOrderID != "" {
		slOrder, err := runner.Exchange.FetchOrder(ctx, decision.StopLossOrderID, runner.State.Symbol)
		if err == nil && (slOrder.Status == "closed" || slOrder.Status == "filled") {
			closure.Type = "stop_loss"
			closure.OrderID = decision.StopLossOrderID
			closure.ExitPrice = slOrder.Price
			logger.Info("🛑 Position closed by Stop Loss",
				zap.String("agent", runner.Config.Name),
				zap.String("order_id", decision.StopLossOrderID),
			)
			return closure, nil
		}
	}

	// Check if Take Profit order was filled
	if decision.TakeProfitOrderID != "" {
		tpOrder, err := runner.Exchange.FetchOrder(ctx, decision.TakeProfitOrderID, runner.State.Symbol)
		if err == nil && (tpOrder.Status == "closed" || tpOrder.Status == "filled") {
			closure.Type = "take_profit"
			closure.OrderID = decision.TakeProfitOrderID
			closure.ExitPrice = tpOrder.Price
			logger.Info("🎯 Position closed by Take Profit",
				zap.String("agent", runner.Config.Name),
				zap.String("order_id", decision.TakeProfitOrderID),
			)
			return closure, nil
		}
	}

	// If neither SL nor TP triggered - manual close by agent decision
	closure.Type = "manual"
	logger.Info("👤 Position closed manually by agent",
		zap.String("agent", runner.Config.Name),
	)

	return closure, nil
}

// triggerReflection creates trade experience and triggers agent reflection
func (w *PositionMonitorWorker) triggerReflection(
	ctx context.Context,
	runner *agents.AgenticRunner,
	position *models.Position,
	closure *PositionClosure,
) error {
	// Get entry decision reason
	entryDecision, err := w.repository.GetLastOpenDecisionForSymbol(ctx, runner.Config.ID, runner.State.Symbol)
	entryReason := "Unknown entry reason"
	if err == nil && entryDecision != nil {
		entryReason = entryDecision.Reason
	}

	// Calculate metrics
	pnl, _ := closure.PnL.Float64()
	margin, _ := position.Margin.Float64()

	duration := time.Since(position.Timestamp)
	if !closure.Timestamp.IsZero() {
		duration = closure.Timestamp.Sub(position.Timestamp)
	}

	pnlPercent := 0.0
	if margin > 0 {
		pnlPercent = (pnl / margin) * 100
	}

	// Build trade experience
	tradeExp := &models.TradeExperience{
		Symbol:        position.Symbol,
		Side:          string(position.Side),
		EntryPrice:    position.EntryPrice,
		ExitPrice:     closure.ExitPrice,
		Size:          position.Size,
		PnL:           closure.PnL,
		PnLPercent:    pnlPercent,
		Duration:      duration,
		EntryReason:   entryReason,
		ExitReason:    formatExitReason(closure.Type),
		WasSuccessful: pnl > 0,
	}

	logger.Info("🤔 Triggering post-trade reflection",
		zap.String("agent", runner.Config.Name),
		zap.Float64("pnl", pnl),
		zap.Float64("pnl_percent", pnlPercent),
		zap.String("closure_type", closure.Type),
		zap.Duration("duration", duration),
	)

	// Trigger reflection
	if err := runner.ReflectionEngine.Reflect(ctx, tradeExp); err != nil {
		return fmt.Errorf("reflection failed: %w", err)
	}

	logger.Info("✅ Reflection completed",
		zap.String("agent", runner.Config.Name),
	)

	return nil
}

// formatExitReason formats exit reason for reflection
func formatExitReason(closureType string) string {
	switch closureType {
	case "stop_loss":
		return "Position closed by Stop Loss order"
	case "take_profit":
		return "Position closed by Take Profit order"
	case "manual":
		return "Position closed manually by agent decision"
	default:
		return "Position closure detected (reason unknown)"
	}
}
