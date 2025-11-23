package agents

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/ai"
	"github.com/selivandex/trader-bot/pkg/embeddings"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// SemanticMemoryManager handles agent's episodic memory system
// Agents remember past experiences and recall relevant ones for current situations
type SemanticMemoryManager struct {
	repository      *Repository
	aiProvider      ai.AgenticProvider  // For summaries
	embeddingClient *embeddings.Client  // Unified embedding client
}

// NewSemanticMemoryManager creates new semantic memory manager
func NewSemanticMemoryManager(
	repository *Repository,
	aiProvider ai.AgenticProvider,
	embeddingClient *embeddings.Client,
) *SemanticMemoryManager {
	return &SemanticMemoryManager{
		repository:      repository,
		aiProvider:      aiProvider,
		embeddingClient: embeddingClient,
	}
}

// Store saves new memory from trade experience
// Also contributes to collective memory for agent's personality
func (smm *SemanticMemoryManager) Store(ctx context.Context, agentID string, personality string, experience *models.TradeExperience) error {
	// Ask AI to summarize what's important to remember
	summary, err := smm.aiProvider.SummarizeMemory(ctx, experience)
	if err != nil {
		return fmt.Errorf("failed to summarize memory: %w", err)
	}

	// Generate embedding for semantic search
	embedding, err := smm.embeddingClient.Generate(ctx, summary.Context+" "+summary.Lesson)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// 1. Store as personal memory
	memory := &models.SemanticMemory{
		AgentID:    agentID,
		Context:    summary.Context,
		Action:     summary.Action,
		Outcome:    summary.Outcome,
		Lesson:     summary.Lesson,
		Embedding:  embedding,
		Importance: summary.Importance,
	}

	err = smm.repository.StoreSemanticMemory(ctx, memory)
	if err != nil {
		return fmt.Errorf("failed to store memory: %w", err)
	}

	logger.Info("💾 Stored personal memory",
		zap.String("agent_id", agentID),
		zap.String("memory_id", memory.ID),
		zap.String("lesson", summary.Lesson),
	)

	// 2. Contribute to collective memory for this personality
	if summary.Importance >= 0.6 { // Only contribute important lessons
		err = smm.repository.ContributeToCollective(
			ctx,
			agentID,
			personality,
			summary,
			embedding,
			experience.WasSuccessful,
		)
		if err != nil {
			logger.Warn("failed to contribute to collective", zap.Error(err))
			// Don't fail the whole operation
		} else {
			logger.Info("🌍 Contributed to collective memory",
				zap.String("personality", personality),
				zap.String("lesson", summary.Lesson),
			)
		}
	}

	return nil
}

// RecallRelevant retrieves most relevant memories for current situation
// Combines personal memories + collective wisdom
func (smm *SemanticMemoryManager) RecallRelevant(
	ctx context.Context,
	agentID string,
	personality string,
	currentSituation string,
	topK int,
) ([]models.SemanticMemory, error) {
	// Generate embedding for current situation
	queryEmbedding, err := smm.embeddingClient.Generate(ctx, currentSituation)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// 1. ✅ Get personal memories via PostgreSQL vector search
	personalMemories, err := smm.repository.SearchSemanticMemoriesByVector(ctx, agentID, queryEmbedding, topK*2)
	if err != nil {
		return nil, fmt.Errorf("failed to vector search personal memories: %w", err)
	}

	// 2. ✅ Get collective memories via PostgreSQL vector search
	var collectiveMemories []models.CollectiveMemory
	if personality != "" {
		collectiveMemories, err = smm.repository.SearchCollectiveMemoriesByVector(ctx, personality, queryEmbedding, topK)
		if err != nil {
			logger.Warn("failed to vector search collective memories, using personal only", zap.Error(err))
			collectiveMemories = []models.CollectiveMemory{}
		}
	}

	// 3. Combine and rank by combined score
	allMemories := smm.mergePersonalAndCollective(personalMemories, collectiveMemories)

	type memoryWithScore struct {
		memory models.SemanticMemory
		score  float64
	}

	memoriesWithScores := []memoryWithScore{}

	for _, mem := range allMemories {
		// PostgreSQL already sorted by similarity (distance)
		// Calculate combined score: importance * relevance boost
		boost := 1.0
		if mem.AccessCount > 0 { // Personal memory
			boost = 1.2
		}

		// Importance is pre-weighted by success rate for collective memories
		score := mem.Importance * boost

		memoriesWithScores = append(memoriesWithScores, memoryWithScore{
			memory: mem,
			score:  score,
		})
	}

	// Sort by combined score (similarity already from PG, now add importance)
	sort.Slice(memoriesWithScores, func(i, j int) bool {
		return memoriesWithScores[i].score > memoriesWithScores[j].score
	})

	// Take top K
	if topK > len(memoriesWithScores) {
		topK = len(memoriesWithScores)
	}

	result := make([]models.SemanticMemory, topK)
	for i := 0; i < topK; i++ {
		result[i] = memoriesWithScores[i].memory

		// Update access count for personal memories
		if result[i].AgentID != "collective" {
			smm.repository.UpdateMemoryAccess(ctx, result[i].ID)
		}
	}

	logger.Debug("recalled memories via PostgreSQL vector search",
		zap.String("agent_id", agentID),
		zap.Int("recalled", len(result)),
		zap.Int("personal", len(personalMemories)),
		zap.Int("collective", len(collectiveMemories)),
	)

	return result, nil
}

// GetAllMemories retrieves all memories for agent
func (smm *SemanticMemoryManager) GetAllMemories(ctx context.Context, agentID string) ([]models.SemanticMemory, error) {
	return smm.repository.GetSemanticMemories(ctx, agentID, 1000)
}

// Forget removes less important memories (memory consolidation)
func (smm *SemanticMemoryManager) Forget(ctx context.Context, agentID string, threshold float64) error {
	deleted, err := smm.repository.DeleteOldMemories(ctx, agentID, threshold)
	if err != nil {
		return fmt.Errorf("failed to forget memories: %w", err)
	}

	logger.Info("🧹 Consolidated memories",
		zap.String("agent_id", agentID),
		zap.Int64("deleted", deleted),
	)

	return nil
}

// cosineSimilarity calculates cosine similarity between two vectors (kept for reference)
func (smm *SemanticMemoryManager) cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	dotProduct := float32(0.0)
	normA := float32(0.0)
	normB := float32(0.0)

	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return float64(dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB)))))
}

// mergePersonalAndCollective combines personal and collective memories
func (smm *SemanticMemoryManager) mergePersonalAndCollective(
	personal []models.SemanticMemory,
	collective []models.CollectiveMemory,
) []models.SemanticMemory {
	// Convert collective to personal format
	merged := make([]models.SemanticMemory, len(personal))
	copy(merged, personal)

	for _, col := range collective {
		// Convert to SemanticMemory format
		mem := models.SemanticMemory{
			ID:           col.ID,
			AgentID:      "collective", // Special marker
			Context:      col.Context,
			Action:       col.Action,
			Outcome:      fmt.Sprintf("Success rate: %.1f%% (%d agents)", col.SuccessRate*100, col.ConfirmationCount),
			Lesson:       col.Lesson,
			Embedding:    col.Embedding,
			Importance:   col.Importance * col.SuccessRate, // Weight by success rate
			AccessCount:  0,                                // Collective memories don't have access count
			LastAccessed: col.LastConfirmedAt,
			CreatedAt:    col.CreatedAt,
		}

		merged = append(merged, mem)
	}

	return merged
}

// ============ CROSS-REFERENCE WITH NEWS ============

// FindNewsRelatedToMemory finds news semantically related to a memory
// Used when agent recalls a memory and wants to check current news context
func (smm *SemanticMemoryManager) FindNewsRelatedToMemory(
	ctx context.Context,
	memory *models.SemanticMemory,
	newsRepo interface {
		SearchNewsByVector(ctx context.Context, embedding []float32, since time.Duration, limit int) ([]models.NewsItem, error)
	},
	since time.Duration,
	limit int,
) ([]models.NewsItem, error) {
	if len(memory.Embedding) == 0 {
		return nil, fmt.Errorf("memory has no embedding")
	}

	// Use memory's embedding to find related news
	news, err := newsRepo.SearchNewsByVector(ctx, memory.Embedding, since, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to find related news: %w", err)
	}

	logger.Debug("found news related to memory",
		zap.String("memory_id", memory.ID),
		zap.String("lesson", memory.Lesson),
		zap.Int("news_count", len(news)),
	)

	return news, nil
}

// FindMemoriesRelatedToNews finds agent memories related to current news
// Used when agent sees important news and wants to recall past similar situations
func (smm *SemanticMemoryManager) FindMemoriesRelatedToNews(
	ctx context.Context,
	agentID string,
	personality string,
	newsEmbedding []float32,
	topK int,
) ([]models.SemanticMemory, error) {
	if len(newsEmbedding) == 0 {
		return nil, fmt.Errorf("news has no embedding")
	}

	// Search personal memories using news embedding
	personalMemories, err := smm.repository.SearchSemanticMemoriesByVector(ctx, agentID, newsEmbedding, topK*2)
	if err != nil {
		return nil, fmt.Errorf("failed to search memories: %w", err)
	}

	// Search collective memories
	var collectiveMemories []models.CollectiveMemory
	if personality != "" {
		collectiveMemories, err = smm.repository.SearchCollectiveMemoriesByVector(ctx, personality, newsEmbedding, topK)
		if err != nil {
			logger.Warn("failed to search collective memories", zap.Error(err))
			collectiveMemories = []models.CollectiveMemory{}
		}
	}

	// Merge and rank
	allMemories := smm.mergePersonalAndCollective(personalMemories, collectiveMemories)

	// Take top K
	if topK > len(allMemories) {
		topK = len(allMemories)
	}

	result := allMemories[:topK]

	logger.Debug("found memories related to news",
		zap.String("agent_id", agentID),
		zap.Int("found", len(result)),
	)

	return result, nil
}

// GenerateContextualSummary creates rich context by combining news + memories
// Returns formatted text for agent's reasoning
func (smm *SemanticMemoryManager) GenerateContextualSummary(
	ctx context.Context,
	agentID string,
	personality string,
	currentNews []models.NewsItem,
) (string, error) {
	if len(currentNews) == 0 {
		return "", nil
	}

	summary := "📰 CURRENT NEWS CONTEXT:\n"

	for i, news := range currentNews {
		summary += fmt.Sprintf("%d. [%s] %s (impact: %d, sentiment: %.2f)\n",
			i+1, news.Source, news.Title, news.Impact, news.Sentiment)

		// Find related memories for this news
		if len(news.Embedding) > 0 {
			memories, err := smm.FindMemoriesRelatedToNews(ctx, agentID, personality, news.Embedding, 2)
			if err == nil && len(memories) > 0 {
				summary += "   💭 RELATED PAST EXPERIENCE:\n"
				for _, mem := range memories {
					summary += fmt.Sprintf("      - %s → %s\n", mem.Context, mem.Lesson)
				}
			}
		}

		// Show related news (cluster)
		if news.ClusterID != nil && !news.IsClusterPrimary {
			summary += "   🔗 (Part of larger story - see cluster for full context)\n"
		}
	}

	return summary, nil
}
