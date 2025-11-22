package telegram

import (
	"context"
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/config"
	"github.com/selivandex/trader-bot/pkg/logger"
)

// UserRepository interface for getting telegram IDs
type UserRepository interface {
	GetTelegramIDByUserID(ctx context.Context, userID string) (int64, error)
}

// Notifier sends notifications to users via Telegram
type Notifier struct {
	api             *tgbotapi.BotAPI
	userRepo        UserRepository
	cfg             *config.TelegramConfig
	templateManager *TemplateManager
}

// NewNotifier creates new Telegram notifier
func NewNotifier(botToken string, userRepo UserRepository, cfg *config.TelegramConfig, templateManager *TemplateManager) (*Notifier, error) {
	if botToken == "" {
		return nil, fmt.Errorf("telegram bot token is required")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API: %w", err)
	}

	bot.Debug = false

	logger.Info("telegram notifier initialized",
		zap.String("bot_username", bot.Self.UserName),
	)

	return &Notifier{
		api:             bot,
		userRepo:        userRepo,
		cfg:             cfg,
		templateManager: templateManager,
	}, nil
}

// SendTradeAlert sends trade notification to user
func (n *Notifier) SendTradeAlert(ctx context.Context, userID, agentName, action, symbol string, size, price, pnl float64) error {
	if !n.cfg.AlertOnTrades {
		return nil
	}

	telegramID, err := n.userRepo.GetTelegramIDByUserID(ctx, userID)
	if err != nil {
		logger.Warn("failed to get telegram ID for trade alert",
			zap.String("user_id", userID),
			zap.Error(err),
		)
		return err
	}

	emoji := n.getTradeEmoji(action, pnl)
	pnlSign := ""
	if pnl > 0 {
		pnlSign = "+"
	}

	data := map[string]interface{}{
		"Emoji":     emoji,
		"AgentName": agentName,
		"Action":    action,
		"Symbol":    symbol,
		"Size":      size,
		"Price":     price,
		"PnL":       pnl,
		"PnLSign":   pnlSign,
		"Time":      time.Now().Format("15:04:05"),
	}

	msg, err := n.templateManager.ExecuteTemplate("trade_executed.tmpl", data)
	if err != nil {
		return err
	}

	return n.sendMessageMarkdown(telegramID, msg)
}

// SendAgentStarted notifies user that agent started trading
func (n *Notifier) SendAgentStarted(ctx context.Context, userID, agentName, symbol string, budget float64) error {
	if !n.cfg.AlertOnTrades {
		return nil
	}

	telegramID, err := n.userRepo.GetTelegramIDByUserID(ctx, userID)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"AgentName": agentName,
		"Symbol":    symbol,
		"Budget":    budget,
	}

	msg, err := n.templateManager.ExecuteTemplate("agent_started.tmpl", data)
	if err != nil {
		return err
	}

	return n.sendMessageMarkdown(telegramID, msg)
}

// SendAgentStopped notifies user that agent stopped trading
func (n *Notifier) SendAgentStopped(ctx context.Context, userID, agentName, symbol string, finalPnL float64) error {
	if !n.cfg.AlertOnTrades {
		return nil
	}

	telegramID, err := n.userRepo.GetTelegramIDByUserID(ctx, userID)
	if err != nil {
		return err
	}

	emoji := "⏸️"
	if finalPnL > 0 {
		emoji = "✅"
	} else if finalPnL < 0 {
		emoji = "❌"
	}

	pnlSign := ""
	if finalPnL > 0 {
		pnlSign = "+"
	}

	data := map[string]interface{}{
		"Emoji":     emoji,
		"AgentName": agentName,
		"Symbol":    symbol,
		"FinalPnL":  finalPnL,
		"PnLSign":   pnlSign,
	}

	msg, err := n.templateManager.ExecuteTemplate("agent_stopped.tmpl", data)
	if err != nil {
		return err
	}

	return n.sendMessageMarkdown(telegramID, msg)
}

// SendCircuitBreakerAlert notifies user about circuit breaker activation
func (n *Notifier) SendCircuitBreakerAlert(ctx context.Context, userID, agentName, reason string) error {
	if !n.cfg.AlertOnErrors {
		return nil
	}

	telegramID, err := n.userRepo.GetTelegramIDByUserID(ctx, userID)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"AgentName": agentName,
		"Reason":    reason,
	}

	msg, err := n.templateManager.ExecuteTemplate("circuit_breaker.tmpl", data)
	if err != nil {
		return err
	}

	return n.sendMessageMarkdown(telegramID, msg)
}

// SendErrorAlert sends error notification to user
func (n *Notifier) SendErrorAlert(ctx context.Context, userID, agentName, errorMsg string) error {
	if !n.cfg.AlertOnErrors {
		return nil
	}

	telegramID, err := n.userRepo.GetTelegramIDByUserID(ctx, userID)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"AgentName": agentName,
		"ErrorMsg":  errorMsg,
	}

	msg, err := n.templateManager.ExecuteTemplate("error_alert.tmpl", data)
	if err != nil {
		return err
	}

	return n.sendMessageMarkdown(telegramID, msg)
}

// SendDailySummary sends daily performance summary to user
func (n *Notifier) SendDailySummary(ctx context.Context, userID string, stats map[string]interface{}) error {
	if !n.cfg.AlertOnTrades {
		return nil
	}

	telegramID, err := n.userRepo.GetTelegramIDByUserID(ctx, userID)
	if err != nil {
		return err
	}

	totalPnL := stats["total_pnl"].(float64)
	totalTrades := stats["total_trades"].(int)
	winRate := stats["win_rate"].(float64)

	emoji := "📊"
	if totalPnL > 0 {
		emoji = "📈"
	} else if totalPnL < 0 {
		emoji = "📉"
	}

	pnlSign := ""
	if totalPnL > 0 {
		pnlSign = "+"
	}

	data := map[string]interface{}{
		"Emoji":       emoji,
		"TotalPnL":    totalPnL,
		"PnLSign":     pnlSign,
		"TotalTrades": totalTrades,
		"WinRate":     winRate,
	}

	msg, err := n.templateManager.ExecuteTemplate("daily_summary.tmpl", data)
	if err != nil {
		return err
	}

	return n.sendMessageMarkdown(telegramID, msg)
}

// Helper methods

func (n *Notifier) sendMessageMarkdown(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"

	_, err := n.api.Send(msg)
	if err != nil {
		logger.Error("failed to send telegram message",
			zap.Int64("chat_id", chatID),
			zap.Error(err),
		)
		return err
	}

	return nil
}

func (n *Notifier) getTradeEmoji(action string, pnl float64) string {
	if pnl > 0 {
		return "💚" // Green heart for profit
	} else if pnl < 0 {
		return "❤️" // Red heart for loss
	}

	// No PnL (opening position)
	switch action {
	case "OPEN_LONG":
		return "📈"
	case "OPEN_SHORT":
		return "📉"
	case "CLOSE":
		return "🔄"
	default:
		return "🤖"
	}
}
