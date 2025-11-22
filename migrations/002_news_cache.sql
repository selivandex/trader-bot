-- News caching table

CREATE TABLE IF NOT EXISTS news_items (
    id SERIAL PRIMARY KEY,
    external_id VARCHAR(255) UNIQUE NOT NULL,
    source VARCHAR(50) NOT NULL, -- twitter, reddit, coindesk
    title TEXT NOT NULL,
    content TEXT,
    url TEXT,
    author VARCHAR(255),
    published_at TIMESTAMP NOT NULL,
    sentiment DECIMAL(5, 4) NOT NULL, -- -1.0 to 1.0
    relevance DECIMAL(5, 4) NOT NULL DEFAULT 0, -- 0.0 to 1.0
    keywords TEXT[], -- Array of matched keywords
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_news_source ON news_items(source);
CREATE INDEX IF NOT EXISTS idx_news_published_at ON news_items(published_at DESC);
CREATE INDEX IF NOT EXISTS idx_news_sentiment ON news_items(sentiment);
CREATE INDEX IF NOT EXISTS idx_news_source_published ON news_items(source, published_at DESC);

-- View for recent relevant news
CREATE OR REPLACE VIEW recent_news AS
SELECT 
    id,
    source,
    title,
    sentiment,
    relevance,
    published_at,
    age_hours,
    CASE 
        WHEN sentiment > 0.2 THEN 'bullish'
        WHEN sentiment < -0.2 THEN 'bearish'
        ELSE 'neutral'
    END as sentiment_label
FROM (
    SELECT 
        *,
        EXTRACT(EPOCH FROM (NOW() - published_at)) / 3600 as age_hours
    FROM news_items
    WHERE published_at > NOW() - INTERVAL '24 hours'
) subq
WHERE age_hours < 24
ORDER BY published_at DESC, relevance DESC
LIMIT 50;

-- Cleanup function for old news (keep last 7 days)
CREATE OR REPLACE FUNCTION cleanup_old_news()
RETURNS void AS $$
BEGIN
    DELETE FROM news_items
    WHERE published_at < NOW() - INTERVAL '7 days';
END;
$$ LANGUAGE plpgsql;

-- Comment
COMMENT ON TABLE news_items IS 'Cached news items from various sources with sentiment analysis';

