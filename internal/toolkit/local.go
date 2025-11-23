package toolkit

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/market"
	"github.com/selivandex/trader-bot/internal/adapters/news"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// LocalToolkit implements AgentToolkit using local database caches
// All methods read from Postgres/ClickHouse, never call exchange APIs
type LocalToolkit struct {
	agentID       string
	marketRepo    *market.Repository
	newsCache     *news.Cache
	agentRepo     AgentRepository
	memoryManager SemanticMemoryManager
	notifier      Notifier // For sending alerts to owner
}

// NewLocalToolkit creates toolkit for agent
func NewLocalToolkit(
	agentID string,
	marketRepo *market.Repository,
	newsCache *news.Cache,
	agentRepo AgentRepository,
	memoryManager SemanticMemoryManager,
	notifier Notifier,
) *LocalToolkit {
	return &LocalToolkit{
		agentID:       agentID,
		marketRepo:    marketRepo,
		newsCache:     newsCache,
		agentRepo:     agentRepo,
		memoryManager: memoryManager,
		notifier:      notifier,
	}
}

// ============ Market Data Tools ============

// GetCandles retrieves OHLCV candles from cache (populated by CandlesWorker)
func (t *LocalToolkit) GetCandles(ctx context.Context, symbol, timeframe string, limit int) ([]models.Candle, error) {
	logger.Debug("toolkit: get_candles",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.String("timeframe", timeframe),
		zap.Int("limit", limit),
	)

	candles, err := t.marketRepo.GetCandles(ctx, symbol, timeframe, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get candles: %w", err)
	}

	return candles, nil
}

// GetCandleCount returns total cached candles
func (t *LocalToolkit) GetCandleCount(ctx context.Context, symbol, timeframe string) (int, error) {
	logger.Debug("toolkit: get_candle_count",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.String("timeframe", timeframe),
	)

	count, err := t.marketRepo.GetCandleCount(ctx, symbol, timeframe)
	if err != nil {
		return 0, fmt.Errorf("failed to get candle count: %w", err)
	}

	return count, nil
}

// GetLatestPrice gets most recent close price from candles
func (t *LocalToolkit) GetLatestPrice(ctx context.Context, symbol, timeframe string) (float64, error) {
	candles, err := t.GetCandles(ctx, symbol, timeframe, 1)
	if err != nil {
		return 0, err
	}

	if len(candles) == 0 {
		return 0, fmt.Errorf("no candles available for %s %s", symbol, timeframe)
	}

	return candles[len(candles)-1].Close.InexactFloat64(), nil
}

// ============ News Tools ============

// SearchNews performs text search in cached news (populated by NewsWorker)
func (t *LocalToolkit) SearchNews(ctx context.Context, query string, since time.Duration, limit int) ([]models.NewsItem, error) {
	logger.Debug("toolkit: search_news",
		zap.String("agent_id", t.agentID),
		zap.String("query", query),
		zap.Duration("since", since),
		zap.Int("limit", limit),
	)

	// Use news cache search functionality
	news, err := t.newsCache.SearchNews(ctx, query, since, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search news: %w", err)
	}

	return news, nil
}

// GetHighImpactNews filters news by AI impact score
func (t *LocalToolkit) GetHighImpactNews(ctx context.Context, minImpact int, since time.Duration) ([]models.NewsItem, error) {
	logger.Debug("toolkit: get_high_impact_news",
		zap.String("agent_id", t.agentID),
		zap.Int("min_impact", minImpact),
		zap.Duration("since", since),
	)

	news, err := t.newsCache.GetRecent(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get news: %w", err)
	}

	// Filter by impact
	highImpact := []models.NewsItem{}
	for _, item := range news {
		if item.Impact >= minImpact {
			highImpact = append(highImpact, item)
		}
	}

	return highImpact, nil
}

// GetNewsBySentiment filters news by sentiment range
func (t *LocalToolkit) GetNewsBySentiment(ctx context.Context, minSentiment, maxSentiment float64, since time.Duration) ([]models.NewsItem, error) {
	logger.Debug("toolkit: get_news_by_sentiment",
		zap.String("agent_id", t.agentID),
		zap.Float64("min_sentiment", minSentiment),
		zap.Float64("max_sentiment", maxSentiment),
		zap.Duration("since", since),
	)

	news, err := t.newsCache.GetRecent(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get news: %w", err)
	}

	// Filter by sentiment
	filtered := []models.NewsItem{}
	for _, item := range news {
		if item.Sentiment >= minSentiment && item.Sentiment <= maxSentiment {
			filtered = append(filtered, item)
		}
	}

	return filtered, nil
}

// ============ On-Chain Tools ============

// GetRecentWhaleMovements gets whale transactions from cache (populated by OnChainWorker)
func (t *LocalToolkit) GetRecentWhaleMovements(ctx context.Context, symbol string, minAmountUSD float64, hours int) ([]models.WhaleTransaction, error) {
	logger.Debug("toolkit: get_whale_movements",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.Float64("min_amount", minAmountUSD),
		zap.Int("hours", hours),
	)

	whales, err := t.agentRepo.GetRecentWhaleTransactions(ctx, symbol, hours, int(minAmountUSD))
	if err != nil {
		return nil, fmt.Errorf("failed to get whale transactions: %w", err)
	}

	return whales, nil
}

// GetNetExchangeFlow calculates net flow from cached exchange flows
func (t *LocalToolkit) GetNetExchangeFlow(ctx context.Context, symbol string, hours int) (float64, error) {
	logger.Debug("toolkit: get_net_exchange_flow",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.Int("hours", hours),
	)

	flows, err := t.agentRepo.GetExchangeFlows(ctx, symbol, hours)
	if err != nil {
		return 0, fmt.Errorf("failed to get exchange flows: %w", err)
	}

	var netFlow float64
	for _, flow := range flows {
		netFlow += flow.NetFlow.InexactFloat64()
	}

	return netFlow, nil
}

// GetLargestWhaleTransaction finds biggest whale tx in time window
func (t *LocalToolkit) GetLargestWhaleTransaction(ctx context.Context, symbol string, hours int) (*models.WhaleTransaction, error) {
	whales, err := t.GetRecentWhaleMovements(ctx, symbol, 0, hours)
	if err != nil {
		return nil, err
	}

	if len(whales) == 0 {
		return nil, fmt.Errorf("no whale transactions found")
	}

	// Find largest
	largest := &whales[0]
	for i := range whales {
		if whales[i].AmountUSD.GreaterThan(largest.AmountUSD) {
			largest = &whales[i]
		}
	}

	return largest, nil
}

// ============ Memory Tools ============

// SearchPersonalMemories queries agent's own semantic memory
func (t *LocalToolkit) SearchPersonalMemories(ctx context.Context, query string, topK int) ([]models.SemanticMemory, error) {
	logger.Debug("toolkit: search_personal_memories",
		zap.String("agent_id", t.agentID),
		zap.String("query", query),
		zap.Int("top_k", topK),
	)

	memories, err := t.memoryManager.RecallRelevant(ctx, t.agentID, "", query, topK)
	if err != nil {
		return nil, fmt.Errorf("failed to search personal memories: %w", err)
	}

	return memories, nil
}

// SearchCollectiveMemories queries collective wisdom with semantic search
func (t *LocalToolkit) SearchCollectiveMemories(ctx context.Context, personality, query string, topK int) ([]models.CollectiveMemory, error) {
	logger.Debug("toolkit: search_collective_memories",
		zap.String("agent_id", t.agentID),
		zap.String("personality", personality),
		zap.String("query", query),
		zap.Int("top_k", topK),
	)

	// Use memory manager's RecallRelevant which includes collective memories
	// RecallRelevant automatically searches both personal + collective and ranks by similarity
	allMemories, err := t.memoryManager.RecallRelevant(ctx, t.agentID, personality, query, topK*2)
	if err != nil {
		return nil, fmt.Errorf("failed to search memories: %w", err)
	}

	// Filter to only collective memories (AgentID == "collective")
	collectiveOnly := []models.CollectiveMemory{}
	for _, mem := range allMemories {
		if mem.AgentID == "collective" {
			// Convert SemanticMemory to CollectiveMemory format
			collective := models.CollectiveMemory{
				ID:          mem.ID,
				Personality: personality,
				Context:     mem.Context,
				Action:      mem.Action,
				Lesson:      mem.Lesson,
				Embedding:   mem.Embedding,
				Importance:  mem.Importance,
				// Parse confirmation count from Outcome field
				ConfirmationCount: 1, // Default
				SuccessRate:       0.5,
				LastConfirmedAt:   mem.LastAccessed,
				CreatedAt:         mem.CreatedAt,
			}
			collectiveOnly = append(collectiveOnly, collective)

			if len(collectiveOnly) >= topK {
				break
			}
		}
	}

	return collectiveOnly, nil
}

// GetRecentMemories gets agent's most recent memories
func (t *LocalToolkit) GetRecentMemories(ctx context.Context, limit int) ([]models.SemanticMemory, error) {
	logger.Debug("toolkit: get_recent_memories",
		zap.String("agent_id", t.agentID),
		zap.Int("limit", limit),
	)

	memories, err := t.agentRepo.GetSemanticMemories(ctx, t.agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent memories: %w", err)
	}

	return memories, nil
}

// ============ Performance Tools ============

// GetRecentTrades gets agent's recent trades from decisions table
func (t *LocalToolkit) GetRecentTrades(ctx context.Context, symbol string, limit int) ([]TradeRecord, error) {
	logger.Debug("toolkit: get_recent_trades",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.Int("limit", limit),
	)

	// Query agent_decisions where executed=true and outcome is set
	// This would need a new repository method
	// For now, return empty slice
	// TODO: Implement GetCompletedTrades in repository

	return []TradeRecord{}, nil
}

// GetWinRateBySignal calculates performance breakdown by signal
func (t *LocalToolkit) GetWinRateBySignal(ctx context.Context, symbol string) (*SignalPerformanceStats, error) {
	logger.Debug("toolkit: get_win_rate_by_signal",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
	)

	// Get agent's statistical memory
	memory, err := t.agentRepo.GetAgentStatisticalMemory(ctx, t.agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent memory: %w", err)
	}

	// Convert to stats format
	stats := &SignalPerformanceStats{
		Technical: SignalStats{
			WinRate: memory.TechnicalSuccessRate,
		},
		News: SignalStats{
			WinRate: memory.NewsSuccessRate,
		},
		OnChain: SignalStats{
			WinRate: memory.OnChainSuccessRate,
		},
		Sentiment: SignalStats{
			WinRate: memory.SentimentSuccessRate,
		},
	}

	return stats, nil
}

// GetCurrentStreak returns current winning/losing streak
func (t *LocalToolkit) GetCurrentStreak(ctx context.Context, symbol string) (int, bool, error) {
	logger.Debug("toolkit: get_current_streak",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
	)

	// TODO: Implement streak calculation from decisions
	// Would need to query recent trades in order and count consecutive wins/losses

	return 0, false, fmt.Errorf("not implemented yet")
}
