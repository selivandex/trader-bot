package metrics

import "time"

// Common metric types that can be reused across the system

// ToolUsageMetric represents tool usage metrics for ClickHouse
type ToolUsageMetric struct {
	Timestamp        time.Time
	AgentName        string
	AgentPersonality string
	ToolName         string
	Params           string // JSON
	ResultCount      int
	AvgSimilarity    float32
	Useful           bool
	ExecutionTimeMs  int
}

func (m *ToolUsageMetric) TableName() string {
	return "tool_usage_metrics"
}

func (m *ToolUsageMetric) Values() []interface{} {
	return []interface{}{
		m.Timestamp,
		m.AgentName,
		m.AgentPersonality,
		m.ToolName,
		m.Params,
		m.ResultCount,
		m.AvgSimilarity,
		m.Useful,
		m.ExecutionTimeMs,
	}
}

// EmbeddingDeduplicationMetric tracks embedding cache hits/misses
type EmbeddingDeduplicationMetric struct {
	Timestamp    time.Time
	TextHash     string
	Model        string
	TextLength   int
	CostSavedUSD float64
	CacheHit     bool
}

func (m *EmbeddingDeduplicationMetric) TableName() string {
	return "embedding_deduplication_metrics"
}

func (m *EmbeddingDeduplicationMetric) Values() []interface{} {
	return []interface{}{
		m.Timestamp,
		m.TextHash,
		m.TextLength,
		m.Model,
		m.CacheHit,
		m.CostSavedUSD,
	}
}

// AgentDecisionMetric tracks agent decision quality
type AgentDecisionMetric struct {
	Timestamp          time.Time
	AgentName          string
	AgentPersonality   string
	Symbol             string
	Action             string
	Confidence         int
	ThinkingIterations int
	ToolsUsed          int
	DecisionTimeMs     int
	MemoriesRecalled   int
}

func (m *AgentDecisionMetric) TableName() string {
	return "agent_decision_metrics"
}

func (m *AgentDecisionMetric) Values() []interface{} {
	return []interface{}{
		m.Timestamp,
		m.AgentName,
		m.AgentPersonality,
		m.Symbol,
		m.Action,
		m.Confidence,
		m.ThinkingIterations,
		m.ToolsUsed,
		m.DecisionTimeMs,
		m.MemoriesRecalled,
	}
}
