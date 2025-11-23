package news

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

const redditAPIURL = "https://www.reddit.com/r/%s/hot.json?limit=%d"

// RedditProvider fetches news from Reddit
type RedditProvider struct {
	enabled    bool
	subreddits []string
	client     *http.Client
	sentiment  SentimentAnalyzer
}

// NewRedditProvider creates new Reddit provider
func NewRedditProvider(enabled bool, subreddits []string, sentiment SentimentAnalyzer) *RedditProvider {
	if len(subreddits) == 0 {
		subreddits = []string{"CryptoCurrency", "Bitcoin", "ethereum"}
	}

	return &RedditProvider{
		enabled:    enabled,
		subreddits: subreddits,
		client:     &http.Client{Timeout: 10 * time.Second},
		sentiment:  sentiment,
	}
}

func (r *RedditProvider) GetName() string {
	return "reddit"
}

func (r *RedditProvider) IsEnabled() bool {
	return r.enabled
}

func (r *RedditProvider) FetchLatestNews(ctx context.Context, keywords []string, limit int) ([]models.NewsItem, error) {
	if !r.enabled {
		return nil, nil
	}

	allPosts := make([]models.NewsItem, 0)

	// Fetch from each subreddit
	for _, subreddit := range r.subreddits {
		posts, err := r.fetchSubreddit(ctx, subreddit, limit/len(r.subreddits))
		if err != nil {
			logger.Warn("failed to fetch reddit posts",
				zap.String("subreddit", subreddit),
				zap.Error(err),
			)
			continue
		}

		// Filter by keywords
		for _, post := range posts {
			if r.isRelevant(post.Title+" "+post.Content, keywords) {
				allPosts = append(allPosts, post)
			}
		}
	}

	logger.Debug("fetched Reddit posts",
		zap.Int("count", len(allPosts)),
		zap.Strings("subreddits", r.subreddits),
	)

	return allPosts, nil
}

// fetchSubreddit fetches posts from specific subreddit
func (r *RedditProvider) fetchSubreddit(ctx context.Context, subreddit string, limit int) ([]models.NewsItem, error) {
	url := fmt.Sprintf(redditAPIURL, subreddit, limit)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set User-Agent (Reddit requires it)
	req.Header.Set("User-Agent", "TradingBot/1.0")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data struct {
			Children []struct {
				Data struct {
					ID          string  `json:"id"`
					Title       string  `json:"title"`
					Selftext    string  `json:"selftext"`
					URL         string  `json:"url"`
					Author      string  `json:"author"`
					CreatedUTC  float64 `json:"created_utc"`
					Score       int     `json:"score"`
					NumComments int     `json:"num_comments"`
					Upvote      int     `json:"ups"`
				} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	posts := make([]models.NewsItem, 0)
	for _, child := range result.Data.Children {
		post := child.Data

		// Analyze sentiment
		sentiment := r.sentiment.AnalyzeSentiment(post.Title + " " + post.Selftext)

		// Calculate relevance based on engagement
		relevance := r.calculateRelevance(post.Score, post.NumComments)

		posts = append(posts, models.NewsItem{
			ID:          fmt.Sprintf("reddit_%s", post.ID),
			Source:      "reddit",
			Title:       post.Title,
			Content:     truncate(post.Selftext, 500),
			URL:         fmt.Sprintf("https://reddit.com/r/%s/comments/%s", subreddit, post.ID),
			Author:      "u/" + post.Author,
			PublishedAt: time.Unix(int64(post.CreatedUTC), 0),
			Sentiment:   sentiment,
			Relevance:   relevance,
			Keywords:    []string{}, // Will be filled by aggregator
		})
	}

	return posts, nil
}

// calculateRelevance calculates relevance from Reddit engagement
func (r *RedditProvider) calculateRelevance(score, comments int) float64 {
	// Combine upvotes and comments
	engagementScore := float64(score + comments*2)

	// Normalize (500+ engagement = 1.0)
	relevance := engagementScore / 500.0
	if relevance > 1.0 {
		relevance = 1.0
	}

	return relevance
}

// isRelevant checks if post is relevant to keywords
func (r *RedditProvider) isRelevant(text string, keywords []string) bool {
	if len(keywords) == 0 {
		return true
	}

	lowerText := strings.ToLower(text)
	for _, keyword := range keywords {
		if strings.Contains(lowerText, strings.ToLower(keyword)) {
			return true
		}
	}

	return false
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
