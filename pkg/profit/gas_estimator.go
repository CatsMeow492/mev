package profit

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

// GasEstimator implements the GasEstimator interface
type GasEstimator struct {
	baseGasPrice     *big.Int
	gasHistory       []GasPriceData
	strategyGasUsage map[interfaces.StrategyType]uint64
	mu               sync.RWMutex
	lastUpdate       time.Time
}

// GasPriceData represents historical gas price information
type GasPriceData struct {
	Timestamp time.Time
	GasPrice  *big.Int
	Priority  interfaces.GasPriority
}

// NewGasEstimator creates a new gas estimator
func NewGasEstimator() *GasEstimator {
	estimator := &GasEstimator{
		baseGasPrice:     big.NewInt(1e9), // 1 gwei default
		gasHistory:       make([]GasPriceData, 0, 1000),
		strategyGasUsage: make(map[interfaces.StrategyType]uint64),
		lastUpdate:       time.Now(),
	}

	// Initialize default gas usage estimates per strategy
	estimator.initializeStrategyGasUsage()
	
	return estimator
}

// initializeStrategyGasUsage sets default gas usage estimates for each strategy
func (g *GasEstimator) initializeStrategyGasUsage() {
	g.strategyGasUsage[interfaces.StrategySandwich] = 300000   // Front-run + back-run transactions
	g.strategyGasUsage[interfaces.StrategyBackrun] = 150000    // Single arbitrage transaction
	g.strategyGasUsage[interfaces.StrategyFrontrun] = 100000   // Single front-run transaction
	g.strategyGasUsage[interfaces.StrategyTimeBandit] = 250000 // Multiple transaction bundle
}

// EstimateGas estimates gas usage for a single transaction
func (g *GasEstimator) EstimateGas(ctx context.Context, tx *types.Transaction) (uint64, error) {
	if tx == nil {
		return 0, fmt.Errorf("transaction cannot be nil")
	}

	// Base gas estimation logic
	baseGas := uint64(21000) // Base transaction cost

	// Add gas for contract interaction
	if len(tx.Data) > 0 {
		// Estimate based on data size and complexity
		dataGas := uint64(len(tx.Data)) * 16 // 16 gas per byte for non-zero data
		baseGas += dataGas

		// Add extra gas for complex contract calls
		if len(tx.Data) > 100 {
			baseGas += 50000 // Additional gas for complex calls
		}
	}

	// Add buffer for gas estimation uncertainty (10%)
	estimatedGas := baseGas + (baseGas / 10)

	return estimatedGas, nil
}

// EstimateBatchGas estimates total gas usage for multiple transactions
func (g *GasEstimator) EstimateBatchGas(ctx context.Context, txs []*types.Transaction) (uint64, error) {
	if len(txs) == 0 {
		return 0, nil
	}

	totalGas := uint64(0)
	
	for _, tx := range txs {
		gasUsage, err := g.EstimateGas(ctx, tx)
		if err != nil {
			return 0, fmt.Errorf("failed to estimate gas for tx %s: %w", tx.Hash, err)
		}
		totalGas += gasUsage
	}

	// Add batch overhead (5% of total)
	batchOverhead := totalGas / 20
	totalGas += batchOverhead

	return totalGas, nil
}

// GetCurrentGasPrice returns the current gas price estimate
func (g *GasEstimator) GetCurrentGasPrice(ctx context.Context) (*big.Int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// If we have recent gas price data, use it
	if len(g.gasHistory) > 0 && time.Since(g.lastUpdate) < 30*time.Second {
		return new(big.Int).Set(g.gasHistory[len(g.gasHistory)-1].GasPrice), nil
	}

	// Otherwise return base gas price
	return new(big.Int).Set(g.baseGasPrice), nil
}

// PredictGasPrice predicts gas price based on priority level
func (g *GasEstimator) PredictGasPrice(ctx context.Context, priority interfaces.GasPriority) (*big.Int, error) {
	currentPrice, err := g.GetCurrentGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current gas price: %w", err)
	}

	multiplier := g.getPriorityMultiplier(priority)
	
	// Calculate predicted price
	multiplierBig := big.NewInt(int64(multiplier * 100))
	predictedPrice := new(big.Int).Mul(currentPrice, multiplierBig)
	predictedPrice.Div(predictedPrice, big.NewInt(100))

	return predictedPrice, nil
}

// getPriorityMultiplier returns the gas price multiplier for different priority levels
func (g *GasEstimator) getPriorityMultiplier(priority interfaces.GasPriority) float64 {
	switch priority {
	case interfaces.GasPriorityLow:
		return 0.9  // 10% below current
	case interfaces.GasPriorityMedium:
		return 1.1  // 10% above current
	case interfaces.GasPriorityHigh:
		return 1.3  // 30% above current
	case interfaces.GasPriorityUrgent:
		return 1.5  // 50% above current
	default:
		return 1.0  // Current price
	}
}

// UpdateGasPrice updates the gas price history with new data
func (g *GasEstimator) UpdateGasPrice(gasPrice *big.Int, priority interfaces.GasPriority) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Add new gas price data
	data := GasPriceData{
		Timestamp: time.Now(),
		GasPrice:  new(big.Int).Set(gasPrice),
		Priority:  priority,
	}

	g.gasHistory = append(g.gasHistory, data)
	g.lastUpdate = time.Now()

	// Keep only last 1000 entries
	if len(g.gasHistory) > 1000 {
		g.gasHistory = g.gasHistory[1:]
	}

	// Update base gas price to recent average
	g.updateBaseGasPrice()
}

// updateBaseGasPrice calculates and updates the base gas price from recent history
func (g *GasEstimator) updateBaseGasPrice() {
	if len(g.gasHistory) == 0 {
		return
	}

	// Calculate average of last 10 entries
	recentCount := 10
	if len(g.gasHistory) < recentCount {
		recentCount = len(g.gasHistory)
	}

	sum := big.NewInt(0)
	for i := len(g.gasHistory) - recentCount; i < len(g.gasHistory); i++ {
		sum.Add(sum, g.gasHistory[i].GasPrice)
	}

	g.baseGasPrice.Div(sum, big.NewInt(int64(recentCount)))
}

// GetStrategyGasUsage returns estimated gas usage for a strategy type
func (g *GasEstimator) GetStrategyGasUsage(strategy interfaces.StrategyType) uint64 {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if usage, exists := g.strategyGasUsage[strategy]; exists {
		return usage
	}

	return 100000 // Default gas usage
}

// UpdateStrategyGasUsage updates gas usage estimate for a strategy based on actual usage
func (g *GasEstimator) UpdateStrategyGasUsage(strategy interfaces.StrategyType, actualGasUsed uint64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Use exponential moving average to update estimate
	currentEstimate := g.strategyGasUsage[strategy]
	alpha := 0.1 // Smoothing factor
	
	newEstimate := uint64(float64(currentEstimate)*(1-alpha) + float64(actualGasUsed)*alpha)
	g.strategyGasUsage[strategy] = newEstimate
}

// GetGasHistory returns recent gas price history
func (g *GasEstimator) GetGasHistory(duration time.Duration) []GasPriceData {
	g.mu.RLock()
	defer g.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	var result []GasPriceData

	for _, data := range g.gasHistory {
		if data.Timestamp.After(cutoff) {
			result = append(result, data)
		}
	}

	return result
}