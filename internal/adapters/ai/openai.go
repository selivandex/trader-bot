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

	"github.com/selivandex/trader-bot/internal/adapters/config"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

const openaiAPIURL = "https://api.openai.com/v1/chat/completions"

// OpenAIProvider implements AI provider for OpenAI GPT
type OpenAIProvider struct {
	apiKey         string
	enabled        bool
	client         *http.Client
	strategyParams *models.StrategyParameters
}

// NewOpenAIProvider creates new OpenAI provider
func NewOpenAIProvider(cfg *config.AIProviderConfig, params *models.StrategyParameters) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey:         cfg.APIKey,
		enabled:        cfg.Enabled && cfg.APIKey != "",
		strategyParams: params,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (o *OpenAIProvider) GetName() string {
	return "GPT"
}

func (o *OpenAIProvider) GetCost() float64 {
	// GPT-4 Turbo pricing: ~$10 per 1M input tokens, ~$30 per 1M output tokens
	// Average request: ~4K input, ~500 output = ~$0.06
	return 0.06
}

func (o *OpenAIProvider) IsEnabled() bool {
	return o.enabled
}

func (o *OpenAIProvider) Analyze(ctx context.Context, prompt *models.TradingPrompt) (*models.AIDecision, error) {
	systemPrompt := buildSystemPrompt(o.strategyParams)
	userPrompt := buildUserPrompt(prompt)

	reqBody := map[string]interface{}{
		"model": "gpt-4-turbo-preview",
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"max_tokens":  1000,
		"temperature": 0.7,
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
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

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
		return nil, fmt.Errorf("empty response from OpenAI")
	}

	responseText := result.Choices[0].Message.Content

	decision, err := parseAIResponse(responseText, "GPT")
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI response: %w", err)
	}

	logger.Info("OpenAI GPT analysis complete",
		zap.String("action", string(decision.Action)),
		zap.Int("confidence", decision.Confidence),
		zap.Duration("latency", latency),
	)

	return decision, nil
}

func (o *OpenAIProvider) EvaluateNews(ctx context.Context, newsItem *models.NewsItem) error {
	systemPrompt := buildNewsEvaluationSystemPrompt()
	userPrompt := buildNewsEvaluationUserPrompt(newsItem)

	reqBody := map[string]interface{}{
		"model": "gpt-4-turbo-preview",
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"max_tokens":  300,
		"temperature": 0.5,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", openaiAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	startTime := time.Now()
	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	latency := time.Since(startTime)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return fmt.Errorf("empty response")
	}

	responseText := result.Choices[0].Message.Content

	var evaluation NewsEvaluation
	if err := json.Unmarshal([]byte(responseText), &evaluation); err != nil {
		return fmt.Errorf("failed to parse news evaluation: %w", err)
	}

	newsItem.Sentiment = evaluation.Sentiment
	newsItem.Impact = evaluation.Impact
	newsItem.Urgency = evaluation.Urgency

	logger.Debug("news evaluated by OpenAI GPT-4",
		zap.String("title", newsItem.Title),
		zap.Float64("sentiment", evaluation.Sentiment),
		zap.Int("impact", evaluation.Impact),
		zap.String("urgency", evaluation.Urgency),
	)

	logger.Info("news evaluation complete",
		zap.String("provider", "openai"),
		zap.Duration("latency", latency),
	)

	return nil
}

// ========== Agentic AI Methods ==========

func (o *OpenAIProvider) Reflect(ctx context.Context, reflectionPrompt *models.ReflectionPrompt) (*models.Reflection, error) {
	systemPrompt, userPrompt := BuildReflectionPrompt(reflectionPrompt)
	responseText, err := o.callOpenAIAPI(ctx, systemPrompt, userPrompt, 1500)
	if err != nil {
		return nil, err
	}
	var reflection models.Reflection
	if err := json.Unmarshal([]byte(responseText), &reflection); err != nil {
		return nil, fmt.Errorf("failed to parse reflection: %w", err)
	}
	return &reflection, nil
}

func (o *OpenAIProvider) GenerateOptions(ctx context.Context, situation *models.TradingSituation) ([]models.TradingOption, error) {
	systemPrompt, userPrompt := BuildGenerateOptionsPrompt(situation)
	responseText, err := o.callOpenAIAPI(ctx, systemPrompt, userPrompt, 2000)
	if err != nil {
		return nil, err
	}
	var options []models.TradingOption
	if err := json.Unmarshal([]byte(responseText), &options); err != nil {
		return nil, fmt.Errorf("failed to parse options: %w", err)
	}
	return options, nil
}

func (o *OpenAIProvider) EvaluateOption(ctx context.Context, option *models.TradingOption, memories []models.SemanticMemory) (*models.OptionEvaluation, error) {
	systemPrompt, userPrompt := BuildEvaluateOptionPrompt(option, memories)
	responseText, err := o.callOpenAIAPI(ctx, systemPrompt, userPrompt, 1500)
	if err != nil {
		return nil, err
	}
	var evaluation models.OptionEvaluation
	if err := json.Unmarshal([]byte(responseText), &evaluation); err != nil {
		return nil, fmt.Errorf("failed to parse evaluation: %w", err)
	}
	return &evaluation, nil
}

func (o *OpenAIProvider) MakeFinalDecision(ctx context.Context, evaluations []models.OptionEvaluation) (*models.AIDecision, error) {
	systemPrompt, userPrompt := BuildFinalDecisionPrompt(evaluations)
	responseText, err := o.callOpenAIAPI(ctx, systemPrompt, userPrompt, 1000)
	if err != nil {
		return nil, err
	}
	var decision models.AIDecision
	decision.Provider = "GPT"
	if err := json.Unmarshal([]byte(responseText), &decision); err != nil {
		return nil, fmt.Errorf("failed to parse decision: %w", err)
	}
	return &decision, nil
}

func (o *OpenAIProvider) CreatePlan(ctx context.Context, planRequest *models.PlanRequest) (*models.TradingPlan, error) {
	systemPrompt, userPrompt := BuildCreatePlanPrompt(planRequest)
	responseText, err := o.callOpenAIAPI(ctx, systemPrompt, userPrompt, 2500)
	if err != nil {
		return nil, err
	}
	var plan models.TradingPlan
	if err := json.Unmarshal([]byte(responseText), &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan: %w", err)
	}
	return &plan, nil
}

func (o *OpenAIProvider) SelfAnalyze(ctx context.Context, performance *models.PerformanceData) (*models.SelfAnalysis, error) {
	systemPrompt, userPrompt := BuildSelfAnalysisPrompt(performance)
	responseText, err := o.callOpenAIAPI(ctx, systemPrompt, userPrompt, 2000)
	if err != nil {
		return nil, err
	}
	var analysis models.SelfAnalysis
	if err := json.Unmarshal([]byte(responseText), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse self-analysis: %w", err)
	}
	return &analysis, nil
}

func (o *OpenAIProvider) FindSimilarMemories(ctx context.Context, currentSituation string, memories []models.SemanticMemory, topK int) ([]models.SemanticMemory, error) {
	if topK > len(memories) {
		topK = len(memories)
	}
	return memories[:topK], nil
}

func (o *OpenAIProvider) SummarizeMemory(ctx context.Context, experience *models.TradeExperience) (*models.MemorySummary, error) {
	systemPrompt, userPrompt := BuildSummarizeMemoryPrompt(experience)
	responseText, err := o.callOpenAIAPI(ctx, systemPrompt, userPrompt, 800)
	if err != nil {
		return nil, err
	}
	var summary models.MemorySummary
	if err := json.Unmarshal([]byte(responseText), &summary); err != nil {
		return nil, fmt.Errorf("failed to parse memory summary: %w", err)
	}
	return &summary, nil
}

func (o *OpenAIProvider) callOpenAIAPI(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	reqBody := map[string]interface{}{
		"model": "gpt-4-turbo-preview",
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"max_tokens":  maxTokens,
		"temperature": 0.7,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", openaiAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty response")
	}

	return result.Choices[0].Message.Content, nil
}
