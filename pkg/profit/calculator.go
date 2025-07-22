package profit

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

// Calculator implements the ProfitCalculator interface
type Calculator struct {
	gasEstimator      interfaces.GasEstimator
	slippageCalculator interfaces.SlippageCalculator
	thresholds        map[interfaces.StrategyType]*ProfitThreshold
	mu                sync.RWMutex
	rng               *rand.Rand
}

// ProfitThreshold defines profitability thresholds per strategy
type ProfitThreshold struct {
	MinNetProfit      *big.Int
	MinProfitMargin   float64
	MinSuccessProbability float64
	MaxRiskScore      float64
}

// MonteCarloConfig defines parameters for Monte Carlo simulation
type MonteCarloConfig struct {
	Iterations        int
	GasVariance       float64
	SlippageVariance  float64
	PriceVariance     float64
	ExecutionDelayMax time.Duration
}

// NewCalculator creates a new profit calculator
func NewCalculator(gasEstimator interfaces.GasEstimator, slippageCalculator interfaces.SlippageCalculator) *Calculator {
	calc := &Calculator{
		gasEstimator:      gasEstimator,
		slippageCalculator: slippageCalculator,
		thresholds:        make(map[interfaces.StrategyType]*ProfitThreshold),
		rng:               rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	// Initialize default thresholds
	calc.initializeDefaultThresholds()
	
	return calc
}

// initializeDefaultThresholds sets up default profitability thresholds for each strategy
func (c *Calculator) initializeDefaultThresholds() {
	c.thresholds[interfaces.StrategySandwich] = &ProfitThreshold{
		MinNetProfit:          big.NewInt(1e16), // 0.01 ETH
		MinProfitMargin:       0.02,             // 2%
		MinSuccessProbability: 0.7,              // 70%
		MaxRiskScore:          0.3,              // 30%
	}

	c.thresholds[interfaces.StrategyBackrun] = &ProfitThreshold{
		MinNetProfit:          big.NewInt(5e15), // 0.005 ETH
		MinProfitMargin:       0.015,            // 1.5%
		MinSuccessProbability: 0.8,              // 80%
		MaxRiskScore:          0.25,             // 25%
	}

	c.thresholds[interfaces.StrategyFrontrun] = &ProfitThreshold{
		MinNetProfit:          big.NewInt(5e15), // 0.005 ETH (reduced from 0.02 ETH)
		MinProfitMargin:       0.015,            // 1.5% (reduced from 3%)
		MinSuccessProbability: 0.4,              // 40% (reduced from 60%)
		MaxRiskScore:          0.5,              // 50% (increased from 40%)
	}

	c.thresholds[interfaces.StrategyTimeBandit] = &ProfitThreshold{
		MinNetProfit:          big.NewInt(8e15), // 0.008 ETH (reduced from 0.03 ETH)
		MinProfitMargin:       0.01,             // 1% (reduced from 2.5%)
		MinSuccessProbability: 0.5,              // 50% (reduced from 75%)
		MaxRiskScore:          0.45,             // 45% (increased from 35%)
	}
}

// CalculateProfit calculates expected profitability for an MEV opportunity
func (c *Calculator) CalculateProfit(ctx context.Context, opportunity *interfaces.MEVOpportunity) (*interfaces.ProfitEstimate, error) {
	if opportunity == nil {
		return nil, fmt.Errorf("opportunity cannot be nil")
	}

	// Calculate gas costs
	gasCosts, err := c.CalculateGasCosts(ctx, opportunity.ExecutionTxs)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate gas costs: %w", err)
	}

	// Calculate slippage costs
	slippageCosts, err := c.CalculateSlippage(ctx, opportunity)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate slippage: %w", err)
	}

	// Calculate net profit
	netProfit := new(big.Int).Sub(opportunity.ExpectedProfit, gasCosts)
	netProfit.Sub(netProfit, slippageCosts)

	// Calculate profit margin
	var profitMargin float64
	if opportunity.ExpectedProfit.Cmp(big.NewInt(0)) > 0 {
		netProfitFloat, _ := netProfit.Float64()
		expectedProfitFloat, _ := opportunity.ExpectedProfit.Float64()
		profitMargin = netProfitFloat / expectedProfitFloat
	}

	// Run Monte Carlo simulation for risk assessment
	mcResult, err := c.runMonteCarloSimulation(ctx, opportunity, gasCosts, slippageCosts)
	if err != nil {
		return nil, fmt.Errorf("Monte Carlo simulation failed: %w", err)
	}

	estimate := &interfaces.ProfitEstimate{
		GrossProfit:        new(big.Int).Set(opportunity.ExpectedProfit),
		GasCosts:          gasCosts,
		SlippageCosts:     slippageCosts,
		NetProfit:         netProfit,
		ProfitMargin:      profitMargin,
		SuccessProbability: mcResult.SuccessProbability,
		RiskScore:         mcResult.RiskScore,
		Confidence:        mcResult.Confidence,
	}

	return estimate, nil
}

// CalculateGasCosts calculates total gas costs for execution transactions
func (c *Calculator) CalculateGasCosts(ctx context.Context, txs []*types.Transaction) (*big.Int, error) {
	if len(txs) == 0 {
		return big.NewInt(0), nil
	}

	totalGasCost := big.NewInt(0)
	
	for _, tx := range txs {
		// Get current gas price
		gasPrice, err := c.gasEstimator.GetCurrentGasPrice(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get gas price: %w", err)
		}

		// Estimate gas usage
		gasUsage, err := c.gasEstimator.EstimateGas(ctx, tx)
		if err != nil {
			return nil, fmt.Errorf("failed to estimate gas for tx %s: %w", tx.Hash, err)
		}

		// Calculate cost for this transaction
		txCost := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasUsage)))
		totalGasCost.Add(totalGasCost, txCost)
	}

	return totalGasCost, nil
}

// CalculateSlippage calculates slippage costs for an opportunity
func (c *Calculator) CalculateSlippage(ctx context.Context, opportunity *interfaces.MEVOpportunity) (*big.Int, error) {
	// Extract slippage information from opportunity metadata
	metadata := opportunity.Metadata
	if metadata == nil {
		return big.NewInt(0), nil
	}

	pool, ok := metadata["pool"].(string)
	if !ok {
		return big.NewInt(0), nil
	}

	token, ok := metadata["token"].(string)
	if !ok {
		return big.NewInt(0), nil
	}

	amount, ok := metadata["amount"].(*big.Int)
	if !ok {
		return big.NewInt(0), nil
	}

	// Calculate slippage estimate
	slippageEst, err := c.slippageCalculator.CalculateSlippage(ctx, pool, token, amount)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate slippage: %w", err)
	}

	return slippageEst.ExpectedSlippage, nil
}

// ValidateProfitability checks if an opportunity meets profitability thresholds
func (c *Calculator) ValidateProfitability(ctx context.Context, opportunity *interfaces.MEVOpportunity) (bool, error) {
	estimate, err := c.CalculateProfit(ctx, opportunity)
	if err != nil {
		return false, fmt.Errorf("failed to calculate profit: %w", err)
	}

	c.mu.RLock()
	threshold, exists := c.thresholds[opportunity.Strategy]
	c.mu.RUnlock()

	if !exists {
		return false, fmt.Errorf("no threshold defined for strategy %s", opportunity.Strategy)
	}

	// Check all threshold criteria
	if estimate.NetProfit.Cmp(threshold.MinNetProfit) < 0 {
		return false, nil
	}

	if estimate.ProfitMargin < threshold.MinProfitMargin {
		return false, nil
	}

	if estimate.SuccessProbability < threshold.MinSuccessProbability {
		return false, nil
	}

	if estimate.RiskScore > threshold.MaxRiskScore {
		return false, nil
	}

	return true, nil
}

// SetThreshold updates the profitability threshold for a strategy
func (c *Calculator) SetThreshold(strategy interfaces.StrategyType, threshold *ProfitThreshold) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.thresholds[strategy] = threshold
}

// GetThreshold returns the profitability threshold for a strategy
func (c *Calculator) GetThreshold(strategy interfaces.StrategyType) (*ProfitThreshold, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	threshold, exists := c.thresholds[strategy]
	return threshold, exists
}

// MonteCarloResult contains results from Monte Carlo simulation
type MonteCarloResult struct {
	SuccessProbability float64
	RiskScore         float64
	Confidence        float64
	ProfitDistribution []float64
}

// runMonteCarloSimulation performs Monte Carlo simulation for risk assessment
func (c *Calculator) runMonteCarloSimulation(ctx context.Context, opportunity *interfaces.MEVOpportunity, baseCosts, baseSlippage *big.Int) (*MonteCarloResult, error) {
	config := &MonteCarloConfig{
		Iterations:        1000,
		GasVariance:       0.2,  // 20% variance
		SlippageVariance:  0.3,  // 30% variance
		PriceVariance:     0.15, // 15% variance
		ExecutionDelayMax: 5 * time.Second,
	}

	profits := make([]float64, config.Iterations)
	successCount := 0

	baseProfit, _ := opportunity.ExpectedProfit.Float64()
	baseCostsFloat, _ := baseCosts.Float64()
	baseSlippageFloat, _ := baseSlippage.Float64()

	for i := 0; i < config.Iterations; i++ {
		// Simulate gas cost variance
		gasMultiplier := 1.0 + c.rng.NormFloat64()*config.GasVariance
		if gasMultiplier < 0.5 {
			gasMultiplier = 0.5
		}
		simulatedGasCosts := baseCostsFloat * gasMultiplier

		// Simulate slippage variance
		slippageMultiplier := 1.0 + c.rng.NormFloat64()*config.SlippageVariance
		if slippageMultiplier < 0 {
			slippageMultiplier = 0
		}
		simulatedSlippage := baseSlippageFloat * slippageMultiplier

		// Simulate price variance
		priceMultiplier := 1.0 + c.rng.NormFloat64()*config.PriceVariance
		if priceMultiplier < 0.1 {
			priceMultiplier = 0.1
		}
		simulatedProfit := baseProfit * priceMultiplier

		// Calculate net profit for this simulation
		netProfit := simulatedProfit - simulatedGasCosts - simulatedSlippage
		profits[i] = netProfit

		// Count as success if profitable
		if netProfit > 0 {
			successCount++
		}
	}

	// Calculate statistics
	successProbability := float64(successCount) / float64(config.Iterations)
	
	// Calculate risk score (probability of significant loss)
	significantLossCount := 0
	lossThreshold := baseProfit * -0.1 // 10% of expected profit
	
	for _, profit := range profits {
		if profit < lossThreshold {
			significantLossCount++
		}
	}
	
	riskScore := float64(significantLossCount) / float64(config.Iterations)
	
	// Calculate confidence based on variance
	variance := c.calculateVariance(profits)
	confidence := math.Max(0, 1.0-variance/baseProfit)

	return &MonteCarloResult{
		SuccessProbability: successProbability,
		RiskScore:         riskScore,
		Confidence:        confidence,
		ProfitDistribution: profits,
	}, nil
}

// calculateVariance calculates the variance of a slice of float64 values
func (c *Calculator) calculateVariance(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Calculate mean
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	// Calculate variance
	sumSquaredDiff := 0.0
	for _, v := range values {
		diff := v - mean
		sumSquaredDiff += diff * diff
	}

	return sumSquaredDiff / float64(len(values))
}