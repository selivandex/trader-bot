package embeddings

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sashabaranov/go-openai"
	"go.uber.org/zap"

	redisAdapter "github.com/selivandex/trader-bot/internal/adapters/redis"
	"github.com/selivandex/trader-bot/pkg/logger"
)

// Client handles unified embedding generation with Redis caching
// Consolidates logic from news.Cache and agents.SemanticMemoryManager
type Client struct {
	openaiClient *openai.Client
	redisClient  *redisAdapter.Client
	model        openai.EmbeddingModel
}

// Config for embedding client
type Config struct {
	OpenAIClient *openai.Client
	RedisClient  *redisAdapter.Client
	Model        openai.EmbeddingModel // Default: openai.AdaEmbeddingV2
	CacheTTL     time.Duration
}

// NewClient creates new unified embedding client
func NewClient(cfg Config) *Client {
	model := cfg.Model
	if model == "" {
		model = openai.AdaEmbeddingV2
	}

	return &Client{
		openaiClient: cfg.OpenAIClient,
		redisClient:  cfg.RedisClient,
		model:        model,
	}
}

// Generate creates embedding for single text with Redis cache
// Returns error if OpenAI is unavailable - NO FALLBACK
func (c *Client) Generate(ctx context.Context, text string) ([]float32, error) {
	// Try Redis cache first
	if c.redisClient != nil {
		cached, hit := c.getCachedEmbedding(ctx, text)
		if hit {
			logger.Debug("embedding cache hit",
				zap.Int("text_len", len(text)),
			)
			return cached, nil
		}
	}

	// Cache miss - generate via OpenAI API
	if c.openaiClient == nil {
		return nil, fmt.Errorf("OpenAI embedding client not configured - please set OPENAI_API_KEY")
	}

	resp, err := c.openaiClient.CreateEmbeddings(
		ctx,
		openai.EmbeddingRequest{
			Model: c.model,
			Input: []string{text},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("OpenAI embedding API failed: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("OpenAI returned no embedding data")
	}

	embedding := resp.Data[0].Embedding

	// Cache for future use (7 days TTL - embeddings are deterministic)
	if c.redisClient != nil {
		c.cacheEmbedding(ctx, text, embedding)
	}

	logger.Debug("embedding generated via OpenAI",
		zap.Int("text_len", len(text)),
		zap.Int("dim", len(embedding)),
	)

	return embedding, nil
}

// GenerateBatch creates embeddings for multiple texts (up to 2048 per batch)
// Much faster than individual calls (10x speedup)
func (c *Client) GenerateBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	if c.openaiClient == nil {
		return nil, fmt.Errorf("OpenAI embedding client not configured - please set OPENAI_API_KEY")
	}

	// OpenAI supports up to 2048 inputs per batch
	const maxBatchSize = 2048

	allEmbeddings := make([][]float32, len(texts))

	for i := 0; i < len(texts); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		// Check cache for batch items
		var uncachedIndices []int
		var uncachedTexts []string

		for j, text := range batch {
			if c.redisClient != nil {
				cached, hit := c.getCachedEmbedding(ctx, text)
				if hit {
					allEmbeddings[i+j] = cached
					continue
				}
			}
			uncachedIndices = append(uncachedIndices, i+j)
			uncachedTexts = append(uncachedTexts, text)
		}

		// If all cached, skip API call
		if len(uncachedTexts) == 0 {
			logger.Debug("batch embedding cache hit (all)",
				zap.Int("batch_size", len(batch)),
			)
			continue
		}

		// Single batch API call for uncached items
		resp, err := c.openaiClient.CreateEmbeddings(
			ctx,
			openai.EmbeddingRequest{
				Model: c.model,
				Input: uncachedTexts,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("batch embedding API failed: %w", err)
		}

		if len(resp.Data) != len(uncachedTexts) {
			return nil, fmt.Errorf("batch response size mismatch: expected %d, got %d", len(uncachedTexts), len(resp.Data))
		}

		// Assign embeddings and cache them
		for j, embeddingData := range resp.Data {
			idx := uncachedIndices[j]
			embedding := embeddingData.Embedding
			allEmbeddings[idx] = embedding

			// Cache for future use
			if c.redisClient != nil {
				c.cacheEmbedding(ctx, uncachedTexts[j], embedding)
			}
		}

		logger.Debug("batch embedding generation successful",
			zap.Int("batch_size", len(batch)),
			zap.Int("cached", len(batch)-len(uncachedTexts)),
			zap.Int("generated", len(uncachedTexts)),
			zap.Int("batch_num", i/maxBatchSize+1),
		)
	}

	return allEmbeddings, nil
}

// getCachedEmbedding retrieves embedding from Redis cache
func (c *Client) getCachedEmbedding(ctx context.Context, text string) ([]float32, bool) {
	hash := md5.Sum([]byte(text))
	cacheKey := fmt.Sprintf("embedding:v2:%s:%x", c.model, hash)

	data, err := c.redisClient.Get(ctx, cacheKey).Result()
	if err != nil {
		return nil, false // Cache miss
	}

	var embedding []float32
	if err := json.Unmarshal([]byte(data), &embedding); err != nil {
		logger.Warn("failed to deserialize cached embedding", zap.Error(err))
		return nil, false
	}

	return embedding, true
}

// cacheEmbedding stores embedding in Redis cache with 7-day TTL
// Embeddings are deterministic - same text always produces same embedding
func (c *Client) cacheEmbedding(ctx context.Context, text string, embedding []float32) {
	hash := md5.Sum([]byte(text))
	cacheKey := fmt.Sprintf("embedding:v2:%s:%x", c.model, hash)

	data, err := json.Marshal(embedding)
	if err != nil {
		logger.Warn("failed to serialize embedding for cache", zap.Error(err))
		return
	}

	// 7 days TTL - embeddings don't change, long cache is safe
	err = c.redisClient.Set(ctx, cacheKey, data, 7*24*time.Hour).Err()
	if err != nil {
		logger.Warn("failed to cache embedding", zap.Error(err))
	}
}

// GetCacheStats returns cache hit statistics
func (c *Client) GetCacheStats(ctx context.Context) (*CacheStats, error) {
	if c.redisClient == nil {
		return &CacheStats{CacheEnabled: false}, nil
	}

	// Note: Counting keys is expensive in Redis, return approximate stats
	// For production, consider using SCAN or sampling
	return &CacheStats{
		CacheEnabled: true,
		CachedCount:  -1, // Use -1 to indicate "not counted" (scanning all keys is expensive)
	}, nil
}

// CacheStats represents cache statistics
type CacheStats struct {
	CacheEnabled bool
	CachedCount  int
}
