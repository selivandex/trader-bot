# Template Architecture

## Overview

All AI prompts are now managed through a centralized template system. This eliminates hardcoded prompts scattered across code and makes prompt engineering much easier.

## Template Structure

```
templates/
├── basic/                  # Basic trading AI prompts
│   ├── analyze.tmpl        # Market analysis and decision making
│   └── evaluate_news.tmpl  # News sentiment evaluation
│
├── agentic/                # Autonomous agent prompts
│   ├── reflection.tmpl         # Post-trade reflection
│   ├── generate_options.tmpl   # Option generation (brainstorming)
│   ├── evaluate_option.tmpl    # Critical evaluation of options
│   ├── final_decision.tmpl     # Final decision making
│   ├── create_plan.tmpl        # 24h planning
│   ├── self_analysis.tmpl      # Performance self-analysis
│   └── summarize_memory.tmpl   # Memory creation
│
├── validators/             # Multi-AI validator prompts
│   ├── risk_manager.tmpl       # Risk-focused validation (Claude)
│   ├── technical_expert.tmpl   # Technical analysis validation (DeepSeek)
│   └── market_psychologist.tmpl # Sentiment validation (GPT)
│
└── telegram/               # Telegram bot notifications
    ├── trade_executed.tmpl
    ├── agent_started.tmpl
    └── ... (30+ templates)
```

## Template Loading

All templates are loaded **ONCE at startup** in `cmd/bot/main.go`:

```go
// Load basic trading prompts
if err := ai.LoadBasicTemplates("./templates/basic"); err != nil {
    logger.Fatal("failed to load basic templates", zap.Error(err))
    panic(err) // Cannot function without templates
}

// Load agentic prompts
if err := ai.LoadAgenticTemplates("./templates/agentic"); err != nil {
    logger.Fatal("failed to load agentic templates", zap.Error(err))
    panic(err)
}

// Load telegram notification templates
templateManager, err := telegram.NewTemplateManager("./templates/telegram")
if err != nil {
    logger.Fatal("failed to load telegram templates", zap.Error(err))
    panic(err)
}
```

**No fallback prompts** - if templates fail to load, application panics. This is intentional:
- Templates are critical for AI functionality
- Better to fail fast than silently use degraded prompts
- Forces proper deployment setup

## Template Manager

**Location:** `pkg/templates/manager.go`

Unified template manager for all template types:

```go
type Manager struct {
    templates *template.Template
    directory string
}

// Load templates
tm, err := templates.NewManager("./templates/basic")

// Render template
output, err := tm.ExecuteTemplate("analyze.tmpl", data)
```

### Built-in Helper Functions

All templates have access to these helpers:

- **`float`** - Convert any type to float64 (handles decimals)
- **`printf`** - fmt.Sprintf formatting
- **`mul`** - Multiply two floats
- **`div`** - Divide (safe, returns 0 if divide by zero)
- **`add`** - Add two integers
- **`lt, gt, le, ge`** - Comparisons (<, >, <=, >=)
- **`eq, ne`** - Equality checks
- **`and`** - Logical AND

Example usage in template:
```go
Price: ${{printf "%.2f" (float .MarketData.Ticker.Last)}}
Percentage: {{printf "%.0f" (mul .Weight 100)}}%
{{if gt (float .RSI) 70}}Overbought{{end}}
```

## Template Format

All AI prompts use this separator format:

```
System instructions go here...
(explain role, output format, rules)

=== USER PROMPT ===

Actual data and task goes here...
(market data, decision to review, etc.)
```

The `SplitPrompt()` function automatically splits by `=== USER PROMPT ===`:
- Everything before → `systemPrompt`
- Everything after → `userPrompt`

## Basic Templates (`templates/basic/`)

### analyze.tmpl

**Purpose:** Standard AI trading analysis  
**Used by:** `Analyze()` method in all providers  
**Input data:**
- StrategyParams (position size, leverage, SL/TP)
- MarketData (ticker, indicators, candles)
- CurrentPosition
- Balance, Equity, DailyPnL
- RecentTrades

**Output:** Trading decision JSON

### evaluate_news.tmpl

**Purpose:** News sentiment and impact evaluation  
**Used by:** `EvaluateNews()` method  
**Input data:**
- Source, Title, Content
- AgeHours (how old is news)

**Output:** Sentiment, impact score, urgency

## Agentic Templates (`templates/agentic/`)

### reflection.tmpl

**Purpose:** Post-trade reflection and learning  
**Input:** ReflectionPrompt (trade experience, prior beliefs)  
**Output:** Analysis, lessons, memory to store

### generate_options.tmpl

**Purpose:** Brainstorm 3-5 trading options  
**Input:** TradingSituation (market data, memories)  
**Output:** Array of trading options with parameters

### evaluate_option.tmpl

**Purpose:** Critically evaluate one option  
**Input:** TradingOption + memories  
**Output:** Pros, cons, risks, score

### final_decision.tmpl

**Purpose:** Choose best option from evaluations  
**Input:** Array of OptionEvaluation  
**Output:** Final AI decision

### create_plan.tmpl

**Purpose:** Create 24h forward-looking plan  
**Input:** PlanRequest (market data, time horizon)  
**Output:** Scenarios, risk limits, triggers

### self_analysis.tmpl

**Purpose:** Agent self-evaluation and adaptation  
**Input:** PerformanceData (stats, signal performance)  
**Output:** Strengths, weaknesses, suggested changes

### summarize_memory.tmpl

**Purpose:** Extract key lesson from trade  
**Input:** TradeExperience  
**Output:** Memory summary (context, lesson, importance)

## Validator Templates (`templates/validators/`)

### risk_manager.tmpl (Claude)

**Role:** Conservative risk manager  
**Focus:** Capital preservation, downside protection  
**Checks:**
- Risk/reward ratio
- Stop-loss placement
- Position sizing
- Worst-case scenarios
- Agent performance state

### technical_expert.tmpl (DeepSeek)

**Role:** Technical analyst  
**Focus:** Charts, indicators, price action  
**Checks:**
- RSI levels (all timeframes)
- MACD histogram
- Bollinger Bands position
- Entry timing (support/resistance)
- Volume confirmation

### market_psychologist.tmpl (GPT)

**Role:** Sentiment analyst  
**Focus:** News, crowd behavior, emotional biases  
**Checks:**
- FOMO detection
- Panic selling detection
- Revenge trading (after losses)
- News impact and timing
- Crowd sentiment

## Prompt Building Flow

```
1. Application starts
   ↓
2. Load templates ONCE (main.go)
   - templates/basic/ → basicTemplates
   - templates/agentic/ → agenticTemplates
   - templates/validators/ → validatorTemplates
   ↓
3. AI Provider calls method (e.g., Analyze())
   ↓
4. Build prompts from template
   - Select template (analyze.tmpl)
   - Prepare data (market data, strategy)
   - Render template with data
   - Split into system + user prompts
   ↓
5. Send to AI API
   ↓
6. Parse JSON response
```

## Benefits

### Before (Hardcoded Prompts)

❌ Prompts scattered across 3 files  
❌ ~1500 lines of prompt text in Go code  
❌ Hard to edit and maintain  
❌ No reusability  
❌ Changes require code changes + rebuild  

### After (Template System)

✅ All prompts in `templates/` directory  
✅ Easy to edit (just change .tmpl file)  
✅ Reusable template manager  
✅ No code changes for prompt updates  
✅ Hot-reload possible (restart required currently)  
✅ Version control friendly (diffs show actual prompt changes)  

## Editing Prompts

To modify AI behavior, just edit the template:

```bash
# Edit validator prompt
vim templates/validators/risk_manager.tmpl

# Edit basic analysis
vim templates/basic/analyze.tmpl

# Restart application
make restart
```

No Go code changes needed!

## Template Best Practices

### DO ✅

- Use `float` helper for all decimal conversions
- Use `printf` for formatting numbers
- Check for nil with `{{if .Field}}`
- Keep prompts clear and specific
- Include examples in prompts
- Use separators for clarity

### DON'T ❌

- Access fields directly without nil check
- Hardcode values that should be data-driven
- Make prompts too generic
- Forget to update test data if changing schema

## Troubleshooting

### Templates not loading

**Error:** `failed to load basic templates`

**Solution:**
1. Check templates directory exists: `ls templates/basic/`
2. Check .tmpl files exist
3. Check template syntax (Go template errors)
4. Check file permissions

### Template render failed

**Error:** `failed to render analyze template`

**Cause:** Usually missing field in data or syntax error

**Solution:**
1. Check template uses correct field names
2. Verify data structure matches template expectations
3. Test template locally with sample data

### Empty prompts

**Symptom:** AI returns errors or nonsense

**Cause:** `SplitPrompt()` failed or separator missing

**Solution:**
- Ensure template contains `=== USER PROMPT ===` separator
- Check template output is not empty

## Migration Summary

**Migrated from hardcoded to templates:**

1. ✅ `prompts.go` → `templates/basic/`
2. ✅ `agentic_prompts.go` → `templates/agentic/`  
3. ✅ Validator prompts → `templates/validators/`
4. ✅ Telegram notifications → `templates/telegram/`

**Total:**
- ~1500 lines of prompt code → 14 template files
- 3 separate template loading points → 1 unified system
- No fallbacks → fail-fast approach
- All prompts now maintainable without code changes

## Future Enhancements

- **Hot-reload:** Watch templates directory and reload on changes
- **A/B testing:** Load different template versions for experimentation
- **User templates:** Allow users to customize agent prompts
- **Prompt versioning:** Track which template version generated which decision

