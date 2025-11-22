package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/internal/adapters/config"
	"github.com/alexanderselivanov/trader/pkg/logger"
	"github.com/alexanderselivanov/trader/pkg/models"
)

const claudeAPIURL = "https://api.anthropic.com/v1/messages"

// ClaudeProvider implements AI provider for Claude
type ClaudeProvider struct {
	apiKey  string
	enabled bool
	client  *http.Client
}

// NewClaudeProvider creates new Claude provider
func NewClaudeProvider(cfg *config.AIProviderConfig) *ClaudeProvider {
	return &ClaudeProvider{
		apiKey:  cfg.APIKey,
		enabled: cfg.Enabled && cfg.APIKey != "",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *ClaudeProvider) GetName() string {
	return "claude"
}

func (c *ClaudeProvider) GetCost() float64 {
	// Claude Sonnet pricing: ~$3 per 1M input tokens, ~$15 per 1M output tokens
	// Average request: ~4K input, ~500 output = ~$0.02
	return 0.02
}

func (c *ClaudeProvider) IsEnabled() bool {
	return c.enabled
}

func (c *ClaudeProvider) Analyze(ctx context.Context, prompt *models.TradingPrompt) (*models.AIDecision, error) {
	systemPrompt := buildSystemPrompt()
	userPrompt := buildUserPrompt(prompt)

	reqBody := map[string]interface{}{
		"model":      "claude-3-5-sonnet-20241022",
		"max_tokens": 1000,
		"system":     systemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", claudeAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	startTime := time.Now()
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	latency := time.Since(startTime)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Content) == 0 {
		return nil, fmt.Errorf("no content in response")
	}

	content := result.Content[0].Text

	logger.Debug("Claude response",
		zap.Duration("latency", latency),
		zap.String("response", content),
	)

	// Parse JSON decision from response
	decision, err := parseAIResponse(content, "claude")
	if err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return decision, nil
}
