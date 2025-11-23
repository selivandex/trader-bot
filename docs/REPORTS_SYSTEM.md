<!-- @format -->

# Agent Reports System

## Overview

Автоматическая система отчетов для агентов с тремя способами использования:

1. **Автоматически** - агент отправляет отчет каждый день в 00:00:01
2. **По команде** - пользователь запрашивает через Telegram `/report`
3. **Из кода** - агент может вызвать `toolkit.GenerateDailyReport()` сам

## Architecture

```
┌──────────────────────────────────────────────────┐
│            REPORTS PACKAGE                        │
│  internal/reports/                                │
├──────────────────────────────────────────────────┤
│  generator.go    - Генерация отчетов             │
│  repository.go   - Интерфейс для данных           │
│  scheduler.go    - Автоматическая отправка       │
│  types.go        - Модели отчетов                 │
└──────────────────┬───────────────────────────────┘
                   │
                   ↓ (используется)
┌────────────────────────────────────────────────────┐
│   Toolkit → Agent может вызвать                    │
│   Scheduler → Отправляет в 00:00:01                │
│   Telegram → Команда /report                       │
└────────────────────────────────────────────────────┘
```

## Types of Reports

### 1. Daily Report (Ежедневный)

```go
type DailyReport struct {
    AgentName   string
    Symbol      string
    Date        time.Time

    Metrics: {
        TotalDecisions      int    // 48 (каждые 30 мин)
        ExecutedTrades      int    // 3
        HoldCount           int    // 45
        WinRate             float64 // 0.667 (66.7%)
        TotalPnL            float64 // +125.50
        BestTrade           float64 // +80.00
        WorstTrade          float64 // -30.00
        LongCount/ShortCount int
        HighConfidenceCount int    // Решения с 80%+ уверенностью
    }

    Decisions []AgentDecision  // Все решения за день
    Insights  []string         // Ключевые инсайты
}
```

**Когда генерируется:**

- Автоматически в 00:00:01 (за вчера)
- По команде `/report today` или `/report yesterday`
- Агент может вызвать `toolkit.SendDailyReportToOwner()`

### 2. Weekly Report (Недельный)

```go
type WeeklyReport struct {
    WeekStart/End time.Time
    DailyReports  []*DailyReport  // 7 дней
    WeekMetrics: {
        TradingDays    int
        ExecutedTrades int
        WinRate        float64
        TotalPnL       float64
        BestDay        float64
        WorstDay       float64
        SharpeRatio    float64
    }
}
```

**Когда генерируется:**

- По команде `/report week`
- Каждое воскресенье в 23:59 (опционально)

### 3. Custom Report (Произвольный период)

```go
request := ReportRequest{
    AgentID: "abc123",
    Symbol:  "BTC/USDT",
    Period:  PeriodCustom,
    StartDate: &start,
    EndDate:   &end,
}
```

**Когда:**

- `/report custom 2024-01-01 2024-01-31`
- Для анализа конкретного периода

## Usage

### 1. Agent Автоматически (в 00:00:01)

```go
// В AdaptiveCoTEngine или manager
func (cot *AdaptiveCoTEngine) checkMidnightReport(ctx context.Context) {
    now := time.Now()

    // Check if it's midnight
    if now.Hour() == 0 && now.Minute() == 0 {
        // Agent generates and sends own report
        if err := cot.toolkit.SendDailyReportToOwner(ctx); err != nil {
            logger.Error("failed to send daily report", zap.Error(err))
        }
    }
}
```

### 2. Telegram Command (on-demand)

```go
// internal/adapters/telegram/agent_bot.go

func (bot *AgentBot) handleReportCommand(ctx context.Context, userID, agentID string, args []string) error {
    period := "yesterday" // default
    if len(args) > 0 {
        period = args[0] // "today", "yesterday", "week"
    }

    var date time.Time
    switch period {
    case "today":
        date = time.Now()
    case "yesterday":
        date = time.Now().AddDate(0, 0, -1)
    case "week":
        // Generate weekly report
        return bot.sendWeeklyReport(ctx, userID, agentID)
    }

    // Use reports package
    generator := reports.NewGenerator(bot.agentRepo, bot.templates)
    report, err := generator.GenerateDailyReport(ctx, agentID, "BTC/USDT", date)
    if err != nil {
        return err
    }

    text, _ := generator.RenderDailyReport(ctx, report)
    bot.telegram.SendMessage(userID, text)

    return nil
}
```

### 3. Agent Вызывает Сам (через toolkit)

```go
// В adaptive CoT reasoning:
Iteration 15:
AI: "I've been trading all day, let me review my performance"
Action: use_tool("GenerateDailyReport", {date: "2024-01-15"})
Result: {text report with metrics}

Iteration 16:
AI: "Win rate only 40% today - should be more conservative"
Action: log_insight("Low win rate today, reduce position sizes")
```

## Templates

### Daily Report Template

```
templates/reports/daily_report.tmpl
```

Содержит:

- Trading activity (решения, сделки, холды)
- Performance (PnL, win rate, лучшая/худшая сделка)
- Key insights (важные открытия дня)

### Alert Templates

```
templates/alerts/
├── liquidation_risk.tmpl
├── max_drawdown.tmpl
├── breaking_news.tmpl
├── whale_alert.tmpl
├── conflicting_signals.tmpl
├── low_confidence.tmpl
├── losing_streak.tmpl
├── valuable_insight.tmpl
└── extreme_risk.tmpl
```

## Implementation

### Scheduler в Manager

```go
// internal/agents/manager.go

func (am *AgenticManager) startReportScheduler(ctx context.Context) {
    reportScheduler := reports.NewScheduler(
        reports.NewGenerator(am.repository, am.templateManager),
        am.notifier,
    )

    go reportScheduler.Start(ctx)
}
```

### Toolkit Integration

Агент имеет доступ к reporting tools:

```go
// Agent can call
toolkit.GenerateDailyReport(ctx, today)
toolkit.GenerateWeeklyReport(ctx, weekStart)
toolkit.SendDailyReportToOwner(ctx) // Auto-send
```

## Report Content Example

```
🤖 *Technical Tom* - Daily Report
📅 Monday, January 15, 2024

━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 TRADING ACTIVITY

Decisions Made: 48
Trades Executed: 3
Hold Decisions: 45

📈 Longs: 2
📉 Shorts: 1
🔄 Closes: 0

High Confidence (≥80%): 12

━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💰 PERFORMANCE

Win Rate: 66.7% (2W / 1L)
Total PnL: $125.50 📈

Best Trade: $80.00
Worst Trade: -$30.00

Balance: $1,000.00 → $1,125.50
Daily Return: +12.55%

━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💡 KEY INSIGHTS

1. Multi-timeframe alignment increased win rate
2. Support level entries performed well (2/2 profitable)
3. Avoided trading during high volatility (good decision)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏰ Generated: 00:00:15
```

## Benefits

### 1. Transparency

Владелец видит ВСЕ, что делал агент за день.

### 2. Accountability

Агент отчитывается о результатах automatically.

### 3. Learning

Insights показывают, что агент узнал.

### 4. On-Demand

Можно запросить отчет в любой момент.

### 5. Agent Self-Awareness

Агент может сам проанализировать свой день и скорректироваться.

## Future Enhancements

- [ ] Monthly reports
- [ ] Comparison with other agents in report
- [ ] Visual charts (equity curve, trade distribution)
- [ ] Export to PDF
- [ ] Email reports
- [ ] Slack/Discord integration
- [ ] Real-time performance dashboard

---

**Reports System делает агентов прозрачными и подотчетными владельцу.**
