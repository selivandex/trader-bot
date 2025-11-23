package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
	"github.com/selivandex/trader-bot/pkg/templates"
)

// setupTest initializes logger for tests
func setupTest(t *testing.T) {
	t.Helper()
	if logger.Log == nil {
		if err := logger.Init("error", ""); err != nil {
			t.Fatalf("Failed to initialize logger: %v", err)
		}
	}
}

// TestPromptTemplatesLoaded verifies that all required templates load successfully
func TestPromptTemplatesLoaded(t *testing.T) {
	setupTest(t)

	// Load ALL templates from root
	allTemplates, err := templates.NewManager("../../../templates")
	if err != nil {
		t.Fatalf("Failed to load templates: %v", err)
	}

	// Set global template renderer
	SetTemplateRenderer(allTemplates)

	// Verify required templates exist
	requiredTemplates := []string{
		// Basic
		"analyze.tmpl",
		"evaluate_news.tmpl",
		"evaluate_news_batch.tmpl",
		// Agentic
		"reflection.tmpl",
		"generate_options.tmpl",
		"evaluate_option.tmpl",
		"final_decision.tmpl",
		"create_plan.tmpl",
		"self_analysis.tmpl",
		"summarize_memory.tmpl",
		// Validators
		"risk_manager.tmpl",
		"technical_expert.tmpl",
		"market_psychologist.tmpl",
	}

	for _, tmpl := range requiredTemplates {
		if !allTemplates.TemplateExists(tmpl) {
			t.Errorf("Required template not found: %s", tmpl)
		}
	}
}

// TestBuildPromptsFromTemplate tests basic analyze template rendering
func TestBuildPromptsFromTemplate(t *testing.T) {
	setupTest(t)

	allTemplates, err := templates.NewManager("../../../templates")
	if err != nil {
		t.Fatalf("Failed to load templates: %v", err)
	}
	SetTemplateRenderer(allTemplates)

	params := &models.StrategyParameters{
		MaxPositionPercent:     30.0,
		MaxLeverage:            3,
		StopLossPercent:        2.0,
		TakeProfitPercent:      5.0,
		MinConfidenceThreshold: 70,
	}

	prompt := &models.TradingPrompt{
		MarketData: &models.MarketData{
			Symbol: "BTC/USDT",
			Ticker: &models.Ticker{
				Symbol:    "BTC/USDT",
				Last:      models.NewDecimal(43500.0),
				Change24h: models.NewDecimal(2.5),
				High24h:   models.NewDecimal(44000.0),
				Low24h:    models.NewDecimal(42800.0),
				Volume24h: models.NewDecimal(1500000000),
				Bid:       models.NewDecimal(43499.5),
				Ask:       models.NewDecimal(43500.5),
			},
			FundingRate:  models.NewDecimal(0.0001),
			OpenInterest: models.NewDecimal(500000000),
		},
		Balance:  models.NewDecimal(10000.0),
		Equity:   models.NewDecimal(10000.0),
		DailyPnL: models.NewDecimal(150.0),
	}

	systemPrompt, userPrompt := buildPromptsFromTemplate(prompt, params)

	// Verify system prompt contains strategy parameters
	if !strings.Contains(systemPrompt, "30%") {
		t.Error("System prompt missing max position percent")
	}
	if !strings.Contains(systemPrompt, "3x") {
		t.Error("System prompt missing max leverage")
	}
	if !strings.Contains(systemPrompt, "2.0%") {
		t.Error("System prompt missing stop loss percent")
	}
	if !strings.Contains(systemPrompt, "5.0%") {
		t.Error("System prompt missing take profit percent")
	}
	if !strings.Contains(systemPrompt, "70%") {
		t.Error("System prompt missing min confidence threshold")
	}

	// Verify user prompt contains market data
	if !strings.Contains(userPrompt, "BTC/USDT") {
		t.Error("User prompt missing symbol")
	}
	if !strings.Contains(userPrompt, "43500") {
		t.Error("User prompt missing current price")
	}
	if !strings.Contains(userPrompt, "2.5") {
		t.Error("User prompt missing 24h change")
	}
	if !strings.Contains(userPrompt, "10000") {
		t.Error("User prompt missing balance")
	}

	t.Logf("System prompt length: %d characters", len(systemPrompt))
	t.Logf("User prompt length: %d characters", len(userPrompt))
}

// TestBuildNewsPromptsFromTemplate tests news evaluation template rendering
func TestBuildNewsPromptsFromTemplate(t *testing.T) {
	setupTest(t)

	allTemplates, err := templates.NewManager("../../../templates")
	if err != nil {
		t.Fatalf("Failed to load templates: %v", err)
	}
	SetTemplateRenderer(allTemplates)

	newsItem := &models.NewsItem{
		Source:      "CoinDesk",
		Title:       "Bitcoin ETF Approved by SEC",
		Content:     "The SEC has approved the first Bitcoin spot ETF, marking a major milestone for crypto adoption.",
		PublishedAt: time.Now().Add(-2 * time.Hour),
	}

	systemPrompt, userPrompt := buildNewsPromptsFromTemplate(newsItem)

	// Verify system prompt contains evaluation instructions
	if !strings.Contains(systemPrompt, "sentiment") {
		t.Error("System prompt missing sentiment instructions")
	}
	if !strings.Contains(systemPrompt, "impact") {
		t.Error("System prompt missing impact instructions")
	}
	if !strings.Contains(systemPrompt, "urgency") {
		t.Error("System prompt missing urgency instructions")
	}

	// Verify user prompt contains news data
	if !strings.Contains(userPrompt, "CoinDesk") {
		t.Error("User prompt missing source")
	}
	if !strings.Contains(userPrompt, "Bitcoin ETF") {
		t.Error("User prompt missing title")
	}
	if !strings.Contains(userPrompt, "2.0 hours ago") {
		t.Error("User prompt missing age")
	}

	t.Logf("News evaluation prompt rendered successfully")
}

// TestBuildReflectionPrompt tests reflection template rendering
func TestBuildReflectionPrompt(t *testing.T) {
	setupTest(t)

	allTemplates, err := templates.NewManager("../../../templates")
	if err != nil {
		t.Fatalf("Failed to load templates: %v", err)
	}
	SetTemplateRenderer(allTemplates)

	reflection := &models.ReflectionPrompt{
		AgentName: "TestAgent",
		Trade: &models.TradeExperience{
			Symbol:        "BTC/USDT",
			Side:          "long",
			EntryPrice:    models.NewDecimal(43000.0),
			ExitPrice:     models.NewDecimal(44500.0),
			PnL:           models.NewDecimal(150.0),
			PnLPercent:    3.5,
			Duration:      4 * time.Hour,
			EntryReason:   "RSI oversold, positive news sentiment",
			ExitReason:    "Take profit hit",
			WasSuccessful: true,
		},
		PriorBeliefs: "Expected 2-3% gain based on technical setup",
	}

	systemPrompt, userPrompt := BuildReflectionPrompt(reflection)

	if systemPrompt == "" && userPrompt == "" {
		t.Fatal("Both prompts are empty")
	}

	// Verify prompt contains trade data
	fullPrompt := systemPrompt + userPrompt
	if !strings.Contains(fullPrompt, "BTC/USDT") {
		t.Error("Prompt missing symbol")
	}
	if !strings.Contains(fullPrompt, "43000") {
		t.Error("Prompt missing entry price")
	}
	if !strings.Contains(fullPrompt, "44500") {
		t.Error("Prompt missing exit price")
	}
	if !strings.Contains(fullPrompt, "3.5") {
		t.Error("Prompt missing PnL percent")
	}

	t.Logf("Reflection prompt rendered successfully")
}

// TestBuildGenerateOptionsPrompt tests option generation template rendering
func TestBuildGenerateOptionsPrompt(t *testing.T) {
	setupTest(t)

	allTemplates, err := templates.NewManager("../../../templates")
	if err != nil {
		t.Fatalf("Failed to load templates: %v", err)
	}
	SetTemplateRenderer(allTemplates)

	situation := &models.TradingSituation{
		MarketData: &models.MarketData{
			Symbol: "BTC/USDT",
			Ticker: &models.Ticker{
				Last:      models.NewDecimal(43500.0),
				Change24h: models.NewDecimal(2.5),
				Bid:       models.NewDecimal(43499.0),
				Ask:       models.NewDecimal(43501.0),
				High24h:   models.NewDecimal(44000.0),
				Low24h:    models.NewDecimal(42800.0),
				Volume24h: models.NewDecimal(1500000000),
			},
		},
		Balance: models.NewDecimal(10000.0),
	}

	systemPrompt, userPrompt := BuildGenerateOptionsPrompt(situation)

	if systemPrompt == "" && userPrompt == "" {
		t.Fatal("Both prompts are empty")
	}

	fullPrompt := systemPrompt + userPrompt
	if !strings.Contains(fullPrompt, "BTC/USDT") {
		t.Error("Prompt missing symbol")
	}
	if !strings.Contains(fullPrompt, "43500") {
		t.Error("Prompt missing current price")
	}

	t.Logf("Generate options prompt rendered successfully")
}

// TestSplitPrompt tests prompt separator splitting
func TestSplitPrompt(t *testing.T) {
	testCases := []struct {
		name           string
		input          string
		expectedSystem string
		expectedUser   string
	}{
		{
			name:           "With separator",
			input:          "System instructions\n\n=== USER PROMPT ===\n\nUser task",
			expectedSystem: "System instructions",
			expectedUser:   "User task",
		},
		{
			name:           "Without separator",
			input:          "All user prompt",
			expectedSystem: "",
			expectedUser:   "All user prompt",
		},
		{
			name:           "Empty input",
			input:          "",
			expectedSystem: "",
			expectedUser:   "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sys, user := SplitPrompt(tc.input)

			if sys != tc.expectedSystem {
				t.Errorf("System prompt mismatch.\nExpected: %q\nGot: %q", tc.expectedSystem, sys)
			}
			if user != tc.expectedUser {
				t.Errorf("User prompt mismatch.\nExpected: %q\nGot: %q", tc.expectedUser, user)
			}
		})
	}
}

// TestExtractJSON tests JSON extraction from various formats
func TestExtractJSON(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Plain JSON",
			input:    `{"action": "HOLD", "confidence": 75}`,
			expected: `{"action": "HOLD", "confidence": 75}`,
		},
		{
			name:     "JSON in markdown code block",
			input:    "```json\n{\"action\": \"HOLD\"}\n```",
			expected: `{"action": "HOLD"}`,
		},
		{
			name:     "JSON with extra text",
			input:    "Here is my decision: {\"action\": \"HOLD\"} - that's it",
			expected: `{"action": "HOLD"}`,
		},
		{
			name:     "Array JSON",
			input:    `[{"id": 1}, {"id": 2}]`,
			expected: `[{"id": 1}, {"id": 2}]`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractJSON(tc.input)
			if result != tc.expected {
				t.Errorf("JSON extraction failed.\nExpected: %s\nGot: %s", tc.expected, result)
			}
		})
	}
}

// TestParseAIResponse tests AI response parsing
func TestParseAIResponse(t *testing.T) {
	testCases := []struct {
		name           string
		input          string
		provider       string
		expectedAction models.AIAction
		expectedError  bool
	}{
		{
			name:           "Valid HOLD response",
			input:          `{"action": "HOLD", "reason": "Wait for better setup", "confidence": 60}`,
			provider:       "test",
			expectedAction: models.ActionHold,
			expectedError:  false,
		},
		{
			name:           "Valid OPEN_LONG response",
			input:          `{"action": "OPEN_LONG", "reason": "Bullish setup", "confidence": 80, "size": 0.5, "stop_loss": 42000, "take_profit": 45000}`,
			provider:       "test",
			expectedAction: models.ActionOpenLong,
			expectedError:  false,
		},
		{
			name:          "Invalid action",
			input:         `{"action": "INVALID", "reason": "Test", "confidence": 50}`,
			provider:      "test",
			expectedError: true,
		},
		{
			name:          "Invalid confidence",
			input:         `{"action": "HOLD", "reason": "Test", "confidence": 150}`,
			provider:      "test",
			expectedError: true,
		},
		{
			name:          "Invalid JSON",
			input:         `not json at all`,
			provider:      "test",
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			decision, err := parseAIResponse(tc.input, tc.provider)

			if tc.expectedError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if decision.Action != tc.expectedAction {
				t.Errorf("Action mismatch. Expected: %s, Got: %s", tc.expectedAction, decision.Action)
			}

			if decision.Provider != tc.provider {
				t.Errorf("Provider mismatch. Expected: %s, Got: %s", tc.provider, decision.Provider)
			}
		})
	}
}

// TestParseNewsEvaluation tests news evaluation parsing
func TestParseNewsEvaluation(t *testing.T) {
	testCases := []struct {
		name              string
		input             string
		expectedSentiment float64
		expectedImpact    int
		expectedUrgency   string
		expectedError     bool
	}{
		{
			name:              "Valid bullish news",
			input:             `{"sentiment": 0.8, "impact": 9, "urgency": "IMMEDIATE", "reasoning": "ETF approval"}`,
			expectedSentiment: 0.8,
			expectedImpact:    9,
			expectedUrgency:   "IMMEDIATE",
			expectedError:     false,
		},
		{
			name:              "Valid bearish news",
			input:             `{"sentiment": -0.6, "impact": 7, "urgency": "HOURS", "reasoning": "Regulatory concern"}`,
			expectedSentiment: -0.6,
			expectedImpact:    7,
			expectedUrgency:   "HOURS",
			expectedError:     false,
		},
		{
			name:              "Neutral news",
			input:             `{"sentiment": 0.0, "impact": 5, "urgency": "DAYS", "reasoning": "General update"}`,
			expectedSentiment: 0.0,
			expectedImpact:    5,
			expectedUrgency:   "DAYS",
			expectedError:     false,
		},
		{
			name:          "Invalid JSON",
			input:         `not json`,
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			eval, err := parseNewsEvaluation(tc.input)

			if tc.expectedError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if eval.Sentiment != tc.expectedSentiment {
				t.Errorf("Sentiment mismatch. Expected: %.2f, Got: %.2f", tc.expectedSentiment, eval.Sentiment)
			}
			if eval.Impact != tc.expectedImpact {
				t.Errorf("Impact mismatch. Expected: %d, Got: %d", tc.expectedImpact, eval.Impact)
			}
			if eval.Urgency != tc.expectedUrgency {
				t.Errorf("Urgency mismatch. Expected: %s, Got: %s", tc.expectedUrgency, eval.Urgency)
			}
		})
	}
}

// TestBuildSummarizeMemoryPrompt tests memory summarization template
func TestBuildSummarizeMemoryPrompt(t *testing.T) {
	setupTest(t)

	allTemplates, err := templates.NewManager("../../../templates")
	if err != nil {
		t.Fatalf("Failed to load templates: %v", err)
	}
	SetTemplateRenderer(allTemplates)

	experience := &models.TradeExperience{
		Symbol:        "ETH/USDT",
		Side:          "short",
		EntryPrice:    models.NewDecimal(2500.0),
		ExitPrice:     models.NewDecimal(2450.0),
		PnL:           models.NewDecimal(50.0),
		PnLPercent:    2.0,
		Duration:      2 * time.Hour,
		EntryReason:   "Overbought RSI at resistance",
		ExitReason:    "Target reached",
		WasSuccessful: true,
	}

	systemPrompt, userPrompt := BuildSummarizeMemoryPrompt(experience)

	if systemPrompt == "" && userPrompt == "" {
		t.Fatal("Both prompts are empty")
	}

	fullPrompt := systemPrompt + userPrompt

	requiredStrings := []string{
		"ETH/USDT",
		"short",
		"2500",
		"2450",
		"MEMORY",
	}

	for _, required := range requiredStrings {
		if !strings.Contains(fullPrompt, required) {
			t.Errorf("Memory prompt missing required string: %s", required)
		}
	}

	t.Logf("Memory summarization prompt: %d characters", len(fullPrompt))
}

// BenchmarkPromptRendering benchmarks template rendering performance
func BenchmarkPromptRendering(b *testing.B) {
	// Initialize logger for benchmark
	if logger.Log == nil {
		if err := logger.Init("error", ""); err != nil {
			b.Fatalf("Failed to initialize logger: %v", err)
		}
	}

	allTemplates, err := templates.NewManager("../../../templates")
	if err != nil {
		b.Fatalf("Failed to load templates: %v", err)
	}
	SetTemplateRenderer(allTemplates)

	params := &models.StrategyParameters{
		MaxPositionPercent:     30.0,
		MaxLeverage:            3,
		StopLossPercent:        2.0,
		TakeProfitPercent:      5.0,
		MinConfidenceThreshold: 70,
	}

	prompt := &models.TradingPrompt{
		MarketData: &models.MarketData{
			Symbol: "BTC/USDT",
			Ticker: &models.Ticker{
				Last:      models.NewDecimal(43500.0),
				Change24h: models.NewDecimal(2.5),
			},
		},
		Balance: models.NewDecimal(10000.0),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = buildPromptsFromTemplate(prompt, params)
	}
}
