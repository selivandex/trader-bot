package agents

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/internal/adapters/exchange"
	"github.com/alexanderselivanov/trader/pkg/logger"
	"github.com/alexanderselivanov/trader/pkg/models"
)

// Tournament manages agent competitions
type Tournament struct {
	db              *sqlx.DB
	repository      *Repository
	agentManager    *Manager
	tournamentModel *models.AgentTournament
	participants    []*TournamentParticipant
	ctx             context.Context
	cancel          context.CancelFunc
	isRunning       bool
}

// TournamentParticipant represents an agent participating in tournament
type TournamentParticipant struct {
	AgentID        string
	AgentName      string
	AgentRunner    *AgentRunner
	InitialBalance decimal.Decimal
	CurrentBalance decimal.Decimal
	TotalReturn    decimal.Decimal
	ReturnPercent  float64
	TradeCount     int
	WinCount       int
	LossCount      int
	WinRate        float64
}

// NewTournament creates a new agent tournament
func NewTournament(
	db *sqlx.DB,
	repository *Repository,
	agentManager *Manager,
	userID string,
	name string,
	symbols []string,
	startBalance float64,
	duration time.Duration,
) (*Tournament, error) {
	// Create tournament model
	tournamentModel := &models.AgentTournament{
		UserID:       userID,
		Name:         name,
		Symbols:      symbols,
		StartBalance: models.NewDecimal(startBalance),
		Duration:     duration,
		IsActive:     true,
	}

	// Save to database
	if err := repository.CreateTournament(context.Background(), tournamentModel); err != nil {
		return nil, fmt.Errorf("failed to create tournament: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Tournament{
		db:              db,
		repository:      repository,
		agentManager:    agentManager,
		tournamentModel: tournamentModel,
		participants:    make([]*TournamentParticipant, 0),
		ctx:             ctx,
		cancel:          cancel,
		isRunning:       false,
	}, nil
}

// AddParticipant adds an agent to the tournament
func (t *Tournament) AddParticipant(ctx context.Context, agentID string, exchangeAdapter exchange.Exchange) error {
	// Load agent config
	config, err := t.repository.GetAgent(ctx, agentID)
	if err != nil {
		return fmt.Errorf("failed to load agent: %w", err)
	}

	// Check if agent is already participating
	for _, p := range t.participants {
		if p.AgentID == agentID {
			return fmt.Errorf("agent already participating")
		}
	}

	// Start agent for each symbol
	startBalance := t.tournamentModel.StartBalance.InexactFloat64()

	// Use first symbol for now (can be extended to multi-pair)
	symbol := t.tournamentModel.Symbols[0]

	if err := t.agentManager.StartAgent(ctx, agentID, symbol, startBalance, exchangeAdapter); err != nil {
		return fmt.Errorf("failed to start agent: %w", err)
	}

	runner, exists := t.agentManager.GetAgentRunner(agentID)
	if !exists {
		return fmt.Errorf("agent runner not found")
	}

	participant := &TournamentParticipant{
		AgentID:        agentID,
		AgentName:      config.Name,
		AgentRunner:    runner,
		InitialBalance: t.tournamentModel.StartBalance,
		CurrentBalance: t.tournamentModel.StartBalance,
		TotalReturn:    models.NewDecimal(0),
		ReturnPercent:  0,
		TradeCount:     0,
		WinCount:       0,
		LossCount:      0,
		WinRate:        0,
	}

	t.participants = append(t.participants, participant)

	logger.Info("participant added to tournament",
		zap.String("tournament_id", t.tournamentModel.ID),
		zap.String("agent_id", agentID),
		zap.String("agent_name", config.Name),
	)

	return nil
}

// Start starts the tournament
func (t *Tournament) Start(ctx context.Context) error {
	if t.isRunning {
		return fmt.Errorf("tournament already running")
	}

	if len(t.participants) < 2 {
		return fmt.Errorf("need at least 2 participants")
	}

	t.isRunning = true

	logger.Info("tournament started",
		zap.String("tournament_id", t.tournamentModel.ID),
		zap.String("name", t.tournamentModel.Name),
		zap.Int("participants", len(t.participants)),
		zap.Duration("duration", t.tournamentModel.Duration),
	)

	// Run tournament monitoring
	go t.monitorTournament(ctx)

	return nil
}

// monitorTournament monitors tournament progress and ends when time is up
func (t *Tournament) monitorTournament(ctx context.Context) {
	endTime := time.Now().Add(t.tournamentModel.Duration)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-t.ctx.Done():
			return

		case <-ticker.C:
			// Update participant stats
			t.updateParticipantStats(ctx)

			// Check if tournament should end
			if time.Now().After(endTime) {
				logger.Info("tournament time expired",
					zap.String("tournament_id", t.tournamentModel.ID),
				)
				if err := t.End(ctx); err != nil {
					logger.Error("failed to end tournament", zap.Error(err))
				}
				return
			}
		}
	}
}

// updateParticipantStats updates statistics for all participants
func (t *Tournament) updateParticipantStats(ctx context.Context) {
	for _, p := range t.participants {
		if p.AgentRunner == nil || !p.AgentRunner.IsRunning {
			continue
		}

		// Get current balance from portfolio
		currentBalance := p.AgentRunner.Portfolio.GetBalance()
		p.CurrentBalance = models.NewDecimal(currentBalance)

		// Calculate return
		p.TotalReturn = p.CurrentBalance.Sub(p.InitialBalance)
		if p.InitialBalance.GreaterThan(models.NewDecimal(0)) {
			p.ReturnPercent = p.TotalReturn.Div(p.InitialBalance).Mul(models.NewDecimal(100)).InexactFloat64()
		}

		// Get trade stats from agent state
		state, err := t.repository.GetAgentState(ctx, p.AgentID, t.tournamentModel.Symbols[0])
		if err == nil {
			p.TradeCount = state.TotalTrades
			p.WinCount = state.WinningTrades
			p.LossCount = state.LosingTrades
			p.WinRate = state.WinRate
		}
	}
}

// End ends the tournament and calculates final results
func (t *Tournament) End(ctx context.Context) error {
	if !t.isRunning {
		return fmt.Errorf("tournament not running")
	}

	// Stop all participating agents
	for _, p := range t.participants {
		if err := t.agentManager.StopAgent(ctx, p.AgentID); err != nil {
			logger.Error("failed to stop agent",
				zap.String("agent_id", p.AgentID),
				zap.Error(err),
			)
		}
	}

	// Update final stats
	t.updateParticipantStats(ctx)

	// Calculate final scores
	scores := t.calculateScores()

	// Save results to database
	if err := t.repository.EndTournament(ctx, t.tournamentModel.ID, scores); err != nil {
		return fmt.Errorf("failed to save tournament results: %w", err)
	}

	t.isRunning = false
	t.cancel()

	logger.Info("tournament ended",
		zap.String("tournament_id", t.tournamentModel.ID),
		zap.String("winner", scores[0].AgentName),
		zap.Float64("winner_return", scores[0].ReturnPct),
	)

	return nil
}

// calculateScores calculates and sorts final scores
func (t *Tournament) calculateScores() []models.AgentScore {
	scores := make([]models.AgentScore, len(t.participants))

	for i, p := range t.participants {
		// Calculate profit factor
		profitFactor := 0.0
		if p.LossCount > 0 {
			profitFactor = float64(p.WinCount) / float64(p.LossCount)
		} else if p.WinCount > 0 {
			profitFactor = float64(p.WinCount)
		}

		// Calculate average PnL per trade
		avgTradePnL := models.NewDecimal(0)
		if p.TradeCount > 0 {
			avgTradePnL = p.TotalReturn.Div(models.NewDecimal(float64(p.TradeCount)))
		}

		// TODO: Calculate Sharpe ratio (needs historical returns)
		sharpeRatio := 0.0

		// Calculate max drawdown (simplified - would need historical data)
		maxDrawdown := 0.0
		if p.TotalReturn.LessThan(models.NewDecimal(0)) {
			maxDrawdown = p.TotalReturn.Div(p.InitialBalance).Mul(models.NewDecimal(100)).InexactFloat64()
		}

		scores[i] = models.AgentScore{
			AgentID:      p.AgentID,
			AgentName:    p.AgentName,
			TotalReturn:  p.TotalReturn,
			ReturnPct:    p.ReturnPercent,
			WinRate:      p.WinRate,
			MaxDrawdown:  maxDrawdown,
			SharpeRatio:  sharpeRatio,
			TradeCount:   p.TradeCount,
			ProfitFactor: profitFactor,
			AvgTradePnL:  avgTradePnL,
		}
	}

	// Sort by return percentage (descending)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].ReturnPct > scores[j].ReturnPct
	})

	return scores
}

// GetLeaderboard returns current leaderboard
func (t *Tournament) GetLeaderboard() []models.AgentScore {
	t.updateParticipantStats(context.Background())
	return t.calculateScores()
}

// GetParticipants returns list of participants
func (t *Tournament) GetParticipants() []*TournamentParticipant {
	return t.participants
}

// IsRunning returns whether tournament is running
func (t *Tournament) IsRunning() bool {
	return t.isRunning
}

// FormatLeaderboard formats leaderboard for display
func FormatLeaderboard(scores []models.AgentScore) string {
	result := "🏆 Tournament Leaderboard\n\n"

	for i, score := range scores {
		emoji := "🥇"
		if i == 1 {
			emoji = "🥈"
		} else if i == 2 {
			emoji = "🥉"
		} else if i > 2 {
			emoji = fmt.Sprintf("%d.", i+1)
		}

		result += fmt.Sprintf("%s %s\n", emoji, score.AgentName)
		result += fmt.Sprintf("   Return: %.2f%% ($%.2f)\n", score.ReturnPct, score.TotalReturn.InexactFloat64())
		result += fmt.Sprintf("   Trades: %d | Win Rate: %.1f%%\n", score.TradeCount, score.WinRate*100)
		result += fmt.Sprintf("   Avg PnL: $%.2f | Profit Factor: %.2f\n\n", score.AvgTradePnL.InexactFloat64(), score.ProfitFactor)
	}

	return result
}
