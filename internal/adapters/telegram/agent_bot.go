package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/internal/adapters/config"
	"github.com/alexanderselivanov/trader/internal/adapters/exchange"
	"github.com/alexanderselivanov/trader/internal/agents"
	"github.com/alexanderselivanov/trader/internal/users"
	"github.com/alexanderselivanov/trader/pkg/logger"
	"github.com/alexanderselivanov/trader/pkg/models"
)

// AgentBot handles Telegram commands for AI agents
type AgentBot struct {
	api            *tgbotapi.BotAPI
	agenticManager *agents.AgenticManager
	userRepo       *users.AgentsRepository
	agentRepo      *agents.Repository
}

// NewAgentBot creates new Telegram bot for agents
func NewAgentBot(cfg *config.TelegramConfig, agenticManager *agents.AgenticManager, userRepo *users.AgentsRepository, agentRepo *agents.Repository) (*AgentBot, error) {
	bot, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	bot.Debug = false

	logger.Info("Telegram agent bot authorized",
		zap.String("username", bot.Self.UserName),
	)

	return &AgentBot{
		api:            bot,
		agenticManager: agenticManager,
		userRepo:       userRepo,
		agentRepo:      agentRepo,
	}, nil
}

// Start starts the bot
func (ab *AgentBot) Start(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := ab.api.GetUpdatesChan(u)

	logger.Info("Agent bot started, listening for commands...")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case update := <-updates:
			if update.Message == nil {
				continue
			}

			if !update.Message.IsCommand() {
				continue
			}

			go ab.handleCommand(update.Message)
		}
	}
}

// handleCommand routes commands
func (ab *AgentBot) handleCommand(message *tgbotapi.Message) {
	ctx := context.Background()
	telegramID := message.From.ID

	// Get or create user
	user, err := ab.userRepo.GetUserByTelegramID(ctx, telegramID)
	if err != nil {
		// Create new user
		user, err = ab.userRepo.CreateUser(ctx, telegramID, message.From.UserName, message.From.FirstName)
		if err != nil {
			ab.sendMessage(telegramID, "❌ Failed to create user")
			return
		}
	}

	cmd := message.Command()
	args := strings.Fields(message.CommandArguments())

	switch cmd {
	case "start":
		ab.handleStart(telegramID, user)
	case "connect":
		ab.handleConnect(ctx, telegramID, user.ID, args)
	case "add_ticker":
		ab.handleAddTicker(ctx, telegramID, user.ID, args)
	case "create_agent":
		ab.handleCreateAgent(ctx, telegramID, user.ID, args)
	case "assign_agent":
		ab.handleAssignAgent(ctx, telegramID, user.ID, args)
	case "start_agent":
		ab.handleStartAgent(ctx, telegramID, user.ID, args)
	case "stop_agent":
		ab.handleStopAgent(ctx, telegramID, user.ID, args)
	case "agents":
		ab.handleListAgents(ctx, telegramID, user.ID)
	case "stats":
		ab.handleStats(ctx, telegramID, user.ID, args)
	case "personalities":
		ab.handlePersonalities(telegramID)
	default:
		ab.sendMessage(telegramID, "Unknown command. Use /start for help.")
	}
}

// handleStart shows welcome message
func (ab *AgentBot) handleStart(telegramID int64, user *models.User) {
	msg := fmt.Sprintf(`🤖 *Welcome to AI Agent Trading System!*

You are registered as: %s

*Setup Steps:*
1️⃣ Connect exchange: /connect binance YOUR_API_KEY YOUR_SECRET
2️⃣ Add trading pair: /add_ticker BTC/USDT 1000
3️⃣ Create agent: /create_agent conservative "Technical Tom"
4️⃣ Assign agent: /assign_agent AGENT_ID BTC/USDT 500
5️⃣ Start trading: /start_agent AGENT_ID

*Commands:*
/personalities - View agent types
/agents - List your agents
/stats AGENT_ID - Agent performance
/stop_agent AGENT_ID - Stop agent`, user.Username)

	ab.sendMessageMarkdown(telegramID, msg)
}

// handleConnect connects user's exchange
func (ab *AgentBot) handleConnect(ctx context.Context, telegramID int64, userID string, args []string) {
	if len(args) < 3 {
		ab.sendMessage(telegramID, "Usage: /connect <exchange> <api_key> <api_secret> [testnet]\nExample: /connect binance your_key your_secret true")
		return
	}

	exchange := args[0]
	apiKey := args[1]
	apiSecret := args[2]
	testnet := true
	if len(args) > 3 {
		testnet, _ = strconv.ParseBool(args[3])
	}

	_, err := ab.userRepo.AddExchange(ctx, userID, exchange, apiKey, apiSecret, testnet)
	if err != nil {
		ab.sendMessage(telegramID, fmt.Sprintf("❌ Failed to connect exchange: %v", err))
		return
	}

	mode := "testnet"
	if !testnet {
		mode = "LIVE"
	}

	ab.sendMessage(telegramID, fmt.Sprintf("✅ %s connected (%s)\n\nNext: /add_ticker BTC/USDT 1000", strings.ToUpper(exchange), mode))
}

// handleAddTicker adds trading pair
func (ab *AgentBot) handleAddTicker(ctx context.Context, telegramID int64, userID string, args []string) {
	if len(args) < 2 {
		ab.sendMessage(telegramID, "Usage: /add_ticker <symbol> <budget>\nExample: /add_ticker BTC/USDT 1000")
		return
	}

	symbol := args[0]
	budget, _ := strconv.ParseFloat(args[1], 64)

	if budget <= 0 {
		ab.sendMessage(telegramID, "❌ Budget must be positive")
		return
	}

	// Get user's exchange (assume first one)
	// TODO: Support multiple exchanges
	exch, err := ab.userRepo.GetUserExchange(ctx, userID, "binance")
	if err != nil {
		ab.sendMessage(telegramID, "❌ Connect exchange first: /connect binance KEY SECRET")
		return
	}

	_, err = ab.userRepo.AddTradingPair(ctx, userID, exch.ID, symbol, budget)
	if err != nil {
		ab.sendMessage(telegramID, fmt.Sprintf("❌ Failed to add ticker: %v", err))
		return
	}

	ab.sendMessage(telegramID, fmt.Sprintf("✅ %s added with budget $%.2f\n\nNext: /create_agent conservative \"My Agent\"", symbol, budget))
}

// handleCreateAgent creates new agent
func (ab *AgentBot) handleCreateAgent(ctx context.Context, telegramID int64, userID string, args []string) {
	if len(args) < 2 {
		ab.sendMessage(telegramID, "Usage: /create_agent <personality> <name>\n\nUse /personalities to see options")
		return
	}

	personality := models.AgentPersonality(args[0])
	name := strings.Join(args[1:], " ")

	config, err := ab.agenticManager.CreateAgentFromPersonality(ctx, userID, personality, name)
	if err != nil {
		ab.sendMessage(telegramID, fmt.Sprintf("❌ Failed: %v", err))
		return
	}

	emoji := agents.GetAgentColorEmoji(personality)
	msg := fmt.Sprintf(`✅ Agent created!

%s *%s* (ID: %s)
Personality: %s

📊 Strategy:
• Position: %.0f%%
• Leverage: %dx
• Stop Loss: %.1f%%
• Take Profit: %.1f%%

🎯 Signal Weights:
• Technical: %.0f%%
• News: %.0f%%
• On-Chain: %.0f%%
• Sentiment: %.0f%%

Next: /assign_agent %s BTC/USDT 500`,
		emoji, config.Name, config.ID[:8]+"...", personality,
		config.Strategy.MaxPositionPercent,
		config.Strategy.MaxLeverage,
		config.Strategy.StopLossPercent,
		config.Strategy.TakeProfitPercent,
		config.Specialization.TechnicalWeight*100,
		config.Specialization.NewsWeight*100,
		config.Specialization.OnChainWeight*100,
		config.Specialization.SentimentWeight*100,
		config.ID[:8]+"...")

	ab.sendMessageMarkdown(telegramID, msg)
}

// handleAssignAgent assigns agent to symbol
func (ab *AgentBot) handleAssignAgent(ctx context.Context, telegramID int64, userID string, args []string) {
	if len(args) < 3 {
		ab.sendMessage(telegramID, "Usage: /assign_agent <agent_id> <symbol> <budget>\nExample: /assign_agent abc123 BTC/USDT 500")
		return
	}

	agentID := args[0]
	symbol := args[1]
	budget, _ := strconv.ParseFloat(args[2], 64)

	// Find trading pair ID
	pairs, err := ab.userRepo.GetUserTradingPairs(ctx, userID)
	if err != nil {
		ab.sendMessage(telegramID, "❌ Failed to get trading pairs")
		return
	}

	var pairID string
	for _, pair := range pairs {
		if pair.Symbol == symbol {
			pairID = pair.ID
			break
		}
	}

	if pairID == "" {
		ab.sendMessage(telegramID, fmt.Sprintf("❌ Symbol %s not found. Add it first: /add_ticker %s 1000", symbol, symbol))
		return
	}

	_, err = ab.userRepo.AssignAgentToSymbol(ctx, userID, agentID, pairID, budget)
	if err != nil {
		ab.sendMessage(telegramID, fmt.Sprintf("❌ Failed: %v", err))
		return
	}

	ab.sendMessage(telegramID, fmt.Sprintf("✅ Agent assigned to %s with $%.2f budget\n\nStart trading: /start_agent %s", symbol, budget, agentID[:8]+"..."))
}

// handleStartAgent starts agent trading
func (ab *AgentBot) handleStartAgent(ctx context.Context, telegramID int64, userID string, args []string) {
	if len(args) < 1 {
		ab.sendMessage(telegramID, "Usage: /start_agent <agent_id>")
		return
	}

	agentID := args[0]

	// Get agent assignments to find symbol and budget
	assignments, err := ab.userRepo.GetAgentAssignments(ctx, userID)
	if err != nil {
		ab.sendMessage(telegramID, "❌ Failed to get assignments")
		return
	}

	var assignment *models.AgentSymbolAssignment
	for _, a := range assignments {
		if a.AgentID == agentID {
			assignment = &a
			break
		}
	}

	if assignment == nil {
		ab.sendMessage(telegramID, "❌ Agent not assigned to any symbol. Use /assign_agent first")
		return
	}

	// Get trading pair to find symbol
	pairs, _ := ab.userRepo.GetUserTradingPairs(ctx, userID)
	var symbol string
	for _, p := range pairs {
		if p.ID == assignment.TradingPairID {
			symbol = p.Symbol
			break
		}
	}

	// Get exchange credentials
	exch, err := ab.userRepo.GetUserExchange(ctx, userID, "binance") // TODO: get from pair
	if err != nil {
		ab.sendMessage(telegramID, "❌ Exchange not found")
		return
	}

	// Create exchange adapter
	var exchangeAdapter exchange.Exchange
	_ = &config.ExchangeConfig{ // TODO: Use when real exchange adapters enabled
		APIKey:  exch.APIKey,
		Secret:  exch.APISecret,
		Testnet: exch.Testnet,
	}

	switch exch.Exchange {
	case "binance":
		// TODO: Re-enable when binance adapter is fixed
		// exchangeAdapter, err = exchange.NewBinanceAdapter(exchangeConfig)
		ab.sendMessage(telegramID, "❌ Binance adapter temporarily disabled. Use mock exchange for testing.")
		// Use mock for now
		exchangeAdapter = &exchange.MockExchange{}
	case "bybit":
		// TODO: Re-enable when bybit adapter is fixed
		// exchangeAdapter, err = exchange.NewBybitAdapter(exchangeConfig)
		ab.sendMessage(telegramID, "❌ Bybit adapter temporarily disabled. Use mock exchange for testing.")
		exchangeAdapter = &exchange.MockExchange{}
	default:
		ab.sendMessage(telegramID, fmt.Sprintf("❌ Unsupported exchange: %s", exch.Exchange))
		return
	}

	// Start agent
	budget, _ := assignment.Budget.Float64()
	err = ab.agenticManager.StartAgenticAgent(ctx, agentID, symbol, budget, exchangeAdapter)
	if err != nil {
		ab.sendMessage(telegramID, fmt.Sprintf("❌ Failed to start agent: %v", err))
		return
	}

	// Get agent config for details
	agentConfig, _ := ab.agentRepo.GetAgent(ctx, agentID)
	emoji := agents.GetAgentColorEmoji(agentConfig.Personality)

	msg := fmt.Sprintf(`✅ Agent started!

%s *%s*
Symbol: %s
Budget: $%.2f
Interval: %v

🧠 The agent will now:
• Think step-by-step (Chain-of-Thought)
• Remember past experiences
• Reflect on each trade
• Adapt its strategy
• Plan ahead

First decision in ~%v`,
		emoji, agentConfig.Name, symbol, budget, agentConfig.DecisionInterval, agentConfig.DecisionInterval)

	ab.sendMessageMarkdown(telegramID, msg)
}

// handleStopAgent stops agent
func (ab *AgentBot) handleStopAgent(ctx context.Context, telegramID int64, userID string, args []string) {
	if len(args) < 1 {
		ab.sendMessage(telegramID, "Usage: /stop_agent <agent_id>")
		return
	}

	agentID := args[0]

	err := ab.agenticManager.StopAgenticAgent(ctx, agentID)
	if err != nil {
		ab.sendMessage(telegramID, fmt.Sprintf("❌ Failed: %v", err))
		return
	}

	ab.sendMessage(telegramID, "✅ Agent stopped")
}

// handleListAgents lists all user's agents
func (ab *AgentBot) handleListAgents(ctx context.Context, telegramID int64, userID string) {
	userAgents, err := ab.agentRepo.GetUserAgents(ctx, userID)
	if err != nil {
		ab.sendMessage(telegramID, "❌ Failed to load agents")
		return
	}

	if len(userAgents) == 0 {
		ab.sendMessage(telegramID, "You have no agents. Create one: /create_agent conservative \"My Agent\"")
		return
	}

	runningAgents := ab.agenticManager.GetRunningAgents()
	runningMap := make(map[string]bool)
	for _, runner := range runningAgents {
		runningMap[runner.Config.ID] = true
	}

	msg := "🤖 *Your AI Agents:*\n\n"
	for _, agent := range userAgents {
		status := "⏸️ Stopped"
		if runningMap[agent.ID] {
			status = "▶️ Running"
		}

		emoji := agents.GetAgentColorEmoji(agent.Personality)
		msg += fmt.Sprintf("%s *%s*\n", emoji, agent.Name)
		msg += fmt.Sprintf("   ID: `%s...`\n", agent.ID[:8])
		msg += fmt.Sprintf("   Status: %s\n", status)
		msg += fmt.Sprintf("   Personality: %s\n\n", agent.Personality)
	}

	msg += "\n💡 /start_agent ID | /stop_agent ID | /stats ID"

	ab.sendMessageMarkdown(telegramID, msg)
}

// handleStats shows agent statistics
func (ab *AgentBot) handleStats(ctx context.Context, telegramID int64, userID string, args []string) {
	if len(args) < 1 {
		ab.sendMessage(telegramID, "Usage: /stats <agent_id>")
		return
	}

	agentID := args[0]

	agentConfig, err := ab.agentRepo.GetAgent(ctx, agentID)
	if err != nil || agentConfig.UserID != userID {
		ab.sendMessage(telegramID, "❌ Agent not found")
		return
	}

	runner, isRunning := ab.agenticManager.GetAgenticRunner(agentID)

	emoji := agents.GetAgentColorEmoji(agentConfig.Personality)
	msg := fmt.Sprintf("📊 *Agent Statistics*\n\n%s *%s*\n", emoji, agentConfig.Name)

	if isRunning {
		msg += fmt.Sprintf("Status: ▶️ Running\n")
		msg += fmt.Sprintf("Last Decision: %s ago\n", time.Since(runner.LastDecisionAt).Round(time.Second))
		msg += fmt.Sprintf("Last Reflection: %s ago\n", time.Since(runner.LastReflectionAt).Round(time.Second))
	} else {
		msg += "Status: ⏸️ Stopped\n"
	}

	ab.sendMessageMarkdown(telegramID, msg)
}

// handlePersonalities shows available personalities
func (ab *AgentBot) handlePersonalities(telegramID int64) {
	msg := "🤖 *Agent Personalities:*\n\n"

	personalities := []models.AgentPersonality{
		models.PersonalityConservative,
		models.PersonalityAggressive,
		models.PersonalityBalanced,
		models.PersonalityScalper,
		models.PersonalitySwing,
		models.PersonalityNewsTrader,
		models.PersonalityWhaleHunter,
		models.PersonalityContrarian,
	}

	for _, p := range personalities {
		emoji := agents.GetAgentColorEmoji(p)
		desc := agents.GetAgentDescription(p)
		msg += fmt.Sprintf("%s *%s*\n%s\n\n", emoji, p, desc)
	}

	msg += "Create: /create_agent <personality> <name>"

	ab.sendMessageMarkdown(telegramID, msg)
}

// Helper methods

func (ab *AgentBot) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	ab.api.Send(msg)
}

func (ab *AgentBot) sendMessageMarkdown(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	ab.api.Send(msg)
}
