-- News cache for AI agents with semantic embeddings

-- Enable pgvector extension if not already enabled (from semantic_memory migration)
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS news_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source VARCHAR(50) NOT NULL,
    title TEXT NOT NULL,
    content TEXT,
    url TEXT,
    author VARCHAR(200),
    published_at TIMESTAMP NOT NULL,
    sentiment DECIMAL(5, 4) DEFAULT 0,  -- -1.0 to 1.0
    relevance DECIMAL(5, 4) DEFAULT 0,  -- 0.0 to 1.0
    impact INTEGER DEFAULT 5,            -- 1-10
    urgency VARCHAR(20),                 -- IMMEDIATE, HOURS, DAYS
    keywords TEXT[],
    embedding vector(1536),              -- Semantic embedding for search (OpenAI Ada v2 compatible)
    embedding_model VARCHAR(50),         -- Track which model generated embedding
    related_news_ids UUID[],             -- Clustered related news (deduplication)
    cluster_id UUID,                     -- News cluster identifier (same event = same cluster)
    is_cluster_primary BOOLEAN DEFAULT true, -- Primary news in cluster (best quality/source)
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(source, url)
);

-- Standard indexes
CREATE INDEX idx_news_items_source ON news_items(source);
CREATE INDEX idx_news_items_published_at ON news_items(published_at DESC);
CREATE INDEX idx_news_items_sentiment ON news_items(sentiment);
CREATE INDEX idx_news_items_impact ON news_items(impact DESC);

-- Vector similarity search index (IVFFlat for fast approximate search)
CREATE INDEX idx_news_items_embedding ON news_items USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);

-- Cluster-based indexes for deduplication
CREATE INDEX idx_news_items_cluster_id ON news_items(cluster_id) WHERE cluster_id IS NOT NULL;
CREATE INDEX idx_news_items_cluster_primary ON news_items(cluster_id, is_cluster_primary) 
    WHERE is_cluster_primary = true;

COMMENT ON TABLE news_items IS 'Cached news articles with AI-evaluated sentiment and semantic embeddings for agents';
COMMENT ON COLUMN news_items.embedding IS 'Semantic embedding (1536d) for similarity search - finds related news by meaning';
COMMENT ON COLUMN news_items.cluster_id IS 'Groups related news about same event for deduplication';
COMMENT ON COLUMN news_items.is_cluster_primary IS 'Primary/canonical news in cluster - highest quality source';

