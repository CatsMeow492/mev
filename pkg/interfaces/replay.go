package interfaces

import (
	"context"
	"math/big"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

// ProfitThreshold defines profitability thresholds per strategy
type ProfitThreshold struct {
	MinNetProfit          *big.Int
	MinProfitMargin       float64
	MinSuccessProbability float64
	MaxRiskScore          float64
}

// HistoricalReplaySystem manages transaction logging and replay testing
type HistoricalReplaySystem interface {
	// Transaction logging
	LogProfitableOpportunity(ctx context.Context, opportunity *MEVOpportunity, tradeResult *TradeResult) error

	// Replay functionality
	ReplayHistoricalTransaction(ctx context.Context, logEntry *HistoricalTransactionLog) (*ReplayResult, error)
	BatchReplayTransactions(ctx context.Context, logEntries []*HistoricalTransactionLog) ([]*ReplayResult, error)

	// Validation and comparison
	ValidateStrategyPerformance(ctx context.Context, strategy StrategyType, timeWindow time.Duration) (*StrategyValidationResult, error)
	CompareActualVsExpected(ctx context.Context, logEntry *HistoricalTransactionLog, replayResult *ReplayResult) (*ProfitabilityComparison, error)

	// Regression testing
	RunRegressionTests(ctx context.Context, config *RegressionTestConfig) (*RegressionTestResults, error)

	// Data management
	GetHistoricalLogs(ctx context.Context, filter *HistoricalLogFilter) ([]*HistoricalTransactionLog, error)
	ArchiveOldLogs(ctx context.Context, olderThan time.Duration) error
}

// TransactionLogger handles logging of profitable opportunities
type TransactionLogger interface {
	LogOpportunity(ctx context.Context, opportunity *MEVOpportunity, tradeResult *TradeResult) error
	GetLogsByStrategy(ctx context.Context, strategy StrategyType, limit int) ([]*HistoricalTransactionLog, error)
	GetLogsByTimeRange(ctx context.Context, start, end time.Time) ([]*HistoricalTransactionLog, error)
	UpdateLogWithActualResult(ctx context.Context, logID string, actualResult *TradeResult) error
}

// ReplayHarness executes historical transaction replays
type ReplayHarness interface {
	ReplayTransaction(ctx context.Context, logEntry *HistoricalTransactionLog) (*ReplayResult, error)
	SetupReplayEnvironment(ctx context.Context, blockNumber uint64) (Fork, error)
	TeardownReplayEnvironment(fork Fork) error
	ValidateReplayAccuracy(original *HistoricalTransactionLog, replay *ReplayResult) (*AccuracyMetrics, error)
}

// PerformanceValidator validates strategy performance over time
type PerformanceValidator interface {
	ValidateStrategy(ctx context.Context, strategy StrategyType, timeWindow time.Duration) (*StrategyValidationResult, error)
	CheckThresholdChanges(ctx context.Context, oldThresholds, newThresholds map[StrategyType]*ProfitThreshold) (*ThresholdValidationResult, error)
	GeneratePerformanceReport(ctx context.Context, strategy StrategyType) (*PerformanceReport, error)
}

// HistoricalTransactionLog represents a logged profitable opportunity
type HistoricalTransactionLog struct {
	ID            string
	OpportunityID string
	Strategy      StrategyType
	CreatedAt     time.Time
	BlockNumber   uint64

	// Original opportunity data
	TargetTransaction *types.Transaction
	ExecutionTxs      []*types.Transaction
	ExpectedProfit    *big.Int
	EstimatedGasCost  *big.Int
	EstimatedSlippage *big.Int
	Confidence        float64

	// Simulation data at time of detection
	OriginalSimResults []*SimulationResult
	MarketConditions   *MarketConditions

	// Actual execution results (if executed)
	ActualTradeResult *TradeResult
	ExecutedAt        *time.Time

	// Metadata
	Metadata map[string]interface{}
}

// ReplayResult contains the results of replaying a historical transaction
type ReplayResult struct {
	LogID      string
	ReplayedAt time.Time
	Success    bool

	// Replay simulation results
	SimulationResults []*SimulationResult
	ReplayedProfit    *big.Int
	ReplayedGasCost   *big.Int
	ReplayedSlippage  *big.Int

	// Comparison with original
	ProfitDifference        *big.Int
	ProfitDifferencePercent float64

	// Environment context
	ReplayBlockNumber uint64
	ReplayConditions  *MarketConditions

	// Performance metrics
	ReplayLatency time.Duration
	AccuracyScore float64

	// Errors or warnings
	Errors   []string
	Warnings []string
}

// ProfitabilityComparison compares expected vs actual/replayed results
type ProfitabilityComparison struct {
	LogID          string
	ComparisonType ComparisonType

	// Profit comparison
	ExpectedProfit     *big.Int
	ActualProfit       *big.Int
	ProfitAccuracy     float64
	ProfitError        *big.Int
	ProfitErrorPercent float64

	// Cost comparison
	ExpectedGasCost *big.Int
	ActualGasCost   *big.Int
	GasCostAccuracy float64

	ExpectedSlippage *big.Int
	ActualSlippage   *big.Int
	SlippageAccuracy float64

	// Overall assessment
	OverallAccuracy   float64
	AccuracyGrade     AccuracyGrade
	RecommendedAction RecommendedAction
}

// StrategyValidationResult contains validation results for a strategy
type StrategyValidationResult struct {
	Strategy           StrategyType
	ValidationPeriod   time.Duration
	TotalOpportunities int
	SuccessfulReplays  int
	FailedReplays      int

	// Accuracy metrics
	AverageProfitAccuracy   float64
	AverageGasCostAccuracy  float64
	AverageSlippageAccuracy float64
	OverallAccuracy         float64

	// Performance metrics
	ProfitabilityTrend TrendDirection
	AccuracyTrend      TrendDirection
	VolumeCapacity     *big.Int

	// Recommendations
	ThresholdAdjustments map[string]float64
	ModelRecalibration   bool
	StrategyStatus       StrategyStatus

	// Detailed breakdown
	AccuracyDistribution map[AccuracyGrade]int
	TimeSeriesData       []*TimeSeriesPoint
}

// RegressionTestConfig defines parameters for regression testing
type RegressionTestConfig struct {
	Strategies           []StrategyType
	TimeWindow           time.Duration
	MaxTransactions      int
	SkipFailedOriginals  bool
	ParallelReplays      int
	AccuracyThreshold    float64
	PerformanceThreshold time.Duration
}

// RegressionTestResults contains comprehensive regression test results
type RegressionTestResults struct {
	TestID      string
	StartedAt   time.Time
	CompletedAt time.Time
	Config      *RegressionTestConfig

	// Overall results
	TotalTests     int
	PassedTests    int
	FailedTests    int
	OverallSuccess bool

	// Strategy breakdown
	StrategyResults map[StrategyType]*StrategyRegressionResult

	// Performance metrics
	AverageReplayLatency time.Duration
	TotalReplayTime      time.Duration
	AccuracyStats        *AccuracyStatistics

	// Alerts and recommendations
	CriticalIssues  []string
	Warnings        []string
	Recommendations []string
}

// StrategyRegressionResult contains regression results for a specific strategy
type StrategyRegressionResult struct {
	Strategy           StrategyType
	TestedTransactions int
	AccuracyScore      float64
	PassRate           float64

	// Detailed metrics
	ProfitAccuracyStats   *AccuracyStatistics
	GasCostAccuracyStats  *AccuracyStatistics
	SlippageAccuracyStats *AccuracyStatistics

	// Issues found
	SignificantDeviations []*DeviationReport
	ModelDrift            bool
	ThresholdViolations   []string
}

// MarketConditions captures market state at time of opportunity
type MarketConditions struct {
	BlockNumber       uint64
	Timestamp         time.Time
	GasPrice          *big.Int
	BaseFee           *big.Int
	NetworkCongestion float64

	// Token prices and liquidity
	TokenPrices     map[string]*big.Int
	PoolLiquidities map[string]*big.Int
	VolatilityIndex float64
}

// AccuracyMetrics contains accuracy measurement results
type AccuracyMetrics struct {
	ProfitAccuracy     float64
	GasCostAccuracy    float64
	SlippageAccuracy   float64
	TimingAccuracy     float64
	OverallScore       float64
	ConfidenceInterval float64
}

// AccuracyStatistics contains statistical measures of accuracy
type AccuracyStatistics struct {
	Mean              float64
	Median            float64
	StandardDeviation float64
	Min               float64
	Max               float64
	P95               float64
	P99               float64
}

// DeviationReport describes significant deviations found
type DeviationReport struct {
	LogID            string
	DeviationType    DeviationType
	ExpectedValue    *big.Int
	ActualValue      *big.Int
	DeviationPercent float64
	Severity         Severity
	Description      string
}

// TimeSeriesPoint represents a point in time series data
type TimeSeriesPoint struct {
	Timestamp          time.Time
	AccuracyScore      float64
	ProfitabilityScore float64
	VolumeProcessed    *big.Int
}

// HistoricalLogFilter defines filters for querying historical logs
type HistoricalLogFilter struct {
	Strategy       *StrategyType
	StartTime      *time.Time
	EndTime        *time.Time
	MinProfit      *big.Int
	MaxProfit      *big.Int
	OnlyExecuted   bool
	OnlyProfitable bool
	Limit          int
	Offset         int
}

// ThresholdValidationResult contains results of threshold validation
type ThresholdValidationResult struct {
	OriginalThresholds map[StrategyType]*ProfitThreshold
	NewThresholds      map[StrategyType]*ProfitThreshold
	ValidationResults  map[StrategyType]*ThresholdValidation
	OverallImpact      ImpactAssessment
	Recommendations    []string
}

// ThresholdValidation validates a specific threshold change
type ThresholdValidation struct {
	Strategy     StrategyType
	OldThreshold *ProfitThreshold
	NewThreshold *ProfitThreshold

	// Historical performance under new thresholds
	WouldHaveDetected int
	WouldHaveMissed   int
	FalsePositives    int
	FalseNegatives    int

	// Impact metrics
	ProfitabilityImpact float64
	VolumeImpact        float64
	RiskImpact          float64
}

// PerformanceReport contains comprehensive performance analysis
type PerformanceReport struct {
	Strategy     StrategyType
	ReportPeriod time.Duration
	GeneratedAt  time.Time

	// Summary metrics
	TotalOpportunities    int
	ExecutedOpportunities int
	ProfitableExecutions  int

	// Financial metrics
	TotalExpectedProfit *big.Int
	TotalActualProfit   *big.Int
	TotalLosses         *big.Int
	NetProfitability    *big.Int
	ROI                 float64

	// Accuracy metrics
	PredictionAccuracy *AccuracyStatistics
	ModelPerformance   *ModelPerformanceMetrics

	// Trends and insights
	Trends      *TrendAnalysis
	Insights    []string
	ActionItems []string
}

// ModelPerformanceMetrics contains model-specific performance data
type ModelPerformanceMetrics struct {
	CalibrationAge         time.Duration
	CalibrationAccuracy    float64
	DriftDetected          bool
	RecommendRecalibration bool
	SampleSize             int
	ConfidenceLevel        float64
}

// TrendAnalysis contains trend analysis results
type TrendAnalysis struct {
	ProfitabilityTrend TrendDirection
	AccuracyTrend      TrendDirection
	VolumeTrend        TrendDirection
	RiskTrend          TrendDirection

	// Correlation analysis
	MarketCorrelations map[string]float64
	SeasonalPatterns   map[string]float64
}

// Enums and constants
type ComparisonType string

const (
	ComparisonActualVsExpected ComparisonType = "actual_vs_expected"
	ComparisonReplayVsExpected ComparisonType = "replay_vs_expected"
	ComparisonReplayVsActual   ComparisonType = "replay_vs_actual"
)

type AccuracyGrade string

const (
	AccuracyExcellent AccuracyGrade = "excellent" // >95%
	AccuracyGood      AccuracyGrade = "good"      // 85-95%
	AccuracyFair      AccuracyGrade = "fair"      // 70-85%
	AccuracyPoor      AccuracyGrade = "poor"      // <70%
)

type RecommendedAction string

const (
	ActionNone             RecommendedAction = "none"
	ActionRecalibrate      RecommendedAction = "recalibrate"
	ActionAdjustThresholds RecommendedAction = "adjust_thresholds"
	ActionDisableStrategy  RecommendedAction = "disable_strategy"
	ActionInvestigate      RecommendedAction = "investigate"
)

type TrendDirection string

const (
	TrendImproving TrendDirection = "improving"
	TrendStable    TrendDirection = "stable"
	TrendDeclining TrendDirection = "declining"
	TrendUnknown   TrendDirection = "unknown"
)

type StrategyStatus string

const (
	StatusHealthy  StrategyStatus = "healthy"
	StatusWarning  StrategyStatus = "warning"
	StatusCritical StrategyStatus = "critical"
	StatusDisabled StrategyStatus = "disabled"
)

type DeviationType string

const (
	DeviationProfit   DeviationType = "profit"
	DeviationGasCost  DeviationType = "gas_cost"
	DeviationSlippage DeviationType = "slippage"
	DeviationTiming   DeviationType = "timing"
)

type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

type ImpactAssessment string

const (
	ImpactPositive ImpactAssessment = "positive"
	ImpactNeutral  ImpactAssessment = "neutral"
	ImpactNegative ImpactAssessment = "negative"
)
