package toolkit

import (
	"context"
	"fmt"
	"math"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/indicators"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// ============ Advanced Tools Implementation ============

// GetCorrelation calculates correlation between two assets
func (t *LocalToolkit) GetCorrelation(ctx context.Context, symbol1, symbol2 string, hours int) (float64, error) {
	logger.Debug("toolkit: get_correlation",
		zap.String("agent_id", t.agentID),
		zap.String("symbol1", symbol1),
		zap.String("symbol2", symbol2),
		zap.Int("hours", hours),
	)

	// Get candles for both symbols
	limit := hours / 1 // 1h candles
	candles1, err := t.GetCandles(ctx, symbol1, "1h", limit)
	if err != nil {
		return 0, fmt.Errorf("failed to get candles for %s: %w", symbol1, err)
	}

	candles2, err := t.GetCandles(ctx, symbol2, "1h", limit)
	if err != nil {
		return 0, fmt.Errorf("failed to get candles for %s: %w", symbol2, err)
	}

	// Calculate returns for both
	returns1 := calculateReturns(candles1)
	returns2 := calculateReturns(candles2)

	// Calculate Pearson correlation
	correlation := pearsonCorrelation(returns1, returns2)

	return correlation, nil
}

// CheckTimeframeAlignment checks if trends align across timeframes
func (t *LocalToolkit) CheckTimeframeAlignment(ctx context.Context, symbol string, timeframes []string) (map[string]string, error) {
	logger.Debug("toolkit: check_timeframe_alignment",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.Strings("timeframes", timeframes),
	)

	alignment := make(map[string]string)

	for _, tf := range timeframes {
		trend, err := t.DetectTrend(ctx, symbol, tf)
		if err != nil {
			alignment[tf] = "unknown"
			continue
		}
		alignment[tf] = trend
	}

	return alignment, nil
}

// GetMarketRegime detects current market regime
func (t *LocalToolkit) GetMarketRegime(ctx context.Context, symbol, timeframe string) (string, error) {
	logger.Debug("toolkit: get_market_regime",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.String("timeframe", timeframe),
	)

	// Get candles
	candles, err := t.GetCandles(ctx, symbol, timeframe, 100)
	if err != nil {
		return "", fmt.Errorf("failed to get candles: %w", err)
	}

	// Calculate volatility
	volatility, err := t.CalculateVolatility(ctx, symbol, timeframe, 14)
	if err != nil {
		return "unknown", err
	}

	// Detect trend
	trend, err := t.DetectTrend(ctx, symbol, timeframe)
	if err != nil {
		return "unknown", err
	}

	// Calculate price range
	var high, low float64
	for i, candle := range candles {
		price := candle.Close.InexactFloat64()
		if i == 0 || price > high {
			high = price
		}
		if i == 0 || price < low {
			low = price
		}
	}
	priceRange := (high - low) / low * 100

	// Determine regime
	if volatility > 800 && priceRange > 15 {
		return "volatile", nil
	} else if trend != "sideways" && priceRange > 10 {
		return "trending", nil
	} else if priceRange < 5 {
		return "ranging", nil
	}

	return "mixed", nil
}

// GetVolatilityTrend checks if volatility is expanding or contracting
func (t *LocalToolkit) GetVolatilityTrend(ctx context.Context, symbol string, hours int) (string, error) {
	logger.Debug("toolkit: get_volatility_trend",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.Int("hours", hours),
	)

	// Get recent and older volatility
	recentVol, err := t.CalculateVolatility(ctx, symbol, "1h", 14)
	if err != nil {
		return "unknown", err
	}

	// Calculate volatility from earlier period
	candles, err := t.GetCandles(ctx, symbol, "1h", hours+14)
	if err != nil {
		return "unknown", err
	}

	if len(candles) < 50 {
		return "unknown", fmt.Errorf("insufficient data")
	}

	olderCandles := candles[:len(candles)-14]
	calc := indicators.NewCalculator()
	olderVol, err := calc.CalculateVolatility(olderCandles, 14)
	if err != nil {
		return "unknown", err
	}

	// Compare
	change := (recentVol - olderVol) / olderVol * 100

	if change > 20 {
		return "expanding", nil
	} else if change < -20 {
		return "contracting", nil
	}

	return "stable", nil
}

// AnalyzeLiquidity analyzes market liquidity
func (t *LocalToolkit) AnalyzeLiquidity(ctx context.Context, symbol string) (float64, error) {
	logger.Debug("toolkit: analyze_liquidity",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
	)

	// Get recent volume
	candles, err := t.GetCandles(ctx, symbol, "1h", 24)
	if err != nil {
		return 0, fmt.Errorf("failed to get candles: %w", err)
	}

	// Calculate average volume
	totalVolume := 0.0
	for _, candle := range candles {
		totalVolume += candle.Volume.InexactFloat64()
	}
	avgVolume := totalVolume / float64(len(candles))

	// Liquidity score: higher volume = higher liquidity
	// Normalize to 0-100 scale
	// $100M+ avg volume = 100/100
	// $10M avg volume = 50/100
	// $1M avg volume = 20/100
	liquidityScore := math.Log10(avgVolume/1_000_000) * 25

	if liquidityScore < 0 {
		liquidityScore = 0
	}
	if liquidityScore > 100 {
		liquidityScore = 100
	}

	return liquidityScore, nil
}

// BacktestStrategy simulates strategy on historical data (simplified)
func (t *LocalToolkit) BacktestStrategy(ctx context.Context, symbol string, lookbackHours int) (*BacktestResult, error) {
	logger.Debug("toolkit: backtest_strategy",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.Int("lookback_hours", lookbackHours),
	)

	// Get agent's current weights
	agent, err := t.agentRepo.GetAgent(ctx, t.agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent config: %w", err)
	}

	// Get historical performance
	metrics, err := t.agentRepo.GetAgentPerformanceMetrics(ctx, t.agentID, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	// Simple backtest result based on actual performance
	return &BacktestResult{
		Symbol:          symbol,
		Period:          time.Duration(lookbackHours) * time.Hour,
		TotalTrades:     metrics.TotalTrades,
		WinningTrades:   metrics.WinningTrades,
		LosingTrades:    metrics.LosingTrades,
		WinRate:         metrics.WinRate,
		TotalReturn:     metrics.TotalPnL,
		SharpeRatio:     metrics.SharpeRatio,
		MaxDrawdown:     0, // TODO
		BestTrade:       metrics.MaxWin,
		WorstTrade:      metrics.MaxLoss,
		StrategyWeights: agent.Specialization,
	}, nil
}

// BacktestResult contains backtest results
type BacktestResult struct {
	Symbol          string
	Period          time.Duration
	TotalTrades     int
	WinningTrades   int
	LosingTrades    int
	WinRate         float64
	TotalReturn     float64
	SharpeRatio     float64
	MaxDrawdown     float64
	BestTrade       float64
	WorstTrade      float64
	StrategyWeights models.AgentSpecialization
}

// Helper functions

func calculateReturns(candles []models.Candle) []float64 {
	if len(candles) < 2 {
		return []float64{}
	}

	returns := make([]float64, len(candles)-1)
	for i := 1; i < len(candles); i++ {
		prev := candles[i-1].Close.InexactFloat64()
		curr := candles[i].Close.InexactFloat64()
		returns[i-1] = (curr - prev) / prev
	}

	return returns
}

func pearsonCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0
	}

	n := float64(len(x))

	// Calculate means
	var sumX, sumY float64
	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
	}
	meanX := sumX / n
	meanY := sumY / n

	// Calculate correlation
	var numerator, denomX, denomY float64
	for i := 0; i < len(x); i++ {
		diffX := x[i] - meanX
		diffY := y[i] - meanY
		numerator += diffX * diffY
		denomX += diffX * diffX
		denomY += diffY * diffY
	}

	if denomX == 0 || denomY == 0 {
		return 0
	}

	return numerator / (math.Sqrt(denomX) * math.Sqrt(denomY))
}

// ============ NEWS + MEMORY CROSS-REFERENCE (Phase 2) ============

// FindNewsRelatedToCurrentSituation finds news semantically related to agent's reasoning
// Use during CoT when agent is analyzing a situation and wants news context
func (t *LocalToolkit) FindNewsRelatedToCurrentSituation(
	ctx context.Context,
	situationDescription string,
	since time.Duration,
	limit int,
) ([]models.NewsItem, error) {
	logger.Debug("toolkit: find_news_related_to_situation",
		zap.String("agent_id", t.agentID),
		zap.String("situation", situationDescription),
	)

	// Use semantic search to find relevant news
	news, err := t.newsCache.SearchNewsSemantics(ctx, situationDescription, since, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to find related news: %w", err)
	}

	return news, nil
}

// GetNewsWithMemoryContext gets news along with agent's related past experiences
// Returns enriched context combining current news + relevant memories
// This is the POWER TOOL that combines semantic news search with memory recall
func (t *LocalToolkit) GetNewsWithMemoryContext(
	ctx context.Context,
	newsQuery string,
	since time.Duration,
	newsLimit int,
) (string, error) {
	logger.Debug("toolkit: get_news_with_memory_context",
		zap.String("agent_id", t.agentID),
		zap.String("query", newsQuery),
	)

	// 1. Search news semantically
	news, err := t.newsCache.SearchNewsSemantics(ctx, newsQuery, since, newsLimit)
	if err != nil {
		return "", fmt.Errorf("failed to search news: %w", err)
	}

	if len(news) == 0 {
		return "📰 No relevant news found for query: " + newsQuery, nil
	}

	// 2. Build rich context with news + related memories
	result := fmt.Sprintf("📰 NEWS + MEMORY CONTEXT for '%s':\n\n", newsQuery)

	for i, item := range news {
		// News header
		result += fmt.Sprintf("═══ NEWS #%d ═══\n", i+1)
		result += fmt.Sprintf("🔖 [%s] %s\n", item.Source, item.Title)
		result += fmt.Sprintf("📊 Impact: %d/10 | Sentiment: %.2f | Published: %s\n",
			item.Impact, item.Sentiment, item.PublishedAt.Format("15:04 MST"))

		// Show similarity if available
		if item.SimilarityScore > 0 {
			result += fmt.Sprintf("🎯 Relevance: %.0f%%\n", item.SimilarityScore*100)
		}

		// Content snippet
		if item.Content != "" {
			contentSnippet := item.Content
			if len(contentSnippet) > 200 {
				contentSnippet = contentSnippet[:200] + "..."
			}
			result += fmt.Sprintf("📝 %s\n", contentSnippet)
		}

		// 3. Find related memories using news embedding
		if len(item.Embedding) > 0 {
			memories, err := t.memoryManager.FindMemoriesRelatedToNews(
				ctx,
				t.agentID,
				"", // personality will be inferred by memory manager
				item.Embedding,
				3, // Top 3 related memories
			)

			if err == nil && len(memories) > 0 {
				result += "\n💭 RELATED PAST EXPERIENCES:\n"
				for j, mem := range memories {
					result += fmt.Sprintf("   %d. Context: %s\n", j+1, mem.Context)
					result += fmt.Sprintf("      Action: %s\n", mem.Action)
					result += fmt.Sprintf("      Outcome: %s\n", mem.Outcome)
					result += fmt.Sprintf("      ✨ Lesson: %s\n", mem.Lesson)
					result += fmt.Sprintf("      🎯 Importance: %.0f%% | Accessed: %dx\n",
						mem.Importance*100, mem.AccessCount)
				}
			} else if len(memories) == 0 {
				result += "\n💭 No similar past experiences found (new situation)\n"
			}
		}

		// Check if part of larger cluster
		if item.ClusterID != nil && !item.IsClusterPrimary {
			result += "\n🔗 Part of larger story (cluster). Use GetRelatedNews() for full coverage.\n"
		} else if item.ClusterID != nil && item.IsClusterPrimary {
			result += "\n⭐ Primary source for this event cluster.\n"
		}

		result += "\n"
	}

	// 4. Add summary recommendations
	result += "═══════════════════════════════════════\n"
	result += "💡 HOW TO USE THIS CONTEXT:\n"
	result += "• Check if past experiences match current situation\n"
	result += "• Look for successful strategies from similar contexts\n"
	result += "• Avoid repeating past mistakes (check lessons)\n"
	result += "• Consider news impact + sentiment + your experience\n"

	logger.Info("generated news+memory context",
		zap.String("agent_id", t.agentID),
		zap.Int("news_count", len(news)),
		zap.String("query", newsQuery),
	)

	return result, nil
}

// GetNewsWithCluster gets a news item plus all related coverage from other sources
// Useful when you see high-impact news and want complete context
func (t *LocalToolkit) GetNewsWithCluster(
	ctx context.Context,
	newsID string,
) (string, error) {
	logger.Debug("toolkit: get_news_with_cluster",
		zap.String("agent_id", t.agentID),
		zap.String("news_id", newsID),
	)

	// This would need a method to get news by ID first
	// For now, return instruction to use GetRelatedNews directly
	return "Use GetRelatedNews(cluster_id) to see all coverage of same event", nil
}
