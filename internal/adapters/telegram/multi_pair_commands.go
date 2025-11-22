package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/alexanderselivanov/trader/pkg/models"
)

// PairBotManager interface for multi-pair operations
type PairBotManager interface {
	StartUserPairBot(ctx context.Context, userID int64, symbol string) error
	StopUserPairBot(ctx context.Context, userID int64, symbol string) error
	StopAllUserBots(ctx context.Context, userID int64) error
}

// Multi-pair specific command handlers

// handleAddPair adds new trading pair
func (mb *MultiUserBot) handleAddPair(ctx context.Context, user *models.User, args []string) (string, error) {
	if len(args) < 2 {
		return `📝 *Add Trading Pair*

Usage: /addpair <symbol> <balance> [exchange]

Examples:
/addpair BTC/USDT 1000
/addpair ETH/USDT 500 binance
/addpair SOL/USDT 300

Note: If no exchange specified, uses your first configured exchange.`, nil
	}

	symbol := strings.ToUpper(args[0])
	balance, err := strconv.ParseFloat(args[1], 64)
	if err != nil || balance <= 0 {
		return "❌ Invalid balance. Must be positive number.", nil
	}

	repo := mb.botManager.GetUserRepository()

	// Check if pair already exists
	existing, err := repo.GetConfigBySymbol(ctx, user.ID, symbol)
	if err != nil {
		return "", err
	}

	if existing != nil {
		return fmt.Sprintf("⚠️ Pair %s already configured. Use /removepair first if you want to reconfigure.", symbol), nil
	}

	// Get existing configs to reuse exchange credentials
	configs, err := repo.GetAllConfigs(ctx, user.ID)
	if err != nil {
		return "", err
	}

	var exchangeName, apiKey, apiSecret string
	var testnet bool

	if len(configs) > 0 {
		// Reuse credentials from first pair
		exchangeName = configs[0].Exchange
		apiKey = configs[0].APIKey
		apiSecret = configs[0].APISecret
		testnet = configs[0].Testnet

		// Override exchange if specified
		if len(args) > 2 {
			exchangeName = strings.ToLower(args[2])
			if exchangeName != "binance" && exchangeName != "bybit" {
				return "❌ Unsupported exchange. Use: binance or bybit", nil
			}
		}
	} else {
		return "⚠️ No exchange configured. Use /connect first to setup your exchange credentials.", nil
	}

	// Create new pair config
	config := &models.UserConfig{
		UserID:             user.ID,
		Exchange:           exchangeName,
		APIKey:             apiKey,
		APISecret:          apiSecret,
		Testnet:            testnet,
		Symbol:             symbol,
		InitialBalance:     models.NewDecimal(balance),
		MaxPositionPercent: models.NewDecimal(30),
		MaxLeverage:        3,
		StopLossPercent:    models.NewDecimal(2),
		TakeProfitPercent:  models.NewDecimal(5),
	}

	if err := repo.AddPairConfig(ctx, config); err != nil {
		return "", fmt.Errorf("failed to add pair: %w", err)
	}

	return fmt.Sprintf(`✅ *Trading Pair Added*

Symbol: %s
Exchange: %s
Balance: $%.2f

Use /start_trading %s to start trading this pair.
Use /listpairs to see all your pairs.`,
		symbol, exchangeName, balance, symbol), nil
}

// handleListPairs lists all user's trading pairs
func (mb *MultiUserBot) handleListPairs(ctx context.Context, user *models.User) (string, error) {
	repo := mb.botManager.GetUserRepository()

	configs, err := repo.GetAllConfigs(ctx, user.ID)
	if err != nil {
		return "", err
	}

	if len(configs) == 0 {
		return "📭 *No Trading Pairs*\n\nUse /connect to setup your exchange, then /addpair to add trading pairs.", nil
	}

	var response strings.Builder
	response.WriteString(fmt.Sprintf("📊 *Your Trading Pairs (%d)*\n\n", len(configs)))

	for i, config := range configs {
		// Get state for this pair
		state, err := repo.GetStateBySymbol(ctx, user.ID, config.Symbol)
		if err != nil {
			continue
		}

		status := "🔴 Stopped"
		if config.IsTrading {
			status = "🟢 Running"
		}

		initialBalance := models.ToFloat64(config.InitialBalance)
		equity := models.ToFloat64(state.Equity)
		pnl := equity - initialBalance
		pnlPercent := (pnl / initialBalance) * 100
		dailyPnL := models.ToFloat64(state.DailyPnL)
		
		response.WriteString(fmt.Sprintf(`%d. *%s* %s
   Exchange: %s
   Balance: $%.2f → Equity: $%.2f
   PnL: $%.2f (%.2f%%)
   Daily: $%.2f

`, i+1, config.Symbol, status, config.Exchange,
			initialBalance, equity,
			pnl, pnlPercent, dailyPnL))
	}

	response.WriteString("Commands:\n")
	response.WriteString("/addpair <symbol> <balance> - Add pair\n")
	response.WriteString("/removepair <symbol> - Remove pair\n")
	response.WriteString("/start_trading [symbol] - Start trading\n")
	response.WriteString("/stop_trading [symbol] - Stop trading")

	return response.String(), nil
}

// handleRemovePair removes trading pair
func (mb *MultiUserBot) handleRemovePair(ctx context.Context, user *models.User, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /removepair BTC/USDT", nil
	}

	symbol := strings.ToUpper(args[0])

	repo := mb.botManager.GetUserRepository()

	if err := repo.RemovePairConfig(ctx, user.ID, symbol); err != nil {
		if strings.Contains(err.Error(), "while bot is active") {
			return fmt.Sprintf("⚠️ Cannot remove %s while trading is active.\nUse /stop_trading %s first.", symbol, symbol), nil
		}
		return "", err
	}

	return fmt.Sprintf("✅ Trading pair %s removed successfully.", symbol), nil
}

// handleStartTradingMulti starts trading (one or all pairs)
func (mb *MultiUserBot) handleStartTradingMulti(ctx context.Context, user *models.User, args []string) (string, error) {
	repo := mb.botManager.GetUserRepository()

	// If symbol specified, start only that pair
	if len(args) > 0 {
		symbol := strings.ToUpper(args[0])

		config, err := repo.GetConfigBySymbol(ctx, user.ID, symbol)
		if err != nil {
			return "", err
		}

		if config == nil {
			return fmt.Sprintf("⚠️ Pair %s not found. Use /listpairs to see your pairs.", symbol), nil
		}

		if config.IsTrading {
			return fmt.Sprintf("⚠️ %s is already trading.", symbol), nil
		}

		// Start specific pair
		if pairManager, ok := mb.botManager.(PairBotManager); ok {
			if err := pairManager.StartUserPairBot(ctx, user.ID, symbol); err != nil {
				return "", fmt.Errorf("failed to start bot: %w", err)
			}
		} else {
			return "", fmt.Errorf("bot manager doesn't support multi-pair")
		}

		return fmt.Sprintf(`🚀 *Trading Started: %s*

Exchange: %s
Balance: $%.2f

Use /status %s to check progress.`,
			symbol, config.Exchange, models.ToFloat64(config.InitialBalance), symbol), nil
	}

	// Start all pairs
	configs, err := repo.GetAllConfigs(ctx, user.ID)
	if err != nil {
		return "", err
	}

	if len(configs) == 0 {
		return "⚠️ No trading pairs configured.\nUse /addpair to add pairs first.", nil
	}

	started := 0
	var errors []string

	for _, config := range configs {
		if config.IsTrading {
			continue // Already trading
		}

		if pairManager, ok := mb.botManager.(PairBotManager); ok {
			if err := pairManager.StartUserPairBot(ctx, user.ID, config.Symbol); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", config.Symbol, err))
			} else {
				started++
			}
		}
	}

	response := fmt.Sprintf("🚀 *Trading Started*\n\n%d pairs now trading.\n\n", started)

	if len(errors) > 0 {
		response += "⚠️ Errors:\n"
		for _, e := range errors {
			response += fmt.Sprintf("- %s\n", e)
		}
	}

	response += "\nUse /status to check all pairs."

	return response, nil
}

// handleStopTradingMulti stops trading (one or all pairs)
func (mb *MultiUserBot) handleStopTradingMulti(ctx context.Context, user *models.User, args []string) (string, error) {
	repo := mb.botManager.GetUserRepository()

	// If symbol specified, stop only that pair
	if len(args) > 0 {
		symbol := strings.ToUpper(args[0])

		config, err := repo.GetConfigBySymbol(ctx, user.ID, symbol)
		if err != nil {
			return "", err
		}

		if config == nil {
			return fmt.Sprintf("⚠️ Pair %s not found.", symbol), nil
		}

		if !config.IsTrading {
			return fmt.Sprintf("⚠️ %s is not trading.", symbol), nil
		}

		// Stop specific pair
		if pairManager, ok := mb.botManager.(PairBotManager); ok {
			if err := pairManager.StopUserPairBot(ctx, user.ID, symbol); err != nil {
			return "", err
		}

		return fmt.Sprintf("⏸️ *Trading Stopped: %s*", symbol), nil
	}

	// Stop all pairs
	if pairManager, ok := mb.botManager.(PairBotManager); ok {
		if err := pairManager.StopAllUserBots(ctx, user.ID); err != nil {
			return "", err
		}
	} else {
		return "", fmt.Errorf("bot manager doesn't support multi-pair")
	}
	
	return "⏸️ *All Trading Stopped*\n\nAll your trading pairs have been stopped.", nil
}

// handleStatusMulti shows status (one or all pairs)
func (mb *MultiUserBot) handleStatusMulti(ctx context.Context, user *models.User, args []string) (string, error) {
	repo := mb.botManager.GetUserRepository()

	// If symbol specified, show only that pair
	if len(args) > 0 {
		symbol := strings.ToUpper(args[0])

		config, err := repo.GetConfigBySymbol(ctx, user.ID, symbol)
		if err != nil {
			return "", err
		}

		if config == nil {
			return fmt.Sprintf("⚠️ Pair %s not found.", symbol), nil
		}

		state, err := repo.GetStateBySymbol(ctx, user.ID, symbol)
		if err != nil {
			return "", err
		}

		status := "🔴 Stopped"
		if config.IsTrading {
			status = "🟢 Running"
		}

		initialBalance := models.ToFloat64(config.InitialBalance)
		equity := models.ToFloat64(state.Equity)
		pnl := equity - initialBalance
		pnlPercent := (pnl / initialBalance) * 100
		
		return fmt.Sprintf(`📊 *Status: %s*

Status: %s
Exchange: %s

💰 Initial: $%.2f
📈 Equity: $%.2f
📊 Total PnL: $%.2f (%.2f%%)
📉 Daily PnL: $%.2f
🏔️ Peak Equity: $%.2f`,
			symbol, status, config.Exchange,
			initialBalance,
			equity,
			pnl, pnlPercent,
			models.ToFloat64(state.DailyPnL),
			models.ToFloat64(state.PeakEquity)), nil
	}

	// Show all pairs
	return mb.handleListPairs(ctx, user)
}

// handleConfigMulti shows config (one or all pairs)
func (mb *MultiUserBot) handleConfigMulti(ctx context.Context, user *models.User, args []string) (string, error) {
	repo := mb.botManager.GetUserRepository()

	if len(args) > 0 {
		symbol := strings.ToUpper(args[0])

		config, err := repo.GetConfigBySymbol(ctx, user.ID, symbol)
		if err != nil {
			return "", err
		}

		if config == nil {
			return fmt.Sprintf("⚠️ Pair %s not found.", symbol), nil
		}

		return mb.formatConfig(config), nil
	}

	// Show all configs
	configs, err := repo.GetAllConfigs(ctx, user.ID)
	if err != nil {
		return "", err
	}

	if len(configs) == 0 {
		return "⚠️ No pairs configured.", nil
	}

	var response strings.Builder
	response.WriteString(fmt.Sprintf("⚙️ *Your Configurations (%d pairs)*\n\n", len(configs)))

	for _, config := range configs {
		response.WriteString(fmt.Sprintf("*%s* (%s)\n", config.Symbol, config.Exchange))
		response.WriteString(fmt.Sprintf("Balance: $%.2f | Leverage: %dx | Trading: %v\n\n",
			models.ToFloat64(config.InitialBalance), config.MaxLeverage, config.IsTrading))
	}

	response.WriteString("Use /config <symbol> for detailed config.")

	return response.String(), nil
}
