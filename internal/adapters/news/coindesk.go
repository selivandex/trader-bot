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

	"github.com/alexanderselivanov/trader/pkg/logger"
	"github.com/alexanderselivanov/trader/pkg/models"
)

const coindeskAPIURL = "https://www.coindesk.com/arc/outboundfeeds/news/?outputType=json&size=%d"

// CoinDeskProvider fetches news from CoinDesk
type CoinDeskProvider struct {
	enabled   bool
	client    *http.Client
	sentiment SentimentAnalyzer
}

// NewCoinDeskProvider creates new CoinDesk provider
func NewCoinDeskProvider(enabled bool, sentiment SentimentAnalyzer) *CoinDeskProvider {
	return &CoinDeskProvider{
		enabled:   enabled,
		client:    &http.Client{Timeout: 10 * time.Second},
		sentiment: sentiment,
	}
}

func (c *CoinDeskProvider) GetName() string {
	return "coindesk"
}

func (c *CoinDeskProvider) IsEnabled() bool {
	return c.enabled
}

func (c *CoinDeskProvider) FetchLatestNews(ctx context.Context, keywords []string, limit int) ([]models.NewsItem, error) {
	if !c.enabled {
		return nil, nil
	}

	url := fmt.Sprintf(coindeskAPIURL, limit)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	var result []struct {
		ID        string `json:"_id"`
		Type      string `json:"type"`
		Canonical string `json:"canonical_url"`
		Headlines struct {
			Basic string `json:"basic"`
		} `json:"headlines"`
		Description struct {
			Basic string `json:"basic"`
		} `json:"description"`
		Credits struct {
			By []struct {
				Name string `json:"name"`
			} `json:"by"`
		} `json:"credits"`
		DisplayDate time.Time `json:"display_date"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	news := make([]models.NewsItem, 0)
	for _, article := range result {
		// Skip non-story types
		if article.Type != "story" {
			continue
		}

		title := article.Headlines.Basic
		description := article.Description.Basic

		// Check relevance
		if !c.isRelevant(title+" "+description, keywords) {
			continue
		}

		// Get author
		author := "CoinDesk"
		if len(article.Credits.By) > 0 {
			author = article.Credits.By[0].Name
		}

		// Analyze sentiment
		sentiment := c.sentiment.AnalyzeSentiment(title + " " + description)

		news = append(news, models.NewsItem{
			ID:          fmt.Sprintf("coindesk_%s", article.ID),
			Source:      "coindesk",
			Title:       title,
			Content:     description,
			URL:         "https://www.coindesk.com" + article.Canonical,
			Author:      author,
			PublishedAt: article.DisplayDate,
			Sentiment:   sentiment,
			Relevance:   0.9, // CoinDesk is highly reliable
			Keywords:    keywords,
		})
	}

	logger.Debug("fetched CoinDesk news",
		zap.Int("count", len(news)),
	)

	return news, nil
}

// isRelevant checks if article is relevant to keywords
func (c *CoinDeskProvider) isRelevant(text string, keywords []string) bool {
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
