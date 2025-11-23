package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config represents application configuration
type Config struct {
	Exchanges  ExchangesConfig   `envconfig:"EXCHANGES"`
	Database   DatabaseConfig    `envconfig:"DATABASE"`
	Logging    LoggingConfig     `envconfig:"LOGGING"`
	Mode       TradingModeConfig `envconfig:""`
	Health     HealthConfig      `envconfig:"HEALTH"`
	OnChain    OnChainConfig     `envconfig:"ONCHAIN"`
	News       NewsConfig        `envconfig:"NEWS"`
	ClickHouse ClickHouseConfig  `envconfig:"CLICKHOUSE"`
	Redis      RedisConfig       `envconfig:"REDIS"`
	Telegram   TelegramConfig    `envconfig:"TELEGRAM"`
	AI         AIConfig          `envconfig:"AI"`
	Trading    TradingConfig     `envconfig:"TRADING"`
	Risk       RiskConfig        `envconfig:"RISK"`
}

// TradingModeConfig represents trading mode
type TradingModeConfig struct {
	Mode string `envconfig:"MODE" default:"paper"` // paper or live
}

// ExchangesConfig represents exchange configurations
type ExchangesConfig struct {
	Binance ExchangeConfig `envconfig:"BINANCE"`
	Bybit   ExchangeConfig `envconfig:"BYBIT"`
}

// ExchangeConfig represents single exchange configuration
// NOTE: API keys are stored per-user in database (user_exchanges table), not here
type ExchangeConfig struct {
	TestnetURLPublic  string `envconfig:"TESTNET_URL_PUBLIC" required:"false"`
	TestnetURLPrivate string `envconfig:"TESTNET_URL_PRIVATE" required:"false"`
	MainnetURLPublic  string `envconfig:"MAINNET_URL_PUBLIC" required:"false"`
	MainnetURLPrivate string `envconfig:"MAINNET_URL_PRIVATE" required:"false"`
	DefaultTestnet    bool   `envconfig:"DEFAULT_TESTNET" default:"true"`
}

// GetAPIURLs returns API URLs configuration for CCXT based on testnet flag
func (e *ExchangeConfig) GetAPIURLs(testnet bool, defaultTestnetPublic, defaultTestnetPrivate, defaultMainnetPublic, defaultMainnetPrivate string) map[string]interface{} {
	if testnet {
		publicURL := defaultTestnetPublic
		privateURL := defaultTestnetPrivate

		if e.TestnetURLPublic != "" {
			publicURL = e.TestnetURLPublic
		}
		if e.TestnetURLPrivate != "" {
			privateURL = e.TestnetURLPrivate
		}

		return map[string]interface{}{
			"api": map[string]interface{}{
				"public":  publicURL,
				"private": privateURL,
			},
		}
	}

	publicURL := defaultMainnetPublic
	privateURL := defaultMainnetPrivate

	if e.MainnetURLPublic != "" {
		publicURL = e.MainnetURLPublic
	}
	if e.MainnetURLPrivate != "" {
		privateURL = e.MainnetURLPrivate
	}

	return map[string]interface{}{
		"api": map[string]interface{}{
			"public":  publicURL,
			"private": privateURL,
		},
	}
}

// TradingConfig represents trading parameters
type TradingConfig struct {
	Symbol                    string  `envconfig:"TRADING_SYMBOL" default:"BTC/USDT"`
	InitialBalance            float64 `envconfig:"TRADING_INITIAL_BALANCE" default:"1000.0"`
	MaxPositionPercent        float64 `envconfig:"TRADING_MAX_POSITION_PERCENT" default:"30.0"`
	MaxLeverage               int     `envconfig:"TRADING_MAX_LEVERAGE" default:"3"`
	StopLossPercent           float64 `envconfig:"TRADING_STOP_LOSS_PERCENT" default:"2.0"`
	TakeProfitPercent         float64 `envconfig:"TRADING_TAKE_PROFIT_PERCENT" default:"5.0"`
	ProfitWithdrawalThreshold float64 `envconfig:"TRADING_PROFIT_WITHDRAWAL_THRESHOLD" default:"1.1"`
}

// AIConfig represents AI provider configurations
type AIConfig struct {
	DeepSeek             AIProviderConfig `envconfig:"DEEPSEEK"`
	Claude               AIProviderConfig `envconfig:"CLAUDE"`
	OpenAI               AIProviderConfig `envconfig:"OPENAI"`
	EnsembleEnabled      bool             `envconfig:"AI_ENSEMBLE_ENABLED" default:"true"`
	EnsembleMinConsensus int              `envconfig:"AI_ENSEMBLE_MIN_CONSENSUS" default:"2"`
	DecisionInterval     time.Duration    `envconfig:"AI_DECISION_INTERVAL" default:"30m"`
}

// AIProviderConfig represents single AI provider configuration
type AIProviderConfig struct {
	APIKey  string  `envconfig:"API_KEY" required:"false"`
	Enabled bool    `envconfig:"ENABLED" default:"false"`
	Weight  float64 `envconfig:"WEIGHT" default:"1.0"`
}

// NewsConfig represents news aggregation configuration
type NewsConfig struct {
	TwitterAPIKey     string   `envconfig:"TWITTER_API_KEY" required:"false"`
	EvaluatorProvider string   `envconfig:"NEWS_EVALUATOR_PROVIDER" default:"deepseek"`
	Keywords          []string `envconfig:"NEWS_KEYWORDS" default:"bitcoin,btc,crypto,cryptocurrency,ethereum,eth"`
	Enabled           bool     `envconfig:"NEWS_ENABLED" default:"true"`
	TwitterEnabled    bool     `envconfig:"TWITTER_ENABLED" default:"false"`
	RedditEnabled     bool     `envconfig:"REDDIT_ENABLED" default:"true"`
	CoinDeskEnabled   bool     `envconfig:"COINDESK_ENABLED" default:"true"`
	EvaluatorEnabled  bool     `envconfig:"NEWS_EVALUATOR_ENABLED" default:"true"`
	EvaluatorEnsemble bool     `envconfig:"NEWS_EVALUATOR_ENSEMBLE" default:"false"`
}

// OnChainConfig represents on-chain monitoring configuration
type OnChainConfig struct {
	WhaleAlert    OnChainProviderConfig `envconfig:"WHALE_ALERT"`
	BlockchainCom OnChainProviderConfig `envconfig:"BLOCKCHAIN_COM"`
	Etherscan     OnChainProviderConfig `envconfig:"ETHERSCAN"`
	MinValueUSD   int                   `envconfig:"ONCHAIN_MIN_VALUE_USD" default:"1000000"`
	Enabled       bool                  `envconfig:"ONCHAIN_ENABLED" default:"true"`
}

// OnChainProviderConfig represents single on-chain provider configuration
type OnChainProviderConfig struct {
	APIKey  string `envconfig:"API_KEY" required:"false"`
	Enabled bool   `envconfig:"ENABLED" default:"false"`
}

// RiskConfig represents risk management parameters
type RiskConfig struct {
	MaxConsecutiveLosses   int           `envconfig:"RISK_MAX_CONSECUTIVE_LOSSES" default:"5"`
	MaxDailyLossPercent    float64       `envconfig:"RISK_MAX_DAILY_LOSS_PERCENT" default:"5.0"`
	MaxDrawdownPercent     float64       `envconfig:"RISK_MAX_DRAWDOWN_PERCENT" default:"15.0"`
	CircuitBreakerCooldown time.Duration `envconfig:"RISK_CIRCUIT_BREAKER_COOLDOWN" default:"4h"`
}

// TelegramConfig represents Telegram bot configuration
type TelegramConfig struct {
	BotToken      string `envconfig:"TELEGRAM_BOT_TOKEN" required:"false"`
	AlertOnTrades bool   `envconfig:"TELEGRAM_ALERT_ON_TRADES" default:"true"`
	AlertOnErrors bool   `envconfig:"TELEGRAM_ALERT_ON_ERRORS" default:"true"`
	AdminID       int64  `envconfig:"TELEGRAM_ADMIN_ID" default:"0"` // Telegram ID of system admin (0 = no admin)
}

// DatabaseConfig represents database connection parameters
type DatabaseConfig struct {
	Host     string `envconfig:"DB_HOST" default:"localhost"`
	Name     string `envconfig:"DB_NAME" default:"trader"`
	User     string `envconfig:"DB_USER" required:"false" default:"postgres"`
	Password string `envconfig:"DB_PASSWORD" required:"false" default:""`
	SSLMode  string `envconfig:"DB_SSLMODE" default:"disable"`
	Port     int    `envconfig:"DB_PORT" default:"5432"`
}

// ClickHouseConfig represents ClickHouse connection parameters
type ClickHouseConfig struct {
	Host     string `envconfig:"CH_HOST" default:"localhost"`
	Database string `envconfig:"CH_DATABASE" default:"trader"`
	User     string `envconfig:"CH_USER" default:"default"`
	Password string `envconfig:"CH_PASSWORD" default:""`
	Port     int    `envconfig:"CH_PORT" default:"9000"`
	Enabled  bool   `envconfig:"CH_ENABLED" default:"false"`
}

// RedisConfig represents Redis connection parameters
type RedisConfig struct {
	Host     string `envconfig:"REDIS_HOST" default:"localhost"`
	Password string `envconfig:"REDIS_PASSWORD" required:"false" default:""`
	Port     int    `envconfig:"REDIS_PORT" default:"6379"`
	DB       int    `envconfig:"REDIS_DB" default:"0"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level string `envconfig:"LOG_LEVEL" default:"info"`
	File  string `envconfig:"LOG_FILE" default:"logs/bot.log"`
}

// HealthConfig represents health check server configuration
type HealthConfig struct {
	Port string `envconfig:"HEALTH_PORT" default:"8080"`
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	var cfg Config

	// Process environment variables
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to process config: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// Validate checks if configuration is valid
func (c *Config) Validate() error {
	// Exchanges are configured per-user in DB for agents, not globally
	// No validation needed here

	// Check at least one AI provider is enabled and configured
	aiConfigured := c.AI.DeepSeek.Enabled && c.AI.DeepSeek.APIKey != ""

	if c.AI.Claude.Enabled && c.AI.Claude.APIKey != "" {
		aiConfigured = true
	}
	if c.AI.OpenAI.Enabled && c.AI.OpenAI.APIKey != "" {
		aiConfigured = true
	}
	if !aiConfigured {
		return fmt.Errorf("at least one AI provider must be enabled and configured")
	}

	// Validate trading parameters
	if c.Trading.MaxPositionPercent <= 0 || c.Trading.MaxPositionPercent > 100 {
		return fmt.Errorf("max_position_percent must be between 0 and 100")
	}
	if c.Trading.MaxLeverage < 1 || c.Trading.MaxLeverage > 125 {
		return fmt.Errorf("max_leverage must be between 1 and 125")
	}
	if c.Trading.StopLossPercent <= 0 {
		return fmt.Errorf("stop_loss_percent must be positive")
	}
	if c.Trading.InitialBalance <= 0 {
		return fmt.Errorf("initial_balance must be positive")
	}

	// Validate risk parameters
	if c.Risk.MaxConsecutiveLosses < 1 {
		return fmt.Errorf("max_consecutive_losses must be at least 1")
	}
	if c.Risk.MaxDailyLossPercent <= 0 {
		return fmt.Errorf("max_daily_loss_percent must be positive")
	}

	// Telegram is optional for agents (can run without it)
	// Bot will send notifications to users who interact with it

	return nil
}

// GetDSN returns PostgreSQL connection string
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

// GetDSN returns ClickHouse DSN
func (c *ClickHouseConfig) GetDSN() string {
	dsn := fmt.Sprintf("clickhouse://%s:%s@%s:%d/%s",
		c.User, c.Password, c.Host, c.Port, c.Database,
	)
	return dsn
}

// IsPaperTrading returns true if bot is in paper trading mode
func (c *Config) IsPaperTrading() bool {
	// Mode is in TradingModeConfig which is embedded as Mode field
	return c.Mode.Mode == "paper"
}

// GetEnabledAIProviders returns list of enabled AI provider names
func (c *AIConfig) GetEnabledAIProviders() []string {
	var providers []string
	if c.DeepSeek.Enabled && c.DeepSeek.APIKey != "" {
		providers = append(providers, "deepseek")
	}
	if c.Claude.Enabled && c.Claude.APIKey != "" {
		providers = append(providers, "claude")
	}
	if c.OpenAI.Enabled && c.OpenAI.APIKey != "" {
		providers = append(providers, "openai")
	}
	return providers
}
