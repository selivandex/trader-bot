package agents

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/ai"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
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
	Enabled                    bool
	MinConfidenceForValidation int               // Only validate if agent confidence >= this
	ValidateActions            []models.AIAction // Which actions to validate (typically BUY/SELL)
	ConsensusThreshold         float64           // 0.66 = 2/3 validators must approve
	RequireUnanimous           bool              // If true, all validators must approve
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
	Confidence        int    // 0-100
	Reasoning         string // Why approve/reject
	RiskConcerns      string // Specific risks identified
	RecommendedAction string // Alternative action if rejected
}

// ConsensusResult is final decision from validator council
type ConsensusResult struct {
	OriginalDecision *models.AgentDecision
	ValidatorVotes   []ValidatorResponse
	FinalVerdict     ValidatorVerdict
	ConsensusScore   float64 // 0.0-1.0, how aligned validators are
	ApprovalRate     float64 // % of validators who approved
	ExecutionAllowed bool
	ConsensusSummary string
}

// ValidatorCouncil manages multiple AI validators for consensus decision-making
type ValidatorCouncil struct {
	config      *ValidatorConfig
	validators  []ValidatorSetup
	agentConfig *models.AgentConfig
}

// NewValidatorCouncil creates new validator council
func NewValidatorCouncil(
	agentConfig *models.AgentConfig,
	aiProviders map[string]ai.AgenticProvider,
	config *ValidatorConfig,
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
	)

	return &ValidatorCouncil{
		config:      config,
		validators:  validators,
		agentConfig: agentConfig,
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
	// Build validator-specific prompt (for future use with custom validation method)
	_ = vc.buildValidatorPrompt(validator.Role, decision, marketData, position)

	// Use agentic provider's evaluation capability
	// We'll use EvaluateOption method with decision as an option
	option := vc.decisionToOption(decision, marketData)

	evaluation, err := validator.AIProvider.EvaluateOption(ctx, option, []models.SemanticMemory{})
	if err != nil {
		return nil, fmt.Errorf("validator evaluation failed: %w", err)
	}

	// Convert evaluation to validator response
	response := &ValidatorResponse{
		ValidatorRole: validator.Role,
		ProviderName:  validator.ProviderName,
		Confidence:    evaluation.ConfidenceScore,
		Reasoning:     evaluation.Reasoning,
	}

	// Determine verdict based on evaluation
	// If confidence < 50 → reject, >= 70 → approve, else abstain
	if evaluation.ConfidenceScore < 50 {
		response.Verdict = VerdictReject
		response.RiskConcerns = fmt.Sprintf("Risks: %v", evaluation.Risks)
		response.RecommendedAction = "HOLD or wait for better setup"
	} else if evaluation.ConfidenceScore >= 70 {
		response.Verdict = VerdictApprove
	} else {
		response.Verdict = VerdictAbstain
		response.Reasoning += " [Uncertain - neutral evaluation]"
	}

	logger.Debug("validator vote cast",
		zap.String("role", string(validator.Role)),
		zap.String("provider", validator.ProviderName),
		zap.String("verdict", string(response.Verdict)),
		zap.Int("confidence", response.Confidence),
	)

	return response, nil
}

// buildValidatorPrompt creates role-specific validation prompt
func (vc *ValidatorCouncil) buildValidatorPrompt(
	role ValidatorRole,
	decision *models.AgentDecision,
	marketData *models.MarketData,
	position *models.Position,
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

// decisionToOption converts agent decision to TradingOption for evaluation
func (vc *ValidatorCouncil) decisionToOption(
	decision *models.AgentDecision,
	marketData *models.MarketData,
) *models.TradingOption {
	return &models.TradingOption{
		Action:          decision.Action,
		Reasoning:       decision.Reason,
		ExpectedOutcome: fmt.Sprintf("Agent confidence: %d%%", decision.Confidence),
		EstimatedRisk:   vc.estimateRisk(decision),
		Timeframe:       "1h-24h", // Typical holding period
	}
}

// estimateRisk converts agent confidence to risk level
func (vc *ValidatorCouncil) estimateRisk(decision *models.AgentDecision) string {
	if decision.Confidence >= 80 {
		return "Low"
	} else if decision.Confidence >= 60 {
		return "Medium"
	}
	return "High"
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
		if response.Verdict == VerdictReject {
			emoji = "❌"
		} else if response.Verdict == VerdictAbstain {
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
