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
	apiKey         string
	enabled        bool
	client         *http.Client
	strategyParams *models.StrategyParameters
}

// NewDeepSeekProvider creates new DeepSeek provider
func NewDeepSeekProvider(cfg *config.AIProviderConfig, params *models.StrategyParameters) *DeepSeekProvider {
	return &DeepSeekProvider{
		apiKey:         cfg.APIKey,
		enabled:        cfg.Enabled && cfg.APIKey != "",
		strategyParams: params,
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
	systemPrompt := buildSystemPrompt(d.strategyParams)
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

// EvaluateNews evaluates news item using DeepSeek AI
func (d *DeepSeekProvider) EvaluateNews(ctx context.Context, newsItem *models.NewsItem) error {
	systemPrompt := buildNewsEvaluationSystemPrompt()
	userPrompt := buildNewsEvaluationUserPrompt(newsItem)

	reqBody := map[string]interface{}{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.3, // Lower temperature for more consistent evaluation
		"max_tokens":  300,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", deepseekAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", d.apiKey))

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

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
		return fmt.Errorf("no choices in response")
	}

	content := result.Choices[0].Message.Content

	// Parse evaluation
	evaluation, err := parseNewsEvaluation(content)
	if err != nil {
		logger.Warn("failed to parse news evaluation",
			zap.String("response", content),
			zap.Error(err),
		)
		return err
	}

	// Update news item with AI evaluation
	newsItem.Sentiment = evaluation.Sentiment
	newsItem.Impact = evaluation.Impact
	newsItem.Urgency = evaluation.Urgency

	logger.Debug("news evaluated by DeepSeek AI",
		zap.String("title", newsItem.Title),
		zap.Float64("sentiment", evaluation.Sentiment),
		zap.Int("impact", evaluation.Impact),
		zap.String("urgency", evaluation.Urgency),
	)

	return nil
}

// ========== Agentic AI Methods ==========

func (d *DeepSeekProvider) Reflect(ctx context.Context, reflectionPrompt *models.ReflectionPrompt) (*models.Reflection, error) {
	systemPrompt, userPrompt := BuildReflectionPrompt(reflectionPrompt)
	responseText, err := d.callDeepSeekAPI(ctx, systemPrompt, userPrompt, 1500)
	if err != nil {
		return nil, err
	}
	var reflection models.Reflection
	if err := json.Unmarshal([]byte(responseText), &reflection); err != nil {
		return nil, fmt.Errorf("failed to parse reflection: %w", err)
	}
	return &reflection, nil
}

func (d *DeepSeekProvider) GenerateOptions(ctx context.Context, situation *models.TradingSituation) ([]models.TradingOption, error) {
	systemPrompt, userPrompt := BuildGenerateOptionsPrompt(situation)
	responseText, err := d.callDeepSeekAPI(ctx, systemPrompt, userPrompt, 2000)
	if err != nil {
		return nil, err
	}
	var options []models.TradingOption
	if err := json.Unmarshal([]byte(responseText), &options); err != nil {
		return nil, fmt.Errorf("failed to parse options: %w", err)
	}
	return options, nil
}

func (d *DeepSeekProvider) EvaluateOption(ctx context.Context, option *models.TradingOption, memories []models.SemanticMemory) (*models.OptionEvaluation, error) {
	systemPrompt, userPrompt := BuildEvaluateOptionPrompt(option, memories)
	responseText, err := d.callDeepSeekAPI(ctx, systemPrompt, userPrompt, 1500)
	if err != nil {
		return nil, err
	}
	var evaluation models.OptionEvaluation
	if err := json.Unmarshal([]byte(responseText), &evaluation); err != nil {
		return nil, fmt.Errorf("failed to parse evaluation: %w", err)
	}
	return &evaluation, nil
}

func (d *DeepSeekProvider) MakeFinalDecision(ctx context.Context, evaluations []models.OptionEvaluation) (*models.AIDecision, error) {
	systemPrompt, userPrompt := BuildFinalDecisionPrompt(evaluations)
	responseText, err := d.callDeepSeekAPI(ctx, systemPrompt, userPrompt, 1000)
	if err != nil {
		return nil, err
	}
	var decision models.AIDecision
	decision.Provider = "DeepSeek"
	if err := json.Unmarshal([]byte(responseText), &decision); err != nil {
		return nil, fmt.Errorf("failed to parse decision: %w", err)
	}
	return &decision, nil
}

func (d *DeepSeekProvider) CreatePlan(ctx context.Context, planRequest *models.PlanRequest) (*models.TradingPlan, error) {
	systemPrompt, userPrompt := BuildCreatePlanPrompt(planRequest)
	responseText, err := d.callDeepSeekAPI(ctx, systemPrompt, userPrompt, 2500)
	if err != nil {
		return nil, err
	}
	var plan models.TradingPlan
	if err := json.Unmarshal([]byte(responseText), &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan: %w", err)
	}
	return &plan, nil
}

func (d *DeepSeekProvider) SelfAnalyze(ctx context.Context, performance *models.PerformanceData) (*models.SelfAnalysis, error) {
	systemPrompt, userPrompt := BuildSelfAnalysisPrompt(performance)
	responseText, err := d.callDeepSeekAPI(ctx, systemPrompt, userPrompt, 2000)
	if err != nil {
		return nil, err
	}
	var analysis models.SelfAnalysis
	if err := json.Unmarshal([]byte(responseText), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse self-analysis: %w", err)
	}
	return &analysis, nil
}

func (d *DeepSeekProvider) FindSimilarMemories(ctx context.Context, currentSituation string, memories []models.SemanticMemory, topK int) ([]models.SemanticMemory, error) {
	if topK > len(memories) {
		topK = len(memories)
	}
	return memories[:topK], nil
}

func (d *DeepSeekProvider) SummarizeMemory(ctx context.Context, experience *models.TradeExperience) (*models.MemorySummary, error) {
	systemPrompt, userPrompt := BuildSummarizeMemoryPrompt(experience)
	responseText, err := d.callDeepSeekAPI(ctx, systemPrompt, userPrompt, 800)
	if err != nil {
		return nil, err
	}
	var summary models.MemorySummary
	if err := json.Unmarshal([]byte(responseText), &summary); err != nil {
		return nil, fmt.Errorf("failed to parse memory summary: %w", err)
	}
	return &summary, nil
}

func (d *DeepSeekProvider) callDeepSeekAPI(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	reqBody := map[string]interface{}{
		"model": "deepseek-chat",
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

	req, err := http.NewRequestWithContext(ctx, "POST", deepseekAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.apiKey)

	resp, err := d.client.Do(req)
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
