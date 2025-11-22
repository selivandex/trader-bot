-- News cache for AI agents

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
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(source, url)
);

CREATE INDEX idx_news_items_source ON news_items(source);
CREATE INDEX idx_news_items_published_at ON news_items(published_at DESC);
CREATE INDEX idx_news_items_sentiment ON news_items(sentiment);
CREATE INDEX idx_news_items_impact ON news_items(impact DESC);

COMMENT ON TABLE news_items IS 'Cached news articles with AI-evaluated sentiment for agents';

