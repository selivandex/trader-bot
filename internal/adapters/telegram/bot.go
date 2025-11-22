package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/internal/adapters/config"
	"github.com/alexanderselivanov/trader/pkg/logger"
)

// Bot represents Telegram bot for notifications and control
type Bot struct {
	api            *tgbotapi.BotAPI
	chatID         int64
	alertOnTrades  bool
	alertOnErrors  bool
	commandHandler CommandHandler
}

// CommandHandler handles bot commands
type CommandHandler interface {
	HandleStatus(ctx context.Context) (string, error)
	HandleBalance(ctx context.Context) (string, error)
	HandlePosition(ctx context.Context) (string, error)
	HandleStop(ctx context.Context) (string, error)
	HandleResume(ctx context.Context) (string, error)
	HandleStats(ctx context.Context) (string, error)
}

// NewBot creates new Telegram bot
func NewBot(cfg *config.TelegramConfig) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}

	logger.Info("telegram bot initialized",
		zap.String("username", api.Self.UserName),
	)

	return &Bot{
		api:           api,
		chatID:        cfg.ChatID,
		alertOnTrades: cfg.AlertOnTrades,
		alertOnErrors: cfg.AlertOnErrors,
	}, nil
}

// SetCommandHandler sets command handler
func (b *Bot) SetCommandHandler(handler CommandHandler) {
	b.commandHandler = handler
}

// Start starts listening for commands
func (b *Bot) Start(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	logger.Info("telegram bot started, listening for commands")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case update := <-updates:
			if update.Message == nil {
				continue
			}

			// Only process messages from configured chat
			if update.Message.Chat.ID != b.chatID {
				continue
			}

			// Handle command
			go b.handleCommand(ctx, update.Message)
		}
	}
}

// handleCommand processes incoming commands
func (b *Bot) handleCommand(ctx context.Context, message *tgbotapi.Message) {
	if !message.IsCommand() {
		return
	}

	command := message.Command()

	logger.Info("received telegram command",
		zap.String("command", command),
		zap.Int64("from_chat", message.Chat.ID),
	)

	var response string
	var err error

	if b.commandHandler == nil {
		response = "⚠️ Command handler not initialized"
	} else {
		switch command {
		case "start":
			response = b.getWelcomeMessage()
		case "help":
			response = b.getHelpMessage()
		case "status":
			response, err = b.commandHandler.HandleStatus(ctx)
		case "balance":
			response, err = b.commandHandler.HandleBalance(ctx)
		case "position":
			response, err = b.commandHandler.HandlePosition(ctx)
		case "stop":
			response, err = b.commandHandler.HandleStop(ctx)
		case "resume":
			response, err = b.commandHandler.HandleResume(ctx)
		case "stats":
			response, err = b.commandHandler.HandleStats(ctx)
		default:
			response = fmt.Sprintf("❓ Unknown command: /%s\nUse /help to see available commands", command)
		}
	}

	if err != nil {
		response = fmt.Sprintf("❌ Error: %v", err)
		logger.Error("command handler error", zap.Error(err), zap.String("command", command))
	}

	if err := b.SendMessage(response); err != nil {
		logger.Error("failed to send telegram response", zap.Error(err))
	}
}

// SendMessage sends text message
func (b *Bot) SendMessage(text string) error {
	msg := tgbotapi.NewMessage(b.chatID, text)
	msg.ParseMode = "Markdown"

	_, err := b.api.Send(msg)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// AlertTradeOpened sends alert when trade is opened
func (b *Bot) AlertTradeOpened(symbol string, side string, size, price float64, stopLoss, takeProfit float64) {
	if !b.alertOnTrades {
		return
	}

	message := fmt.Sprintf(
		"🟢 *TRADE OPENED*\n\n"+
			"Symbol: `%s`\n"+
			"Side: *%s*\n"+
			"Size: `%.4f`\n"+
			"Entry Price: `$%.2f`\n"+
			"Stop Loss: `$%.2f`\n"+
			"Take Profit: `$%.2f`\n"+
			"Time: `%s`",
		symbol, strings.ToUpper(side), size, price,
		stopLoss, takeProfit,
		time.Now().Format("15:04:05"),
	)

	if err := b.SendMessage(message); err != nil {
		logger.Error("failed to send trade alert", zap.Error(err))
	}
}

// AlertTradeClosed sends alert when trade is closed
func (b *Bot) AlertTradeClosed(symbol string, side string, exitPrice, pnl, pnlPercent float64) {
	if !b.alertOnTrades {
		return
	}

	emoji := "🔴"
	if pnl > 0 {
		emoji = "🟢"
	}

	message := fmt.Sprintf(
		"%s *TRADE CLOSED*\n\n"+
			"Symbol: `%s`\n"+
			"Side: *%s*\n"+
			"Exit Price: `$%.2f`\n"+
			"PnL: `$%.2f` (%.2f%%)\n"+
			"Time: `%s`",
		emoji, symbol, strings.ToUpper(side), exitPrice,
		pnl, pnlPercent,
		time.Now().Format("15:04:05"),
	)

	if err := b.SendMessage(message); err != nil {
		logger.Error("failed to send trade alert", zap.Error(err))
	}
}

// AlertAIDecision sends AI decision alert
func (b *Bot) AlertAIDecision(provider string, action string, reason string, confidence int) {
	message := fmt.Sprintf(
		"🤖 *AI DECISION*\n\n"+
			"Provider: `%s`\n"+
			"Action: *%s*\n"+
			"Confidence: `%d%%`\n"+
			"Reason: _%s_",
		provider, action, confidence, reason,
	)

	if err := b.SendMessage(message); err != nil {
		logger.Error("failed to send AI decision alert", zap.Error(err))
	}
}

// AlertCircuitBreaker sends circuit breaker alert
func (b *Bot) AlertCircuitBreaker(reason string, cooldownMinutes int) {
	message := fmt.Sprintf(
		"🚨 *CIRCUIT BREAKER OPENED*\n\n"+
			"Reason: %s\n"+
			"Trading stopped for %d minutes\n\n"+
			"Use /resume to manually resume trading after cooldown",
		reason, cooldownMinutes,
	)

	if err := b.SendMessage(message); err != nil {
		logger.Error("failed to send circuit breaker alert", zap.Error(err))
	}
}

// AlertError sends error alert
func (b *Bot) AlertError(errorMsg string) {
	if !b.alertOnErrors {
		return
	}

	message := fmt.Sprintf(
		"❌ *ERROR*\n\n"+
			"`%s`\n\n"+
			"Time: `%s`",
		errorMsg,
		time.Now().Format("15:04:05"),
	)

	if err := b.SendMessage(message); err != nil {
		logger.Error("failed to send error alert", zap.Error(err))
	}
}

// AlertProfitTarget sends alert when profit target is reached
func (b *Bot) AlertProfitTarget(profit, profitPercent float64) {
	message := fmt.Sprintf(
		"💰 *PROFIT TARGET REACHED*\n\n"+
			"Profit: `$%.2f` (+%.2f%%)\n"+
			"Consider withdrawing profits!\n"+
			"Time: `%s`",
		profit, profitPercent,
		time.Now().Format("15:04:05"),
	)

	if err := b.SendMessage(message); err != nil {
		logger.Error("failed to send profit alert", zap.Error(err))
	}
}

// getWelcomeMessage returns welcome message
func (b *Bot) getWelcomeMessage() string {
	return `👋 *Welcome to AI Trading Bot!*

I will send you alerts about:
• Trade executions
• AI decisions
• Circuit breaker events
• Errors and warnings

Use /help to see available commands.`
}

// getHelpMessage returns help message with all commands
func (b *Bot) getHelpMessage() string {
	return `📖 *Available Commands:*

/status - Current bot status
/balance - Account balance and equity
/position - Current open position
/stats - Performance statistics
/stop - Stop trading bot
/resume - Resume trading
/help - Show this help message

💡 *Tip:* Bot will automatically send alerts about trades and important events.`
}

// Close closes bot connection
func (b *Bot) Close() {
	if b.api != nil {
		b.api.StopReceivingUpdates()
		logger.Info("telegram bot stopped")
	}
}
