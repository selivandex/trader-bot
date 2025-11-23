package toolkit

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// ToolFunc is the signature for all tool functions
// Takes context and generic params map, returns generic result and error
type ToolFunc func(ctx context.Context, params map[string]interface{}) (interface{}, error)

// ToolMetadata contains tool information for introspection
type ToolMetadata struct {
	Name        string
	Description string
	ParamTypes  map[string]string // param name -> type description
	ReturnType  string
}

// ToolRegistry manages all available tools for agents
// Provides type-safe dynamic dispatch without hardcoded switch statements
type ToolRegistry struct {
	tools         map[string]ToolFunc
	metadata      map[string]ToolMetadata
	toolkit       AgentToolkit  // Underlying toolkit implementation
	metricsLogger MetricsLogger // Optional ClickHouse metrics logger
}

// MetricsLogger interface for logging tool usage to ClickHouse
type MetricsLogger interface {
	LogToolUsage(ctx context.Context, toolName string, params interface{}, resultCount int, avgSimilarity float32, useful bool, executionTimeMs int)
	Close() error // Graceful shutdown - flush remaining buffer
}

// NewToolRegistry creates new tool registry
func NewToolRegistry(toolkit AgentToolkit) *ToolRegistry {
	registry := &ToolRegistry{
		tools:    make(map[string]ToolFunc),
		metadata: make(map[string]ToolMetadata),
		toolkit:  toolkit,
	}

	// Register all available tools
	registry.registerTools()

	return registry
}

// SetMetricsLogger sets optional ClickHouse metrics logger
func (r *ToolRegistry) SetMetricsLogger(metricsLogger MetricsLogger) {
	r.metricsLogger = metricsLogger

	if metricsLogger != nil {
		logger.Info("tool registry metrics logger set",
			zap.Int("tools_count", len(r.tools)),
		)
	}
}

// Execute runs a tool by name with given parameters
// Returns result and error, with proper type checking and logging
func (r *ToolRegistry) Execute(ctx context.Context, name string, params map[string]interface{}) (interface{}, error) {
	fn, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s (available: %d tools)", name, len(r.tools))
	}

	logger.Debug("executing tool",
		zap.String("tool", name),
		zap.Any("params", params),
	)

	startTime := time.Now()
	result, err := fn(ctx, params)
	duration := time.Since(startTime)
	executionMs := int(duration.Milliseconds())

	if err != nil {
		logger.Warn("tool execution failed",
			zap.String("tool", name),
			zap.Error(err),
			zap.Duration("duration", duration),
		)

		// Log failed execution to ClickHouse
		if r.metricsLogger != nil {
			r.metricsLogger.LogToolUsage(ctx, name, params, 0, 0, false, executionMs)
		}

		return nil, fmt.Errorf("tool %s failed: %w", name, err)
	}

	logger.Debug("tool executed successfully",
		zap.String("tool", name),
		zap.Duration("duration", duration),
	)

	// Log successful execution to ClickHouse with result analysis
	if r.metricsLogger != nil {
		resultCount, avgSimilarity, useful := analyzeToolResult(result)
		r.metricsLogger.LogToolUsage(ctx, name, params, resultCount, avgSimilarity, useful, executionMs)
	}

	return result, nil
}

// analyzeToolResult extracts metrics from tool result
func analyzeToolResult(result interface{}) (resultCount int, avgSimilarity float32, useful bool) {
	switch res := result.(type) {
	case []models.NewsItem:
		resultCount = len(res)
		if resultCount > 0 {
			// Calculate average similarity score if available
			sumSimilarity := float64(0)
			count := 0
			for _, item := range res {
				if item.SimilarityScore > 0 {
					sumSimilarity += item.SimilarityScore
					count++
				}
			}
			if count > 0 {
				avgSimilarity = float32(sumSimilarity / float64(count))
			}
			useful = resultCount > 0
		}
	case []models.Candle:
		resultCount = len(res)
		useful = resultCount >= 10 // Enough candles for analysis
	case []models.WhaleTransaction:
		resultCount = len(res)
		useful = resultCount > 0
	case float64:
		resultCount = 1
		useful = true
	case string:
		resultCount = 1
		useful = len(res) > 0
	default:
		// Unknown result type - assume useful if not nil
		useful = result != nil
		resultCount = 1
	}

	return resultCount, avgSimilarity, useful
}

// GetMetadata returns metadata for a tool
func (r *ToolRegistry) GetMetadata(name string) (ToolMetadata, bool) {
	meta, ok := r.metadata[name]
	return meta, ok
}

// ListTools returns all available tool names
func (r *ToolRegistry) ListTools() []string {
	tools := make([]string, 0, len(r.tools))
	for name := range r.tools {
		tools = append(tools, name)
	}
	return tools
}

// GetToolCount returns number of registered tools
func (r *ToolRegistry) GetToolCount() int {
	return len(r.tools)
}

// Close gracefully shuts down registry and flushes metrics buffer
func (r *ToolRegistry) Close() error {
	if r.metricsLogger != nil {
		return r.metricsLogger.Close()
	}
	return nil
}

// registerTools registers all available tools with their wrappers
// This is the ONLY place where we need to add new tools
func (r *ToolRegistry) registerTools() {
	// Market data tools
	r.register("GetCandles", ToolMetadata{
		Description: "Fetch OHLCV candles for symbol and timeframe",
		ParamTypes:  map[string]string{"symbol": "string", "timeframe": "string", "limit": "int"},
		ReturnType:  "[]Candle",
	}, r.wrapGetCandles)

	r.register("CalculateRSI", ToolMetadata{
		Description: "Calculate RSI indicator for symbol",
		ParamTypes:  map[string]string{"symbol": "string", "timeframe": "string", "period": "int"},
		ReturnType:  "float64",
	}, r.wrapCalculateRSI)

	r.register("CalculateVolatility", ToolMetadata{
		Description: "Calculate price volatility (standard deviation)",
		ParamTypes:  map[string]string{"symbol": "string", "timeframe": "string", "period": "int"},
		ReturnType:  "float64",
	}, r.wrapCalculateVolatility)

	r.register("DetectTrend", ToolMetadata{
		Description: "Detect market trend direction and strength",
		ParamTypes:  map[string]string{"symbol": "string", "timeframe": "string"},
		ReturnType:  "TrendInfo",
	}, r.wrapDetectTrend)

	r.register("FindSupportLevels", ToolMetadata{
		Description: "Find support/resistance price levels",
		ParamTypes:  map[string]string{"symbol": "string", "timeframe": "string", "lookback": "int"},
		ReturnType:  "[]float64",
	}, r.wrapFindSupportLevels)

	r.register("CheckTimeframeAlignment", ToolMetadata{
		Description: "Check if multiple timeframes show same trend",
		ParamTypes:  map[string]string{"symbol": "string", "timeframes": "[]string"},
		ReturnType:  "AlignmentInfo",
	}, r.wrapCheckTimeframeAlignment)

	// News tools
	r.register("SearchNews", ToolMetadata{
		Description: "Search news by keywords (substring match)",
		ParamTypes:  map[string]string{"query": "string", "hours": "int", "limit": "int"},
		ReturnType:  "[]NewsItem",
	}, r.wrapSearchNews)

	r.register("SearchNewsSemantics", ToolMetadata{
		Description: "Search news by semantic meaning (requires embeddings)",
		ParamTypes:  map[string]string{"semantic_query": "string", "hours": "int", "limit": "int"},
		ReturnType:  "[]NewsItem",
	}, r.wrapSearchNewsSemantics)

	r.register("GetNewsWithMemoryContext", ToolMetadata{
		Description: "Get news with related agent memories (power tool)",
		ParamTypes:  map[string]string{"news_query": "string", "hours": "int", "news_limit": "int"},
		ReturnType:  "ContextualNewsResult",
	}, r.wrapGetNewsWithMemoryContext)

	r.register("GetHighImpactNews", ToolMetadata{
		Description: "Get high impact news items",
		ParamTypes:  map[string]string{"min_impact": "int", "hours": "int"},
		ReturnType:  "[]NewsItem",
	}, r.wrapGetHighImpactNews)

	r.register("FindNewsForMemory", ToolMetadata{
		Description: "Find current news related to past memory",
		ParamTypes:  map[string]string{"memory_id": "string", "hours": "int", "limit": "int"},
		ReturnType:  "[]NewsItem",
	}, r.wrapFindNewsForMemory)

	// On-chain tools
	r.register("GetRecentWhaleMovements", ToolMetadata{
		Description: "Get recent large transactions (whale movements)",
		ParamTypes:  map[string]string{"symbol": "string", "min_amount": "float64", "hours": "int"},
		ReturnType:  "[]WhaleTransaction",
	}, r.wrapGetRecentWhaleMovements)

	// Risk tools
	r.register("CalculatePositionRisk", ToolMetadata{
		Description: "Calculate risk metrics for potential position",
		ParamTypes:  map[string]string{"symbol": "string", "side": "string", "size": "float64", "leverage": "float64", "stop_loss": "float64"},
		ReturnType:  "RiskMetrics",
	}, r.wrapCalculatePositionRisk)

	// Memory tools
	r.register("SearchPersonalMemories", ToolMetadata{
		Description: "Search agent's personal memories",
		ParamTypes:  map[string]string{"query": "string", "top_k": "int"},
		ReturnType:  "[]SemanticMemory",
	}, r.wrapSearchPersonalMemories)

	logger.Info("tool registry initialized",
		zap.Int("tools_registered", len(r.tools)),
	)
}

// register adds a tool to the registry
func (r *ToolRegistry) register(name string, metadata ToolMetadata, fn ToolFunc) {
	metadata.Name = name
	r.tools[name] = fn
	r.metadata[name] = metadata
}

// ============ TOOL WRAPPERS ============
// Each wrapper handles type conversion and parameter extraction

func (r *ToolRegistry) wrapGetCandles(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	symbol, err := getString(params, "symbol")
	if err != nil {
		return nil, err
	}
	timeframe, err := getString(params, "timeframe")
	if err != nil {
		return nil, err
	}
	limit, err := getInt(params, "limit")
	if err != nil {
		return nil, err
	}

	return r.toolkit.GetCandles(ctx, symbol, timeframe, limit)
}

func (r *ToolRegistry) wrapCalculateRSI(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	symbol, err := getString(params, "symbol")
	if err != nil {
		return nil, err
	}
	timeframe, err := getString(params, "timeframe")
	if err != nil {
		return nil, err
	}
	period, err := getInt(params, "period")
	if err != nil {
		return nil, err
	}

	return r.toolkit.CalculateRSI(ctx, symbol, timeframe, period)
}

func (r *ToolRegistry) wrapCalculateVolatility(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	symbol, err := getString(params, "symbol")
	if err != nil {
		return nil, err
	}
	timeframe, err := getString(params, "timeframe")
	if err != nil {
		return nil, err
	}
	period, err := getInt(params, "period")
	if err != nil {
		return nil, err
	}

	return r.toolkit.CalculateVolatility(ctx, symbol, timeframe, period)
}

func (r *ToolRegistry) wrapSearchNews(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	query, err := getString(params, "query")
	if err != nil {
		return nil, err
	}
	hours, err := getInt(params, "hours")
	if err != nil {
		return nil, err
	}
	limit, err := getInt(params, "limit")
	if err != nil {
		return nil, err
	}

	return r.toolkit.SearchNews(ctx, query, time.Duration(hours)*time.Hour, limit)
}

func (r *ToolRegistry) wrapSearchNewsSemantics(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	semanticQuery, err := getString(params, "semantic_query")
	if err != nil {
		return nil, err
	}
	hours, err := getInt(params, "hours")
	if err != nil {
		return nil, err
	}
	limit, err := getInt(params, "limit")
	if err != nil {
		return nil, err
	}

	return r.toolkit.SearchNewsSemantics(ctx, semanticQuery, time.Duration(hours)*time.Hour, limit)
}

func (r *ToolRegistry) wrapGetNewsWithMemoryContext(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	newsQuery, err := getString(params, "news_query")
	if err != nil {
		return nil, err
	}
	hours, err := getInt(params, "hours")
	if err != nil {
		return nil, err
	}
	newsLimit, err := getInt(params, "news_limit")
	if err != nil {
		return nil, err
	}

	return r.toolkit.GetNewsWithMemoryContext(ctx, newsQuery, time.Duration(hours)*time.Hour, newsLimit)
}

func (r *ToolRegistry) wrapGetHighImpactNews(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	minImpact, err := getInt(params, "min_impact")
	if err != nil {
		return nil, err
	}
	hours, err := getInt(params, "hours")
	if err != nil {
		return nil, err
	}

	return r.toolkit.GetHighImpactNews(ctx, minImpact, time.Duration(hours)*time.Hour)
}

func (r *ToolRegistry) wrapFindNewsForMemory(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	memoryID, err := getString(params, "memory_id")
	if err != nil {
		return nil, err
	}
	hours, err := getInt(params, "hours")
	if err != nil {
		return nil, err
	}
	limit, err := getInt(params, "limit")
	if err != nil {
		return nil, err
	}

	return r.toolkit.FindNewsForMemory(ctx, memoryID, time.Duration(hours)*time.Hour, limit)
}

func (r *ToolRegistry) wrapGetRecentWhaleMovements(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	symbol, err := getString(params, "symbol")
	if err != nil {
		return nil, err
	}
	minAmount, err := getFloat(params, "min_amount")
	if err != nil {
		return nil, err
	}
	hours, err := getInt(params, "hours")
	if err != nil {
		return nil, err
	}

	return r.toolkit.GetRecentWhaleMovements(ctx, symbol, minAmount, hours)
}

func (r *ToolRegistry) wrapCalculatePositionRisk(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	symbol, err := getString(params, "symbol")
	if err != nil {
		return nil, err
	}
	side, err := getString(params, "side")
	if err != nil {
		return nil, err
	}
	size, err := getFloat(params, "size")
	if err != nil {
		return nil, err
	}
	leverage, err := getFloat(params, "leverage")
	if err != nil {
		return nil, err
	}
	stopLoss, err := getFloat(params, "stop_loss")
	if err != nil {
		return nil, err
	}

	return r.toolkit.CalculatePositionRisk(ctx, symbol, models.PositionSide(side), size, leverage, stopLoss)
}

func (r *ToolRegistry) wrapSearchPersonalMemories(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	query, err := getString(params, "query")
	if err != nil {
		return nil, err
	}
	topK, err := getInt(params, "top_k")
	if err != nil {
		return nil, err
	}

	return r.toolkit.SearchPersonalMemories(ctx, query, topK)
}

func (r *ToolRegistry) wrapDetectTrend(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	symbol, err := getString(params, "symbol")
	if err != nil {
		return nil, err
	}
	timeframe, err := getString(params, "timeframe")
	if err != nil {
		return nil, err
	}

	return r.toolkit.DetectTrend(ctx, symbol, timeframe)
}

func (r *ToolRegistry) wrapFindSupportLevels(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	symbol, err := getString(params, "symbol")
	if err != nil {
		return nil, err
	}
	timeframe, err := getString(params, "timeframe")
	if err != nil {
		return nil, err
	}
	lookback, err := getInt(params, "lookback")
	if err != nil {
		return nil, err
	}

	return r.toolkit.FindSupportLevels(ctx, symbol, timeframe, lookback)
}

func (r *ToolRegistry) wrapCheckTimeframeAlignment(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	symbol, err := getString(params, "symbol")
	if err != nil {
		return nil, err
	}

	timeframesRaw, ok := params["timeframes"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("timeframes must be array")
	}

	timeframes := make([]string, len(timeframesRaw))
	for i, tf := range timeframesRaw {
		timeframes[i], ok = tf.(string)
		if !ok {
			return nil, fmt.Errorf("timeframes must contain strings")
		}
	}

	return r.toolkit.CheckTimeframeAlignment(ctx, symbol, timeframes)
}

// ============ PARAMETER HELPERS ============

func getString(params map[string]interface{}, key string) (string, error) {
	val, ok := params[key]
	if !ok {
		return "", fmt.Errorf("missing required parameter: %s", key)
	}
	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("parameter %s must be string, got %T", key, val)
	}
	return str, nil
}

func getInt(params map[string]interface{}, key string) (int, error) {
	val, ok := params[key]
	if !ok {
		return 0, fmt.Errorf("missing required parameter: %s", key)
	}

	// Handle both int and float64 (JSON numbers become float64)
	switch v := val.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("parameter %s must be number, got %T", key, val)
	}
}

func getFloat(params map[string]interface{}, key string) (float64, error) {
	val, ok := params[key]
	if !ok {
		return 0, fmt.Errorf("missing required parameter: %s", key)
	}

	switch v := val.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("parameter %s must be number, got %T", key, val)
	}
}
