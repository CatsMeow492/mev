package interfaces

import (
	"context"
	"math/big"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

// StrategyEngine coordinates all MEV strategy detection
type StrategyEngine interface {
	AnalyzeTransaction(ctx context.Context, tx *types.Transaction, simResult *SimulationResult) ([]*MEVOpportunity, error)
	GetActiveStrategies() []StrategyType
	EnableStrategy(strategy StrategyType) error
	DisableStrategy(strategy StrategyType) error
	UpdateStrategyConfig(strategy StrategyType, config interface{}) error
}

// SandwichDetector identifies sandwich attack opportunities
type SandwichDetector interface {
	DetectOpportunity(ctx context.Context, tx *types.Transaction, simResult *SimulationResult) (*SandwichOpportunity, error)
	ValidateOpportunity(ctx context.Context, opportunity *SandwichOpportunity) error
	ConstructTransactions(ctx context.Context, opportunity *SandwichOpportunity) ([]*types.Transaction, error)
	GetConfiguration() *SandwichConfig
}

// BackrunDetector finds arbitrage opportunities from price gaps
type BackrunDetector interface {
	DetectOpportunity(ctx context.Context, tx *types.Transaction, simResult *SimulationResult) (*BackrunOpportunity, error)
	CalculateOptimalTradeSize(ctx context.Context, opportunity *BackrunOpportunity) (*big.Int, error)
	ValidateArbitrage(ctx context.Context, opportunity *BackrunOpportunity) error
	GetConfiguration() *BackrunConfig
}

// FrontrunDetector detects frontrunnable high-value transactions
type FrontrunDetector interface {
	DetectOpportunity(ctx context.Context, tx *types.Transaction, simResult *SimulationResult) (*FrontrunOpportunity, error)
	CalculateOptimalGasPrice(ctx context.Context, targetTx *types.Transaction) (*big.Int, error)
	ValidateProfitability(ctx context.Context, opportunity *FrontrunOpportunity) error
	GetConfiguration() *FrontrunConfig
}

// TimeBanditDetector analyzes transaction reordering opportunities
type TimeBanditDetector interface {
	DetectOpportunity(ctx context.Context, txs []*types.Transaction, simResults []*SimulationResult) (*TimeBanditOpportunity, error)
	FindOptimalOrdering(ctx context.Context, txs []*types.Transaction) ([]*types.Transaction, error)
	ValidateDependencies(ctx context.Context, txs []*types.Transaction) error
	GetConfiguration() *TimeBanditConfig
}

// CrossLayerArbitrageDetector detects arbitrage opportunities between L1 and L2
type CrossLayerArbitrageDetector interface {
	DetectOpportunity(ctx context.Context, bridgeEvent *BridgeEvent, l1Price, l2Price *big.Int) (*CrossLayerOpportunity, error)
	ComparePrices(ctx context.Context, token string) (*PriceComparison, error)
	ConstructBridgeTransaction(ctx context.Context, opportunity *CrossLayerOpportunity) (*types.Transaction, error)
	GetConfiguration() *CrossLayerConfig
}

// MEVOpportunity represents a detected MEV opportunity
type MEVOpportunity struct {
	ID              string
	Strategy        StrategyType
	TargetTx        string
	ExpectedProfit  *big.Int
	GasCost         *big.Int
	NetProfit       *big.Int
	Confidence      float64
	Status          OpportunityStatus
	CreatedAt       time.Time
	ExecutionTxs    []*types.Transaction
	Metadata        map[string]interface{}
}

// SandwichOpportunity represents a sandwich attack opportunity
type SandwichOpportunity struct {
	TargetTx        *types.Transaction
	FrontrunTx      *types.Transaction
	BackrunTx       *types.Transaction
	ExpectedProfit  *big.Int
	SlippageTolerance float64
	PriceImpact     *big.Int
	Pool            string
	Token0          string
	Token1          string
}

// BackrunOpportunity represents an arbitrage opportunity
type BackrunOpportunity struct {
	TargetTx       *types.Transaction
	ArbitrageTx    *types.Transaction
	Pool1          string
	Pool2          string
	Token          string
	PriceGap       *big.Int
	OptimalAmount  *big.Int
	ExpectedProfit *big.Int
}

// FrontrunOpportunity represents a frontrun opportunity
type FrontrunOpportunity struct {
	TargetTx       *types.Transaction
	FrontrunTx     *types.Transaction
	ExpectedProfit *big.Int
	GasPremium     *big.Int
	SuccessProbability float64
}

// TimeBanditOpportunity represents a transaction reordering opportunity
type TimeBanditOpportunity struct {
	OriginalTxs    []*types.Transaction
	OptimalOrder   []*types.Transaction
	ExpectedProfit *big.Int
	Dependencies   map[string][]string
}

// CrossLayerOpportunity represents a cross-layer arbitrage opportunity
type CrossLayerOpportunity struct {
	BridgeEvent    *BridgeEvent
	Token          string
	L1Price        *big.Int
	L2Price        *big.Int
	PriceGap       *big.Int
	Amount         *big.Int
	ExpectedProfit *big.Int
	BridgeTx       *types.Transaction
	Direction      ArbitrageDirection
}

// PriceComparison represents price comparison between L1 and L2
type PriceComparison struct {
	Token     string
	L1Price   *big.Int
	L2Price   *big.Int
	PriceGap  *big.Int
	Timestamp time.Time
}

// Strategy configuration types
type SandwichConfig struct {
	MinSwapAmount     *big.Int
	MaxSlippage       float64
	GasPremiumPercent float64
	MinProfitThreshold *big.Int
}

type BackrunConfig struct {
	MinPriceGap       *big.Int
	MaxTradeSize      *big.Int
	MinProfitThreshold *big.Int
	SupportedPools    []string
}

type FrontrunConfig struct {
	MinTxValue        *big.Int
	MaxGasPremium     *big.Int
	MinSuccessProbability float64
	MinProfitThreshold *big.Int
}

type TimeBanditConfig struct {
	MaxBundleSize     int
	MinProfitThreshold *big.Int
	MaxDependencyDepth int
}

type CrossLayerConfig struct {
	MinPriceGap       *big.Int
	MinAmount         *big.Int
	MaxAmount         *big.Int
	BridgeFee         *big.Int
	MinProfitThreshold *big.Int
	SupportedTokens   []string
}

// Enums
type StrategyType string

const (
	StrategySandwich     StrategyType = "sandwich"
	StrategyBackrun      StrategyType = "backrun"
	StrategyFrontrun     StrategyType = "frontrun"
	StrategyTimeBandit   StrategyType = "time_bandit"
	StrategyCrossLayer   StrategyType = "cross_layer"
)

type OpportunityStatus string

const (
	StatusDetected    OpportunityStatus = "detected"
	StatusValidated   OpportunityStatus = "validated"
	StatusProfitable  OpportunityStatus = "profitable"
	StatusUnprofitable OpportunityStatus = "unprofitable"
	StatusExecuted    OpportunityStatus = "executed"
	StatusFailed      OpportunityStatus = "failed"
)

type ArbitrageDirection string

const (
	DirectionL1ToL2 ArbitrageDirection = "l1_to_l2"
	DirectionL2ToL1 ArbitrageDirection = "l2_to_l1"
)