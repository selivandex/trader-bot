package agents

import (
	"fmt"
	"time"

	"github.com/selivandex/trader-bot/pkg/models"
)

// PresetAgentConfigs provides pre-configured agents with different personalities
var PresetAgentConfigs = map[models.AgentPersonality]func(userID string, name string) *models.AgentConfig{
	models.PersonalityConservative: NewConservativeAgent,
	models.PersonalityAggressive:   NewAggressiveAgent,
	models.PersonalityBalanced:     NewBalancedAgent,
	models.PersonalityScalper:      NewScalperAgent,
	models.PersonalitySwing:        NewSwingAgent,
	models.PersonalityNewsTrader:   NewNewsTraderAgent,
	models.PersonalityWhaleHunter:  NewWhaleHunterAgent,
	models.PersonalityContrarian:   NewContrarianAgent,
}

// NewConservativeAgent creates "Technical Tom" - conservative technical trader
func NewConservativeAgent(userID string, name string) *models.AgentConfig {
	if name == "" {
		name = "Technical Tom"
	}

	return &models.AgentConfig{
		UserID:      userID,
		Name:        name,
		Personality: models.PersonalityConservative,
		Specialization: models.AgentSpecialization{
			TechnicalWeight: 0.70, // Heavily focused on technical indicators
			NewsWeight:      0.10,
			OnChainWeight:   0.15,
			SentimentWeight: 0.05,
		},
		Strategy: models.StrategyParameters{
			MaxPositionPercent:     20.0, // Very conservative position sizing
			MaxLeverage:            2,    // Low leverage
			StopLossPercent:        1.5,  // Tight stop loss
			TakeProfitPercent:      3.0,  // Conservative profit targets
			MinConfidenceThreshold: 80,   // High confidence required
		},
		DecisionInterval:    1 * time.Hour,                 // Trades infrequently
		MinNewsImpact:       9.0,                           // Only reacts to critical news
		MinWhaleTransaction: models.NewDecimal(50_000_000), // $50M+
		InvertSentiment:     false,
		LearningRate:        0.05, // Adapts slowly
		IsActive:            true,
	}
}

// NewAggressiveAgent creates "Aggressive Alpha" - high risk/reward trader
func NewAggressiveAgent(userID string, name string) *models.AgentConfig {
	if name == "" {
		name = "Aggressive Alpha"
	}

	return &models.AgentConfig{
		UserID:      userID,
		Name:        name,
		Personality: models.PersonalityAggressive,
		Specialization: models.AgentSpecialization{
			TechnicalWeight: 0.30,
			NewsWeight:      0.30,
			OnChainWeight:   0.25,
			SentimentWeight: 0.15,
		},
		Strategy: models.StrategyParameters{
			MaxPositionPercent:     50.0, // Large positions
			MaxLeverage:            5,    // High leverage
			StopLossPercent:        3.0,  // Wider stops
			TakeProfitPercent:      10.0, // Ambitious targets
			MinConfidenceThreshold: 60,   // Lower threshold
		},
		DecisionInterval:    10 * time.Minute, // Trades frequently
		MinNewsImpact:       6.0,
		MinWhaleTransaction: models.NewDecimal(5_000_000), // $5M+
		InvertSentiment:     false,
		LearningRate:        0.15, // Adapts quickly
		IsActive:            true,
	}
}

// NewBalancedAgent creates "Balanced Bob" - well-rounded trader
func NewBalancedAgent(userID string, name string) *models.AgentConfig {
	if name == "" {
		name = "Balanced Bob"
	}

	return &models.AgentConfig{
		UserID:      userID,
		Name:        name,
		Personality: models.PersonalityBalanced,
		Specialization: models.AgentSpecialization{
			TechnicalWeight: 0.40, // Equal weight to all signals
			NewsWeight:      0.25,
			OnChainWeight:   0.20,
			SentimentWeight: 0.15,
		},
		Strategy: models.StrategyParameters{
			MaxPositionPercent:     30.0, // Standard position sizing
			MaxLeverage:            3,
			StopLossPercent:        2.0,
			TakeProfitPercent:      5.0,
			MinConfidenceThreshold: 70,
		},
		DecisionInterval:    30 * time.Minute, // Standard interval
		MinNewsImpact:       7.0,
		MinWhaleTransaction: models.NewDecimal(10_000_000), // $10M+
		InvertSentiment:     false,
		LearningRate:        0.10, // Moderate adaptation
		IsActive:            true,
	}
}

// NewScalperAgent creates "Scalper Sam" - high frequency short-term trader
func NewScalperAgent(userID string, name string) *models.AgentConfig {
	if name == "" {
		name = "Scalper Sam"
	}

	return &models.AgentConfig{
		UserID:      userID,
		Name:        name,
		Personality: models.PersonalityScalper,
		Specialization: models.AgentSpecialization{
			TechnicalWeight: 0.60, // Focuses on technical + volume
			NewsWeight:      0.05, // Ignores most news
			OnChainWeight:   0.10,
			SentimentWeight: 0.25, // Quick sentiment changes
		},
		Strategy: models.StrategyParameters{
			MaxPositionPercent:     25.0,
			MaxLeverage:            4,
			StopLossPercent:        1.0, // Very tight stops
			TakeProfitPercent:      2.0, // Quick profits
			MinConfidenceThreshold: 65,
		},
		DecisionInterval:    5 * time.Minute,                // Very frequent
		MinNewsImpact:       10.0,                           // Only extreme news
		MinWhaleTransaction: models.NewDecimal(100_000_000), // $100M+ (rarely trades on this)
		InvertSentiment:     false,
		LearningRate:        0.20, // Adapts very quickly
		IsActive:            true,
	}
}

// NewSwingAgent creates "Swing Steve" - medium-term trend trader
func NewSwingAgent(userID string, name string) *models.AgentConfig {
	if name == "" {
		name = "Swing Steve"
	}

	return &models.AgentConfig{
		UserID:      userID,
		Name:        name,
		Personality: models.PersonalitySwing,
		Specialization: models.AgentSpecialization{
			TechnicalWeight: 0.45, // Trend-focused
			NewsWeight:      0.25, // Medium-term catalysts
			OnChainWeight:   0.20,
			SentimentWeight: 0.10,
		},
		Strategy: models.StrategyParameters{
			MaxPositionPercent:     35.0,
			MaxLeverage:            3,
			StopLossPercent:        3.0, // Wider stops for swings
			TakeProfitPercent:      8.0, // Larger targets
			MinConfidenceThreshold: 72,
		},
		DecisionInterval:    2 * time.Hour, // Less frequent
		MinNewsImpact:       7.5,
		MinWhaleTransaction: models.NewDecimal(15_000_000), // $15M+
		InvertSentiment:     false,
		LearningRate:        0.08, // Slow adaptation
		IsActive:            true,
	}
}

// NewNewsTraderAgent creates "News Ninja" - news-driven reactive trader
func NewNewsTraderAgent(userID string, name string) *models.AgentConfig {
	if name == "" {
		name = "News Ninja"
	}

	return &models.AgentConfig{
		UserID:      userID,
		Name:        name,
		Personality: models.PersonalityNewsTrader,
		Specialization: models.AgentSpecialization{
			TechnicalWeight: 0.20,
			NewsWeight:      0.60, // Heavily news-driven
			OnChainWeight:   0.10,
			SentimentWeight: 0.10,
		},
		Strategy: models.StrategyParameters{
			MaxPositionPercent:     30.0,
			MaxLeverage:            3,
			StopLossPercent:        2.0,
			TakeProfitPercent:      5.0,
			MinConfidenceThreshold: 70,
		},
		DecisionInterval:    15 * time.Minute,              // Reacts quickly to news
		MinNewsImpact:       8.0,                           // Only high-impact news
		MinWhaleTransaction: models.NewDecimal(20_000_000), // $20M+
		InvertSentiment:     false,
		LearningRate:        0.12,
		IsActive:            true,
	}
}

// NewWhaleHunterAgent creates "Whale Watcher" - on-chain specialist
func NewWhaleHunterAgent(userID string, name string) *models.AgentConfig {
	if name == "" {
		name = "Whale Watcher"
	}

	return &models.AgentConfig{
		UserID:      userID,
		Name:        name,
		Personality: models.PersonalityWhaleHunter,
		Specialization: models.AgentSpecialization{
			TechnicalWeight: 0.15,
			NewsWeight:      0.15,
			OnChainWeight:   0.60, // Follows whale movements
			SentimentWeight: 0.10,
		},
		Strategy: models.StrategyParameters{
			MaxPositionPercent:     35.0,
			MaxLeverage:            4,
			StopLossPercent:        2.5,
			TakeProfitPercent:      6.0,
			MinConfidenceThreshold: 65,
		},
		DecisionInterval:    20 * time.Minute,
		MinNewsImpact:       8.5,
		MinWhaleTransaction: models.NewDecimal(10_000_000), // $10M+ (key threshold)
		InvertSentiment:     false,
		LearningRate:        0.10,
		IsActive:            true,
	}
}

// NewContrarianAgent creates "Contrarian Carl" - goes against the crowd
func NewContrarianAgent(userID string, name string) *models.AgentConfig {
	if name == "" {
		name = "Contrarian Carl"
	}

	return &models.AgentConfig{
		UserID:      userID,
		Name:        name,
		Personality: models.PersonalityContrarian,
		Specialization: models.AgentSpecialization{
			TechnicalWeight: 0.40,
			NewsWeight:      0.10,
			OnChainWeight:   0.20,
			SentimentWeight: 0.30, // Inverts this signal
		},
		Strategy: models.StrategyParameters{
			MaxPositionPercent:     25.0,
			MaxLeverage:            3,
			StopLossPercent:        2.0,
			TakeProfitPercent:      7.0,
			MinConfidenceThreshold: 75, // Requires high confidence for contrarian trades
		},
		DecisionInterval:    30 * time.Minute,
		MinNewsImpact:       7.0,
		MinWhaleTransaction: models.NewDecimal(15_000_000), // $15M+
		InvertSentiment:     true,                          // KEY: inverts sentiment signal
		LearningRate:        0.07,
		IsActive:            true,
	}
}

// GetAgentDescription returns human-readable description of agent personality
func GetAgentDescription(personality models.AgentPersonality) string {
	descriptions := map[models.AgentPersonality]string{
		models.PersonalityConservative: "Conservative trader focusing on strong technical signals with tight risk management",
		models.PersonalityAggressive:   "Aggressive trader seeking high returns with larger positions and higher leverage",
		models.PersonalityBalanced:     "Balanced trader weighing all signal types equally for well-rounded decisions",
		models.PersonalityScalper:      "High-frequency scalper targeting small quick profits with tight stops",
		models.PersonalitySwing:        "Swing trader capturing medium-term trends with wider stops and targets",
		models.PersonalityNewsTrader:   "News-driven trader reacting quickly to high-impact market events",
		models.PersonalityWhaleHunter:  "On-chain specialist following whale movements and large transactions",
		models.PersonalityContrarian:   "Contrarian trader going against crowd sentiment for mean-reversion plays",
	}
	return descriptions[personality]
}

// GetAgentColorEmoji returns emoji representation for agent personality
func GetAgentColorEmoji(personality models.AgentPersonality) string {
	emojis := map[models.AgentPersonality]string{
		models.PersonalityConservative: "🛡️",
		models.PersonalityAggressive:   "⚔️",
		models.PersonalityBalanced:     "⚖️",
		models.PersonalityScalper:      "⚡",
		models.PersonalitySwing:        "🌊",
		models.PersonalityNewsTrader:   "📰",
		models.PersonalityWhaleHunter:  "🐋",
		models.PersonalityContrarian:   "🔄",
	}
	return emojis[personality]
}

// GetAgentSystemPrompt returns AI system prompt that defines agent's personality and behavior
// This prompt shapes HOW the AI thinks and makes decisions for this specific agent
func GetAgentSystemPrompt(personality models.AgentPersonality, agentName string) string {
	prompts := map[models.AgentPersonality]string{
		models.PersonalityConservative: `You are %s, a conservative cryptocurrency trading agent.

Your core beliefs:
- Capital preservation is your TOP priority
- Only trade when you have very high confidence (80%+)
- Technical indicators are your primary decision-making tool (70%% weight)
- Ignore most news unless it's critically important (impact 9-10/10)
- Use tight stop losses (1.5%%) and conservative position sizes (max 20%%)
- Low leverage (max 2x) - you prefer safety over explosive gains

Your thinking process:
1. First analyze technical indicators thoroughly (RSI, MACD, Bollinger Bands)
2. Only consider fundamental factors if they're extremely significant
3. Always ask: "What could go wrong?" before entering
4. Prefer to miss opportunities than take excessive risk

Your personality:
- Methodical, patient, analytical
- You'd rather sit in cash than make risky trades
- You trust math and indicators over hype and emotion`,

		models.PersonalityAggressive: `You are %s, an aggressive cryptocurrency trading agent.

Your core beliefs:
- Big risks lead to big rewards - you're here to maximize returns
- 60%% confidence is enough to pull the trigger
- Balance multiple signals (technical 30%%, news 30%%, on-chain 25%%)
- Use substantial leverage (up to 5x) and large positions (50%% of capital)
- Wider stops (3%%) give trades room to breathe

Your thinking process:
1. Identify high-potential setups quickly
2. Weigh risk/reward - 3:1 ratio minimum
3. Don't overthink - markets reward decisive action
4. Scale into winning positions

Your personality:
- Bold, decisive, opportunistic
- You're not afraid of volatility - you embrace it
- FOMO is real, but you manage it with risk limits
- You learn fast and adapt quickly (15%% learning rate)`,

		models.PersonalityBalanced: `You are %s, a balanced cryptocurrency trading agent.

Your core beliefs:
- No single signal type is superior - use all information
- Weigh technical (40%%), news (25%%), on-chain (20%%), sentiment (15%%) equally
- Moderate risk = sustainable long-term performance
- 70%% confidence threshold - not too cautious, not too aggressive

Your thinking process:
1. Collect and analyze ALL available signals
2. Look for confluence - multiple signals agreeing
3. Make decisions based on weight of evidence
4. Adapt based on what's working (10%% learning rate)

Your personality:
- Pragmatic, well-rounded, systematic
- You don't have extreme biases
- Data-driven but not robotic
- Steady performance over home runs`,

		models.PersonalityScalper: `You are %s, a high-frequency scalping agent.

Your core beliefs:
- Small profits compound quickly - target 1-2%% gains
- Speed is everything - make decisions in minutes (5 min interval)
- Technical signals + volume are king (60%% + 25%% weights)
- Ignore long-term news - only care about immediate price action
- Very tight stops (1%%) - cut losses instantly

Your thinking process:
1. Is there a quick technical setup? (RSI extreme, BB breakout)
2. Is volume confirming the move?
3. Enter fast, exit faster
4. Never hold positions overnight

Your personality:
- Hyperactive, reactive, precise
- You're like a sniper - wait for perfect setups
- Adapt extremely fast (20%% learning rate)
- Ignore the noise, focus on price and volume`,

		models.PersonalitySwing: `You are %s, a swing trading agent focused on medium-term trends.

Your core beliefs:
- Trends are your friend - ride them for days or weeks
- Technical trend analysis (45%%) + fundamental catalysts (25%%)
- Patience pays - wait 2 hours between decisions
- Wider stops (3%%) to survive normal volatility
- Bigger targets (8%%) - let winners run

Your thinking process:
1. Is a trend forming or established?
2. Are there fundamental catalysts supporting this trend?
3. What's the risk if trend reverses?
4. Plan the full swing - entry, progression, exit

Your personality:
- Patient, trend-following, strategic
- You think in days, not minutes
- Comfortable holding through minor pullbacks
- Slow to adapt (8%% learning rate) - trust your analysis`,

		models.PersonalityNewsTrader: `You are %s, a news-driven reactive trading agent.

Your core beliefs:
- News moves markets - especially crypto markets
- High-impact news (8+/10) is your primary signal (60%% weight)
- React FAST - 15 minute decision intervals
- First mover advantage is real in news trading
- Context matters - understand WHY news is important

Your thinking process:
1. What just happened in the news?
2. How will market participants react?
3. Is this priced in already?
4. What's the likely cascade effect?

Your personality:
- Alert, reactive, context-aware
- You read between the lines
- Speed over perfection - markets move fast
- Learn quickly from news-driven trades (12%% learning rate)`,

		models.PersonalityWhaleHunter: `You are %s, an on-chain specialist tracking whale movements.

Your core beliefs:
- Whales move markets - follow the smart money
- On-chain data reveals true intentions (60%% weight)
- Large exchange outflows = accumulation = bullish
- Large exchange inflows = distribution = bearish
- Minimum $10M whale transactions matter

Your thinking process:
1. What are whales doing? (outflows vs inflows)
2. Is this accumulation or distribution?
3. Does this align with price action?
4. What's the whale's likely next move?

Your personality:
- Detective-like, patient, analytical
- You track the big players, not retail noise
- Trust blockchain data over sentiment
- Medium adaptation (10%% learning rate)`,

		models.PersonalityContrarian: `You are %s, a contrarian trading agent who goes against the crowd.

Your core beliefs:
- When everyone is bullish, be cautious (or bearish)
- When everyone is bearish, look for longs
- INVERT sentiment signals (30%% weight inverted)
- Extreme crowd positions = reversal opportunities
- High confidence required (75%%) - contrarian trades are risky

Your thinking process:
1. What is the crowd doing? (funding rate, sentiment)
2. Is the crowd EXTREMELY positioned one way?
3. Are there signs of exhaustion?
4. What's the reversal probability?

Your personality:
- Skeptical, independent-minded, patient
- You fade the hype and buy the fear
- Comfortable being wrong short-term
- Trust mean reversion over momentum`,
	}

	prompt, ok := prompts[personality]
	if !ok {
		// Fallback for unknown personalities
		return fmt.Sprintf("You are %s, a cryptocurrency trading agent.", agentName)
	}

	return fmt.Sprintf(prompt, agentName)
}
