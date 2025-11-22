# Validator Council - Usage Examples

## Basic Usage

The Validator Council is automatically integrated into the agent decision cycle. When enabled, it validates trading decisions before execution.

### Example 1: Default Configuration

```go
// Create agent with default validator config
agent := agents.NewConservativeAgent(userID, "Safe Sam")

// Validation config is set automatically:
// - Enabled: true
// - MinConfidenceForValidation: 70
// - ConsensusThreshold: 0.66 (2/3 approval)
```

### Example 2: Custom Configuration

```go
// Create agent with custom validator settings
agent := &models.AgentConfig{
    UserID: userID,
    Name: "Custom Trader",
    Personality: models.PersonalityAggressive,
    // ... other settings ...
    ValidationConfig: &models.ValidationConfig{
        Enabled:                    true,
        MinConfidenceForValidation: 80,   // Only validate high-confidence decisions
        ConsensusThreshold:         0.80, // Require 4/5 approval
        RequireUnanimous:           false,
        ValidateOnlyHighRisk:       false,
    },
}
```

### Example 3: Disable Validation

```go
// For scalping or high-frequency strategies
agent := agents.NewScalperAgent(userID, "Fast Freddy")

// Validation is disabled by default for scalpers
// Or disable manually:
agent.ValidationConfig.Enabled = false
```

## Decision Flow Examples

### Scenario 1: Unanimous Approval ✅

```
Agent Decision:
  Action: OPEN_LONG BTC/USDT
  Confidence: 82%
  Reason: "RSI oversold (28), positive news, whale accumulation"

Validator Council:
  ✅ Risk Manager (Claude): APPROVE 85%
     "Risk/reward 3:1, stop-loss well-placed at $42,000"
  
  ✅ Market Psychologist (GPT): APPROVE 78%
     "Strong institutional buying, positive sentiment shift"
  
  ✅ Technical Expert (DeepSeek): APPROVE 80%
     "Clean breakout above resistance, MACD bullish"

Consensus: APPROVED (100% approval rate)
Result: ✅ Trade EXECUTED
```

### Scenario 2: Majority Approval ✅

```
Agent Decision:
  Action: OPEN_SHORT ETH/USDT
  Confidence: 75%
  Reason: "Overbought RSI, negative funding rate"

Validator Council:
  ✅ Risk Manager (Claude): APPROVE 72%
     "Acceptable risk/reward, good entry timing"
  
  ✅ Market Psychologist (GPT): APPROVE 65%
     "Sentiment turning bearish, but close call"
  
  ❌ Technical Expert (DeepSeek): REJECT 88%
     "Price at strong support, high risk of bounce"

Consensus: APPROVED (67% approval rate >= 66% threshold)
Result: ✅ Trade EXECUTED (with caution)
```

### Scenario 3: Rejected by Council ❌

```
Agent Decision:
  Action: OPEN_LONG SOL/USDT
  Confidence: 68%
  Reason: "FOMO on recent rally, high momentum"

Validator Council:
  ❌ Risk Manager (Claude): REJECT 90%
     "Chasing price, poor risk/reward at resistance"
  
  ⚪ Market Psychologist (GPT): ABSTAIN 55%
     "Mixed signals, retail FOMO building"
  
  ❌ Technical Expert (DeepSeek): REJECT 85%
     "Overbought on all timeframes, bearish divergence"

Consensus: REJECTED (33% approval < 66% threshold)
Result: ❌ Trade BLOCKED - Agent decision overruled
```

### Scenario 4: No Consensus (Tie) ⚪

```
Agent Decision:
  Action: OPEN_LONG AVAX/USDT
  Confidence: 70%
  Reason: "Mixed signals, waiting for confirmation"

Validator Council:
  ✅ Risk Manager (Claude): APPROVE 60%
     "Acceptable but not great setup"
  
  ❌ Market Psychologist (GPT): REJECT 62%
     "No clear catalyst, wait for better entry"
  
  ⚪ Technical Expert (DeepSeek): ABSTAIN 50%
     "Neutral technicals, could go either way"

Consensus: NO CONSENSUS (33% approve, 33% reject, 33% abstain)
Result: ❌ Trade NOT EXECUTED (no consensus = safe default)
```

## Validator Decision Matrix

| Agent Confidence | Validators Approve | Consensus Threshold | Result |
|-----------------|-------------------|---------------------|--------|
| 85% | 3/3 (100%) | 66% | ✅ EXECUTE |
| 75% | 2/3 (67%) | 66% | ✅ EXECUTE |
| 70% | 2/3 (67%) | 75% | ❌ BLOCK (below threshold) |
| 65% | 1/3 (33%) | 66% | ❌ BLOCK |
| 55% | N/A | N/A | ⚪ NOT VALIDATED (below min confidence) |

## Per-Personality Validator Settings

### Conservative Agent
- **Validates:** 70%+ confidence
- **Threshold:** 66% (2/3)
- **Rationale:** Even conservative agents benefit from validation

### Aggressive Agent
- **Validates:** 55%+ confidence
- **Threshold:** 75% (3/4) ← Stricter!
- **Rationale:** High risk requires more scrutiny

### Balanced Agent
- **Validates:** 65%+ confidence
- **Threshold:** 66% (2/3)
- **Rationale:** Standard validation

### Scalper Agent
- **Validates:** DISABLED
- **Rationale:** Too frequent, latency matters

### Swing Agent
- **Validates:** 68%+ confidence
- **Threshold:** 66% (2/3)
- **Rationale:** Position trades benefit from validation

### News Trader
- **Validates:** 65%+ confidence
- **Threshold:** 66% (2/3)
- **Rationale:** News-driven decisions need context check

### Whale Hunter
- **Validates:** 60%+ confidence
- **Threshold:** 66% (2/3)
- **Rationale:** On-chain signals validated by council

### Contrarian Agent
- **Validates:** 70%+ confidence
- **Threshold:** 75% (3/4) ← Stricter!
- **Rationale:** Going against crowd is risky

## Monitoring Validator Performance

### Log Output

```
2025-11-22 10:15:23 INFO 🏛️ Submitting decision to validator council
  agent=AggressiveTrader action=OPEN_LONG confidence=75

2025-11-22 10:15:26 DEBUG validator vote cast
  role=risk_manager provider=Claude verdict=APPROVE confidence=80

2025-11-22 10:15:27 DEBUG validator vote cast
  role=market_psychologist provider=GPT verdict=APPROVE confidence=75

2025-11-22 10:15:28 DEBUG validator vote cast
  role=technical_expert provider=DeepSeek verdict=REJECT confidence=85

2025-11-22 10:15:28 INFO 🏛️ Validator council verdict
  agent=AggressiveTrader verdict=APPROVE approval_rate=0.67 execution_allowed=true

2025-11-22 10:15:28 INFO ✅ Decision APPROVED by validator council, executing
  agent=AggressiveTrader action=OPEN_LONG
```

### Metrics to Track

1. **Validation Rate**
   - % of decisions validated
   - Target: 20-40% (only high confidence/risk)

2. **Approval Rate**
   - % of validated decisions approved
   - Target: 60-80% (not too strict, not too loose)

3. **Rejection Accuracy**
   - % of rejected decisions that would have been losers
   - Target: >70% (validators should be right)

4. **False Negatives**
   - % of approved decisions that resulted in losses
   - Target: <30% (some losses are normal)

## Best Practices

### ✅ DO

- Enable for production trading
- Use for new/unproven agents
- Enable in volatile markets
- Monitor rejection rates
- Review rejected decisions manually

### ❌ DON'T

- Enable for backtesting (too slow)
- Use for scalping (latency matters)
- Set threshold too high (>0.90)
- Ignore consistent rejections
- Blame validators for all losses

## Troubleshooting

### Problem: All decisions rejected

**Cause:** Validators too strict or agent too aggressive

**Solution:**
```go
// Lower consensus threshold
agent.ValidationConfig.ConsensusThreshold = 0.60 // 3/5 instead of 2/3

// Or lower min confidence
agent.ValidationConfig.MinConfidenceForValidation = 75
```

### Problem: No consensus reached

**Cause:** Validators equally divided

**Solution:**
```go
// Treat no consensus as rejection (safe default)
// This is built-in behavior - no action needed

// Or add 4th validator for tiebreaker
// (requires code changes)
```

### Problem: Validation too slow

**Cause:** 3 sequential AI calls

**Solution:**
```go
// Validators run in parallel (already implemented)
// Or disable for low-stakes decisions:
agent.ValidationConfig.ValidateOnlyHighRisk = true
```

## Future Enhancements

### Planned Features

1. **Validator Reputation Tracking**
   - Track which validator was right/wrong
   - Adjust weights based on accuracy

2. **Conditional Validation**
   - Only validate in specific market conditions
   - E.g., only in high volatility

3. **Custom Validator Roles**
   - User-defined validator perspectives
   - E.g., "Macro Analyst", "Liquidity Expert"

4. **Learning from Rejections**
   - Feed rejected decisions into agent memory
   - "Remember: validators rejected similar setup"

## Conclusion

The Validator Council provides a robust safety net for autonomous trading agents. By requiring consensus from multiple AI models with different perspectives, it significantly reduces the risk of executing poor trades while maintaining high approval rates for quality setups.

For position traders and investors, the 2-5 second validation latency is negligible compared to the risk reduction benefit.

