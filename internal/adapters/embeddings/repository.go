package embeddings

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
)

// Repository handles persistent embedding storage in Postgres
// This is NOT a cache - embeddings are expensive to generate and deterministic,
// so we store them permanently to avoid redundant OpenAI API calls
type Repository struct {
	db interface {
		QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
		ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	}
}

// NewRepository creates new Postgres embedding repository
func NewRepository(db interface {
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}) *Repository {
	return &Repository{db: db}
}

// Get retrieves embedding from Postgres repository
func (r *Repository) Get(ctx context.Context, textHash string) ([]float32, bool) {
	query := `
		UPDATE embedding_cache
		SET last_used_at = NOW(), use_count = use_count + 1
		WHERE text_hash = $1
		RETURNING embedding
	`

	var embeddingBytes []byte
	err := r.db.QueryRowContext(ctx, query, textHash).Scan(&embeddingBytes)
	if err != nil {
		return nil, false // Cache miss
	}

	// Deserialize embedding (stored as float32 array in pgvector format)
	var embedding []float32
	if err := json.Unmarshal(embeddingBytes, &embedding); err != nil {
		logger.Warn("failed to deserialize cached embedding from Postgres", zap.Error(err))
		return nil, false
	}

	return embedding, true
}

// Set stores embedding in Postgres repository
func (r *Repository) Set(ctx context.Context, textHash string, embedding []float32, model string, textLength int) error {
	// Serialize embedding
	embeddingBytes, err := json.Marshal(embedding)
	if err != nil {
		return fmt.Errorf("failed to serialize embedding: %w", err)
	}

	query := `
		INSERT INTO embedding_cache (text_hash, embedding, model, text_length, created_at, last_used_at, use_count)
		VALUES ($1, $2, $3, $4, NOW(), NOW(), 1)
		ON CONFLICT (text_hash) DO UPDATE SET
			last_used_at = NOW(),
			use_count = embedding_cache.use_count + 1
	`

	_, err = r.db.ExecContext(ctx, query, textHash, embeddingBytes, model, textLength)
	if err != nil {
		return fmt.Errorf("failed to cache embedding in Postgres: %w", err)
	}

	return nil
}

// GetStats returns repository statistics
func (r *Repository) GetStats(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM embedding_cache`
	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get cache stats: %w", err)
	}
	return count, nil
}
