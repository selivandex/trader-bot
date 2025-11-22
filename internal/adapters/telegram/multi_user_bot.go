package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/internal/adapters/config"
	"github.com/alexanderselivanov/trader/pkg/logger"
	"github.com/alexanderselivanov/trader/pkg/models"
)

// MultiUserBot represents multi-user Telegram bot
type MultiUserBot struct {
	*Bot       // Embed base bot
	botManager BotManager
}

// BotManager interface for managing user bots
type BotManager interface {
	StartUserBot(ctx context.Context, userID int64) error
	StopUserBot(ctx context.Context, userID int64) error
	GetUserBot(userID int64) (interface{}, bool)
	GetActiveBotCount() int
	GetUserRepository() UserRepository
}

// UserRepository interface for user operations
type UserRepository interface {
	GetUserByTelegramID(ctx context.Context, telegramID int64) (*models.User, error)
	CreateUser(ctx context.Context, telegramID int64, username, firstName string) (*models.User, error)
	GetConfig(ctx context.Context, userID int64) (*models.UserConfig, error)
	SaveConfig(ctx context.Context, config *models.UserConfig) error
	GetState(ctx context.Context, userID int64) (*models.UserState, error)
	SetTradingStatus(ctx context.Context, userID int64, isTrading bool) error
}

// NewMultiUserBot creates new multi-user Telegram bot
func NewMultiUserBot(cfg *config.TelegramConfig, botManager BotManager) (*MultiUserBot, error) {
	baseBot, err := NewBot(cfg)
	if err != nil {
		return nil, err
	}

	return &MultiUserBot{
		Bot:        baseBot,
		botManager: botManager,
	}, nil
}

// Start starts listening for commands
func (mb *MultiUserBot) Start(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := mb.api.GetUpdatesChan(u)

	logger.Info("multi-user telegram bot started")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case update := <-updates:
			if update.Message == nil {
				continue
			}

			// Handle command
			go mb.handleMultiUserCommand(ctx, update.Message)
		}
	}
}

// handleMultiUserCommand processes commands for multi-user setup
func (mb *MultiUserBot) handleMultiUserCommand(ctx context.Context, message *tgbotapi.Message) {
	if !message.IsCommand() {
		return
	}

	command := message.Command()
	args := strings.Fields(message.CommandArguments())

	telegramID := message.From.ID
	username := message.From.UserName
	firstName := message.From.FirstName

	logger.Info("received telegram command",
		zap.String("command", command),
		zap.Int64("telegram_id", telegramID),
		zap.String("username", username),
	)

	var response string
	var err error

	// Get or create user
	repo := mb.botManager.GetUserRepository()
	user, err := repo.GetUserByTelegramID(ctx, telegramID)

	if err != nil {
		response = fmt.Sprintf("❌ Database error: %v", err)
		mb.sendToUser(telegramID, response)
		return
	}

	// Commands that don't require registration
	switch command {
	case "start":
		if user == nil {
			user, err = repo.CreateUser(ctx, telegramID, username, firstName)
			if err != nil {
				response = fmt.Sprintf("❌ Failed to register: %v", err)
			} else {
				response = mb.getWelcomeMessage(user.FirstName)
			}
		} else {
			response = fmt.Sprintf("👋 Welcome back, %s!\n\nUse /help to see available commands.", user.FirstName)
		}

	case "help":
		response = mb.getHelpMessage()

	default:
		// All other commands require registration
		if user == nil {
			response = "⚠️ You need to register first. Use /start command."
			mb.sendToUser(telegramID, response)
			return
		}

		// Handle registered user commands
		response, err = mb.handleRegisteredUserCommand(ctx, user, command, args)
	}

	if err != nil {
		response = fmt.Sprintf("❌ Error: %v", err)
		logger.Error("command handler error", zap.Error(err), zap.String("command", command))
	}

	mb.sendToUser(telegramID, response)
}

// handleRegisteredUserCommand handles commands for registered users
func (mb *MultiUserBot) handleRegisteredUserCommand(ctx context.Context, user *models.User, command string, args []string) (string, error) {
	switch command {
	case "connect":
		return mb.handleConnect(ctx, user, args)
		
	case "addpair":
		return mb.handleAddPair(ctx, user, args)
		
	case "listpairs", "pairs":
		return mb.handleListPairs(ctx, user)
		
	case "removepair":
		return mb.handleRemovePair(ctx, user, args)
		
	case "setpair", "pair":
		return mb.handleSetPair(ctx, user, args)
		
	case "setbalance", "balance":
		return mb.handleSetBalance(ctx, user, args)
		
	case "start_trading":
		return mb.handleStartTradingMulti(ctx, user, args)
		
	case "stop_trading":
		return mb.handleStopTradingMulti(ctx, user, args)
		
	case "status":
		return mb.handleStatusMulti(ctx, user, args)
		
	case "mystats", "stats":
		return mb.handleMyStats(ctx, user)
		
	case "position":
		return mb.handlePosition(ctx, user)
		
	case "config":
		return mb.handleConfigMulti(ctx, user, args)
		
	case "help":
		return mb.getHelpMessage(), nil

	default:
		return fmt.Sprintf("❓ Unknown command: /%s\nUse /help to see available commands", command), nil
	}
}

// handleConnect handles exchange connection
func (mb *MultiUserBot) handleConnect(ctx context.Context, user *models.User, args []string) (string, error) {
	if len(args) < 3 {
		return `📝 *Connect Exchange*

Usage: /connect <exchange> <api_key> <api_secret> [testnet]

Examples:
/connect binance YOUR_API_KEY YOUR_SECRET true
/connect bybit YOUR_API_KEY YOUR_SECRET false

Exchanges: binance, bybit
Testnet: true/false (default: true)`, nil
	}

	exchangeName := strings.ToLower(args[0])
	apiKey := args[1]
	apiSecret := args[2]
	testnet := true

	if len(args) > 3 {
		testnet = strings.ToLower(args[3]) == "true"
	}

	// Validate exchange
	if exchangeName != "binance" && exchangeName != "bybit" {
		return "❌ Unsupported exchange. Use: binance or bybit", nil
	}

	// Save configuration
	config := &models.UserConfig{
		UserID:             user.ID,
		Exchange:           exchangeName,
		APIKey:             apiKey,
		APISecret:          apiSecret,
		Testnet:            testnet,
		Symbol:             "BTC/USDT",
		InitialBalance:     models.NewDecimal(1000),
		MaxPositionPercent: models.NewDecimal(30),
		MaxLeverage:        3,
		StopLossPercent:    models.NewDecimal(2),
		TakeProfitPercent:  models.NewDecimal(5),
	}

	repo := mb.botManager.GetUserRepository()
	if err := repo.SaveConfig(ctx, config); err != nil {
		return "", fmt.Errorf("failed to save config: %w", err)
	}

	return fmt.Sprintf(`✅ *Exchange Connected*

Exchange: %s
Testnet: %v
Symbol: %s
Balance: $%.2f

Next steps:
/setpair BTC/USDT - Change trading pair
/setbalance 1000 - Set initial balance
/start_trading - Start trading!`,
		exchangeName, testnet, config.Symbol, models.ToFloat64(config.InitialBalance)), nil
}

// handleSetPair changes trading pair
func (mb *MultiUserBot) handleSetPair(ctx context.Context, user *models.User, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /setpair BTC/USDT", nil
	}

	symbol := strings.ToUpper(args[0])

	repo := mb.botManager.GetUserRepository()
	config, err := repo.GetConfig(ctx, user.ID)
	if err != nil {
		return "", err
	}

	if config == nil {
		return "⚠️ Connect exchange first: /connect", nil
	}

	config.Symbol = symbol
	if err := repo.SaveConfig(ctx, config); err != nil {
		return "", err
	}

	return fmt.Sprintf("✅ Trading pair set to: %s", symbol), nil
}

// handleSetBalance sets initial balance
func (mb *MultiUserBot) handleSetBalance(ctx context.Context, user *models.User, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /setbalance 1000", nil
	}

	balance, err := strconv.ParseFloat(args[0], 64)
	if err != nil || balance <= 0 {
		return "❌ Invalid balance. Must be positive number.", nil
	}

	repo := mb.botManager.GetUserRepository()
	config, err := repo.GetConfig(ctx, user.ID)
	if err != nil {
		return "", err
	}

	if config == nil {
		return "⚠️ Connect exchange first: /connect", nil
	}

	config.InitialBalance = models.NewDecimal(balance)
	if err := repo.SaveConfig(ctx, config); err != nil {
		return "", err
	}

	return fmt.Sprintf("✅ Initial balance set to: $%.2f", balance), nil
}

// handleStartTrading starts user's trading bot
func (mb *MultiUserBot) handleStartTrading(ctx context.Context, user *models.User) (string, error) {
	repo := mb.botManager.GetUserRepository()
	config, err := repo.GetConfig(ctx, user.ID)
	if err != nil {
		return "", err
	}

	if config == nil {
		return "⚠️ Configure your bot first:\n1. /connect <exchange> <api_key> <secret>\n2. /setpair BTC/USDT\n3. /setbalance 1000", nil
	}

	// Start bot
	if err := mb.botManager.StartUserBot(ctx, user.ID); err != nil {
		return "", fmt.Errorf("failed to start bot: %w", err)
	}

	return fmt.Sprintf(`🚀 *Trading Started!*

Exchange: %s
Symbol: %s
Balance: $%.2f

Your bot is now actively trading.
Use /status to check progress.`,
		config.Exchange, config.Symbol, models.ToFloat64(config.InitialBalance)), nil
}

// handleStopTrading stops user's trading bot
func (mb *MultiUserBot) handleStopTrading(ctx context.Context, user *models.User) (string, error) {
	if err := mb.botManager.StopUserBot(ctx, user.ID); err != nil {
		return "", fmt.Errorf("failed to stop bot: %w", err)
	}

	return "⏸️ *Trading Stopped*\n\nYour bot has been stopped. Use /start_trading to resume.", nil
}

// handleStatus shows user bot status
func (mb *MultiUserBot) handleStatus(ctx context.Context, user *models.User) (string, error) {
	repo := mb.botManager.GetUserRepository()

	config, err := repo.GetConfig(ctx, user.ID)
	if err != nil {
		return "", err
	}

	if config == nil {
		return "⚠️ Not configured. Use /connect to setup.", nil
	}

	state, err := repo.GetState(ctx, user.ID)
	if err != nil {
		return "", err
	}

	status := "🔴 Stopped"
	if config.IsTrading {
		status = "🟢 Running"
	}

	return fmt.Sprintf(`📊 *Your Bot Status*

Status: %s
Exchange: %s
Symbol: %s

💰 Balance: $%.2f
📈 Equity: $%.2f
📊 Daily PnL: $%.2f

Active Bots: %d`,
		status,
		config.Exchange,
		config.Symbol,
		models.ToFloat64(state.Balance),
		models.ToFloat64(state.Equity),
		models.ToFloat64(state.DailyPnL),
		mb.botManager.GetActiveBotCount(),
	), nil
}

// handleMyStats shows user statistics
func (mb *MultiUserBot) handleMyStats(ctx context.Context, user *models.User) (string, error) {
	// TODO: Implement stats retrieval
	return "📊 Statistics feature coming soon!", nil
}

// handlePosition shows current position
func (mb *MultiUserBot) handlePosition(ctx context.Context, user *models.User) (string, error) {
	// TODO: Implement position retrieval
	return "📈 Position tracking coming soon!", nil
}

// Helper functions

func (mb *MultiUserBot) formatConfig(config *models.UserConfig) string {
	return fmt.Sprintf(`⚙️ *Your Configuration*

Exchange: %s (testnet: %v)
Symbol: %s
Balance: $%.2f
Max Position: %.1f%%
Max Leverage: %dx
Stop Loss: %.1f%%
Take Profit: %.1f%%
Trading: %v`,
		config.Exchange,
		config.Testnet,
		config.Symbol,
		models.ToFloat64(config.InitialBalance),
		models.ToFloat64(config.MaxPositionPercent),
		config.MaxLeverage,
		models.ToFloat64(config.StopLossPercent),
		models.ToFloat64(config.TakeProfitPercent),
		config.IsTrading,
	)
}

func (mb *MultiUserBot) sendToUser(telegramID int64, text string) {
	msg := tgbotapi.NewMessage(telegramID, text)
	msg.ParseMode = "Markdown"

	if _, err := mb.api.Send(msg); err != nil {
		logger.Error("failed to send message", zap.Error(err))
	}
}

func (mb *MultiUserBot) getWelcomeMessage(firstName string) string {
	return fmt.Sprintf(`👋 *Welcome, %s!*

You've been registered in the AI Trading Bot.

🚀 *Quick Start:*
1. Connect your exchange:
   /connect binance YOUR_API YOUR_SECRET true

2. Configure trading:
   /setpair BTC/USDT
   /setbalance 1000

3. Start trading:
   /start_trading

Use /help for all commands.`, firstName)
}

func (mb *MultiUserBot) getHelpMessage() string {
	return `📖 *Available Commands*

*Setup:*
/connect <exchange> <api_key> <secret> - Connect exchange
/addpair <symbol> <balance> - Add trading pair
/listpairs - Show all your pairs
/removepair <symbol> - Remove pair

*Trading:*
/start_trading [symbol] - Start trading (all or specific pair)
/stop_trading [symbol] - Stop trading (all or specific pair)

*Info:*
/status [symbol] - Bot status (all or specific pair)
/config [symbol] - View configuration
/mystats - Performance statistics
/position [symbol] - Current position

*Help:*
/help - Show this message

💡 You can trade multiple pairs simultaneously!
Example: BTC/USDT ($1000) + ETH/USDT ($500)`
}
