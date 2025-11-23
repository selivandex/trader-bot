package telegram

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/config"
	"github.com/selivandex/trader-bot/internal/adapters/exchange"
	"github.com/selivandex/trader-bot/internal/agents"
	"github.com/selivandex/trader-bot/internal/users"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
	"github.com/selivandex/trader-bot/pkg/templates"
)

// AgentBot handles Telegram commands for AI agents
type AgentBot struct {
	api             *tgbotapi.BotAPI
	cfg             *config.Config
	agenticManager  *agents.AgenticManager
	userRepo        *users.AgentsRepository
	agentRepo       *agents.Repository
	adminRepo       *users.AdminRepository
	templateManager templates.Renderer
}

// NewAgentBot creates new Telegram bot for agents
func NewAgentBot(cfg *config.Config, agenticManager *agents.AgenticManager, userRepo *users.AgentsRepository, agentRepo *agents.Repository, adminRepo *users.AdminRepository, templateRenderer templates.Renderer) (*AgentBot, error) {
	bot, err := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	bot.Debug = false

	logger.Info("Telegram agent bot authorized",
		zap.String("username", bot.Self.UserName),
		zap.Bool("admin_enabled", cfg.Telegram.AdminID != 0),
	)

	return &AgentBot{
		api:             bot,
		cfg:             cfg,
		agenticManager:  agenticManager,
		userRepo:        userRepo,
		agentRepo:       agentRepo,
		adminRepo:       adminRepo,
		templateManager: templateRenderer,
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

	cmd := message.Command()
	args := strings.Fields(message.CommandArguments())

	// Check if admin command
	if ab.isAdminCommand(cmd) {
		if !ab.isAdmin(telegramID) {
			ab.sendTemplateWithName(telegramID, "errors.tmpl", "admin_required", nil)
			return
		}
		ab.handleAdminCommand(ctx, telegramID, cmd, args)
		return
	}

	// Get or create user
	user, err := ab.userRepo.GetUserByTelegramID(ctx, telegramID)
	if err != nil {
		// Create new user
		user, err = ab.userRepo.CreateUser(ctx, telegramID, message.From.UserName, message.From.FirstName)
		if err != nil {
			ab.sendTemplateWithName(telegramID, "errors.tmpl", "user_create_failed", nil)
			return
		}
	}

	// Check if user is active or banned
	if !user.IsActive {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "account_deactivated", nil)
		return
	}
	if user.IsBanned {
		reason := "No reason provided"
		if user.BanReason != "" {
			reason = user.BanReason
		}
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "account_banned", map[string]interface{}{
			"Reason": reason,
		})
		return
	}

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
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "unknown_command", nil)
	}
}

// isAdmin checks if user is system admin
func (ab *AgentBot) isAdmin(telegramID int64) bool {
	return ab.cfg.Telegram.AdminID != 0 && ab.cfg.Telegram.AdminID == telegramID
}

// isAdminCommand checks if command requires admin access
func (ab *AgentBot) isAdminCommand(cmd string) bool {
	adminCommands := map[string]bool{
		"admin":           true,
		"system_stats":    true,
		"all_users":       true,
		"all_agents":      true,
		"news_stats":      true,
		"trade_stats":     true,
		"ban_user":        true,
		"unban_user":      true,
		"deactivate_user": true,
		"activate_user":   true,
		"stop_any_agent":  true,
		"user_info":       true,
	}
	return adminCommands[cmd]
}

// handleStart shows welcome message
func (ab *AgentBot) handleStart(telegramID int64, user *models.User) {
	data := map[string]interface{}{
		"Username": user.Username,
	}

	msg, err := ab.templateManager.ExecuteTemplate("welcome.tmpl", data)
	if err != nil {
		logger.Error("failed to render welcome template", zap.Error(err))
		return
	}

	ab.sendMessageMarkdown(telegramID, msg)
}

// handleConnect connects user's exchange
func (ab *AgentBot) handleConnect(ctx context.Context, telegramID int64, userID string, args []string) {
	if len(args) < 3 {
		ab.sendTemplateWithName(telegramID, "usage.tmpl", "connect", nil)
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
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	mode := "testnet"
	if !testnet {
		mode = "LIVE"
	}

	data := map[string]interface{}{
		"Exchange": strings.ToUpper(exchange),
		"Mode":     mode,
	}

	msg, err := ab.templateManager.ExecuteTemplate("exchange_connected.tmpl", data)
	if err != nil {
		logger.Error("failed to render template", zap.Error(err))
		return
	}

	ab.sendMessage(telegramID, msg)
}

// handleAddTicker adds trading pair
func (ab *AgentBot) handleAddTicker(ctx context.Context, telegramID int64, userID string, args []string) {
	if len(args) < 2 {
		ab.sendTemplateWithName(telegramID, "usage.tmpl", "add_ticker", nil)
		return
	}

	symbol := args[0]
	budget, _ := strconv.ParseFloat(args[1], 64)

	if budget <= 0 {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "budget_must_be_positive", nil)
		return
	}

	// Get user's exchange (assume first one)
	// TODO: Support multiple exchanges
	exch, err := ab.userRepo.GetUserExchange(ctx, userID, "binance")
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "exchange_not_found", nil)
		return
	}

	_, err = ab.userRepo.AddTradingPair(ctx, userID, exch.ID, symbol, budget)
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	data := map[string]interface{}{
		"Symbol": symbol,
		"Budget": budget,
	}

	msg, err := ab.templateManager.ExecuteTemplate("ticker_added.tmpl", data)
	if err != nil {
		logger.Error("failed to render template", zap.Error(err))
		return
	}

	ab.sendMessage(telegramID, msg)
}

// handleCreateAgent creates new agent
func (ab *AgentBot) handleCreateAgent(ctx context.Context, telegramID int64, userID string, args []string) {
	if len(args) < 2 {
		ab.sendTemplateWithName(telegramID, "usage.tmpl", "create_agent", nil)
		return
	}

	personality := models.AgentPersonality(args[0])
	name := strings.Join(args[1:], " ")

	config, err := ab.agenticManager.CreateAgentFromPersonality(ctx, userID, personality, name)
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	data := map[string]interface{}{
		"Emoji":           agents.GetAgentColorEmoji(personality),
		"Name":            config.Name,
		"ShortID":         config.ID[:8] + "...",
		"Personality":     personality,
		"MaxPosition":     config.Strategy.MaxPositionPercent,
		"MaxLeverage":     config.Strategy.MaxLeverage,
		"StopLoss":        config.Strategy.StopLossPercent,
		"TakeProfit":      config.Strategy.TakeProfitPercent,
		"TechnicalWeight": config.Specialization.TechnicalWeight * 100,
		"NewsWeight":      config.Specialization.NewsWeight * 100,
		"OnChainWeight":   config.Specialization.OnChainWeight * 100,
		"SentimentWeight": config.Specialization.SentimentWeight * 100,
	}

	msg, err := ab.templateManager.ExecuteTemplate("agent_created.tmpl", data)
	if err != nil {
		logger.Error("failed to render template", zap.Error(err))
		return
	}

	ab.sendMessageMarkdown(telegramID, msg)
}

// handleAssignAgent assigns agent to symbol
func (ab *AgentBot) handleAssignAgent(ctx context.Context, telegramID int64, userID string, args []string) {
	if len(args) < 3 {
		ab.sendTemplateWithName(telegramID, "usage.tmpl", "assign_agent", nil)
		return
	}

	agentID := args[0]
	symbol := args[1]
	budget, _ := strconv.ParseFloat(args[2], 64)

	// Find trading pair ID
	pairs, err := ab.userRepo.GetUserTradingPairs(ctx, userID)
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
			"Error": "get trading pairs",
		})
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
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "symbol_not_found", map[string]interface{}{
			"Symbol": symbol,
		})
		return
	}

	_, err = ab.userRepo.AssignAgentToSymbol(ctx, userID, agentID, pairID, budget)
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	data := map[string]interface{}{
		"Symbol":  symbol,
		"Budget":  budget,
		"ShortID": agentID[:8] + "...",
	}

	msg, err := ab.templateManager.ExecuteTemplate("agent_assigned.tmpl", data)
	if err != nil {
		logger.Error("failed to render template", zap.Error(err))
		return
	}

	ab.sendMessage(telegramID, msg)
}

// handleStartAgent starts agent trading
func (ab *AgentBot) handleStartAgent(ctx context.Context, telegramID int64, userID string, args []string) {
	if len(args) < 1 {
		ab.sendTemplateWithName(telegramID, "usage.tmpl", "start_agent", nil)
		return
	}

	agentID := args[0]
	paperMode := false

	// Check for --paper flag
	for _, arg := range args[1:] {
		if arg == "--paper" {
			paperMode = true
			break
		}
	}

	// Get agent assignments to find symbol and budget
	assignments, err := ab.userRepo.GetAgentAssignments(ctx, userID)
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
			"Error": "get assignments",
		})
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
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "agent_not_assigned", nil)
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
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "exchange_not_found", nil)
		return
	}

	// Create exchange adapter
	var exchangeAdapter exchange.Exchange

	if paperMode {
		// Create mock exchange for paper trading
		budget, _ := assignment.Budget.Float64()
		exchangeAdapter = exchange.NewMockExchange(exch.Exchange, budget)

		data := map[string]interface{}{
			"Exchange": exch.Exchange,
			"Symbol":   symbol,
			"Budget":   budget,
		}

		msg, err := ab.templateManager.ExecuteTemplate("paper_mode_info.tmpl", data)
		if err != nil {
			logger.Error("failed to render template", zap.Error(err))
		} else {
			ab.sendMessageMarkdown(telegramID, msg)
		}

		logger.Info("📝 Paper trading mode enabled",
			zap.String("symbol", symbol),
			zap.Float64("balance", budget),
		)
	} else {
		// Create real exchange adapter
		switch exch.Exchange {
		case "binance":
			exchangeAdapter, err = exchange.NewBinanceAdapter(
				exch.APIKey,
				exch.APISecret,
				exch.Testnet,
				&ab.cfg.Exchanges.Binance,
			)
			if err != nil {
				ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
					"Error": "connect Binance: " + err.Error(),
				})
				return
			}
			logger.Info("✅ Binance connected",
				zap.String("symbol", symbol),
				zap.Bool("testnet", exch.Testnet),
			)
		case "bybit":
			exchangeAdapter, err = exchange.NewBybitAdapter(
				exch.APIKey,
				exch.APISecret,
				exch.Testnet,
				&ab.cfg.Exchanges.Bybit,
			)
			if err != nil {
				ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
					"Error": "connect Bybit: " + err.Error(),
				})
				return
			}
			logger.Info("✅ Bybit connected",
				zap.String("symbol", symbol),
				zap.Bool("testnet", exch.Testnet),
			)
		default:
			ab.sendTemplateWithName(telegramID, "errors.tmpl", "unsupported_exchange", map[string]interface{}{
				"Exchange": exch.Exchange,
			})
			return
		}
	}

	// Start agent
	budget, _ := assignment.Budget.Float64()
	err = ab.agenticManager.StartAgenticAgent(ctx, agentID, symbol, budget, exchangeAdapter)
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
			"Error": "start agent: " + err.Error(),
		})
		return
	}

	// Get agent config for details
	agentConfig, _ := ab.agentRepo.GetAgent(ctx, agentID)

	modeStr := "🟢 LIVE"
	if paperMode {
		modeStr = "📝 PAPER"
	}

	data := map[string]interface{}{
		"Emoji":     agents.GetAgentColorEmoji(agentConfig.Personality),
		"AgentName": agentConfig.Name,
		"Mode":      modeStr,
		"Symbol":    symbol,
		"Budget":    budget,
		"Interval":  agentConfig.DecisionInterval,
	}

	msg, err := ab.templateManager.ExecuteTemplate("agent_started.tmpl", data)
	if err != nil {
		logger.Error("failed to render template", zap.Error(err))
		return
	}

	ab.sendMessageMarkdown(telegramID, msg)
}

// handleStopAgent stops agent
func (ab *AgentBot) handleStopAgent(ctx context.Context, telegramID int64, userID string, args []string) {
	if len(args) < 1 {
		ab.sendTemplateWithName(telegramID, "usage.tmpl", "stop_agent", nil)
		return
	}

	agentID := args[0]

	err := ab.agenticManager.StopAgenticAgent(ctx, agentID)
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	ab.sendTemplateWithName(telegramID, "errors.tmpl", "agent_stopped", nil)
}

// handleListAgents lists all user's agents
func (ab *AgentBot) handleListAgents(ctx context.Context, telegramID int64, userID string) {
	userAgents, err := ab.agentRepo.GetUserAgents(ctx, userID)
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
			"Error": "load agents",
		})
		return
	}

	if len(userAgents) == 0 {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "no_agents", nil)
		return
	}

	runningAgents := ab.agenticManager.GetRunningAgents()
	runningMap := make(map[string]bool)
	for _, runner := range runningAgents {
		runningMap[runner.Config.ID] = true
	}

	agentsList := make([]map[string]interface{}, 0, len(userAgents))
	for _, agent := range userAgents {
		status := "⏸️ Stopped"
		if runningMap[agent.ID] {
			status = "▶️ Running"
		}

		agentsList = append(agentsList, map[string]interface{}{
			"Emoji":       agents.GetAgentColorEmoji(agent.Personality),
			"Name":        agent.Name,
			"ShortID":     agent.ID[:8],
			"Status":      status,
			"Personality": agent.Personality,
		})
	}

	data := map[string]interface{}{
		"Agents": agentsList,
	}

	msg, err := ab.templateManager.ExecuteTemplate("agents_list.tmpl", data)
	if err != nil {
		logger.Error("failed to render template", zap.Error(err))
		return
	}

	ab.sendMessageMarkdown(telegramID, msg)
}

// handleStats shows agent statistics
func (ab *AgentBot) handleStats(ctx context.Context, telegramID int64, userID string, args []string) {
	if len(args) < 1 {
		ab.sendTemplateWithName(telegramID, "usage.tmpl", "stats", nil)
		return
	}

	agentID := args[0]

	agentConfig, err := ab.agentRepo.GetAgent(ctx, agentID)
	if err != nil || agentConfig.UserID != userID {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "agent_not_found", nil)
		return
	}

	runner, isRunning := ab.agenticManager.GetAgenticRunner(agentID)

	status := "⏸️ Stopped"
	lastDecision := ""
	lastReflection := ""

	if isRunning {
		status = "▶️ Running"
		lastDecision = time.Since(runner.LastDecisionAt).Round(time.Second).String() + " ago"
		lastReflection = time.Since(runner.LastReflectionAt).Round(time.Second).String() + " ago"
	}

	data := map[string]interface{}{
		"Emoji":          agents.GetAgentColorEmoji(agentConfig.Personality),
		"Name":           agentConfig.Name,
		"Status":         status,
		"IsRunning":      isRunning,
		"LastDecision":   lastDecision,
		"LastReflection": lastReflection,
	}

	msg, err := ab.templateManager.ExecuteTemplate("agent_stats.tmpl", data)
	if err != nil {
		logger.Error("failed to render template", zap.Error(err))
		return
	}

	ab.sendMessageMarkdown(telegramID, msg)
}

// handlePersonalities shows available personalities
func (ab *AgentBot) handlePersonalities(telegramID int64) {
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

	personalityList := make([]map[string]interface{}, 0, len(personalities))
	for _, p := range personalities {
		personalityList = append(personalityList, map[string]interface{}{
			"Emoji":       agents.GetAgentColorEmoji(p),
			"Name":        p,
			"Description": agents.GetAgentDescription(p),
		})
	}

	data := map[string]interface{}{
		"Personalities": personalityList,
	}

	msg, err := ab.templateManager.ExecuteTemplate("personalities_list.tmpl", data)
	if err != nil {
		logger.Error("failed to render template", zap.Error(err))
		return
	}

	ab.sendMessageMarkdown(telegramID, msg)
}

// Admin command handlers

func (ab *AgentBot) handleAdminCommand(ctx context.Context, telegramID int64, cmd string, args []string) {
	switch cmd {
	case "admin":
		ab.handleAdminHelp(telegramID)
	case "system_stats":
		ab.handleSystemStats(ctx, telegramID)
	case "all_users":
		ab.handleAllUsers(ctx, telegramID)
	case "all_agents":
		ab.handleAllAgents(ctx, telegramID)
	case "news_stats":
		ab.handleNewsStats(ctx, telegramID)
	case "trade_stats":
		ab.handleTradeStats(ctx, telegramID)
	case "ban_user":
		ab.handleBanUser(ctx, telegramID, args)
	case "unban_user":
		ab.handleUnbanUser(ctx, telegramID, args)
	case "deactivate_user":
		ab.handleDeactivateUser(ctx, telegramID, args)
	case "activate_user":
		ab.handleActivateUser(ctx, telegramID, args)
	case "stop_any_agent":
		ab.handleStopAnyAgent(ctx, telegramID, args)
	case "user_info":
		ab.handleUserInfo(ctx, telegramID, args)
	default:
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "unknown_admin_command", nil)
	}
}

func (ab *AgentBot) handleAdminHelp(telegramID int64) {
	ab.sendTemplate(telegramID, "admin_help.tmpl", nil)
}

func (ab *AgentBot) handleSystemStats(ctx context.Context, telegramID int64) {
	stats, err := ab.adminRepo.GetSystemStats(ctx)
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
			"Error": "get stats: " + err.Error(),
		})
		return
	}

	winRate := 0.0
	if stats.TotalTrades > 0 {
		winRate = float64(stats.ProfitableTrades) / float64(stats.TotalTrades) * 100
	}

	activeUsersPercent := 0.0
	if stats.TotalUsers > 0 {
		activeUsersPercent = float64(stats.ActiveUsers) / float64(stats.TotalUsers) * 100
	}

	activeAgentsPercent := 0.0
	if stats.TotalAgents > 0 {
		activeAgentsPercent = float64(stats.ActiveAgents) / float64(stats.TotalAgents) * 100
	}

	data := map[string]interface{}{
		"TotalUsers":          stats.TotalUsers,
		"ActiveUsers":         stats.ActiveUsers,
		"ActiveUsersPercent":  activeUsersPercent,
		"TotalAgents":         stats.TotalAgents,
		"ActiveAgents":        stats.ActiveAgents,
		"ActiveAgentsPercent": activeAgentsPercent,
		"TotalTrades":         stats.TotalTrades,
		"WinRate":             winRate,
		"TotalVolume":         stats.TotalVolume,
		"TotalProfit":         stats.TotalProfit,
	}

	if stats.TopPerformingAgent != nil {
		data["TopAgent"] = map[string]interface{}{
			"Name":     stats.TopPerformingAgent.AgentName,
			"Username": stats.TopPerformingAgent.Username,
			"Profit":   stats.TopPerformingAgent.TotalProfit,
			"WinRate":  stats.TopPerformingAgent.WinRate * 100,
		}
	}

	if stats.WorstPerformingAgent != nil {
		data["WorstAgent"] = map[string]interface{}{
			"Name":     stats.WorstPerformingAgent.AgentName,
			"Username": stats.WorstPerformingAgent.Username,
			"Profit":   stats.WorstPerformingAgent.TotalProfit,
		}
	}

	ab.sendTemplate(telegramID, "system_stats.tmpl", data)
}

func (ab *AgentBot) handleAllUsers(ctx context.Context, telegramID int64) {
	allUsers, err := ab.adminRepo.GetAllUsers(ctx)
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	if len(allUsers) == 0 {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "no_users_found", nil)
		return
	}

	usersList := make([]map[string]interface{}, 0, 20)
	for i, u := range allUsers {
		if i >= 20 {
			break
		}

		status := "✅"
		if u.IsBanned {
			status = "🚫"
		}

		usersList = append(usersList, map[string]interface{}{
			"Status":       status,
			"Username":     u.Username,
			"TelegramID":   u.TelegramID,
			"ActiveAgents": u.ActiveAgents,
			"TotalAgents":  u.TotalAgents,
			"TotalTrades":  u.TotalTrades,
			"TotalProfit":  u.TotalProfit,
			"RegisteredAt": u.RegisteredAt.Format("2006-01-02"),
		})
	}

	data := map[string]interface{}{
		"Users":     usersList,
		"HasMore":   len(allUsers) > 20,
		"MoreCount": len(allUsers) - 20,
	}

	ab.sendTemplate(telegramID, "all_users.tmpl", data)
}

func (ab *AgentBot) handleAllAgents(ctx context.Context, telegramID int64) {
	allAgents, err := ab.adminRepo.GetAllAgents(ctx)
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	if len(allAgents) == 0 {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "no_agents_found", nil)
		return
	}

	agentsList := make([]map[string]interface{}, 0, 15)
	for i, a := range allAgents {
		if i >= 15 {
			break
		}

		agentsList = append(agentsList, map[string]interface{}{
			"Emoji":       agents.GetAgentColorEmoji(a.Personality),
			"Name":        a.AgentName,
			"Username":    a.Username,
			"ShortID":     a.AgentID[:8],
			"TotalTrades": a.TotalTrades,
			"WinRate":     a.WinRate * 100,
			"TotalProfit": a.TotalProfit,
			"LastTrade":   time.Since(a.LastTradeAt).Round(time.Hour).String(),
		})
	}

	data := map[string]interface{}{
		"Agents":    agentsList,
		"HasMore":   len(allAgents) > 15,
		"MoreCount": len(allAgents) - 15,
	}

	ab.sendTemplate(telegramID, "all_agents.tmpl", data)
}

func (ab *AgentBot) handleNewsStats(ctx context.Context, telegramID int64) {
	stats, err := ab.adminRepo.GetNewsStats(ctx)
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	total := stats.PositiveSentiment + stats.NegativeSentiment + stats.NeutralSentiment
	posPercent := 0.0
	negPercent := 0.0
	neuPercent := 0.0

	if total > 0 {
		posPercent = float64(stats.PositiveSentiment) / float64(total) * 100
		negPercent = float64(stats.NegativeSentiment) / float64(total) * 100
		neuPercent = float64(stats.NeutralSentiment) / float64(total) * 100
	}

	data := map[string]interface{}{
		"TotalNews":         stats.TotalNews,
		"Last24h":           stats.Last24h,
		"PositiveSentiment": stats.PositiveSentiment,
		"PositivePercent":   posPercent,
		"NegativeSentiment": stats.NegativeSentiment,
		"NegativePercent":   negPercent,
		"NeutralSentiment":  stats.NeutralSentiment,
		"NeutralPercent":    neuPercent,
	}

	ab.sendTemplate(telegramID, "news_stats.tmpl", data)
}

func (ab *AgentBot) handleTradeStats(ctx context.Context, telegramID int64) {
	stats, err := ab.adminRepo.GetTradeStats(ctx)
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	data := map[string]interface{}{
		"TotalTrades": stats.TotalTrades,
		"AvgProfit":   stats.AvgProfit,
		"TotalVolume": stats.TotalVolume,
		"Last24h":     stats.Last24h,
		"Last7d":      stats.Last7d,
		"Last30d":     stats.Last30d,
	}

	if len(stats.ByExchange) > 0 {
		data["ByExchange"] = stats.ByExchange
	}

	if len(stats.BySymbol) > 0 {
		topSymbols := make([]map[string]interface{}, 0, 5)
		count := 0
		for symbol, trades := range stats.BySymbol {
			if count >= 5 {
				break
			}
			topSymbols = append(topSymbols, map[string]interface{}{
				"Symbol": symbol,
				"Count":  trades,
			})
			count++
		}
		data["TopSymbols"] = topSymbols
	}

	ab.sendTemplate(telegramID, "trade_stats.tmpl", data)
}

func (ab *AgentBot) handleBanUser(ctx context.Context, telegramID int64, args []string) {
	if len(args) < 2 {
		ab.sendTemplateWithName(telegramID, "usage.tmpl", "ban_user", nil)
		return
	}

	userID := args[0]
	reason := strings.Join(args[1:], " ")

	err := ab.adminRepo.BanUser(ctx, userID, reason)
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	// Get user info to notify
	userStats, _ := ab.adminRepo.GetUserByID(ctx, userID)

	ab.sendTemplate(telegramID, "user_banned.tmpl", map[string]interface{}{
		"Username": userStats.Username,
		"Reason":   reason,
	})

	// Try to notify banned user
	if userStats != nil {
		ab.sendTemplate(userStats.TelegramID, "user_banned_notification.tmpl", map[string]interface{}{
			"Reason": reason,
		})
	}
}

func (ab *AgentBot) handleUnbanUser(ctx context.Context, telegramID int64, args []string) {
	if len(args) < 1 {
		ab.sendTemplateWithName(telegramID, "usage.tmpl", "unban_user", nil)
		return
	}

	userID := args[0]

	err := ab.adminRepo.UnbanUser(ctx, userID)
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	userStats, _ := ab.adminRepo.GetUserByID(ctx, userID)

	ab.sendTemplate(telegramID, "user_unbanned.tmpl", map[string]interface{}{
		"Username": userStats.Username,
	})

	// Notify unbanned user
	if userStats != nil {
		ab.sendTemplate(userStats.TelegramID, "user_restored.tmpl", nil)
	}
}

func (ab *AgentBot) handleDeactivateUser(ctx context.Context, telegramID int64, args []string) {
	if len(args) < 1 {
		ab.sendTemplateWithName(telegramID, "usage.tmpl", "deactivate_user", nil)
		return
	}

	userID := args[0]

	err := ab.adminRepo.DeactivateUser(ctx, userID)
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	userStats, _ := ab.adminRepo.GetUserByID(ctx, userID)

	ab.sendTemplate(telegramID, "user_deactivated.tmpl", map[string]interface{}{
		"Username": userStats.Username,
	})

	// Notify deactivated user
	if userStats != nil {
		ab.sendTemplate(userStats.TelegramID, "user_deactivated_notification.tmpl", nil)
	}
}

func (ab *AgentBot) handleActivateUser(ctx context.Context, telegramID int64, args []string) {
	if len(args) < 1 {
		ab.sendTemplateWithName(telegramID, "usage.tmpl", "activate_user", nil)
		return
	}

	userID := args[0]

	err := ab.adminRepo.ActivateUser(ctx, userID)
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	userStats, _ := ab.adminRepo.GetUserByID(ctx, userID)

	ab.sendTemplate(telegramID, "user_activated.tmpl", map[string]interface{}{
		"Username": userStats.Username,
	})

	// Notify activated user
	if userStats != nil {
		ab.sendTemplate(userStats.TelegramID, "user_activated_notification.tmpl", nil)
	}
}

func (ab *AgentBot) handleStopAnyAgent(ctx context.Context, telegramID int64, args []string) {
	if len(args) < 1 {
		ab.sendTemplateWithName(telegramID, "usage.tmpl", "stop_any_agent", nil)
		return
	}

	agentID := args[0]

	// Stop in manager
	err := ab.agenticManager.StopAgenticAgent(ctx, agentID)
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
			"Error": "stop agent: " + err.Error(),
		})
		return
	}

	// Update DB
	err = ab.adminRepo.StopAgentByAdmin(ctx, agentID)
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
			"Error": "DB update: " + err.Error(),
		})
		return
	}

	ab.sendTemplate(telegramID, "agent_stopped_admin.tmpl", map[string]interface{}{
		"ShortID": agentID[:8],
	})
}

func (ab *AgentBot) handleUserInfo(ctx context.Context, telegramID int64, args []string) {
	if len(args) < 1 {
		ab.sendTemplateWithName(telegramID, "usage.tmpl", "user_info", nil)
		return
	}

	targetTelegramID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "invalid_telegram_id", nil)
		return
	}

	// Get user by telegram ID
	user, err := ab.userRepo.GetUserByTelegramID(ctx, targetTelegramID)
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "user_not_found", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	userStats, err := ab.adminRepo.GetUserByID(ctx, user.ID)
	if err != nil {
		ab.sendTemplateWithName(telegramID, "errors.tmpl", "failed", map[string]interface{}{
			"Error": "get stats: " + err.Error(),
		})
		return
	}

	status := "✅ Active"
	if userStats.IsBanned {
		status = "🚫 Banned"
	}

	data := map[string]interface{}{
		"Username":     userStats.Username,
		"TelegramID":   userStats.TelegramID,
		"Status":       status,
		"UserID":       userStats.UserID,
		"TotalAgents":  userStats.TotalAgents,
		"ActiveAgents": userStats.ActiveAgents,
		"TotalTrades":  userStats.TotalTrades,
		"TotalProfit":  userStats.TotalProfit,
		"RegisteredAt": userStats.RegisteredAt.Format("2006-01-02 15:04"),
	}

	ab.sendTemplate(telegramID, "user_info.tmpl", data)
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

// sendTemplate sends message using template
func (ab *AgentBot) sendTemplate(chatID int64, templateName string, data interface{}) {
	msg, err := ab.templateManager.ExecuteTemplate(templateName, data)
	if err != nil {
		logger.Error("failed to render template",
			zap.String("template", templateName),
			zap.Error(err))
		return
	}
	ab.sendMessageMarkdown(chatID, msg)
}

// sendTemplateWithName sends message using named template within a file (e.g., errors.tmpl, usage.tmpl)
func (ab *AgentBot) sendTemplateWithName(chatID int64, templateFile, templateName string, data interface{}) {
	tmpl := ab.templateManager.GetTemplate(templateFile)
	if tmpl == nil {
		logger.Error("template file not found", zap.String("file", templateFile))
		return
	}

	var buf bytes.Buffer
	err := tmpl.ExecuteTemplate(&buf, templateName, data)
	if err != nil {
		logger.Error("failed to render named template",
			zap.String("file", templateFile),
			zap.String("name", templateName),
			zap.Error(err))
		return
	}

	ab.sendMessageMarkdown(chatID, buf.String())
}
