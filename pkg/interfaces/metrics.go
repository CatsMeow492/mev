package interfaces

import (
	"context"
	"math/big"
	"time"
)

// MetricsCollector collects performance and profitability metrics
type MetricsCollector interface {
	// Trade metrics
	RecordTrade(ctx context.Context, trade *TradeResult) error
	RecordLatency(ctx context.Context, operation string, duration time.Duration) error
	RecordOpportunity(ctx context.Context, opportunity *MEVOpportunity) error
	
	// Rolling window calculations
	GetTradeSuccessRate(windowSize int) (float64, error)
	GetProfitabilityMetrics(windowSize int) (*ProfitabilityMetrics, error)
	GetLatencyMetrics(operation string, windowSize int) (*LatencyMetrics, error)
	
	// Performance tracking
	GetPerformanceMetrics() (*PerformanceMetrics, error)
	GetSystemMetrics() (*SystemMetrics, error)
	
	// Prometheus metrics
	GetPrometheusMetrics() (map[string]interface{}, error)
	RegisterPrometheusCollectors() error
}

// AlertManager manages alerts and notifications
type AlertManager interface {
	SendAlert(ctx context.Context, alert *Alert) error
	RegisterAlertRule(rule *AlertRule) error
	GetActiveAlerts() ([]*Alert, error)
	AcknowledgeAlert(alertID string) error
}

// ShutdownManager implements automatic shutdown logic
type ShutdownManager interface {
	CheckShutdownConditions(ctx context.Context) (*ShutdownDecision, error)
	InitiateShutdown(ctx context.Context, reason string) error
	GetShutdownStatus() (*ShutdownStatus, error)
	SetManualOverride(enabled bool) error
}

// TradeResult represents the result of a trade execution
type TradeResult struct {
	ID              string
	Strategy        StrategyType
	OpportunityID   string
	ExecutedAt      time.Time
	Success         bool
	ActualProfit    *big.Int
	ExpectedProfit  *big.Int
	GasCost         *big.Int
	NetProfit       *big.Int
	ExecutionTime   time.Duration
	TransactionHash string
	ErrorMessage    string
}

// ProfitabilityMetrics contains profitability statistics
type ProfitabilityMetrics struct {
	WindowSize       int
	TotalTrades      int
	ProfitableTrades int
	LossTrades       int
	SuccessRate      float64
	LossRate         float64
	TotalProfit      *big.Int
	TotalLoss        *big.Int
	NetProfit        *big.Int
	AverageProfit    *big.Int
	MedianProfit     *big.Int
	MaxProfit        *big.Int
	MaxLoss          *big.Int
	ProfitMargin     float64
	LastUpdated      time.Time
}

// LatencyMetrics contains latency statistics
type LatencyMetrics struct {
	Operation       string
	WindowSize      int
	SampleCount     int
	AverageLatency  time.Duration
	MedianLatency   time.Duration
	P95Latency      time.Duration
	P99Latency      time.Duration
	MinLatency      time.Duration
	MaxLatency      time.Duration
	LastUpdated     time.Time
}

// PerformanceMetrics contains overall system performance metrics
type PerformanceMetrics struct {
	// Trade performance
	TradeMetrics map[int]*ProfitabilityMetrics // keyed by window size
	
	// Latency performance
	LatencyMetrics map[string]*LatencyMetrics // keyed by operation
	
	// System performance
	TransactionsProcessed uint64
	OpportunitiesDetected uint64
	OpportunitiesExecuted uint64
	
	// Current status
	IsHealthy       bool
	WarningMode     bool
	ShutdownPending bool
	LastUpdated     time.Time
}

// SystemMetrics contains system resource metrics
type SystemMetrics struct {
	CPUUsage        float64
	MemoryUsage     float64
	GoroutineCount  int
	HeapSize        uint64
	GCPauseTime     time.Duration
	NetworkLatency  time.Duration
	LastUpdated     time.Time
}

// Alert represents a system alert
type Alert struct {
	ID          string
	Type        AlertType
	Severity    AlertSeverity
	Message     string
	Details     map[string]interface{}
	CreatedAt   time.Time
	AcknowledgedAt *time.Time
	ResolvedAt  *time.Time
}

// AlertRule defines conditions for triggering alerts
type AlertRule struct {
	ID          string
	Name        string
	Type        AlertType
	Condition   string
	Threshold   float64
	WindowSize  int
	Enabled     bool
	CreatedAt   time.Time
}

// ShutdownDecision contains the result of shutdown condition evaluation
type ShutdownDecision struct {
	ShouldShutdown bool
	Reason         string
	Metrics        map[string]float64
	Timestamp      time.Time
}

// ShutdownStatus contains current shutdown status
type ShutdownStatus struct {
	IsShutdown      bool
	ShutdownReason  string
	ShutdownTime    *time.Time
	ManualOverride  bool
	CanRestart      bool
}

// Enums
type AlertType string

const (
	AlertTypeProfitability AlertType = "profitability"
	AlertTypeLatency       AlertType = "latency"
	AlertTypeSystem        AlertType = "system"
	AlertTypeConnection    AlertType = "connection"
	AlertTypeShutdown      AlertType = "shutdown"
)

type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityError    AlertSeverity = "error"
	AlertSeverityCritical AlertSeverity = "critical"
)