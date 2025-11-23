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

const claudeAPIURL = "https://api.anthropic.com/v1/messages"

// ClaudeProvider implements AI provider for Claude
type ClaudeProvider struct {
	apiKey         string
	enabled        bool
	client         *http.Client
	strategyParams *models.StrategyParameters
}

// NewClaudeProvider creates new Claude provider
func NewClaudeProvider(cfg *config.AIProviderConfig, params *models.StrategyParameters) *ClaudeProvider {
	return &ClaudeProvider{
		apiKey:         cfg.APIKey,
		enabled:        cfg.Enabled && cfg.APIKey != "",
		strategyParams: params,
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
	systemPrompt, userPrompt := buildPromptsFromTemplate(prompt, c.strategyParams)

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

// EvaluateNews evaluates news item using Claude AI
func (c *ClaudeProvider) EvaluateNews(ctx context.Context, newsItem *models.NewsItem) error {
	systemPrompt, userPrompt := buildNewsPromptsFromTemplate(newsItem)

	reqBody := map[string]interface{}{
		"model":      "claude-3-5-sonnet-20241022",
		"max_tokens": 300,
		"system":     systemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.3, // Lower temperature for more consistent evaluation
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", claudeAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Content) == 0 {
		return fmt.Errorf("no content in response")
	}

	content := result.Content[0].Text

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

	logger.Debug("news evaluated by Claude AI",
		zap.String("title", newsItem.Title),
		zap.Float64("sentiment", evaluation.Sentiment),
		zap.Int("impact", evaluation.Impact),
		zap.String("urgency", evaluation.Urgency),
	)

	return nil
}

// EvaluateNewsBatch evaluates multiple news items in single API call (more efficient)
func (c *ClaudeProvider) EvaluateNewsBatch(ctx context.Context, newsItems []*models.NewsItem) error {
	if !c.enabled || len(newsItems) == 0 {
		return nil
	}

	systemPrompt, userPrompt := buildNewsBatchPromptsFromTemplate(newsItems)
	responseText, err := c.callClaudeAPI(ctx, systemPrompt, userPrompt, 2000)
	if err != nil {
		return fmt.Errorf("Claude API call failed: %w", err)
	}

	// Parse and apply evaluations (common logic)
	if err := parseAndApplyNewsBatchEvaluations(responseText, newsItems, "Claude"); err != nil {
		// Fallback to single item evaluation
		logger.Warn("batch parsing failed, falling back to single evaluations", zap.Error(err))
		for _, item := range newsItems {
			_ = c.EvaluateNews(ctx, item)
		}
	}

	return nil
}

// ========== Agentic AI Methods ==========

// Reflect analyzes past trade and generates insights
func (c *ClaudeProvider) Reflect(ctx context.Context, reflectionPrompt *models.ReflectionPrompt) (*models.Reflection, error) {
	systemPrompt, userPrompt := BuildReflectionPrompt(reflectionPrompt)
	responseText, err := c.callClaudeAPI(ctx, systemPrompt, userPrompt, 1500)
	if err != nil {
		return nil, err
	}

	var reflection models.Reflection
	if err := json.Unmarshal([]byte(responseText), &reflection); err != nil {
		return nil, fmt.Errorf("failed to parse reflection: %w", err)
	}
	return &reflection, nil
}

// GenerateOptions creates multiple trading options
func (c *ClaudeProvider) GenerateOptions(ctx context.Context, situation *models.TradingSituation) ([]models.TradingOption, error) {
	systemPrompt, userPrompt := BuildGenerateOptionsPrompt(situation)
	responseText, err := c.callClaudeAPI(ctx, systemPrompt, userPrompt, 2000)
	if err != nil {
		return nil, err
	}

	var options []models.TradingOption
	if err := json.Unmarshal([]byte(responseText), &options); err != nil {
		return nil, fmt.Errorf("failed to parse options: %w", err)
	}
	return options, nil
}

// EvaluateOption analyzes pros/cons of specific option
func (c *ClaudeProvider) EvaluateOption(ctx context.Context, option *models.TradingOption, memories []models.SemanticMemory) (*models.OptionEvaluation, error) {
	systemPrompt, userPrompt := BuildEvaluateOptionPrompt(option, memories)
	responseText, err := c.callClaudeAPI(ctx, systemPrompt, userPrompt, 1500)
	if err != nil {
		return nil, err
	}

	var evaluation models.OptionEvaluation
	if err := json.Unmarshal([]byte(responseText), &evaluation); err != nil {
		return nil, fmt.Errorf("failed to parse evaluation: %w", err)
	}
	return &evaluation, nil
}

// MakeFinalDecision chooses best option from evaluations
func (c *ClaudeProvider) MakeFinalDecision(ctx context.Context, evaluations []models.OptionEvaluation) (*models.AIDecision, error) {
	systemPrompt, userPrompt := BuildFinalDecisionPrompt(evaluations)
	responseText, err := c.callClaudeAPI(ctx, systemPrompt, userPrompt, 1000)
	if err != nil {
		return nil, err
	}

	var decision models.AIDecision
	decision.Provider = "Claude"
	if err := json.Unmarshal([]byte(responseText), &decision); err != nil {
		return nil, fmt.Errorf("failed to parse decision: %w", err)
	}
	return &decision, nil
}

// CreatePlan develops multi-step trading plan
func (c *ClaudeProvider) CreatePlan(ctx context.Context, planRequest *models.PlanRequest) (*models.TradingPlan, error) {
	systemPrompt, userPrompt := BuildCreatePlanPrompt(planRequest)
	responseText, err := c.callClaudeAPI(ctx, systemPrompt, userPrompt, 2500)
	if err != nil {
		return nil, err
	}

	var plan models.TradingPlan
	if err := json.Unmarshal([]byte(responseText), &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan: %w", err)
	}
	return &plan, nil
}

// SelfAnalyze evaluates own performance and suggests improvements
func (c *ClaudeProvider) SelfAnalyze(ctx context.Context, performance *models.PerformanceData) (*models.SelfAnalysis, error) {
	systemPrompt, userPrompt := BuildSelfAnalysisPrompt(performance)
	responseText, err := c.callClaudeAPI(ctx, systemPrompt, userPrompt, 2000)
	if err != nil {
		return nil, err
	}

	var analysis models.SelfAnalysis
	if err := json.Unmarshal([]byte(responseText), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse self-analysis: %w", err)
	}
	return &analysis, nil
}

// FindSimilarMemories uses semantic search (handled by MemoryManager)
func (c *ClaudeProvider) FindSimilarMemories(ctx context.Context, currentSituation string, memories []models.SemanticMemory, topK int) ([]models.SemanticMemory, error) {
	if topK > len(memories) {
		topK = len(memories)
	}
	return memories[:topK], nil
}

// SummarizeMemory creates concise memory from trade experience
func (c *ClaudeProvider) SummarizeMemory(ctx context.Context, experience *models.TradeExperience) (*models.MemorySummary, error) {
	systemPrompt, userPrompt := BuildSummarizeMemoryPrompt(experience)
	responseText, err := c.callClaudeAPI(ctx, systemPrompt, userPrompt, 800)
	if err != nil {
		return nil, err
	}

	var summary models.MemorySummary
	if err := json.Unmarshal([]byte(responseText), &summary); err != nil {
		return nil, fmt.Errorf("failed to parse memory summary: %w", err)
	}
	return &summary, nil
}

// ValidateDecision validates trading decision from validator perspective
func (c *ClaudeProvider) ValidateDecision(ctx context.Context, request *models.ValidationRequest) (*models.ValidationResponse, error) {
	if !c.enabled {
		return nil, fmt.Errorf("Claude provider is not enabled")
	}

	// Use pre-built prompts from request (built by ValidatorCouncil from templates)
	systemPrompt := request.SystemPrompt
	userPrompt := request.UserPrompt

	responseText, err := c.callClaudeAPI(ctx, systemPrompt, userPrompt, 1500)
	if err != nil {
		return nil, err
	}

	var response models.ValidationResponse
	if err := json.Unmarshal([]byte(responseText), &response); err != nil {
		logger.Warn("failed to parse validation response as JSON, using text",
			zap.Error(err),
			zap.String("response", responseText),
		)
		// Fallback: return basic response with reasoning as text
		return &models.ValidationResponse{
			Verdict:    "ABSTAIN",
			Confidence: 50,
			Reasoning:  responseText,
			KeyRisks:   []string{"Failed to parse structured response"},
		}, nil
	}
	return &response, nil
}

// callClaudeAPI helper method for agentic calls
func (c *ClaudeProvider) callClaudeAPI(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	reqBody := map[string]interface{}{
		"model":      "claude-3-5-sonnet-20241022",
		"max_tokens": maxTokens,
		"system":     systemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", claudeAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty response")
	}

	return result.Content[0].Text, nil
}
