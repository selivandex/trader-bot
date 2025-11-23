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
