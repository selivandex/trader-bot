package news

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/selivandex/trader-bot/pkg/models"
)

// Repository handles database operations for news
type Repository struct {
	db *sqlx.DB
}

// NewRepository creates new news repository
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// SaveNewsItems saves news items to database (upsert)
// Now includes embeddings, impact, urgency, and clustering metadata
func (r *Repository) SaveNewsItems(ctx context.Context, news []models.NewsItem) (int, error) {
	if len(news) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO news_items (
			id, source, title, content, url, author,
			published_at, sentiment, relevance, impact, urgency, keywords,
			embedding, embedding_model, cluster_id, is_cluster_primary, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (source, url) DO UPDATE SET
			sentiment = EXCLUDED.sentiment,
			relevance = EXCLUDED.relevance,
			impact = EXCLUDED.impact,
			urgency = EXCLUDED.urgency,
			embedding = COALESCE(EXCLUDED.embedding, news_items.embedding),
			embedding_model = COALESCE(EXCLUDED.embedding_model, news_items.embedding_model),
			cluster_id = COALESCE(EXCLUDED.cluster_id, news_items.cluster_id),
			is_cluster_primary = COALESCE(EXCLUDED.is_cluster_primary, news_items.is_cluster_primary)
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	saved := 0
	for _, item := range news {
		// Convert embedding to pgvector format
		var embeddingStr interface{}
		if len(item.Embedding) > 0 {
			embeddingStr = pq.Array(item.Embedding)
		}

		_, err := stmt.ExecContext(ctx,
			item.ID,
			item.Source,
			item.Title,
			item.Content,
			item.URL,
			item.Author,
			item.PublishedAt,
			item.Sentiment,
			item.Relevance,
			item.Impact,
			item.Urgency,
			pq.Array(item.Keywords),
			embeddingStr,
			item.EmbeddingModel,
			item.ClusterID,
			item.IsClusterPrimary,
			time.Now(),
		)

		if err == nil {
			saved++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit: %w", err)
	}

	return saved, nil
}

// GetRecentNews gets recent news from database
func (r *Repository) GetRecentNews(ctx context.Context, since time.Duration, limit int) ([]models.NewsItem, error) {
	cutoff := time.Now().Add(-since)

	query := `
		SELECT 
			external_id, source, title, content, url, author,
			published_at, sentiment, relevance, keywords
		FROM news_items
		WHERE published_at > $1
		ORDER BY published_at DESC, relevance DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query news: %w", err)
	}
	defer rows.Close()

	news := make([]models.NewsItem, 0)
	for rows.Next() {
		var item models.NewsItem
		var keywords pq.StringArray

		err := rows.Scan(
			&item.ID,
			&item.Source,
			&item.Title,
			&item.Content,
			&item.URL,
			&item.Author,
			&item.PublishedAt,
			&item.Sentiment,
			&item.Relevance,
			&keywords,
		)

		if err != nil {
			continue
		}

		item.Keywords = keywords
		news = append(news, item)
	}

	return news, nil
}

// GetHighImpactNews fetches high impact news items
func (r *Repository) GetHighImpactNews(ctx context.Context, minImpact int, since time.Duration, limit int) ([]models.NewsItem, error) {
	cutoff := time.Now().Add(-since)

	query := `
		SELECT id, source, title, content, url, author, published_at, sentiment, relevance, impact, urgency
		FROM news_items
		WHERE impact >= $1 AND published_at > $2
		ORDER BY impact DESC, published_at DESC
		LIMIT $3
	`

	rows, err := r.db.QueryContext(ctx, query, minImpact, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query high impact news: %w", err)
	}
	defer rows.Close()

	news := make([]models.NewsItem, 0)
	for rows.Next() {
		var item models.NewsItem
		if err := rows.Scan(
			&item.ID, &item.Source, &item.Title, &item.Content,
			&item.URL, &item.Author, &item.PublishedAt,
			&item.Sentiment, &item.Relevance, &item.Impact, &item.Urgency,
		); err == nil {
			news = append(news, item)
		}
	}

	return news, nil
}

// CleanupOldNews removes old news (older than specified duration)
func (r *Repository) CleanupOldNews(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	query := `
		DELETE FROM news_items
		WHERE published_at < $1
	`

	result, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old news: %w", err)
	}

	deleted, _ := result.RowsAffected()
	return deleted, nil
}

// GetSentimentSummary gets aggregated sentiment from database
func (r *Repository) GetSentimentSummary(ctx context.Context, since time.Duration) (*models.NewsSummary, error) {
	cutoff := time.Now().Add(-since)

	row := r.db.QueryRowContext(ctx, `
		SELECT 
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE sentiment > 0.2) as positive,
			COUNT(*) FILTER (WHERE sentiment < -0.2) as negative,
			COUNT(*) FILTER (WHERE sentiment BETWEEN -0.2 AND 0.2) as neutral,
			COALESCE(AVG(sentiment), 0) as avg_sentiment
		FROM news_items
		WHERE published_at > $1
	`, cutoff)

	var total, positive, negative, neutral int
	var avgSentiment float64

	if err := row.Scan(&total, &positive, &negative, &neutral, &avgSentiment); err != nil {
		return nil, fmt.Errorf("failed to get sentiment summary: %w", err)
	}

	// Get top 5 recent items
	recentNews, err := r.GetRecentNews(ctx, since, 5)
	if err != nil {
		return nil, err
	}

	summary := &models.NewsSummary{
		TotalItems:       total,
		PositiveCount:    positive,
		NegativeCount:    negative,
		NeutralCount:     neutral,
		AverageSentiment: avgSentiment,
		RecentNews:       recentNews,
		UpdatedAt:        time.Now(),
	}

	summary.OverallSentiment = summary.GetOverallSentiment()

	return summary, nil
}

// GetWeightedSentiment calculates impact-weighted sentiment
func (r *Repository) GetWeightedSentiment(ctx context.Context, since time.Duration) (*WeightedSentiment, error) {
	cutoff := time.Now().Add(-since)

	row := r.db.QueryRowContext(ctx, `
		SELECT 
			COALESCE(SUM(CASE WHEN sentiment > 0 THEN sentiment * impact * 10 ELSE 0 END), 0) as bullish_score,
			COALESCE(SUM(CASE WHEN sentiment < 0 THEN ABS(sentiment) * impact * 10 ELSE 0 END), 0) as bearish_score,
			COALESCE(SUM(sentiment * impact), 0) as net_sentiment,
			COUNT(*) as news_count,
			COUNT(*) FILTER (WHERE impact >= 7) as high_impact_count,
			COALESCE(AVG(sentiment), 0) as avg_sentiment
		FROM news_items
		WHERE published_at > $1
	`, cutoff)

	var ws WeightedSentiment
	err := row.Scan(
		&ws.BullishScore,
		&ws.BearishScore,
		&ws.NetSentiment,
		&ws.NewsCount,
		&ws.HighImpactCount,
		&ws.AverageSentiment,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get weighted sentiment: %w", err)
	}

	return &ws, nil
}

// WeightedSentiment represents impact-weighted sentiment metrics
type WeightedSentiment struct {
	BullishScore     float64 `db:"bullish_score"`
	BearishScore     float64 `db:"bearish_score"`
	NetSentiment     float64 `db:"net_sentiment"`
	NewsCount        int     `db:"news_count"`
	HighImpactCount  int     `db:"high_impact_count"`
	AverageSentiment float64 `db:"avg_sentiment"`
}

// GetSentimentTimeSeries gets sentiment averages for different time windows
func (r *Repository) GetSentimentTimeSeries(ctx context.Context) (*SentimentTimeSeries, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT 
			COALESCE(AVG(sentiment) FILTER (WHERE published_at > NOW() - INTERVAL '1 hour'), 0),
			COALESCE(AVG(sentiment) FILTER (WHERE published_at > NOW() - INTERVAL '6 hours'), 0),
			COALESCE(AVG(sentiment) FILTER (WHERE published_at > NOW() - INTERVAL '24 hours'), 0)
		FROM news_items
	`)

	var ts SentimentTimeSeries
	if err := row.Scan(&ts.LastHour, &ts.Last6Hours, &ts.Last24Hours); err != nil {
		return nil, fmt.Errorf("failed to get sentiment time series: %w", err)
	}

	return &ts, nil
}

// SentimentTimeSeries represents sentiment across different time windows
type SentimentTimeSeries struct {
	LastHour    float64 `db:"last_hour"`
	Last6Hours  float64 `db:"last_6hours"`
	Last24Hours float64 `db:"last_24hours"`
}

// ============ SEMANTIC SEARCH & CLUSTERING ============

// SearchNewsByVector performs semantic search using embedding similarity
// Returns news items ranked by cosine similarity to query embedding
func (r *Repository) SearchNewsByVector(
	ctx context.Context,
	queryEmbedding []float32,
	since time.Duration,
	limit int,
) ([]models.NewsItem, error) {
	cutoff := time.Now().Add(-since)

	// pgvector cosine distance search (1 - cosine_similarity)
	// Lower distance = more similar
	query := `
		SELECT 
			id, source, title, content, url, author, published_at,
			sentiment, relevance, impact, urgency, keywords,
			embedding, embedding_model, cluster_id, is_cluster_primary,
			1 - (embedding <=> $1) as similarity_score
		FROM news_items
		WHERE published_at > $2
			AND embedding IS NOT NULL
		ORDER BY embedding <=> $1
		LIMIT $3
	`

	rows, err := r.db.QueryContext(ctx, query, pq.Array(queryEmbedding), cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to vector search news: %w", err)
	}
	defer rows.Close()

	news := make([]models.NewsItem, 0)
	for rows.Next() {
		var item models.NewsItem
		var keywords pq.StringArray
		var embedding pq.Float64Array
		var embeddingModel, clusterID *string

		err := rows.Scan(
			&item.ID, &item.Source, &item.Title, &item.Content,
			&item.URL, &item.Author, &item.PublishedAt,
			&item.Sentiment, &item.Relevance, &item.Impact, &item.Urgency,
			&keywords, &embedding, &embeddingModel, &clusterID, &item.IsClusterPrimary,
			&item.SimilarityScore,
		)

		if err != nil {
			continue
		}

		item.Keywords = keywords
		if embeddingModel != nil {
			item.EmbeddingModel = *embeddingModel
		}
		if clusterID != nil {
			item.ClusterID = clusterID
		}

		// Convert embedding
		if len(embedding) > 0 {
			item.Embedding = make([]float32, len(embedding))
			for i, v := range embedding {
				item.Embedding[i] = float32(v)
			}
		}

		news = append(news, item)
	}

	return news, nil
}

// FindSimilarNews finds news similar to given embedding (for clustering)
func (r *Repository) FindSimilarNews(
	ctx context.Context,
	embedding []float32,
	similarityThreshold float64,
	timeWindow time.Duration,
	limit int,
) ([]models.NewsItem, error) {
	cutoff := time.Now().Add(-timeWindow)

	// Find news with similarity > threshold (cosine distance < 1 - threshold)
	query := `
		SELECT 
			id, source, title, published_at, impact, cluster_id, is_cluster_primary,
			1 - (embedding <=> $1) as similarity_score
		FROM news_items
		WHERE published_at > $2
			AND embedding IS NOT NULL
			AND (1 - (embedding <=> $1)) > $3
		ORDER BY embedding <=> $1
		LIMIT $4
	`

	rows, err := r.db.QueryContext(
		ctx, query,
		pq.Array(embedding),
		cutoff,
		similarityThreshold,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find similar news: %w", err)
	}
	defer rows.Close()

	news := make([]models.NewsItem, 0)
	for rows.Next() {
		var item models.NewsItem
		var clusterID *string

		err := rows.Scan(
			&item.ID, &item.Source, &item.Title, &item.PublishedAt,
			&item.Impact, &clusterID, &item.IsClusterPrimary,
			&item.SimilarityScore,
		)

		if err != nil {
			continue
		}

		if clusterID != nil {
			item.ClusterID = clusterID
		}

		news = append(news, item)
	}

	return news, nil
}

// UpdateNewsCluster assigns news item to a cluster
func (r *Repository) UpdateNewsCluster(ctx context.Context, newsID string, clusterID string, isPrimary bool) error {
	query := `
		UPDATE news_items
		SET cluster_id = $2, is_cluster_primary = $3
		WHERE id = $1
	`

	_, err := r.db.ExecContext(ctx, query, newsID, clusterID, isPrimary)
	if err != nil {
		return fmt.Errorf("failed to update news cluster: %w", err)
	}

	return nil
}

// GetClusterNews retrieves all news in a cluster
func (r *Repository) GetClusterNews(ctx context.Context, clusterID string) ([]models.NewsItem, error) {
	query := `
		SELECT 
			id, source, title, content, url, author, published_at,
			sentiment, relevance, impact, urgency, keywords,
			cluster_id, is_cluster_primary
		FROM news_items
		WHERE cluster_id = $1
		ORDER BY is_cluster_primary DESC, impact DESC, published_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster news: %w", err)
	}
	defer rows.Close()

	news := make([]models.NewsItem, 0)
	for rows.Next() {
		var item models.NewsItem
		var keywords pq.StringArray
		var clusterID *string

		err := rows.Scan(
			&item.ID, &item.Source, &item.Title, &item.Content,
			&item.URL, &item.Author, &item.PublishedAt,
			&item.Sentiment, &item.Relevance, &item.Impact, &item.Urgency,
			&keywords, &clusterID, &item.IsClusterPrimary,
		)

		if err != nil {
			continue
		}

		item.Keywords = keywords
		if clusterID != nil {
			item.ClusterID = clusterID
		}

		news = append(news, item)
	}

	return news, nil
}

// LinkRelatedNews updates related_news_ids array for cross-referencing
func (r *Repository) LinkRelatedNews(ctx context.Context, newsID string, relatedIDs []string) error {
	query := `
		UPDATE news_items
		SET related_news_ids = $2
		WHERE id = $1
	`

	_, err := r.db.ExecContext(ctx, query, newsID, pq.Array(relatedIDs))
	if err != nil {
		return fmt.Errorf("failed to link related news: %w", err)
	}

	return nil
}
