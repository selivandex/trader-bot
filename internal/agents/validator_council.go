package agents

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/ai"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
	"github.com/selivandex/trader-bot/pkg/templates"
)

// ValidatorVerdict represents validator's vote
type ValidatorVerdict string

const (
	VerdictApprove ValidatorVerdict = "APPROVE"
	VerdictReject  ValidatorVerdict = "REJECT"
	VerdictAbstain ValidatorVerdict = "ABSTAIN"
)

// ValidatorRole defines validator's perspective
type ValidatorRole string

const (
	RoleRiskManager        ValidatorRole = "risk_manager"        // Conservative, focuses on downside
	RoleTechnicalExpert    ValidatorRole = "technical_expert"    // Analyzes charts and indicators
	RoleMarketPsychologist ValidatorRole = "market_psychologist" // News and sentiment
)

// ValidatorConfig configures validation council
type ValidatorConfig struct {
	ValidateActions            []models.AIAction
	MinConfidenceForValidation int
	ConsensusThreshold         float64
	Enabled                    bool
	RequireUnanimous           bool
}

// ValidatorSetup defines individual validator configuration
type ValidatorSetup struct {
	Role         ValidatorRole
	AIProvider   ai.AgenticProvider
	ProviderName string
	Weight       float64 // Vote weight (1.0 for equal votes)
}

// ValidatorResponse represents validator's analysis
type ValidatorResponse struct {
	ValidatorRole     ValidatorRole
	ProviderName      string
	Verdict           ValidatorVerdict
	Reasoning         string
	RiskConcerns      string
	RecommendedAction string
	Confidence        int
}

// ConsensusResult is final decision from validator council
type ConsensusResult struct {
	OriginalDecision *models.AgentDecision
	FinalVerdict     ValidatorVerdict
	ConsensusSummary string
	ValidatorVotes   []ValidatorResponse
	ConsensusScore   float64
	ApprovalRate     float64
	ExecutionAllowed bool
}

// ValidatorCouncil manages multiple AI validators for consensus decision-making
type ValidatorCouncil struct {
	config          *ValidatorConfig
	agentConfig     *models.AgentConfig
	templateManager *templates.Manager
	validators      []ValidatorSetup
}

// NewValidatorCouncil creates new validator council
func NewValidatorCouncil(
	agentConfig *models.AgentConfig,
	aiProviders map[string]ai.AgenticProvider,
	config *ValidatorConfig,
	templateManager *templates.Manager,
) *ValidatorCouncil {
	// Use agent's validation config if available, otherwise use provided config or defaults
	if agentConfig.ValidationConfig != nil {
		config = &ValidatorConfig{
			Enabled:                    agentConfig.ValidationConfig.Enabled,
			MinConfidenceForValidation: agentConfig.ValidationConfig.MinConfidenceForValidation,
			ValidateActions: []models.AIAction{
				models.ActionOpenLong,
				models.ActionOpenShort,
			},
			ConsensusThreshold: agentConfig.ValidationConfig.ConsensusThreshold,
			RequireUnanimous:   agentConfig.ValidationConfig.RequireUnanimous,
		}
	} else if config == nil {
		config = DefaultValidatorConfig()
	}

	validators := []ValidatorSetup{}

	// Setup validators with different roles and AI models
	// Each validator has unique perspective and uses different AI model
	for providerName, aiProvider := range aiProviders {
		var role ValidatorRole
		weight := 1.0

		// Assign roles based on provider strengths
		switch providerName {
		case "Claude":
			role = RoleRiskManager // Claude is thoughtful and cautious
		case "GPT":
			role = RoleMarketPsychologist // GPT excels at context and sentiment
		case "DeepSeek":
			role = RoleTechnicalExpert // DeepSeek is fast and analytical
		default:
			role = RoleTechnicalExpert
		}

		validators = append(validators, ValidatorSetup{
			Role:         role,
			AIProvider:   aiProvider,
			ProviderName: providerName,
			Weight:       weight,
		})
	}

	logger.Info("🏛️ Validator council initialized",
		zap.String("agent", agentConfig.Name),
		zap.Int("validators", len(validators)),
		zap.Float64("consensus_threshold", config.ConsensusThreshold),
		zap.Bool("templates_loaded", templateManager != nil),
	)

	return &ValidatorCouncil{
		config:          config,
		validators:      validators,
		agentConfig:     agentConfig,
		templateManager: templateManager,
	}
}

// DefaultValidatorConfig returns sensible defaults
func DefaultValidatorConfig() *ValidatorConfig {
	return &ValidatorConfig{
		Enabled:                    true,
		MinConfidenceForValidation: 60, // Validate decisions with 60%+ confidence
		ValidateActions: []models.AIAction{
			models.ActionOpenLong,
			models.ActionOpenShort,
		},
		ConsensusThreshold: 0.66, // 2/3 majority
		RequireUnanimous:   false,
	}
}

// ShouldValidate checks if decision requires validation
func (vc *ValidatorCouncil) ShouldValidate(decision *models.AgentDecision) bool {
	if !vc.config.Enabled {
		return false
	}

	// Check confidence threshold
	if decision.Confidence < vc.config.MinConfidenceForValidation {
		return false
	}

	// Check if action is in validation list
	for _, action := range vc.config.ValidateActions {
		if decision.Action == action {
			return true
		}
	}

	return false
}

// ValidateDecision submits decision to validator council for consensus
func (vc *ValidatorCouncil) ValidateDecision(
	ctx context.Context,
	decision *models.AgentDecision,
	marketData *models.MarketData,
	position *models.Position,
) (*ConsensusResult, error) {
	logger.Info("🏛️ Submitting decision to validator council",
		zap.String("agent", vc.agentConfig.Name),
		zap.String("action", string(decision.Action)),
		zap.Int("confidence", decision.Confidence),
		zap.Int("validators", len(vc.validators)),
	)

	// Run all validators in parallel
	var wg sync.WaitGroup
	responses := make([]ValidatorResponse, len(vc.validators))
	errors := make([]error, len(vc.validators))

	for i, validator := range vc.validators {
		wg.Add(1)
		go func(idx int, v ValidatorSetup) {
			defer wg.Done()

			response, err := vc.runValidator(ctx, v, decision, marketData, position)
			if err != nil {
				errors[idx] = err
				logger.Warn("validator failed",
					zap.String("role", string(v.Role)),
					zap.String("provider", v.ProviderName),
					zap.Error(err),
				)
				// Set abstain verdict on error
				responses[idx] = ValidatorResponse{
					ValidatorRole: v.Role,
					ProviderName:  v.ProviderName,
					Verdict:       VerdictAbstain,
					Confidence:    0,
					Reasoning:     fmt.Sprintf("Validator error: %v", err),
				}
				return
			}

			responses[idx] = *response
		}(i, validator)
	}

	wg.Wait()

	// Calculate consensus
	result := vc.calculateConsensus(decision, responses)

	logger.Info("🏛️ Validator council decision",
		zap.String("agent", vc.agentConfig.Name),
		zap.String("original_action", string(decision.Action)),
		zap.String("verdict", string(result.FinalVerdict)),
		zap.Float64("approval_rate", result.ApprovalRate),
		zap.Bool("execution_allowed", result.ExecutionAllowed),
	)

	return result, nil
}

// runValidator executes single validator analysis
func (vc *ValidatorCouncil) runValidator(
	ctx context.Context,
	validator ValidatorSetup,
	decision *models.AgentDecision,
	marketData *models.MarketData,
	position *models.Position,
) (*ValidatorResponse, error) {
	// Build prompts from templates
	systemPrompt, userPrompt := vc.buildPromptsFromTemplates(validator.Role, decision, marketData, position)

	// Build validation request with pre-rendered prompts
	validationRequest := &models.ValidationRequest{
		ValidatorRole:     string(validator.Role),
		SystemPrompt:      systemPrompt,
		UserPrompt:        userPrompt,
		AgentDecision:     decision,
		AgentProfile:      vc.agentConfig,
		MarketData:        marketData,
		CurrentPosition:   position,
		RecentPerformance: nil, // TODO: Pass performance snapshot
	}

	// Use new ValidateDecision method for comprehensive validation
	aiResponse, err := validator.AIProvider.ValidateDecision(ctx, validationRequest)
	if err != nil {
		return nil, fmt.Errorf("validator evaluation failed: %w", err)
	}

	// Convert AI response to validator response
	response := &ValidatorResponse{
		ValidatorRole:     validator.Role,
		ProviderName:      validator.ProviderName,
		Verdict:           VerdictAbstain, // Default
		Confidence:        aiResponse.Confidence,
		Reasoning:         aiResponse.Reasoning,
		RiskConcerns:      fmt.Sprintf("%v", aiResponse.KeyRisks),
		RecommendedAction: aiResponse.RecommendedChanges,
	}

	// Parse verdict from AI response
	switch aiResponse.Verdict {
	case "APPROVE":
		response.Verdict = VerdictApprove
	case "REJECT":
		response.Verdict = VerdictReject
	case "ABSTAIN":
		response.Verdict = VerdictAbstain
	default:
		// Fallback: determine by confidence level
		if aiResponse.Confidence < 50 {
			response.Verdict = VerdictReject
		} else if aiResponse.Confidence >= 70 {
			response.Verdict = VerdictApprove
		} else {
			response.Verdict = VerdictAbstain
		}
	}

	logger.Debug("validator vote cast",
		zap.String("role", string(validator.Role)),
		zap.String("provider", validator.ProviderName),
		zap.String("verdict", string(response.Verdict)),
		zap.Int("confidence", response.Confidence),
	)

	return response, nil
}

// buildPromptsFromTemplates builds validation prompts using loaded templates
func (vc *ValidatorCouncil) buildPromptsFromTemplates(
	role ValidatorRole,
	decision *models.AgentDecision,
	marketData *models.MarketData,
	position *models.Position,
) (systemPrompt string, userPrompt string) {
	// If templates not loaded, use fallback
	if vc.templateManager == nil {
		return vc.buildFallbackPrompt(role, decision, marketData)
	}

	// Select template based on role
	var templateName string
	switch role {
	case RoleRiskManager:
		templateName = "risk_manager.tmpl"
	case RoleTechnicalExpert:
		templateName = "technical_expert.tmpl"
	case RoleMarketPsychologist:
		templateName = "market_psychologist.tmpl"
	default:
		templateName = "risk_manager.tmpl"
	}

	// Build request data for template
	requestData := &models.ValidationRequest{
		ValidatorRole:     string(role),
		AgentDecision:     decision,
		AgentProfile:      vc.agentConfig,
		MarketData:        marketData,
		CurrentPosition:   position,
		RecentPerformance: nil,
	}

	// Render template
	renderedPrompt, err := vc.templateManager.ExecuteTemplate(templateName, requestData)
	if err != nil {
		logger.Warn("failed to render validator template, using fallback",
			zap.Error(err),
			zap.String("template", templateName),
		)
		return vc.buildFallbackPrompt(role, decision, marketData)
	}

	// System prompt is minimal, all content in user prompt
	systemPrompt = "You are a professional trading validator. Analyze the provided decision carefully and respond with structured JSON."
	userPrompt = renderedPrompt

	return systemPrompt, userPrompt
}

// buildFallbackPrompt creates basic prompt if templates unavailable
func (vc *ValidatorCouncil) buildFallbackPrompt(
	role ValidatorRole,
	decision *models.AgentDecision,
	marketData *models.MarketData,
) (systemPrompt string, userPrompt string) {
	systemPrompt = fmt.Sprintf(`You are a %s reviewing a trading decision.

Respond in JSON:
{
  "verdict": "APPROVE" | "REJECT" | "ABSTAIN",
  "confidence": 0-100,
  "reasoning": "Your analysis",
  "key_risks": ["risk1", "risk2"],
  "key_opportunities": ["opp1"],
  "recommended_changes": "What to change",
  "critical_concerns": "Red flags"
}`, role)

	userPrompt = fmt.Sprintf(`Decision: %s %s at $%.2f (confidence: %d%%)
Reason: %s

Market: 24h change %.2f%%

Validate this decision.`,
		decision.Action,
		decision.Symbol,
		marketData.Ticker.Last.InexactFloat64(),
		decision.Confidence,
		decision.Reason,
		marketData.Ticker.Change24h.InexactFloat64(),
	)

	return systemPrompt, userPrompt
}

// DEPRECATED: buildValidatorPrompt - old method, kept for reference
func (vc *ValidatorCouncil) _buildValidatorPrompt_DEPRECATED(
	role ValidatorRole,
	decision *models.AgentDecision,
	marketData *models.MarketData,
	_ *models.Position, // Reserved for future use
) string {
	baseContext := fmt.Sprintf(`You are a senior validator in a trading council reviewing a decision made by a junior trading agent.

Agent Profile: %s (%s personality)
Proposed Action: %s
Agent's Confidence: %d%%
Agent's Reasoning: %s

Market Context:
- Symbol: %s
- Current Price: $%.2f
- 24h Change: %.2f%%
- Volume 24h: $%.0f

`,
		vc.agentConfig.Name,
		vc.agentConfig.Personality,
		decision.Action,
		decision.Confidence,
		decision.Reason,
		marketData.Symbol,
		marketData.Ticker.Last.InexactFloat64(),
		marketData.Ticker.Change24h.InexactFloat64(),
		marketData.Ticker.Volume24h.InexactFloat64(),
	)

	// Add role-specific instructions
	var roleInstructions string
	switch role {
	case RoleRiskManager:
		roleInstructions = `Your Role: RISK MANAGER
Focus on downside protection and capital preservation.

Critical Questions:
1. What could go wrong with this trade?
2. Is the stop-loss adequate?
3. Is position sizing appropriate given market volatility?
4. Are there hidden risks the agent missed?
5. What's the worst-case scenario?

Approve only if:
- Risk/reward ratio is favorable (>2:1)
- Stop-loss is well-placed
- Market conditions are stable enough
- No major catalysts that could invalidate the thesis`

	case RoleTechnicalExpert:
		roleInstructions = `Your Role: TECHNICAL ANALYST
Focus on chart patterns, indicators, and price action.

Critical Questions:
1. Do technical indicators confirm this entry?
2. Is price at a good level (support/resistance)?
3. Is momentum aligned with the trade direction?
4. Are we near key technical levels?
5. What do multiple timeframes show?

Approve only if:
- Technical setup is clean and confirmed
- Entry timing is good (not chasing)
- Key indicators align
- Price structure supports the thesis`

	case RoleMarketPsychologist:
		roleInstructions = `Your Role: MARKET PSYCHOLOGIST
Focus on sentiment, news, and crowd behavior.

Critical Questions:
1. What is market sentiment right now?
2. Are we following or contrarian to the crowd?
3. Is there recent news that affects this trade?
4. Is this decision emotionally driven or rational?
5. Are we in a regime change?

Approve only if:
- Sentiment aligns with strategy
- No conflicting major news events
- Decision appears rational, not FOMO/panic
- Market psychology supports the move`
	}

	return baseContext + roleInstructions + `

Your Task: Evaluate this decision and provide:
1. VERDICT: APPROVE, REJECT, or ABSTAIN
2. CONFIDENCE: 0-100
3. REASONING: Why you approve/reject
4. RISKS: Specific concerns

Be thorough and critical. The agent trusts your judgment.`
}

// calculateConsensus determines final verdict from validator votes
func (vc *ValidatorCouncil) calculateConsensus(
	decision *models.AgentDecision,
	responses []ValidatorResponse,
) *ConsensusResult {
	totalWeight := 0.0
	approveWeight := 0.0
	rejectWeight := 0.0
	abstainWeight := 0.0

	approveCount := 0
	rejectCount := 0
	abstainCount := 0

	for i, response := range responses {
		weight := vc.validators[i].Weight

		totalWeight += weight

		switch response.Verdict {
		case VerdictApprove:
			approveWeight += weight
			approveCount++
		case VerdictReject:
			rejectWeight += weight
			rejectCount++
		case VerdictAbstain:
			abstainWeight += weight
			abstainCount++
		}
	}

	approvalRate := approveWeight / totalWeight
	rejectionRate := rejectWeight / totalWeight

	// Determine final verdict
	var finalVerdict ValidatorVerdict
	executionAllowed := false

	if vc.config.RequireUnanimous {
		// Unanimous mode: all must approve
		if approveCount == len(responses) {
			finalVerdict = VerdictApprove
			executionAllowed = true
		} else {
			finalVerdict = VerdictReject
		}
	} else {
		// Consensus threshold mode
		if approvalRate >= vc.config.ConsensusThreshold {
			finalVerdict = VerdictApprove
			executionAllowed = true
		} else if rejectionRate >= vc.config.ConsensusThreshold {
			finalVerdict = VerdictReject
		} else {
			finalVerdict = VerdictAbstain // No consensus
		}
	}

	// Calculate consensus score (how aligned are validators?)
	consensusScore := 0.0
	if approvalRate > rejectionRate && approvalRate > abstainWeight/totalWeight {
		consensusScore = approvalRate
	} else if rejectionRate > approvalRate {
		consensusScore = rejectionRate
	} else {
		consensusScore = 0.5 // Split vote
	}

	summary := vc.buildConsensusSummary(responses, finalVerdict, approvalRate)

	return &ConsensusResult{
		OriginalDecision: decision,
		ValidatorVotes:   responses,
		FinalVerdict:     finalVerdict,
		ConsensusScore:   consensusScore,
		ApprovalRate:     approvalRate,
		ExecutionAllowed: executionAllowed,
		ConsensusSummary: summary,
	}
}

// buildConsensusSummary creates human-readable consensus explanation
func (vc *ValidatorCouncil) buildConsensusSummary(
	responses []ValidatorResponse,
	verdict ValidatorVerdict,
	approvalRate float64,
) string {
	summary := fmt.Sprintf("🏛️ Validator Council Decision: %s (%.0f%% approval)\n\n", verdict, approvalRate*100)

	for _, response := range responses {
		emoji := "✅"
		switch response.Verdict {
		case VerdictReject:
			emoji = "❌"
		case VerdictAbstain:
			emoji = "⚪"
		}

		summary += fmt.Sprintf("%s %s (%s): %s [%d%% confidence]\n",
			emoji,
			response.ValidatorRole,
			response.ProviderName,
			response.Verdict,
			response.Confidence,
		)

		if response.Reasoning != "" {
			summary += fmt.Sprintf("   Reasoning: %s\n", response.Reasoning)
		}

		if response.RiskConcerns != "" {
			summary += fmt.Sprintf("   ⚠️ Concerns: %s\n", response.RiskConcerns)
		}
	}

	return summary
}
