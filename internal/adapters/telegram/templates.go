package telegram

import (
	"bytes"
	"fmt"
	"path/filepath"
	"text/template"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
)

// TemplateManager manages all Telegram notification templates
type TemplateManager struct {
	templates *template.Template
}

// NewTemplateManager creates and loads all templates
func NewTemplateManager(templatesDir string) (*TemplateManager, error) {
	if templatesDir == "" {
		templatesDir = "./templates/telegram"
	}

	// Parse all templates in directory
	pattern := filepath.Join(templatesDir, "*.tmpl")
	templates, err := template.ParseGlob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates from %s: %w", templatesDir, err)
	}

	// Verify all required templates exist
	requiredTemplates := []string{
		// Notifications
		"trade_executed.tmpl",
		"agent_started.tmpl",
		"agent_stopped.tmpl",
		"circuit_breaker.tmpl",
		"error_alert.tmpl",
		"daily_summary.tmpl",
		// Bot commands
		"welcome.tmpl",
		"exchange_connected.tmpl",
		"ticker_added.tmpl",
		"agent_created.tmpl",
		"agent_assigned.tmpl",
		"paper_mode_info.tmpl",
		"personalities_list.tmpl",
		"agents_list.tmpl",
		"agent_stats.tmpl",
		// Admin
		"admin_help.tmpl",
		"system_stats.tmpl",
		"user_info.tmpl",
		"all_users.tmpl",
		"all_agents.tmpl",
		"news_stats.tmpl",
		"trade_stats.tmpl",
		"user_banned.tmpl",
		"user_banned_notification.tmpl",
		"user_unbanned.tmpl",
		"user_restored.tmpl",
		"agent_stopped_admin.tmpl",
		// Named templates (errors, usage)
		"errors.tmpl",
		"usage.tmpl",
	}

	for _, name := range requiredTemplates {
		if templates.Lookup(name) == nil {
			return nil, fmt.Errorf("required template not found: %s", name)
		}
	}

	logger.Info("telegram templates loaded",
		zap.Int("count", len(templates.Templates())),
		zap.String("directory", templatesDir),
	)

	return &TemplateManager{
		templates: templates,
	}, nil
}

// GetTemplate returns template by name
func (tm *TemplateManager) GetTemplate(name string) *template.Template {
	return tm.templates.Lookup(name)
}

// ExecuteTemplate renders template with data
func (tm *TemplateManager) ExecuteTemplate(name string, data interface{}) (string, error) {
	tmpl := tm.templates.Lookup(name)
	if tmpl == nil {
		return "", fmt.Errorf("template %s not found", name)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", name, err)
	}

	return buf.String(), nil
}
