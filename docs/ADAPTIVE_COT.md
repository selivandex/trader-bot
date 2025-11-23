<!-- @format -->

# Adaptive Chain-of-Thought (True AI Reasoning)

## Overview

**Adaptive CoT** is the next evolution of agent reasoning. Unlike the fixed pipeline approach, agents **decide what to do next** at each iteration, making them truly autonomous.

## Fixed vs Adaptive CoT

### Fixed Pipeline (Old)

```
ALWAYS:
1. Observe
2. Recall memories
3. Generate options  
4. Evaluate each
5. Decide
DONE (always 5 steps, 5 API calls)
```

**Problems:**
- ❌ Rigid structure
- ❌ Can't adapt to situation
- ❌ Tools only mentioned, not used
- ❌ Can't ask questions
- ❌ Can't reconsider

### Adaptive Reasoning (New)

```
Iteration 1: "What should I do first?"
→ Agent: "Check volatility"
→ Action: use_tool(CalculateVolatility)
→ Result: ATR = 450

Iteration 2: "What next?"
→ Agent: "Am I near support?"
→ Action: use_tool(FindSupportLevels)
→ Result: Support at $42,800

Iteration 3: "What next?"
→ Agent: "Did this work before?"
→ Action: recall_memory("near support entry")
→ Result: 2/3 profitable

Iteration 4: "What next?"
→ Agent: "Check breaking news?"
→ Action: use_tool(GetHighImpactNews)
→ Result: No breaking news

Iteration 5: "What next?"
→ Agent: "Ready to generate options"
→ Action: generate_options
→ Result: [Long 2x, Long 3x, Wait]

Iteration 6: "What next?"
→ Agent: "Evaluate the options"
→ Action: evaluate_option
→ Result: Long 2x scores 81/100

Iteration 7: "What next?"
→ Agent: "Confident enough to decide"
→ Action: decide
→ DONE (7 iterations, variable)
```

**Benefits:**
- ✅ Flexible (3-20 iterations)
- ✅ Tools actually used
- ✅ Self-questioning
- ✅ Can reconsider
- ✅ Stops when confident

## Architecture

```go
type AdaptiveCoTEngine struct {
    config     *AgentConfig
    aiProvider AgenticProvider
    toolkit    AgentToolkit  // Dynamic tool execution
}

func ThinkAdaptively() {
    state := InitialState()
    history := []ThoughtStep{}
    
    for iteration := 1; iteration <= 20; iteration++ {
        // Agent decides next step
        nextStep := DecideNextStep(state, history)
        
        // Execute action
        switch nextStep.Action {
        case "use_tool":
            result := toolkit.ExecuteTool(nextStep.Tool, nextStep.Params)
            state.AddToolResult(result)
            
        case "ask_question":
            answer := AnswerQuestion(nextStep.Question, state)
            state.AddAnswer(answer)
            
        case "decide":
            return FinalizeDecision(state)
        }
        
        history.Append(nextStep)
    }
}
```

## Thinking State

Agent maintains context across iterations:

```go
type ThinkingState struct {
    Observation      string
    MarketData       *MarketData
    RecalledMemories []SemanticMemory
    ToolResults      map[string]interface{}  // Accumulated tool results
    Questions        []QuestionAnswer        // Self-questioning log
    Options          []TradingOption         // Generated when ready
    Evaluations      []OptionEvaluation      // Evaluated when ready
    Insights         []string                // Discovered insights
    IterationCount   int
}
```

## Available Actions

At each iteration, agent chooses:

### 1. **use_tool** - Gather Data

```json
{
  "action": "use_tool",
  "tool_name": "CalculateVolatility",
  "tool_params": {"symbol": "BTC/USDT", "timeframe": "1h", "period": 14},
  "reasoning": "Need to check if volatility is acceptable for trade"
}
```

**When:** Need specific data not in current context

### 2. **ask_question** - Self-Clarification

```json
{
  "action": "ask_question",
  "question": "Is this risk too high for my conservative personality?",
  "reasoning": "Need to align decision with my risk profile"
}
```

**When:** Need to structure thinking or identify gaps

### 3. **recall_memory** - Learn from Past

```json
{
  "action": "recall_memory",
  "question": "profitable entries near support",
  "reasoning": "Want to see if this setup worked before"
}
```

**When:** Need context from similar situations

### 4. **generate_options** - Brainstorm

```json
{
  "action": "generate_options",
  "reasoning": "I have enough data - volatility checked, support found, news clear"
}
```

**When:** Gathered sufficient context, ready to explore strategies

### 5. **evaluate_option** - Critique

```json
{
  "action": "evaluate_option",
  "reasoning": "Options generated, now need to assess each critically"
}
```

**When:** Options exist, need pros/cons/risks analysis

### 6. **decide** - Commit

```json
{
  "action": "decide",
  "reasoning": "Evaluations complete, Long 2x scores 85/100, I'm confident",
  "confidence": 0.85
}
```

**When:** High confidence (>70%), evaluations complete

### 7. **alert_owner** - Escalate

```json
{
  "action": "alert_owner",
  "reasoning": "Position near liquidation! Current drawdown 18%",
  "confidence": 0.95
}
```

**When:** Critical situation needs human awareness

### 8. **log_insight** - Learn

```json
{
  "action": "log_insight",
  "reasoning": "Discovered: 1m + 1h trend alignment = 90% success rate"
}
```

**When:** Found valuable pattern to remember

### 9. **reconsider** - Rethink

```json
{
  "action": "reconsider",
  "reconsider_what": "options",
  "reasoning": "Just saw breaking news - need to regenerate options"
}
```

**When:** New information contradicts previous thinking

## Example Session

```
Agent: Technical Tom
Session: adaptive-cot-abc123-1700000000

Iteration 1:
Action: use_tool(CalculateVolatility)
Reasoning: "Need to check if market is stable enough"
Result: ATR = 380 (moderate)

Iteration 2:
Action: ask_question
Question: "Is 380 ATR acceptable for my conservative style?"
Answer: "Yes, my risk tolerance allows up to 500 ATR"

Iteration 3:
Action: use_tool(FindSupportLevels)
Reasoning: "Want to enter near support for better R:R"
Result: [42800, 42200, 41500]

Iteration 4:
Action: use_tool(GetHighImpactNews)
Reasoning: "Check for breaking news before committing"
Result: No high impact news in last hour

Iteration 5:
Action: recall_memory
Question: "entries near support level"
Result: 3 memories, 2 profitable

Iteration 6:
Action: log_insight
Reasoning: "Support entries worked 67% of time historically"

Iteration 7:
Action: generate_options
Reasoning: "I have: volatility OK, near support, no news, historical success"
Result: [Long 2x conservative, Long 3x aggressive, Wait for breakout]

Iteration 8:
Action: evaluate_option
Reasoning: "Need to assess each option's risk"
Result: Long 2x = 81/100, Long 3x = 68/100, Wait = 55/100

Iteration 9:
Action: use_tool(CalculatePositionRisk)
Tool Params: {side: "long", size: 0.5, leverage: 2, stopLoss: 42800}
Reasoning: "Validate risk before deciding"
Result: Risk score 4/10 (acceptable)

Iteration 10:
Action: decide
Reasoning: "Long 2x scores highest (81), risk acceptable (4/10), aligns with conservative personality"
Confidence: 0.81
Decision: OPEN_LONG

DONE - 10 iterations, 5 tools used, 1 question asked
```

## Integration

### Enable Adaptive CoT

```go
// In manager.go - instead of regular CoT
adaptiveCot := NewAdaptiveCoTEngine(
    config,
    aiProvider,
    memoryManager,
    toolkit,  // Toolkit is essential for adaptive reasoning
)

decision, trace := adaptiveCot.ThinkAdaptively(ctx, marketData, position)
```

### Template Structure

```
templates/agentic/
├── _toolkit_tools.tmpl       # Partial: Available tools
├── _reasoning_actions.tmpl   # Partial: Available actions (NEW!)
└── adaptive_think.tmpl        # Main: Uses both partials
```

## Benefits

### 1. True Autonomy
Agent decides its own reasoning path, not following script.

### 2. Tool Integration
Tools are **actually called** based on agent's decisions, not just listed.

### 3. Adaptability
- Simple situations: 5-7 iterations
- Complex situations: 15-20 iterations  
- Critical situations: Can alert owner mid-reasoning

### 4. Self-Awareness
Agent asks itself questions:
- "Do I have enough info?"
- "Is this too risky?"
- "What am I missing?"

### 5. Learning
- Logs insights during thinking
- Can reconsider when new data appears
- Builds better reasoning over time

## Cost Considerations

### Fixed CoT (Old)
- Always 5 API calls
- Cost: ~$0.02 per decision (Claude)

### Adaptive CoT (New)
- Variable 3-20 API calls
- Average: ~10 calls
- Cost: ~$0.04 per decision
- **But:** Much smarter decisions

### Optimization
- Set max iterations per personality:
  - Scalper: max 7 iterations (fast decisions)
  - Swing trader: max 15 iterations (thorough analysis)
- Cache tool results within session
- Skip iterations if confidence > 0.9 early

## Future Enhancements

- [ ] Multi-agent consultation (agent asks other agents)
- [ ] Parallel tool execution (call multiple tools at once)
- [ ] Learning optimal iteration count
- [ ] Adaptive max_iterations based on market volatility
- [ ] Tool result caching across agents
- [ ] Reasoning replay for debugging

---

**Adaptive CoT transforms agents from following scripts to true autonomous thinking.**

