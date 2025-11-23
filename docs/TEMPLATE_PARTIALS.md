<!-- @format -->

# Template Partials System

## Overview

To follow DRY (Don't Repeat Yourself) principle, common template sections are extracted into **partial templates** that can be reused across multiple prompts.

## Partial Templates

### Location

All partial templates are stored in the same directory as their usage:

```
templates/agentic/
├── _toolkit_tools.tmpl      # Partial: Available toolkit tools list
├── generate_options.tmpl     # Uses: {{template "toolkit_tools"}}
├── evaluate_option.tmpl      # Uses: {{template "toolkit_tools"}}
├── final_decision.tmpl       # Uses: {{template "toolkit_tools"}}
├── reflection.tmpl           # Uses: {{template "toolkit_tools"}}
├── create_plan.tmpl          # Uses: {{template "toolkit_tools"}}
└── self_analysis.tmpl        # Uses: {{template "toolkit_tools"}}
```

**Naming convention:** Partial templates start with underscore `_` prefix.

## Current Partials

### `_toolkit_tools.tmpl`

**Purpose:** Describes available agent toolkit tools  
**Defined as:** `{{define "toolkit_tools"}}`  
**Used in:** All agentic templates

**Contents:**

```
AVAILABLE TOOLS (query local cache - zero latency):
- GetCandles(timeframe, limit) - Fetch additional timeframe candles
- SearchNews(query, hours, limit) - Search news by keywords
- GetHighImpactNews(minImpact, hours) - Get breaking news
- GetRecentWhaleMovements(symbol, minAmount, hours) - Track large transactions
- GetNetExchangeFlow(symbol, hours) - Check exchange inflow/outflow
- SearchPersonalMemories(query, topK) - Recall similar past experiences
- SearchCollectiveMemories(personality, query, topK) - Learn from other agents
```

## Usage

### Defining a Partial

Create file with `{{define "name"}}...{{end}}`:

```go
{{/* File: templates/agentic/_my_partial.tmpl */}}

{{define "my_partial"}}
This is reusable content.
It can contain Go template logic: {{if .Something}}...{{end}}
{{end}}
```

### Using a Partial

Include partial in any template with `{{template "name"}}`:

```go
{{/* File: templates/agentic/my_prompt.tmpl */}}

You are an AI agent.

{{template "my_partial"}}

Now do your task...
```

### Passing Data to Partials

Partials inherit the parent template's data context:

```go
{{define "greeting"}}
Hello, {{.Name}}!
{{end}}

{{/* In parent template: */}}
{{template "greeting" .}}  {{/* Passes entire . context */}}
```

## Template Loading

The template manager automatically loads ALL `.tmpl` files in a directory, including partials:

```go
// pkg/templates/manager.go
func NewManager(dir string) (*Manager, error) {
    tmpl := template.New("").Funcs(funcMap)

    // Parse all .tmpl files (including partials)
    tmpl, err = tmpl.ParseGlob(filepath.Join(dir, "*.tmpl"))

    return &Manager{templates: tmpl}, nil
}
```

**Important:** Partials must be in the SAME directory as templates that use them.

## Benefits

### Before (Duplicated)

```go
// generate_options.tmpl
AVAILABLE TOOLS:
- GetCandles(...)
- SearchNews(...)
...

// evaluate_option.tmpl
AVAILABLE TOOLS:
- GetCandles(...)
- SearchNews(...)
...

// final_decision.tmpl
AVAILABLE TOOLS:
- GetCandles(...)
- SearchNews(...)
...
```

❌ **Problems:**

- 7 files with identical content
- Update requires changing 7 files
- Easy to have inconsistencies
- Harder to maintain

### After (Partial)

```go
// _toolkit_tools.tmpl
{{define "toolkit_tools"}}
AVAILABLE TOOLS:
- GetCandles(...)
- SearchNews(...)
...
{{end}}

// generate_options.tmpl
{{template "toolkit_tools"}}

// evaluate_option.tmpl
{{template "toolkit_tools"}}

// final_decision.tmpl
{{template "toolkit_tools"}}
```

✅ **Benefits:**

- Single source of truth
- Update once, applies everywhere
- Guaranteed consistency
- Easy to maintain

## Adding New Partials

### Step 1: Create Partial File

```bash
# Create file with _ prefix
vim templates/agentic/_my_new_partial.tmpl
```

```go
{{define "my_new_partial"}}
Your reusable content here.
Can use Go template syntax: {{.Variable}}
{{end}}
```

### Step 2: Use in Templates

```go
{{/* In any template in same directory */}}

Some prompt text...

{{template "my_new_partial"}}

More prompt text...
```

### Step 3: No Restart Needed (Future)

Currently requires app restart to reload templates.  
Future: hot-reload will detect partial changes.

## Best Practices

### DO ✅

- **Use descriptive names:** `toolkit_tools`, `risk_warnings`, `json_format`
- **Keep partials focused:** One clear purpose per partial
- **Document partials:** Add comment at top explaining purpose
- **Prefix with underscore:** `_partial_name.tmpl` for visibility
- **Validate after changes:** Test that all templates using partial still work

### DON'T ❌

- **Don't make partials too generic:** "common_stuff" is not clear
- **Don't nest partials deeply:** Keep it simple (1-2 levels max)
- **Don't put logic in partials:** Partials are for content reuse, not complex logic
- **Don't forget context:** Partials use parent template's data context

## Testing Partials

### Check Partial is Loaded

```bash
# Grep for partial usage
cd templates/agentic
grep "template \"toolkit_tools\"" *.tmpl

# Should show all files using it:
# generate_options.tmpl:{{template "toolkit_tools"}}
# evaluate_option.tmpl:{{template "toolkit_tools"}}
# ...
```

### Verify Partial Renders

```go
// In Go code (test file)
tm, _ := templates.NewManager("./templates/agentic")
output, _ := tm.ExecuteTemplate("generate_options.tmpl", data)

// Check output contains expected text
if !strings.Contains(output, "AVAILABLE TOOLS") {
    t.Error("toolkit_tools partial not rendered")
}
```

## Common Partials to Create

Future candidates for extraction:

### `_json_format.tmpl`

- Standard JSON output format instructions
- Used in: All prompts that return JSON

### `_risk_rules.tmpl`

- Trading risk management rules
- Used in: Decision-making prompts

### `_market_context.tmpl`

- Standard market data display format
- Used in: Prompts that show current market state

### `_personality_traits.tmpl`

- Agent personality characteristics
- Used in: Prompts that need personality context

## Troubleshooting

### Error: `template "name" not defined`

**Cause:** Partial not loaded or wrong name

**Solution:**

1. Check partial file exists: `ls templates/agentic/_*.tmpl`
2. Check `{{define "name"}}` matches `{{template "name"}}`
3. Verify partial is in same directory as parent template
4. Restart application to reload templates

### Partial Not Rendering

**Cause:** Template syntax error or context issue

**Solution:**

1. Check partial syntax is valid Go template
2. Verify data context has required fields
3. Test partial standalone with sample data
4. Check logs for template render errors

### Inconsistent Output

**Cause:** Caching or old template version

**Solution:**

1. Clear any template caches
2. Restart application
3. Verify file was saved with changes
4. Check file permissions (readable)

## Migration Checklist

When extracting common content to partial:

- [ ] Create `_partial_name.tmpl` file
- [ ] Wrap content in `{{define "name"}}...{{end}}`
- [ ] Find all templates with duplicate content
- [ ] Replace with `{{template "name"}}`
- [ ] Test all affected templates render correctly
- [ ] Document partial in this file
- [ ] Commit changes with descriptive message

## Example: Creating a New Partial

```bash
# Step 1: Identify repeated content across templates
grep -r "DECISION RULES:" templates/agentic/

# Step 2: Create partial
cat > templates/agentic/_decision_rules.tmpl << 'EOF'
{{define "decision_rules"}}
DECISION RULES:
- If confidence > 80%: High conviction, execute
- If confidence 60-79%: Moderate conviction, execute with caution
- If confidence < 60%: Low conviction, HOLD
- If market conditions unclear: Wait for better setup
{{end}}
EOF

# Step 3: Replace in templates
# Before:
#   DECISION RULES:
#   - If confidence > 80%: ...
#
# After:
#   {{template "decision_rules"}}

# Step 4: Test
make restart
./scripts/test_templates.sh

# Step 5: Commit
git add templates/agentic/_decision_rules.tmpl
git add templates/agentic/*.tmpl
git commit -m "feat: extract decision_rules to partial template"
```

---

**Partials = DRY templates = Maintainable prompts** 🎯
