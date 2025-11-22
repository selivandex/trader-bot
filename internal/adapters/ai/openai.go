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

const openaiAPIURL = "https://api.openai.com/v1/chat/completions"

// OpenAIProvider implements AI provider for OpenAI
type OpenAIProvider struct {
	apiKey  string
	enabled bool
	client  *http.Client
}

// NewOpenAIProvider creates new OpenAI provider
func NewOpenAIProvider(cfg *config.AIProviderConfig) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey:  cfg.APIKey,
		enabled: cfg.Enabled && cfg.APIKey != "",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (o *OpenAIProvider) GetName() string {
	return "openai"
}

func (o *OpenAIProvider) GetCost() float64 {
	// GPT-4 pricing: ~$10 per 1M input tokens, ~$30 per 1M output tokens
	// Average request: ~4K input, ~500 output = ~$0.055
	return 0.055
}

func (o *OpenAIProvider) IsEnabled() bool {
	return o.enabled
}

func (o *OpenAIProvider) Analyze(ctx context.Context, prompt *models.TradingPrompt) (*models.AIDecision, error) {
	systemPrompt := buildSystemPrompt()
	userPrompt := buildUserPrompt(prompt)

	reqBody := map[string]interface{}{
		"model": "gpt-4-turbo-preview",
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

	req, err := http.NewRequestWithContext(ctx, "POST", openaiAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", o.apiKey))

	startTime := time.Now()
	resp, err := o.client.Do(req)
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

	logger.Debug("OpenAI response",
		zap.Duration("latency", latency),
		zap.String("response", content),
	)

	// Parse JSON decision from response
	decision, err := parseAIResponse(content, "openai")
	if err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return decision, nil
}
