package agents

import (
	"context"
	"fmt"
	"math"
	"sort"

	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/internal/adapters/ai"
	"github.com/alexanderselivanov/trader/pkg/logger"
	"github.com/alexanderselivanov/trader/pkg/models"
)

// SemanticMemoryManager handles agent's episodic memory system
// Agents remember past experiences and recall relevant ones for current situations
type SemanticMemoryManager struct {
	repository *Repository
	aiProvider ai.AgenticProvider // For generating embeddings and summaries
}

// NewSemanticMemoryManager creates new semantic memory manager
func NewSemanticMemoryManager(repository *Repository, aiProvider ai.AgenticProvider) *SemanticMemoryManager {
	return &SemanticMemoryManager{
		repository: repository,
		aiProvider: aiProvider,
	}
}

// Store saves new memory from trade experience
func (smm *SemanticMemoryManager) Store(ctx context.Context, agentID string, experience *models.TradeExperience) error {
	// Ask AI to summarize what's important to remember
	summary, err := smm.aiProvider.SummarizeMemory(ctx, experience)
	if err != nil {
		return fmt.Errorf("failed to summarize memory: %w", err)
	}

	// Generate embedding for semantic search
	// For now, use simple text embedding (could use OpenAI embeddings API later)
	embedding := smm.generateSimpleEmbedding(summary.Context + " " + summary.Lesson)

	// Create memory object
	memory := &models.SemanticMemory{
		AgentID:    agentID,
		Context:    summary.Context,
		Action:     summary.Action,
		Outcome:    summary.Outcome,
		Lesson:     summary.Lesson,
		Embedding:  embedding,
		Importance: summary.Importance,
	}

	// Store via repository
	err = smm.repository.StoreSemanticMemory(ctx, memory)
	if err != nil {
		return fmt.Errorf("failed to store memory: %w", err)
	}

	logger.Info("💾 Stored new memory",
		zap.String("agent_id", agentID),
		zap.String("memory_id", memory.ID),
		zap.String("lesson", summary.Lesson),
		zap.Float64("importance", summary.Importance),
	)

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
	queryEmbedding := smm.generateSimpleEmbedding(currentSituation)

	// 1. Get personal memories
	personalMemories, err := smm.repository.GetSemanticMemories(ctx, agentID, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to query personal memories: %w", err)
	}

	// 2. Get collective memories for agent's personality
	collectiveMemories, err := smm.repository.GetCollectiveMemories(ctx, personality, 50)
	if err != nil {
		logger.Warn("failed to get collective memories, using personal only", zap.Error(err))
		collectiveMemories = []models.CollectiveMemory{}
	}

	// 3. Combine both into unified list
	allMemories := smm.mergePersonalAndCollective(personalMemories, collectiveMemories)

	type memoryWithScore struct {
		memory models.SemanticMemory
		score  float64
	}

	memoriesWithScores := []memoryWithScore{}

	for _, mem := range allMemories {
		// Calculate similarity score (cosine similarity)
		similarity := smm.cosineSimilarity(queryEmbedding, mem.Embedding)

		// Combined score: similarity * importance
		// Personal memories get slight boost (1.2x) as they're more specific to this agent
		boost := 1.0
		if mem.AccessCount > 0 { // Personal memory (has access count)
			boost = 1.2
		}

		score := similarity * mem.Importance * boost

		memoriesWithScores = append(memoriesWithScores, memoryWithScore{
			memory: mem,
			score:  score,
		})
	}

	// Sort by score
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

		// Update access count and timestamp
		smm.repository.UpdateMemoryAccess(ctx, result[i].ID)
	}

	logger.Debug("recalled memories",
		zap.String("agent_id", agentID),
		zap.Int("recalled", len(result)),
		zap.Int("total", len(memoriesWithScores)),
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

// generateSimpleEmbedding creates simple embedding from text
// In production, use OpenAI embeddings API or sentence transformers
func (smm *SemanticMemoryManager) generateSimpleEmbedding(text string) []float32 {
	// Simple bag-of-words embedding (128 dimensions)
	// This is a placeholder - in production use proper embeddings
	embedding := make([]float32, 128)

	// Hash-based simple embedding
	for i, char := range text {
		idx := (int(char) + i) % 128
		embedding[idx] += 1.0
	}

	// Normalize
	norm := float32(0.0)
	for _, v := range embedding {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))

	if norm > 0 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}

	return embedding
}

// cosineSimilarity calculates cosine similarity between two vectors
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
