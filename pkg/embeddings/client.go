package embeddings

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/sashabaranov/go-openai"
	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/metrics"
)

// EmbeddingRepository interface for storage implementations
// Not really a "cache" - embeddings are deterministic and expensive,
// so we store them permanently to avoid redundant API calls
type EmbeddingRepository interface {
	Get(ctx context.Context, textHash string) ([]float32, bool)
	Set(ctx context.Context, textHash string, embedding []float32, model string, textLength int) error
}

// Client handles unified embedding generation with deduplication via repository
type Client struct {
	repository          EmbeddingRepository
	metricsBuffer       metrics.Buffer
	openaiClient        *openai.Client
	model               openai.EmbeddingModel
	deduplicationHits   int64
	deduplicationMisses int64
}

// Config for embedding client
type Config struct {
	OpenAIClient  *openai.Client
	Repository    EmbeddingRepository   // Optional repository for deduplication
	MetricsBuffer metrics.Buffer        // Optional metrics buffer for ClickHouse
	Model         openai.EmbeddingModel // Default: openai.AdaEmbeddingV2
}

// NewClient creates new unified embedding client with optional deduplication
func NewClient(cfg Config) *Client {
	model := cfg.Model
	if model == "" {
		model = openai.AdaEmbeddingV2
	}

	if cfg.Repository != nil {
		logger.Info("embedding deduplication enabled (Postgres repository)")
	}

	return &Client{
		openaiClient:  cfg.OpenAIClient,
		repository:    cfg.Repository,
		metricsBuffer: cfg.MetricsBuffer,
		model:         model,
	}
}

// Generate creates embedding for single text with deduplication and retry logic
func (c *Client) Generate(ctx context.Context, text string) ([]float32, error) {
	// Try repository first (deduplication)
	if c.repository != nil {
		textHash := c.hashText(text)
		existing, found := c.repository.Get(ctx, textHash)
		if found {
			atomic.AddInt64(&c.deduplicationHits, 1)

			// Log to ClickHouse
			if c.metricsBuffer != nil {
				if err := c.metricsBuffer.Add(&metrics.EmbeddingDeduplicationMetric{
					Timestamp:    time.Now(),
					TextHash:     textHash[:16], // Store prefix only
					TextLength:   len(text),
					Model:        string(c.model),
					CacheHit:     true,
					CostSavedUSD: 0.0001,
				}); err != nil {
					logger.Error("failed to add deduplication metric", zap.Error(err))
				}
			}

			logger.Debug("✅ embedding deduplication HIT (saved $0.0001)",
				zap.Int("text_len", len(text)),
				zap.String("hash", textHash[:12]),
			)
			return existing, nil
		}
		atomic.AddInt64(&c.deduplicationMisses, 1)
	}

	// Not found - generate via OpenAI API with retry
	if c.openaiClient == nil {
		return nil, fmt.Errorf("OpenAI embedding client not configured - please set OPENAI_API_KEY")
	}

	// Retry with exponential backoff (up to 3 attempts)
	embeddings, err := c.generateWithRetry(ctx, []string{text}, 3)
	if err != nil {
		return nil, fmt.Errorf("OpenAI embedding API failed after retries: %w", err)
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("OpenAI returned no embedding data")
	}

	result := embeddings[0]

	// Store in repository for future deduplication
	if c.repository != nil {
		textHash := c.hashText(text)
		if err := c.repository.Set(ctx, textHash, result, string(c.model), len(text)); err != nil {
			logger.Warn("failed to store embedding in repository", zap.Error(err))
			// Non-critical, continue
		}
	}

	// Log miss to ClickHouse
	if c.metricsBuffer != nil {
		textHash := c.hashText(text)
		if err := c.metricsBuffer.Add(&metrics.EmbeddingDeduplicationMetric{
			Timestamp:    time.Now(),
			TextHash:     textHash[:16],
			TextLength:   len(text),
			Model:        string(c.model),
			CacheHit:     false,
			CostSavedUSD: 0,
		}); err != nil {
			logger.Error("failed to add embedding miss metric", zap.Error(err))
		}
	}

	logger.Debug("💸 embedding generated via OpenAI API (cost: $0.0001)",
		zap.Int("text_len", len(text)),
		zap.Int("dim", len(result)),
		zap.String("hash", c.hashText(text)[:12]),
	)

	return result, nil
}

// generateWithRetry calls OpenAI API with exponential backoff retry
func (c *Client) generateWithRetry(ctx context.Context, texts []string, maxRetries int) ([][]float32, error) {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoffDuration := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			logger.Debug("retrying OpenAI embedding request",
				zap.Int("attempt", attempt+1),
				zap.Int("max_retries", maxRetries),
				zap.Duration("backoff", backoffDuration),
			)

			select {
			case <-time.After(backoffDuration):
				// Continue
			case <-ctx.Done():
				return nil, fmt.Errorf("context canceled during retry backoff: %w", ctx.Err())
			}
		}

		resp, err := c.openaiClient.CreateEmbeddings(
			ctx,
			openai.EmbeddingRequest{
				Model: c.model,
				Input: texts,
			},
		)

		if err == nil {
			// Success - extract embeddings
			embeddings := make([][]float32, len(resp.Data))
			for i, data := range resp.Data {
				embeddings[i] = data.Embedding
			}
			return embeddings, nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err) {
			logger.Warn("non-retryable OpenAI error, aborting",
				zap.Error(err),
			)
			return nil, err
		}

		logger.Warn("retryable OpenAI error encountered",
			zap.Error(err),
			zap.Int("attempt", attempt+1),
		)
	}

	return nil, fmt.Errorf("max retries (%d) exceeded: %w", maxRetries, lastErr)
}

// isRetryableError checks if error should trigger a retry
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Rate limit errors (429)
	if strings.Contains(errStr, "429") || strings.Contains(errStr, "rate limit") {
		return true
	}

	// Timeout errors
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		return true
	}

	// Temporary network errors
	if strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "connection reset") {
		return true
	}

	// Server errors (5xx)
	if strings.Contains(errStr, "500") || strings.Contains(errStr, "502") || strings.Contains(errStr, "503") {
		return true
	}

	// Check for openai.APIError (typed error)
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		// Retry on rate limits and server errors
		return apiErr.HTTPStatusCode == http.StatusTooManyRequests ||
			apiErr.HTTPStatusCode >= 500
	}

	return false
}

// GenerateBatch creates embeddings for multiple texts (up to 2048 per batch)
// Much faster than individual calls (10x speedup)
// Uses repository for deduplication and retry logic with exponential backoff
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

		// Check repository for batch items (deduplication)
		var uncachedIndices []int
		var uncachedTexts []string

		for j, text := range batch {
			if c.repository != nil {
				textHash := c.hashText(text)
				existing, found := c.repository.Get(ctx, textHash)
				if found {
					allEmbeddings[i+j] = existing
					continue
				}
			}
			uncachedIndices = append(uncachedIndices, i+j)
			uncachedTexts = append(uncachedTexts, text)
		}

		// If all found in repository, skip API call
		if len(uncachedTexts) == 0 {
			logger.Debug("batch embedding deduplication (all found in repository)",
				zap.Int("batch_size", len(batch)),
			)
			continue
		}

		// Batch API call with retry for new items
		embeddings, err := c.generateWithRetry(ctx, uncachedTexts, 3)
		if err != nil {
			return nil, fmt.Errorf("batch embedding API failed after retries: %w", err)
		}

		if len(embeddings) != len(uncachedTexts) {
			return nil, fmt.Errorf("batch response size mismatch: expected %d, got %d", len(uncachedTexts), len(embeddings))
		}

		// Assign embeddings and store in repository
		for j, embedding := range embeddings {
			idx := uncachedIndices[j]
			allEmbeddings[idx] = embedding

			// Store in repository
			if c.repository != nil {
				textHash := c.hashText(uncachedTexts[j])
				if err := c.repository.Set(ctx, textHash, embedding, string(c.model), len(uncachedTexts[j])); err != nil {
					logger.Warn("failed to store embedding in repository", zap.Error(err))
					// Non-critical, continue
				}
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

// hashText creates SHA256 hash of text for cache key
func (c *Client) hashText(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}

// GetDeduplicationEnabled returns whether deduplication is enabled
func (c *Client) GetDeduplicationEnabled() bool {
	return c.repository != nil
}

// GetDeduplicationStats returns deduplication statistics
func (c *Client) GetDeduplicationStats() (hits, misses int64, savingsUSD float64) {
	hits = atomic.LoadInt64(&c.deduplicationHits)
	misses = atomic.LoadInt64(&c.deduplicationMisses)
	savingsUSD = float64(hits) * 0.0001 // $0.0001 per embedding
	return
}

// LogDeduplicationStats logs current deduplication statistics
func (c *Client) LogDeduplicationStats() {
	if c.repository == nil {
		return
	}

	hits, misses, savings := c.GetDeduplicationStats()
	total := hits + misses

	if total == 0 {
		return
	}

	hitRate := float64(hits) / float64(total) * 100

	logger.Info("📊 Embedding deduplication stats",
		zap.Int64("hits", hits),
		zap.Int64("misses", misses),
		zap.Int64("total", total),
		zap.Float64("hit_rate_%", hitRate),
		zap.Float64("savings_usd", savings),
	)
}
