-- Rollback embedding cache and full HNSW index

DROP INDEX IF EXISTS idx_news_items_embedding_full;
DROP TABLE IF EXISTS embedding_cache;

