-- Embedding repository for deduplication (NOT a cache!)
-- Embeddings are:
--   1. Deterministic: same text → same embedding
--   2. Expensive: ~$0.0001 per call → ~$3/month for 1000 news/day
--   3. Permanent: we store them to avoid redundant API calls
-- This table saves ~50% API costs through deduplication

CREATE TABLE IF NOT EXISTS embedding_cache (
    text_hash VARCHAR(64) PRIMARY KEY,  -- SHA256 hash of input text
    embedding vector(1536) NOT NULL,    -- OpenAI Ada-002 embedding (1536 dimensions)
    model VARCHAR(50) NOT NULL,         -- Embedding model used (e.g., "ada-002")
    text_length INTEGER NOT NULL,       -- Original text length (for metrics)
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMP NOT NULL DEFAULT NOW(),
    use_count INTEGER DEFAULT 1         -- Track deduplication hits
);

-- Index for cleanup queries (remove old unused embeddings)
CREATE INDEX idx_embedding_cache_last_used ON embedding_cache(last_used_at);
CREATE INDEX idx_embedding_cache_model ON embedding_cache(model);

COMMENT ON TABLE embedding_cache IS 'Embedding repository for deduplication (NOT a cache) - saves API costs by storing deterministic embeddings permanently';
COMMENT ON COLUMN embedding_cache.text_hash IS 'SHA256 hash of input text for deduplication';
COMMENT ON COLUMN embedding_cache.use_count IS 'Number of times this embedding was reused (deduplication hits)';

-- Full HNSW index for all news (not just 7 days)
-- Smaller m/ef_construction for balance between speed and memory
-- Use this for queries spanning > 7 days
CREATE INDEX IF NOT EXISTS idx_news_items_embedding_full ON news_items 
USING hnsw (embedding vector_cosine_ops)
WITH (m = 8, ef_construction = 32)
WHERE embedding IS NOT NULL;

COMMENT ON INDEX idx_news_items_embedding_full IS 'Full HNSW index for semantic search across all news history (complements 7-day partial index)';

