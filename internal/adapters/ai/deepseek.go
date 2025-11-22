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

const deepseekAPIURL = "https://api.deepseek.com/v1/chat/completions"

// DeepSeekProvider implements AI provider for DeepSeek
type DeepSeekProvider struct {
	apiKey  string
	enabled bool
	client  *http.Client
}

// NewDeepSeekProvider creates new DeepSeek provider
func NewDeepSeekProvider(cfg *config.AIProviderConfig) *DeepSeekProvider {
	return &DeepSeekProvider{
		apiKey:  cfg.APIKey,
		enabled: cfg.Enabled && cfg.APIKey != "",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (d *DeepSeekProvider) GetName() string {
	return "deepseek"
}

func (d *DeepSeekProvider) GetCost() float64 {
	// DeepSeek pricing: ~$0.14 per 1M input tokens, ~$0.28 per 1M output tokens
	// Average request: ~4K input, ~500 output = ~$0.0007
	return 0.0007
}

func (d *DeepSeekProvider) IsEnabled() bool {
	return d.enabled
}

func (d *DeepSeekProvider) Analyze(ctx context.Context, prompt *models.TradingPrompt) (*models.AIDecision, error) {
	systemPrompt := buildSystemPrompt()
	userPrompt := buildUserPrompt(prompt)

	reqBody := map[string]interface{}{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.7,
		"max_tokens":  1000,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", deepseekAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", d.apiKey))

	startTime := time.Now()
	resp, err := d.client.Do(req)
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
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	content := result.Choices[0].Message.Content

	logger.Debug("DeepSeek response",
		zap.Duration("latency", latency),
		zap.String("response", content),
	)

	// Parse JSON decision from response
	decision, err := parseAIResponse(content, "deepseek")
	if err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return decision, nil
}
