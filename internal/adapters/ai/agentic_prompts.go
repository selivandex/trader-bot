package ai

import (
	"fmt"

	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
	"go.uber.org/zap"
)

// AgenticPrompts provides generic prompts for all agentic AI methods
// Prompts are loaded from templates for maintainability
// Uses global template renderer set via SetTemplateRenderer in prompts.go

// executeTemplate renders template with data using global renderer
func executeTemplate(templateName string, data interface{}) (string, error) {
	if globalTemplates == nil {
		return "", fmt.Errorf("templates not loaded")
	}

	output, err := globalTemplates.ExecuteTemplate(templateName, data)
	if err != nil {
		return "", err
	}

	return output, nil
}

// BuildReflectionPrompt creates system and user prompts for trade reflection
func BuildReflectionPrompt(reflectionPrompt *models.ReflectionPrompt) (systemPrompt string, userPrompt string) {
	if globalTemplates == nil {
		logger.Error("templates not loaded")
		return "", ""
	}

	output, err := executeTemplate("reflection.tmpl", reflectionPrompt)
	if err != nil {
		logger.Error("failed to render reflection template", zap.Error(err))
		return "", ""
	}

	return SplitPrompt(output)
}

// BuildGenerateOptionsPrompt creates prompts for option generation
func BuildGenerateOptionsPrompt(situation *models.TradingSituation) (systemPrompt string, userPrompt string) {
	if globalTemplates == nil {
		logger.Error("templates not loaded")
		return "", ""
	}

	output, err := executeTemplate("generate_options.tmpl", situation)
	if err != nil {
		logger.Error("failed to render generate_options template", zap.Error(err))
		return "", ""
	}

	return SplitPrompt(output)
}

// BuildEvaluateOptionPrompt creates prompts for option evaluation
func BuildEvaluateOptionPrompt(option *models.TradingOption, memories []models.SemanticMemory) (systemPrompt string, userPrompt string) {
	if globalTemplates == nil {
		logger.Error("templates not loaded")
		return "", ""
	}

	data := map[string]interface{}{
		"OptionID":    option.OptionID,
		"Action":      option.Action,
		"Description": option.Description,
		"Parameters":  option.Parameters,
		"Reasoning":   option.Reasoning,
		"Memories":    memories,
	}

	output, err := executeTemplate("evaluate_option.tmpl", data)
	if err != nil {
		logger.Error("failed to render evaluate_option template", zap.Error(err))
		return "", ""
	}

	return SplitPrompt(output)
}

// BuildFinalDecisionPrompt creates prompts for final decision
func BuildFinalDecisionPrompt(evaluations []models.OptionEvaluation) (systemPrompt string, userPrompt string) {
	if globalTemplates == nil {
		logger.Error("templates not loaded")
		return "", ""
	}

	// Format evaluations text
	evalsText := "=== EVALUATED OPTIONS ===\n\n"
	for i, eval := range evaluations {
		evalsText += fmt.Sprintf("Option %d: %s\n", i+1, eval.OptionID)
		evalsText += fmt.Sprintf("Score: %.1f/100, Confidence: %.0f%%\n", eval.Score, eval.Confidence*100)
		evalsText += fmt.Sprintf("Expected: %s\n\n", eval.ExpectedOutcome)
	}

	data := map[string]interface{}{
		"EvaluationsText": evalsText,
		"Evaluations":     evaluations,
	}

	output, err := executeTemplate("final_decision.tmpl", data)
	if err != nil {
		logger.Error("failed to render final_decision template", zap.Error(err))
		return "", ""
	}

	return SplitPrompt(output)
}

// BuildCreatePlanPrompt creates prompts for trading plan
func BuildCreatePlanPrompt(planRequest *models.PlanRequest) (systemPrompt string, userPrompt string) {
	if globalTemplates == nil {
		logger.Error("templates not loaded")
		return "", ""
	}

	output, err := executeTemplate("create_plan.tmpl", planRequest)
	if err != nil {
		logger.Error("failed to render create_plan template", zap.Error(err))
		return "", ""
	}

	return SplitPrompt(output)
}

// BuildSelfAnalysisPrompt creates prompts for self-analysis
func BuildSelfAnalysisPrompt(performance *models.PerformanceData) (systemPrompt string, userPrompt string) {
	if globalTemplates == nil {
		logger.Error("templates not loaded")
		return "", ""
	}

	output, err := executeTemplate("self_analysis.tmpl", performance)
	if err != nil {
		logger.Error("failed to render self_analysis template", zap.Error(err))
		return "", ""
	}

	return SplitPrompt(output)
}

// BuildSummarizeMemoryPrompt creates prompts for memory summarization
func BuildSummarizeMemoryPrompt(experience *models.TradeExperience) (systemPrompt string, userPrompt string) {
	if globalTemplates == nil {
		logger.Error("templates not loaded")
		return "", ""
	}

	output, err := executeTemplate("summarize_memory.tmpl", experience)
	if err != nil {
		logger.Error("failed to render summarize_memory template", zap.Error(err))
		return "", ""
	}

	return SplitPrompt(output)
}

// BuildAdaptiveThinkPrompt creates prompt for adaptive CoT reasoning
func BuildAdaptiveThinkPrompt(thinkingData *AdaptiveThinkingData) (systemPrompt string, userPrompt string) {
	if globalTemplates == nil {
		logger.Error("templates not loaded")
		return "", ""
	}

	output, err := executeTemplate("adaptive_think.tmpl", thinkingData)
	if err != nil {
		logger.Error("failed to render adaptive_think template", zap.Error(err))
		return "", ""
	}

	return SplitPrompt(output)
}

// AdaptiveThinkingData contains all context for adaptive thinking prompt
type AdaptiveThinkingData struct {
	AgentName        string
	Observation      string
	MarketData       *models.MarketData // For template to format news/on-chain
	CurrentPosition  *models.Position
	RecalledMemories []models.SemanticMemory
	ToolResults      map[string]interface{}
	Questions        []QuestionAnswer
	Options          []models.TradingOption
	Evaluations      []models.OptionEvaluation
	History          []AdaptiveThoughtStep
	Iteration        int
}

// QuestionAnswer represents self-questioning
type QuestionAnswer struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// AdaptiveThoughtStep represents one thinking iteration (for template data)
type AdaptiveThoughtStep struct {
	Iteration  int     `json:"iteration"`
	Action     string  `json:"action"`
	Reasoning  string  `json:"reasoning"`
	Confidence float64 `json:"confidence"`
}

// AnswerOwnQuestion formats agent's self-answer using template
func AnswerOwnQuestion(question string, volatility float64, hasVolatility bool, supports []float64, hasSupports bool) string {
	if globalTemplates == nil {
		return fmt.Sprintf("Based on current data: %s", question)
	}

	data := map[string]interface{}{
		"Question":      question,
		"Volatility":    volatility,
		"HasVolatility": hasVolatility,
		"Supports":      supports,
		"HasSupports":   hasSupports,
	}

	output, err := executeTemplate("answer_question.tmpl", data)
	if err != nil {
		logger.Error("failed to render answer_question template", zap.Error(err))
		return fmt.Sprintf("Based on current data: %s", question)
	}

	return output
}

// ObserveMarket formats basic market observation using template
func ObserveMarket(symbol string, price, change24h float64, hasPosition bool, positionSide string, positionPnL float64) string {
	if globalTemplates == nil {
		return fmt.Sprintf("Market: %s at $%.2f", symbol, price)
	}

	data := map[string]interface{}{
		"Symbol":       symbol,
		"Price":        price,
		"Change24h":    change24h,
		"HasPosition":  hasPosition,
		"PositionSide": positionSide,
		"PositionPnL":  positionPnL,
	}

	output, err := executeTemplate("observe_market.tmpl", data)
	if err != nil {
		logger.Error("failed to render observe_market template", zap.Error(err))
		return fmt.Sprintf("Market: %s at $%.2f", symbol, price)
	}

	return output
}

// FormatAdaptiveReasoning formats reasoning trace from adaptive CoT
func FormatAdaptiveReasoning(agentName string, history []AdaptiveThoughtStep, toolsUsed, questionsAsked int, decision *models.AIDecision) string {
	if globalTemplates == nil {
		return fmt.Sprintf("Decision: %s (confidence: %d%%)", decision.Action, decision.Confidence)
	}

	data := map[string]interface{}{
		"AgentName":      agentName,
		"IterationCount": len(history),
		"ToolsUsedCount": toolsUsed,
		"QuestionsCount": questionsAsked,
		"History":        history,
		"Decision":       decision,
	}

	output, err := executeTemplate("format_reasoning.tmpl", data)
	if err != nil {
		logger.Error("failed to render format_reasoning template", zap.Error(err))
		return fmt.Sprintf("Decision: %s (confidence: %d%%)", decision.Action, decision.Confidence)
	}

	return output
}
