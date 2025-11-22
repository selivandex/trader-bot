package news

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/pkg/logger"
	"github.com/alexanderselivanov/trader/pkg/models"
)

const twitterAPIURL = "https://api.twitter.com/2/tweets/search/recent"

// TwitterProvider fetches news from Twitter (X)
type TwitterProvider struct {
	apiKey    string
	enabled   bool
	client    *http.Client
	sentiment SentimentAnalyzer
}

// SentimentAnalyzer analyzes text sentiment
type SentimentAnalyzer interface {
	AnalyzeSentiment(text string) float64
}

// NewTwitterProvider creates new Twitter provider
func NewTwitterProvider(apiKey string, enabled bool, sentiment SentimentAnalyzer) *TwitterProvider {
	return &TwitterProvider{
		apiKey:    apiKey,
		enabled:   enabled && apiKey != "",
		client:    &http.Client{Timeout: 10 * time.Second},
		sentiment: sentiment,
	}
}

func (t *TwitterProvider) GetName() string {
	return "twitter"
}

func (t *TwitterProvider) IsEnabled() bool {
	return t.enabled
}

func (t *TwitterProvider) FetchLatestNews(ctx context.Context, keywords []string, limit int) ([]models.NewsItem, error) {
	if !t.enabled {
		return nil, nil
	}

	// Build search query
	query := strings.Join(keywords, " OR ")
	query += " -is:retweet lang:en" // Exclude retweets, English only

	params := url.Values{}
	params.Add("query", query)
	params.Add("max_results", fmt.Sprintf("%d", min(limit, 100)))
	params.Add("tweet.fields", "created_at,author_id,public_metrics")
	params.Add("expansions", "author_id")
	params.Add("user.fields", "username")

	reqURL := fmt.Sprintf("%s?%s", twitterAPIURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.apiKey))

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID            string    `json:"id"`
			Text          string    `json:"text"`
			AuthorID      string    `json:"author_id"`
			CreatedAt     time.Time `json:"created_at"`
			PublicMetrics struct {
				LikeCount    int `json:"like_count"`
				RetweetCount int `json:"retweet_count"`
			} `json:"public_metrics"`
		} `json:"data"`
		Includes struct {
			Users []struct {
				ID       string `json:"id"`
				Username string `json:"username"`
			} `json:"users"`
		} `json:"includes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Map user IDs to usernames
	userMap := make(map[string]string)
	for _, user := range result.Includes.Users {
		userMap[user.ID] = user.Username
	}

	news := make([]models.NewsItem, 0, len(result.Data))
	for _, tweet := range result.Data {
		sentiment := t.sentiment.AnalyzeSentiment(tweet.Text)

		// Calculate relevance based on engagement
		relevance := calculateRelevance(
			tweet.PublicMetrics.LikeCount,
			tweet.PublicMetrics.RetweetCount,
		)

		news = append(news, models.NewsItem{
			ID:          tweet.ID,
			Source:      "twitter",
			Title:       truncate(tweet.Text, 100),
			Content:     tweet.Text,
			URL:         fmt.Sprintf("https://twitter.com/%s/status/%s", userMap[tweet.AuthorID], tweet.ID),
			Author:      "@" + userMap[tweet.AuthorID],
			PublishedAt: tweet.CreatedAt,
			Sentiment:   sentiment,
			Relevance:   relevance,
			Keywords:    keywords,
		})
	}

	logger.Debug("fetched Twitter news",
		zap.Int("count", len(news)),
		zap.String("query", query),
	)

	return news, nil
}

func calculateRelevance(likes, retweets int) float64 {
	// Simple engagement-based relevance score
	score := float64(likes + retweets*2)

	// Normalize to 0-1 range (tweets with 1000+ engagement = 1.0)
	relevance := score / 1000.0
	if relevance > 1.0 {
		relevance = 1.0
	}

	return relevance
}

// truncate moved to reddit.go to avoid duplication

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
