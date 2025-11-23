<!-- @format -->

# Telegram Report Commands

## Overview

Команды для получения отчетов от агентов через Telegram бота.

## Available Commands

### 1. `/report` - Вчерашний отчет (default)

```
/report
/report AGENT_ID

→ Возвращает отчет за вчерашний день
```

**Example:**
```
User: /report agent-123

Bot:
🤖 *Technical Tom* - Daily Report
📅 Sunday, November 22, 2024

📊 Trading Activity: 48 decisions, 3 executed
💰 Performance: +$125.50 (12.5%)
Win Rate: 66.7% (2W/1L)
...
```

---

### 2. `/report today` - Сегодняшний отчет

```
/report AGENT_ID today
/report today

→ Отчет за текущий день (до текущего момента)
```

---

### 3. `/report yesterday` - Вчерашний отчет

```
/report AGENT_ID yesterday

→ Отчет за вчера (полные 24 часа)
```

---

### 4. `/report week` - Недельный отчет

```
/report AGENT_ID week
/report week

→ Отчет за последние 7 дней
```

**Example:**
```
User: /report agent-123 week

Bot:
📅 Weekly Report
Nov 16 - Nov 22, 2024

Trading Days: 7/7
Total Trades: 15
Win Rate: 60% (9W/6L)
Total PnL: +$487.30
Sharpe: 1.85
...
```

---

### 5. `/report custom` - Произвольный период

```
/report AGENT_ID custom START END
/report custom 2024-01-01 2024-01-31

→ Отчет за указанный период
```

**Date Formats:**
- `YYYY-MM-DD` - полная дата
- `today`, `yesterday` - ключевые слова
- `7d`, `30d` - N дней назад
- `this_week`, `last_week`
- `this_month`, `last_month`

**Examples:**
```bash
# Последние 7 дней
/report agent-123 custom 7d today

# Январь 2024
/report agent-123 custom 2024-01-01 2024-01-31

# Эта неделя
/report agent-123 custom this_week today

# Последний месяц
/report agent-123 custom 30d today
```

---

### 6. `/report compare` - Сравнение агентов

```
/report compare AGENT_ID1 AGENT_ID2 [period]

→ Сравнительный отчет двух агентов
```

**Example:**
```
User: /report compare agent-123 agent-456 week

Bot:
📊 Agent Comparison - Last 7 Days

Technical Tom:
- Win Rate: 60%
- PnL: +$487

Aggressive Alpha:
- Win Rate: 55%
- PnL: +$623

🏆 Winner: Aggressive Alpha (+28% more profit)
```

---

### 7. `/report performance` - Детальная статистика

```
/report AGENT_ID performance [period]

→ Углубленный анализ производительности
```

**Includes:**
- Win rate by signal type
- Best/worst trades
- Average hold time
- Signal performance breakdown
- Sharpe ratio, max drawdown
- Profit factor

---

## Implementation in agent_bot.go

```go
// internal/adapters/telegram/agent_bot.go

func (bot *AgentBot) handleReportCommand(update tgbotapi.Update) error {
    ctx := context.Background()
    userID := fmt.Sprintf("%d", update.Message.From.ID)
    
    // Parse command: /report [AGENT_ID] [period] [start] [end]
    args := parseArgs(update.Message.Text)
    
    // Get user's agents
    agents, err := bot.agentRepo.GetUserAgents(ctx, userID)
    if err != nil || len(agents) == 0 {
        bot.reply(update, "You have no active agents")
        return nil
    }
    
    // Determine agent
    var agentID string
    var period string = "yesterday" // default
    
    if len(args) == 0 {
        // /report → use first agent, yesterday
        agentID = agents[0].ID
    } else if isAgentID(args[0]) {
        agentID = args[0]
        if len(args) > 1 {
            period = args[1]
        }
    } else {
        // /report today → use first agent
        agentID = agents[0].ID
        period = args[0]
    }
    
    // Find agent
    var agent *models.AgentConfig
    for _, a := range agents {
        if a.ID == agentID {
            agent = a
            break
        }
    }
    
    if agent == nil {
        bot.reply(update, "Agent not found")
        return nil
    }
    
    // Generate report based on period
    generator := reports.NewGenerator(bot.agentRepoAdapter, bot.templates)
    
    var reportText string
    
    switch period {
    case "today":
        reportText, err = bot.generateDailyReport(ctx, generator, agentID, agent.Symbol, time.Now())
        
    case "yesterday":
        reportText, err = bot.generateDailyReport(ctx, generator, agentID, agent.Symbol, time.Now().AddDate(0, 0, -1))
        
    case "week":
        reportText, err = bot.generateWeeklyReport(ctx, generator, agentID, agent.Symbol)
        
    case "custom":
        // Parse dates from args
        if len(args) < 3 {
            bot.reply(update, "Usage: /report custom START_DATE END_DATE")
            return nil
        }
        
        start, end, err := parseDateRange(args[1], args[2])
        if err != nil {
            bot.reply(update, "Invalid date format. Use YYYY-MM-DD or 7d, 30d, etc")
            return nil
        }
        
        reportText, err = bot.generateCustomReport(ctx, generator, agentID, agent.Symbol, start, end)
        
    default:
        bot.reply(update, "Unknown period. Use: today, yesterday, week, or custom")
        return nil
    }
    
    if err != nil {
        bot.reply(update, fmt.Sprintf("Error generating report: %v", err))
        return err
    }
    
    // Send report
    bot.sendFormattedMessage(update.Message.Chat.ID, reportText)
    
    return nil
}

// Helper functions

func (bot *AgentBot) generateDailyReport(ctx context.Context, gen *reports.Generator, agentID, symbol string, date time.Time) (string, error) {
    report, err := gen.GenerateDailyReport(ctx, agentID, symbol, date)
    if err != nil {
        return "", err
    }
    return gen.RenderDailyReport(ctx, report)
}

func (bot *AgentBot) generateWeeklyReport(ctx context.Context, gen *reports.Generator, agentID, symbol string) (string, error) {
    weekStart := getWeekStart(time.Now())
    report, err := gen.GenerateWeeklyReport(ctx, agentID, symbol, weekStart)
    if err != nil {
        return "", err
    }
    return gen.RenderWeeklyReport(ctx, report)
}

func (bot *AgentBot) generateCustomReport(ctx context.Context, gen *reports.Generator, agentID, symbol string, start, end time.Time) (string, error) {
    report, err := gen.GenerateCustomReport(ctx, agentID, symbol, start, end)
    if err != nil {
        return "", err
    }
    return gen.RenderCustomReport(ctx, report)
}

// parseDateRange parses date range from strings
func parseDateRange(startStr, endStr string) (time.Time, time.Time, error) {
    start, err := parseDate(startStr)
    if err != nil {
        return time.Time{}, time.Time{}, fmt.Errorf("invalid start date: %w", err)
    }
    
    end, err := parseDate(endStr)
    if err != nil {
        return time.Time{}, time.Time{}, fmt.Errorf("invalid end date: %w", err)
    }
    
    return start, end, nil
}

// parseDate parses various date formats
func parseDate(s string) (time.Time, error) {
    now := time.Now()
    
    switch s {
    case "today":
        return now, nil
    case "yesterday":
        return now.AddDate(0, 0, -1), nil
    case "this_week":
        return getWeekStart(now), nil
    case "last_week":
        return getWeekStart(now).AddDate(0, 0, -7), nil
    case "this_month":
        return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()), nil
    case "last_month":
        return time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location()), nil
    }
    
    // Try relative days: "7d", "30d"
    if len(s) > 1 && s[len(s)-1] == 'd' {
        days := 0
        if _, err := fmt.Sscanf(s, "%dd", &days); err == nil {
            return now.AddDate(0, 0, -days), nil
        }
    }
    
    // Try YYYY-MM-DD format
    t, err := time.Parse("2006-01-02", s)
    if err == nil {
        return t, nil
    }
    
    // Try other common formats
    formats := []string{
        "2006-01-02 15:04:05",
        "02/01/2006",
        "02-01-2006",
    }
    
    for _, format := range formats {
        if t, err := time.Parse(format, s); err == nil {
            return t, nil
        }
    }
    
    return time.Time{}, fmt.Errorf("unrecognized date format: %s", s)
}

func getWeekStart(t time.Time) time.Time {
    // Start of week (Monday)
    offset := int(t.Weekday()) - 1
    if offset < 0 {
        offset = 6 // Sunday
    }
    return t.AddDate(0, 0, -offset)
}

func isAgentID(s string) bool {
    // Agent IDs are UUIDs or have "agent-" prefix
    return len(s) > 10 && (s[:6] == "agent-" || len(s) == 36)
}
```

## Command Examples

### Basic Usage

```
/report                              → Yesterday, first agent
/report today                        → Today, first agent  
/report agent-123                    → Yesterday, specific agent
/report agent-123 today              → Today, specific agent
/report agent-123 week               → Weekly, specific agent
```

### Advanced Usage

```bash
# Last 7 days
/report agent-123 custom 7d today

# January 2024
/report agent-123 custom 2024-01-01 2024-01-31

# This week so far
/report agent-123 custom this_week today

# Last week
/report agent-123 custom last_week last_week

# Specific dates
/report agent-123 custom 2024-11-15 2024-11-22
```

### Multiple Agents

```bash
# If you have multiple agents
/agents                              → List all agents
/report agent-technical-tom today    → Tom's report
/report agent-whale-watcher week     → Whale Watcher's report
```

## Response Format

Bot always responds with formatted report using templates:
- Markdown formatting
- Emojis for clarity
- Sections separated by lines
- Key metrics highlighted

## Error Handling

```
No agents:          "You have no active agents. Create one with /create_agent"
Agent not found:    "Agent 'xyz' not found. Use /agents to see your agents"
Invalid date:       "Invalid date format. Use YYYY-MM-DD or 7d, 30d, etc"
No data:            "No data available for this period"
Generation failed:  "Error generating report: [details]"
```

## Future Commands

- `/report compare AGENT1 AGENT2 [period]` - Compare two agents
- `/report performance AGENT_ID` - Detailed performance breakdown
- `/report export AGENT_ID week pdf` - Export to PDF
- `/report schedule daily 00:00` - Schedule automatic reports
- `/report insights AGENT_ID` - Only key learnings

---

**All reports use the same `internal/reports` package - DRY principle!**

