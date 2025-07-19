package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the MEV engine
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	RPC        RPCConfig        `mapstructure:"rpc"`
	Simulation SimulationConfig `mapstructure:"simulation"`
	Strategies StrategiesConfig `mapstructure:"strategies"`
	Queue      QueueConfig      `mapstructure:"queue"`
	Monitoring MonitoringConfig `mapstructure:"monitoring"`
	Database   DatabaseConfig   `mapstructure:"database"`
}

// ServerConfig contains server configuration
type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

// RPCConfig contains RPC connection configuration
type RPCConfig struct {
	BaseURL           string        `mapstructure:"base_url"`
	WebSocketURL      string        `mapstructure:"websocket_url"`
	BackupURLs        []string      `mapstructure:"backup_urls"`
	ConnectionTimeout time.Duration `mapstructure:"connection_timeout"`
	ReconnectDelay    time.Duration `mapstructure:"reconnect_delay"`
	MaxReconnectDelay time.Duration `mapstructure:"max_reconnect_delay"`
	MaxRetries        int           `mapstructure:"max_retries"`
}

// SimulationConfig contains simulation engine configuration
type SimulationConfig struct {
	AnvilPath       string        `mapstructure:"anvil_path"`
	ForkURL         string        `mapstructure:"fork_url"`
	MaxForks        int           `mapstructure:"max_forks"`
	ForkTimeout     time.Duration `mapstructure:"fork_timeout"`
	SimulationTimeout time.Duration `mapstructure:"simulation_timeout"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
}

// StrategiesConfig contains strategy-specific configuration
type StrategiesConfig struct {
	Sandwich  SandwichStrategyConfig  `mapstructure:"sandwich"`
	Backrun   BackrunStrategyConfig   `mapstructure:"backrun"`
	Frontrun  FrontrunStrategyConfig  `mapstructure:"frontrun"`
	TimeBandit TimeBanditStrategyConfig `mapstructure:"time_bandit"`
}

// SandwichStrategyConfig contains sandwich strategy configuration
type SandwichStrategyConfig struct {
	Enabled           bool    `mapstructure:"enabled"`
	MinSwapAmount     string  `mapstructure:"min_swap_amount"`
	MaxSlippage       float64 `mapstructure:"max_slippage"`
	GasPremiumPercent float64 `mapstructure:"gas_premium_percent"`
	MinProfitThreshold string `mapstructure:"min_profit_threshold"`
}

// BackrunStrategyConfig contains backrun strategy configuration
type BackrunStrategyConfig struct {
	Enabled           bool     `mapstructure:"enabled"`
	MinPriceGap       string   `mapstructure:"min_price_gap"`
	MaxTradeSize      string   `mapstructure:"max_trade_size"`
	MinProfitThreshold string  `mapstructure:"min_profit_threshold"`
	SupportedPools    []string `mapstructure:"supported_pools"`
}

// FrontrunStrategyConfig contains frontrun strategy configuration
type FrontrunStrategyConfig struct {
	Enabled              bool    `mapstructure:"enabled"`
	MinTxValue           string  `mapstructure:"min_tx_value"`
	MaxGasPremium        string  `mapstructure:"max_gas_premium"`
	MinSuccessProbability float64 `mapstructure:"min_success_probability"`
	MinProfitThreshold   string  `mapstructure:"min_profit_threshold"`
}

// TimeBanditStrategyConfig contains time bandit strategy configuration
type TimeBanditStrategyConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	MaxBundleSize      int    `mapstructure:"max_bundle_size"`
	MinProfitThreshold string `mapstructure:"min_profit_threshold"`
	MaxDependencyDepth int    `mapstructure:"max_dependency_depth"`
}

// QueueConfig contains transaction queue configuration
type QueueConfig struct {
	MaxSize         int           `mapstructure:"max_size"`
	MaxAge          time.Duration `mapstructure:"max_age"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
	MinGasPrice     string        `mapstructure:"min_gas_price"`
}

// MonitoringConfig contains monitoring and alerting configuration
type MonitoringConfig struct {
	Enabled              bool          `mapstructure:"enabled"`
	MetricsPort          int           `mapstructure:"metrics_port"`
	LossRateWarning      float64       `mapstructure:"loss_rate_warning"`
	LossRateShutdown     float64       `mapstructure:"loss_rate_shutdown"`
	WindowSize           int           `mapstructure:"window_size"`
	AlertWebhookURL      string        `mapstructure:"alert_webhook_url"`
	PerformanceInterval  time.Duration `mapstructure:"performance_interval"`
}

// DatabaseConfig contains database configuration
type DatabaseConfig struct {
	RedisURL     string `mapstructure:"redis_url"`
	PostgresURL  string `mapstructure:"postgres_url"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

// Load loads configuration from file and environment variables
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// Set defaults
	setDefaults()

	// Enable environment variable support
	viper.AutomaticEnv()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "30s")
	viper.SetDefault("server.idle_timeout", "120s")

	// RPC defaults
	viper.SetDefault("rpc.base_url", "https://mainnet.base.org")
	viper.SetDefault("rpc.websocket_url", "wss://mainnet.base.org")
	viper.SetDefault("rpc.connection_timeout", "30s")
	viper.SetDefault("rpc.reconnect_delay", "1s")
	viper.SetDefault("rpc.max_reconnect_delay", "60s")
	viper.SetDefault("rpc.max_retries", 5)

	// Simulation defaults
	viper.SetDefault("simulation.anvil_path", "anvil")
	viper.SetDefault("simulation.fork_url", "https://mainnet.base.org")
	viper.SetDefault("simulation.max_forks", 10)
	viper.SetDefault("simulation.fork_timeout", "30s")
	viper.SetDefault("simulation.simulation_timeout", "5s")
	viper.SetDefault("simulation.cleanup_interval", "60s")

	// Strategy defaults
	viper.SetDefault("strategies.sandwich.enabled", true)
	viper.SetDefault("strategies.sandwich.min_swap_amount", "10000000000000000000") // 10 ETH
	viper.SetDefault("strategies.sandwich.max_slippage", 0.02)
	viper.SetDefault("strategies.sandwich.gas_premium_percent", 0.1)
	viper.SetDefault("strategies.sandwich.min_profit_threshold", "100000000000000000") // 0.1 ETH

	viper.SetDefault("strategies.backrun.enabled", true)
	viper.SetDefault("strategies.backrun.min_price_gap", "1000000000000000000") // 1 ETH
	viper.SetDefault("strategies.backrun.max_trade_size", "100000000000000000000") // 100 ETH
	viper.SetDefault("strategies.backrun.min_profit_threshold", "50000000000000000") // 0.05 ETH

	viper.SetDefault("strategies.frontrun.enabled", true)
	viper.SetDefault("strategies.frontrun.min_tx_value", "5000000000000000000") // 5 ETH
	viper.SetDefault("strategies.frontrun.max_gas_premium", "10000000000000000000") // 10 ETH
	viper.SetDefault("strategies.frontrun.min_success_probability", 0.7)
	viper.SetDefault("strategies.frontrun.min_profit_threshold", "100000000000000000") // 0.1 ETH

	viper.SetDefault("strategies.time_bandit.enabled", false)
	viper.SetDefault("strategies.time_bandit.max_bundle_size", 5)
	viper.SetDefault("strategies.time_bandit.min_profit_threshold", "200000000000000000") // 0.2 ETH
	viper.SetDefault("strategies.time_bandit.max_dependency_depth", 3)

	// Queue defaults
	viper.SetDefault("queue.max_size", 10000)
	viper.SetDefault("queue.max_age", "300s")
	viper.SetDefault("queue.cleanup_interval", "60s")
	viper.SetDefault("queue.min_gas_price", "1000000000") // 1 gwei

	// Monitoring defaults
	viper.SetDefault("monitoring.enabled", true)
	viper.SetDefault("monitoring.metrics_port", 9090)
	viper.SetDefault("monitoring.loss_rate_warning", 0.7)
	viper.SetDefault("monitoring.loss_rate_shutdown", 0.8)
	viper.SetDefault("monitoring.window_size", 100)
	viper.SetDefault("monitoring.performance_interval", "60s")

	// Database defaults
	viper.SetDefault("database.redis_url", "redis://localhost:6379")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 5)
}