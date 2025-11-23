package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
	"github.com/selivandex/trader-bot/pkg/templates"
	"go.uber.org/zap"
)

var globalTemplates templates.Renderer

// SetTemplateRenderer sets global template renderer (called from main.go at startup)
func SetTemplateRenderer(renderer templates.Renderer) {
	globalTemplates = renderer
}

// buildPromptsFromTemplate renders analyze template with all data
func buildPromptsFromTemplate(prompt *models.TradingPrompt, params *models.StrategyParameters) (systemPrompt string, userPrompt string) {
	if globalTemplates == nil {
		logger.Error("templates not loaded - cannot build prompts")
		return "", ""
	}

	data := map[string]interface{}{
		"StrategyParams":  params,
		"MarketData":      prompt.MarketData,
		"CurrentPosition": prompt.CurrentPosition,
		"Balance":         prompt.Balance,
		"Equity":          prompt.Equity,
		"DailyPnL":        prompt.DailyPnL,
		"RecentTrades":    prompt.RecentTrades,
	}

	output, err := globalTemplates.ExecuteTemplate("analyze.tmpl", data)
	if err != nil {
		logger.Error("failed to render analyze template", zap.Error(err))
		return "", ""
	}

	return SplitPrompt(output)
}

// buildNewsPromptsFromTemplate renders news evaluation template (single news - DEPRECATED)
func buildNewsPromptsFromTemplate(newsItem *models.NewsItem) (systemPrompt string, userPrompt string) {
	if globalTemplates == nil {
		logger.Error("templates not loaded - cannot build news prompts")
		return "", ""
	}

	ageHours := time.Since(newsItem.PublishedAt).Hours()

	data := map[string]interface{}{
		"Source":   newsItem.Source,
		"Title":    newsItem.Title,
		"Content":  truncateContent(newsItem.Content, 500),
		"AgeHours": ageHours,
	}

	output, err := globalTemplates.ExecuteTemplate("evaluate_news.tmpl", data)
	if err != nil {
		logger.Error("failed to render evaluate_news template", zap.Error(err))
		return "", ""
	}

	return SplitPrompt(output)
}

// buildNewsBatchPromptsFromTemplate renders news batch evaluation template
func buildNewsBatchPromptsFromTemplate(newsItems []*models.NewsItem) (systemPrompt string, userPrompt string) {
	if globalTemplates == nil {
		logger.Error("templates not loaded - cannot build news batch prompts")
		return "", ""
	}

	// Prepare news items with age calculation
	type NewsItemWithAge struct {
		Source   string
		Title    string
		Content  string
		AgeHours float64
	}

	itemsWithAge := make([]NewsItemWithAge, len(newsItems))
	for i, item := range newsItems {
		itemsWithAge[i] = NewsItemWithAge{
			Source:   item.Source,
			Title:    item.Title,
			Content:  truncateContent(item.Content, 300), // Shorter per item for batch
			AgeHours: time.Since(item.PublishedAt).Hours(),
		}
	}

	data := map[string]interface{}{
		"NewsItems": itemsWithAge,
	}

	output, err := globalTemplates.ExecuteTemplate("evaluate_news_batch.tmpl", data)
	if err != nil {
		logger.Error("failed to render evaluate_news_batch template", zap.Error(err))
		return "", ""
	}

	return SplitPrompt(output)
}

// SplitPrompt splits template output into system and user prompts
func SplitPrompt(output string) (systemPrompt string, userPrompt string) {
	separator := "=== USER PROMPT ==="
	idx := bytes.Index([]byte(output), []byte(separator))

	if idx == -1 {
		return "", output
	}

	systemPrompt = strings.TrimSpace(output[:idx])
	userPrompt = strings.TrimSpace(output[idx+len(separator):])
	return systemPrompt, userPrompt
}

// === PARSING FUNCTIONS ===

// parseAIResponse parses AI response and extracts decision
func parseAIResponse(content, provider string) (*models.AIDecision, error) {
	// Try to extract JSON from response
	jsonStr := extractJSON(content)

	var response struct {
		Action     string  `json:"action"`
		Reason     string  `json:"reason"`
		Size       float64 `json:"size"`
		StopLoss   float64 `json:"stop_loss"`
		TakeProfit float64 `json:"take_profit"`
		Confidence int     `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w (content: %s)", err, jsonStr)
	}

	// Validate action
	action := models.AIAction(strings.ToUpper(response.Action))
	validActions := map[models.AIAction]bool{
		models.ActionHold:      true,
		models.ActionClose:     true,
		models.ActionOpenLong:  true,
		models.ActionOpenShort: true,
		models.ActionScaleIn:   true,
		models.ActionScaleOut:  true,
	}

	if !validActions[action] {
		return nil, fmt.Errorf("invalid action: %s", response.Action)
	}

	// Validate confidence
	if response.Confidence < 0 || response.Confidence > 100 {
		return nil, fmt.Errorf("invalid confidence: %d", response.Confidence)
	}

	decision := &models.AIDecision{
		Provider:   provider,
		Prompt:     "",
		Response:   jsonStr,
		Action:     action,
		Reason:     response.Reason,
		Size:       models.NewDecimal(response.Size),
		StopLoss:   models.NewDecimal(response.StopLoss),
		TakeProfit: models.NewDecimal(response.TakeProfit),
		Confidence: response.Confidence,
		Executed:   false,
		CreatedAt:  time.Now(),
	}

	return decision, nil
}

// parseNewsEvaluation parses AI evaluation response
func parseNewsEvaluation(content string) (*NewsEvaluation, error) {
	jsonStr := extractJSON(content)

	var eval struct {
		Sentiment float64 `json:"sentiment"`
		Impact    int     `json:"impact"`
		Urgency   string  `json:"urgency"`
		Reasoning string  `json:"reasoning"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &eval); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	// Validate
	if eval.Sentiment < -1.0 || eval.Sentiment > 1.0 {
		eval.Sentiment = 0
	}

	if eval.Impact < 1 || eval.Impact > 10 {
		eval.Impact = 5
	}

	if eval.Urgency != "IMMEDIATE" && eval.Urgency != "HOURS" && eval.Urgency != "DAYS" {
		eval.Urgency = "HOURS"
	}

	return &NewsEvaluation{
		Sentiment: eval.Sentiment,
		Impact:    eval.Impact,
		Urgency:   eval.Urgency,
		Reasoning: eval.Reasoning,
	}, nil
}

// NewsEvaluation represents AI evaluation of news
type NewsEvaluation struct {
	Sentiment float64 `json:"sentiment"`
	Impact    int     `json:"impact"`
	Urgency   string  `json:"urgency"`
	Reasoning string  `json:"reasoning"`
}

// parseAndApplyNewsBatchEvaluations parses AI response and applies to news items
func parseAndApplyNewsBatchEvaluations(responseText string, newsItems []*models.NewsItem, providerName string) error {
	jsonStr := extractJSON(responseText)

	var evaluations []struct {
		Index     int     `json:"index"`
		Sentiment float64 `json:"sentiment"`
		Impact    int     `json:"impact"`
		Urgency   string  `json:"urgency"`
		Reasoning string  `json:"reasoning"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &evaluations); err != nil {
		return fmt.Errorf("failed to parse batch evaluation: %w", err)
	}

	// Apply evaluations to news items
	appliedCount := 0
	for _, eval := range evaluations {
		if eval.Index >= 0 && eval.Index < len(newsItems) {
			newsItems[eval.Index].Sentiment = eval.Sentiment
			newsItems[eval.Index].Impact = eval.Impact
			newsItems[eval.Index].Urgency = eval.Urgency
			appliedCount++
		}
	}

	logger.Info("news batch evaluated",
		zap.String("provider", providerName),
		zap.Int("total", len(newsItems)),
		zap.Int("evaluated", appliedCount),
	)

	return nil
}

// === UTILITY FUNCTIONS ===

// extractJSON extracts JSON from text that might contain markdown or extra content
func extractJSON(text string) string {
	// Remove markdown code blocks
	re := regexp.MustCompile("```(?:json)?\\s*([\\s\\S]*?)```")
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Try to find JSON object or array
	startObj := strings.Index(text, "{")
	startArr := strings.Index(text, "[")

	var start int
	var endChar string

	// Determine which comes first: object or array
	if startObj >= 0 && (startArr < 0 || startObj < startArr) {
		start = startObj
		endChar = "}"
	} else if startArr >= 0 {
		start = startArr
		endChar = "]"
	} else {
		return strings.TrimSpace(text)
	}

	end := strings.LastIndex(text, endChar)
	if end > start {
		return strings.TrimSpace(text[start : end+1])
	}

	return strings.TrimSpace(text)
}

func truncateContent(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// FormatDecisionReason formats decision reason using template
func FormatDecisionReason(personality, aiProvider, aiReason string, signals map[string]SignalWithWeight) string {
	if globalTemplates == nil {
		return fmt.Sprintf("[%s via %s] %s", personality, aiProvider, aiReason)
	}

	data := map[string]interface{}{
		"Personality": personality,
		"AIProvider":  aiProvider,
		"AIReason":    aiReason,
		"Technical":   signals["technical"],
		"News":        signals["news"],
		"OnChain":     signals["onchain"],
		"Sentiment":   signals["sentiment"],
	}

	output, err := globalTemplates.ExecuteTemplate("format_decision_reason.tmpl", data)
	if err != nil {
		logger.Error("failed to render format_decision_reason template", zap.Error(err))
		return fmt.Sprintf("[%s via %s] %s", personality, aiProvider, aiReason)
	}

	return output
}

// SignalWithWeight combines signal score with agent's weight
type SignalWithWeight struct {
	Score     float64
	Weight    float64
	Direction string
}
