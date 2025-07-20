package replay

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/simulation"
)

// ReplayHarnessImpl implements the ReplayHarness interface
type ReplayHarnessImpl struct {
	forkManager    interfaces.ForkManager
	strategyEngine interfaces.StrategyEngine
	replayer       interfaces.TransactionReplayer
	analyzer       interfaces.StateAnalyzer
}

// NewReplayHarness creates a new replay harness
func NewReplayHarness(forkManager interfaces.ForkManager, strategyEngine interfaces.StrategyEngine) interfaces.ReplayHarness {
	// Initialize the state analyzer
	analyzer, err := simulation.NewStateAnalyzer()
	if err != nil {
		// Log error but continue with nil analyzer
		fmt.Printf("Warning: Failed to create state analyzer: %v\n", err)
	}

	return &ReplayHarnessImpl{
		forkManager:    forkManager,
		strategyEngine: strategyEngine,
		replayer:       simulation.NewTransactionReplayer(),
		analyzer:       analyzer,
	}
}

// ReplayTransaction replays a historical transaction and measures results
func (rh *ReplayHarnessImpl) ReplayTransaction(ctx context.Context, logEntry *interfaces.HistoricalTransactionLog) (*interfaces.ReplayResult, error) {
	if logEntry == nil {
		return nil, fmt.Errorf("log entry cannot be nil")
	}

	startTime := time.Now()
	result := &interfaces.ReplayResult{
		LogID:      logEntry.ID,
		ReplayedAt: startTime,
		Errors:     make([]string, 0),
		Warnings:   make([]string, 0),
	}

	// Setup replay environment
	fork, err := rh.SetupReplayEnvironment(ctx, logEntry.BlockNumber)
	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to setup replay environment: %v", err))
		return result, nil
	}
	defer rh.TeardownReplayEnvironment(fork)

	// Capture market conditions at replay time
	replayConditions, err := rh.captureReplayConditions(ctx, fork)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to capture replay conditions: %v", err))
	} else {
		result.ReplayConditions = replayConditions
	}

	// Replay the target transaction if available
	if logEntry.TargetTransaction != nil {
		simResult, err := rh.replayer.ReplayTransaction(ctx, fork, logEntry.TargetTransaction)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to replay target transaction: %v", err))
		} else {
			result.SimulationResults = append(result.SimulationResults, simResult)
		}
	}

	// Replay execution transactions
	if len(logEntry.ExecutionTxs) > 0 {
		execResults, err := rh.replayer.BatchReplayTransactions(ctx, fork, logEntry.ExecutionTxs)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to replay execution transactions: %v", err))
		} else {
			result.SimulationResults = append(result.SimulationResults, execResults...)
		}
	}

	// If we have simulation results, analyze them
	if len(result.SimulationResults) > 0 {
		err = rh.analyzeReplayResults(ctx, logEntry, result)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to analyze replay results: %v", err))
		}
	}

	// Set success based on whether we have any results and no critical errors
	result.Success = len(result.SimulationResults) > 0 && len(result.Errors) == 0

	// Get block number from fork
	blockNumber, err := fork.GetBlockNumber()
	if err == nil {
		result.ReplayBlockNumber = blockNumber.Uint64()
	}

	result.ReplayLatency = time.Since(startTime)

	return result, nil
}

// SetupReplayEnvironment creates a fork environment for replay
func (rh *ReplayHarnessImpl) SetupReplayEnvironment(ctx context.Context, blockNumber uint64) (interfaces.Fork, error) {
	// Get an available fork from the pool
	fork, err := rh.forkManager.GetAvailableFork(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get available fork: %w", err)
	}

	// Reset the fork to clean state
	err = fork.Reset()
	if err != nil {
		rh.forkManager.ReleaseFork(fork)
		return nil, fmt.Errorf("failed to reset fork: %w", err)
	}

	// Note: In a full implementation, you would also:
	// 1. Set the fork to the specific block number for historical accuracy
	// 2. Apply any necessary state modifications
	// 3. Set up proper gas price and network conditions

	return fork, nil
}

// TeardownReplayEnvironment cleans up the replay environment
func (rh *ReplayHarnessImpl) TeardownReplayEnvironment(fork interfaces.Fork) error {
	if fork == nil {
		return nil
	}

	// Return the fork to the pool
	return rh.forkManager.ReleaseFork(fork)
}

// ValidateReplayAccuracy compares replay results with original execution
func (rh *ReplayHarnessImpl) ValidateReplayAccuracy(original *interfaces.HistoricalTransactionLog, replay *interfaces.ReplayResult) (*interfaces.AccuracyMetrics, error) {
	if original == nil || replay == nil {
		return nil, fmt.Errorf("original log and replay result cannot be nil")
	}

	metrics := &interfaces.AccuracyMetrics{}

	// Calculate profit accuracy
	if original.ExpectedProfit != nil && replay.ReplayedProfit != nil {
		expectedFloat, _ := original.ExpectedProfit.Float64()
		actualFloat, _ := replay.ReplayedProfit.Float64()

		if expectedFloat > 0 {
			diff := abs(actualFloat - expectedFloat)
			metrics.ProfitAccuracy = 1.0 - (diff / expectedFloat)
		} else {
			metrics.ProfitAccuracy = 1.0 // Perfect accuracy if both are zero
		}
	}

	// Calculate gas cost accuracy
	if original.EstimatedGasCost != nil && replay.ReplayedGasCost != nil {
		expectedFloat, _ := original.EstimatedGasCost.Float64()
		actualFloat, _ := replay.ReplayedGasCost.Float64()

		if expectedFloat > 0 {
			diff := abs(actualFloat - expectedFloat)
			metrics.GasCostAccuracy = 1.0 - (diff / expectedFloat)
		} else {
			metrics.GasCostAccuracy = 1.0
		}
	}

	// Calculate slippage accuracy
	if original.EstimatedSlippage != nil && replay.ReplayedSlippage != nil {
		expectedFloat, _ := original.EstimatedSlippage.Float64()
		actualFloat, _ := replay.ReplayedSlippage.Float64()

		if expectedFloat > 0 {
			diff := abs(actualFloat - expectedFloat)
			metrics.SlippageAccuracy = 1.0 - (diff / expectedFloat)
		} else {
			metrics.SlippageAccuracy = 1.0
		}
	}

	// Calculate timing accuracy (compare execution latency)
	if len(replay.SimulationResults) > 0 {
		// This is a simplified timing accuracy calculation
		// In reality, you'd compare against expected execution time
		metrics.TimingAccuracy = 0.9 // Default reasonable accuracy
	}

	// Calculate overall score as weighted average
	totalWeight := 0.0
	totalScore := 0.0

	if metrics.ProfitAccuracy > 0 {
		totalScore += metrics.ProfitAccuracy * 0.4 // 40% weight
		totalWeight += 0.4
	}
	if metrics.GasCostAccuracy > 0 {
		totalScore += metrics.GasCostAccuracy * 0.3 // 30% weight
		totalWeight += 0.3
	}
	if metrics.SlippageAccuracy > 0 {
		totalScore += metrics.SlippageAccuracy * 0.2 // 20% weight
		totalWeight += 0.2
	}
	if metrics.TimingAccuracy > 0 {
		totalScore += metrics.TimingAccuracy * 0.1 // 10% weight
		totalWeight += 0.1
	}

	if totalWeight > 0 {
		metrics.OverallScore = totalScore / totalWeight
	}

	// Calculate confidence interval (simplified)
	metrics.ConfidenceInterval = 0.95 // 95% confidence

	return metrics, nil
}

// analyzeReplayResults analyzes the simulation results to extract profit metrics
func (rh *ReplayHarnessImpl) analyzeReplayResults(ctx context.Context, logEntry *interfaces.HistoricalTransactionLog, result *interfaces.ReplayResult) error {
	if rh.analyzer == nil {
		return fmt.Errorf("state analyzer not available")
	}

	totalProfit := big.NewInt(0)
	totalGasCost := big.NewInt(0)
	totalSlippage := big.NewInt(0)

	for _, simResult := range result.SimulationResults {
		// Analyze gas usage
		gasAnalysis, err := rh.analyzer.CalculateGasUsage(simResult)
		if err == nil && gasAnalysis.TotalCost != nil {
			totalGasCost.Add(totalGasCost, gasAnalysis.TotalCost)
		}

		// Analyze price impact for slippage
		priceImpact, err := rh.analyzer.MeasurePriceImpact(simResult)
		if err == nil {
			// Convert price impact to slippage amount (simplified)
			impactAmount := big.NewInt(int64(priceImpact.ImpactBps * 1000)) // Convert basis points
			totalSlippage.Add(totalSlippage, impactAmount)
		}

		// Extract profit from state changes (simplified)
		if len(simResult.StateChanges) > 0 {
			for _, accountChange := range simResult.StateChanges {
				if accountChange.Balance != nil && accountChange.Balance.Sign() > 0 {
					totalProfit.Add(totalProfit, accountChange.Balance)
				}
			}
		}
	}

	// Set the calculated values
	result.ReplayedProfit = totalProfit
	result.ReplayedGasCost = totalGasCost
	result.ReplayedSlippage = totalSlippage

	return nil
}

// captureReplayConditions captures market conditions during replay
func (rh *ReplayHarnessImpl) captureReplayConditions(ctx context.Context, fork interfaces.Fork) (*interfaces.MarketConditions, error) {
	blockNumber, err := fork.GetBlockNumber()
	if err != nil {
		return nil, fmt.Errorf("failed to get block number: %w", err)
	}

	return &interfaces.MarketConditions{
		BlockNumber:       blockNumber.Uint64(),
		Timestamp:         time.Now(),
		GasPrice:          big.NewInt(20000000000), // 20 gwei default
		NetworkCongestion: 0.5,                     // 50% default
		TokenPrices:       make(map[string]*big.Int),
		PoolLiquidities:   make(map[string]*big.Int),
		VolatilityIndex:   0.3,
	}, nil
}
