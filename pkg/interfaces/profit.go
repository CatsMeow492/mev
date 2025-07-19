package interfaces

import (
	"context"
	"math/big"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

// ProfitCalculator calculates expected profitability for MEV opportunities
type ProfitCalculator interface {
	CalculateProfit(ctx context.Context, opportunity *MEVOpportunity) (*ProfitEstimate, error)
	CalculateGasCosts(ctx context.Context, txs []*types.Transaction) (*big.Int, error)
	CalculateSlippage(ctx context.Context, opportunity *MEVOpportunity) (*big.Int, error)
	ValidateProfitability(ctx context.Context, opportunity *MEVOpportunity) (bool, error)
}

// GasEstimator estimates gas costs for strategy execution
type GasEstimator interface {
	EstimateGas(ctx context.Context, tx *types.Transaction) (uint64, error)
	EstimateBatchGas(ctx context.Context, txs []*types.Transaction) (uint64, error)
	GetCurrentGasPrice(ctx context.Context) (*big.Int, error)
	PredictGasPrice(ctx context.Context, priority GasPriority) (*big.Int, error)
}

// SlippageCalculator models price impact and slippage
type SlippageCalculator interface {
	CalculateSlippage(ctx context.Context, pool string, token string, amount *big.Int) (*SlippageEstimate, error)
	GetHistoricalSlippage(pool string, token string, timeWindow time.Duration) ([]*SlippageData, error)
	UpdateSlippageModel(pool string, token string, actualSlippage *big.Int) error
}

// ProfitEstimate contains profit calculation results
type ProfitEstimate struct {
	GrossProfit    *big.Int
	GasCosts       *big.Int
	SlippageCosts  *big.Int
	NetProfit      *big.Int
	ProfitMargin   float64
	SuccessProbability float64
	RiskScore      float64
	Confidence     float64
}

// SlippageEstimate contains slippage calculation results
type SlippageEstimate struct {
	ExpectedSlippage *big.Int
	MaxSlippage      *big.Int
	PriceImpact      float64
	Confidence       float64
}

// SlippageData represents historical slippage data
type SlippageData struct {
	Timestamp   time.Time
	Pool        string
	Token       string
	Amount      *big.Int
	Slippage    *big.Int
	PriceImpact float64
}

// GasPriority defines gas price priority levels
type GasPriority string

const (
	GasPriorityLow    GasPriority = "low"
	GasPriorityMedium GasPriority = "medium"
	GasPriorityHigh   GasPriority = "high"
	GasPriorityUrgent GasPriority = "urgent"
)