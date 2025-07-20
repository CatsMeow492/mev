package replay

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// HistoricalReplaySystemImpl implements the HistoricalReplaySystem interface
type HistoricalReplaySystemImpl struct {
	db                *sql.DB
	transactionLogger interfaces.TransactionLogger
	replayHarness     interfaces.ReplayHarness
	validator         interfaces.PerformanceValidator
	forkManager       interfaces.ForkManager
	strategyEngine    interfaces.StrategyEngine
	profitCalculator  interfaces.ProfitCalculator
	mu                sync.RWMutex
	config            *ReplaySystemConfig
}

// ReplaySystemConfig contains configuration for the replay system
type ReplaySystemConfig struct {
	MaxConcurrentReplays  int
	ReplayTimeout         time.Duration
	AccuracyThreshold     float64
	RetentionPeriod       time.Duration
	EnableRealTimeLogging bool
	BatchSize             int
}

// NewHistoricalReplaySystem creates a new historical replay system
func NewHistoricalReplaySystem(
	db *sql.DB,
	forkManager interfaces.ForkManager,
	strategyEngine interfaces.StrategyEngine,
	profitCalculator interfaces.ProfitCalculator,
	config *ReplaySystemConfig,
) *HistoricalReplaySystemImpl {
	if config == nil {
		config = &ReplaySystemConfig{
			MaxConcurrentReplays:  5,
			ReplayTimeout:         30 * time.Second,
			AccuracyThreshold:     0.85,
			RetentionPeriod:       90 * 24 * time.Hour,
			EnableRealTimeLogging: true,
			BatchSize:             100,
		}
	}

	system := &HistoricalReplaySystemImpl{
		db:               db,
		forkManager:      forkManager,
		strategyEngine:   strategyEngine,
		profitCalculator: profitCalculator,
		config:           config,
	}

	// Initialize sub-components
	system.transactionLogger = NewTransactionLogger(db)
	system.replayHarness = NewReplayHarness(forkManager, strategyEngine)
	system.validator = NewPerformanceValidator(db, system.transactionLogger)

	return system
}

// LogProfitableOpportunity logs a profitable opportunity for historical analysis
func (hrs *HistoricalReplaySystemImpl) LogProfitableOpportunity(ctx context.Context, opportunity *interfaces.MEVOpportunity, tradeResult *interfaces.TradeResult) error {
	if opportunity == nil {
		return fmt.Errorf("opportunity cannot be nil")
	}

	// Create historical log entry
	logEntry := &interfaces.HistoricalTransactionLog{
		ID:                generateLogID(),
		OpportunityID:     opportunity.ID,
		Strategy:          opportunity.Strategy,
		CreatedAt:         time.Now(),
		BlockNumber:       0, // Would need to get from transaction context
		ExpectedProfit:    new(big.Int).Set(opportunity.ExpectedProfit),
		EstimatedGasCost:  new(big.Int).Set(opportunity.GasCost),
		Confidence:        opportunity.Confidence,
		ExecutionTxs:      opportunity.ExecutionTxs,
		ActualTradeResult: tradeResult,
		Metadata:          opportunity.Metadata,
	}

	// Set execution timestamp if trade result is provided
	if tradeResult != nil {
		logEntry.ExecutedAt = &tradeResult.ExecutedAt
	}

	// Capture market conditions
	marketConditions, err := hrs.captureCurrentMarketConditions(ctx)
	if err != nil {
		// Log warning but don't fail - market conditions are supplementary
		fmt.Printf("Warning: Failed to capture market conditions: %v\n", err)
	} else {
		logEntry.MarketConditions = marketConditions
	}

	// Log the opportunity
	return hrs.transactionLogger.LogOpportunity(ctx, opportunity, tradeResult)
}

// ReplayHistoricalTransaction replays a single historical transaction
func (hrs *HistoricalReplaySystemImpl) ReplayHistoricalTransaction(ctx context.Context, logEntry *interfaces.HistoricalTransactionLog) (*interfaces.ReplayResult, error) {
	if logEntry == nil {
		return nil, fmt.Errorf("log entry cannot be nil")
	}

	startTime := time.Now()
	replayResult := &interfaces.ReplayResult{
		LogID:      logEntry.ID,
		ReplayedAt: startTime,
		Errors:     make([]string, 0),
		Warnings:   make([]string, 0),
	}

	// Use replay harness to execute the replay
	result, err := hrs.replayHarness.ReplayTransaction(ctx, logEntry)
	if err != nil {
		replayResult.Success = false
		replayResult.Errors = append(replayResult.Errors, err.Error())
		return replayResult, nil // Return result with error info rather than failing
	}

	// Copy results from harness
	replayResult.Success = result.Success
	replayResult.SimulationResults = result.SimulationResults
	replayResult.ReplayedProfit = result.ReplayedProfit
	replayResult.ReplayedGasCost = result.ReplayedGasCost
	replayResult.ReplayedSlippage = result.ReplayedSlippage
	replayResult.ReplayBlockNumber = result.ReplayBlockNumber
	replayResult.ReplayConditions = result.ReplayConditions

	// Calculate profit difference
	if logEntry.ExpectedProfit != nil && replayResult.ReplayedProfit != nil {
		replayResult.ProfitDifference = new(big.Int).Sub(replayResult.ReplayedProfit, logEntry.ExpectedProfit)

		if logEntry.ExpectedProfit.Cmp(big.NewInt(0)) > 0 {
			expectedFloat, _ := logEntry.ExpectedProfit.Float64()
			differenceFloat, _ := replayResult.ProfitDifference.Float64()
			replayResult.ProfitDifferencePercent = (differenceFloat / expectedFloat) * 100
		}
	}

	// Calculate accuracy score
	accuracyMetrics, err := hrs.replayHarness.ValidateReplayAccuracy(logEntry, replayResult)
	if err != nil {
		replayResult.Warnings = append(replayResult.Warnings, fmt.Sprintf("Failed to calculate accuracy: %v", err))
	} else {
		replayResult.AccuracyScore = accuracyMetrics.OverallScore
	}

	replayResult.ReplayLatency = time.Since(startTime)

	return replayResult, nil
}

// BatchReplayTransactions replays multiple historical transactions concurrently
func (hrs *HistoricalReplaySystemImpl) BatchReplayTransactions(ctx context.Context, logEntries []*interfaces.HistoricalTransactionLog) ([]*interfaces.ReplayResult, error) {
	if len(logEntries) == 0 {
		return []*interfaces.ReplayResult{}, nil
	}

	// Set up concurrency control
	semaphore := make(chan struct{}, hrs.config.MaxConcurrentReplays)
	results := make([]*interfaces.ReplayResult, len(logEntries))
	var wg sync.WaitGroup
	var mu sync.Mutex
	errorCount := 0

	for i, logEntry := range logEntries {
		wg.Add(1)
		go func(index int, entry *interfaces.HistoricalTransactionLog) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Create context with timeout
			replayCtx, cancel := context.WithTimeout(ctx, hrs.config.ReplayTimeout)
			defer cancel()

			// Replay the transaction
			result, err := hrs.ReplayHistoricalTransaction(replayCtx, entry)

			mu.Lock()
			if err != nil {
				errorCount++
				// Create error result
				results[index] = &interfaces.ReplayResult{
					LogID:      entry.ID,
					ReplayedAt: time.Now(),
					Success:    false,
					Errors:     []string{err.Error()},
				}
			} else {
				results[index] = result
			}
			mu.Unlock()
		}(i, logEntry)
	}

	wg.Wait()

	if errorCount > 0 {
		return results, fmt.Errorf("encountered errors in %d out of %d replays", errorCount, len(logEntries))
	}

	return results, nil
}

// ValidateStrategyPerformance validates a strategy's performance over time
func (hrs *HistoricalReplaySystemImpl) ValidateStrategyPerformance(ctx context.Context, strategy interfaces.StrategyType, timeWindow time.Duration) (*interfaces.StrategyValidationResult, error) {
	return hrs.validator.ValidateStrategy(ctx, strategy, timeWindow)
}

// CompareActualVsExpected compares actual execution results with expected results
func (hrs *HistoricalReplaySystemImpl) CompareActualVsExpected(ctx context.Context, logEntry *interfaces.HistoricalTransactionLog, replayResult *interfaces.ReplayResult) (*interfaces.ProfitabilityComparison, error) {
	if logEntry == nil || replayResult == nil {
		return nil, fmt.Errorf("log entry and replay result cannot be nil")
	}

	comparison := &interfaces.ProfitabilityComparison{
		LogID:          logEntry.ID,
		ComparisonType: interfaces.ComparisonReplayVsExpected,
	}

	// Compare profits
	if logEntry.ExpectedProfit != nil && replayResult.ReplayedProfit != nil {
		comparison.ExpectedProfit = new(big.Int).Set(logEntry.ExpectedProfit)
		comparison.ActualProfit = new(big.Int).Set(replayResult.ReplayedProfit)

		// Calculate profit error
		comparison.ProfitError = new(big.Int).Sub(comparison.ActualProfit, comparison.ExpectedProfit)

		if comparison.ExpectedProfit.Cmp(big.NewInt(0)) > 0 {
			expectedFloat, _ := comparison.ExpectedProfit.Float64()
			errorFloat, _ := comparison.ProfitError.Float64()
			comparison.ProfitErrorPercent = (errorFloat / expectedFloat) * 100
			comparison.ProfitAccuracy = 1.0 - (abs(errorFloat) / expectedFloat)
		}
	}

	// Compare gas costs
	if logEntry.EstimatedGasCost != nil && replayResult.ReplayedGasCost != nil {
		comparison.ExpectedGasCost = new(big.Int).Set(logEntry.EstimatedGasCost)
		comparison.ActualGasCost = new(big.Int).Set(replayResult.ReplayedGasCost)

		if comparison.ExpectedGasCost.Cmp(big.NewInt(0)) > 0 {
			expectedFloat, _ := comparison.ExpectedGasCost.Float64()
			actualFloat, _ := comparison.ActualGasCost.Float64()
			comparison.GasCostAccuracy = 1.0 - (abs(actualFloat-expectedFloat) / expectedFloat)
		}
	}

	// Compare slippage
	if logEntry.EstimatedSlippage != nil && replayResult.ReplayedSlippage != nil {
		comparison.ExpectedSlippage = new(big.Int).Set(logEntry.EstimatedSlippage)
		comparison.ActualSlippage = new(big.Int).Set(replayResult.ReplayedSlippage)

		if comparison.ExpectedSlippage.Cmp(big.NewInt(0)) > 0 {
			expectedFloat, _ := comparison.ExpectedSlippage.Float64()
			actualFloat, _ := comparison.ActualSlippage.Float64()
			comparison.SlippageAccuracy = 1.0 - (abs(actualFloat-expectedFloat) / expectedFloat)
		}
	}

	// Calculate overall accuracy
	accuracyCount := 0
	accuracySum := 0.0

	if comparison.ProfitAccuracy > 0 {
		accuracySum += comparison.ProfitAccuracy
		accuracyCount++
	}
	if comparison.GasCostAccuracy > 0 {
		accuracySum += comparison.GasCostAccuracy
		accuracyCount++
	}
	if comparison.SlippageAccuracy > 0 {
		accuracySum += comparison.SlippageAccuracy
		accuracyCount++
	}

	if accuracyCount > 0 {
		comparison.OverallAccuracy = accuracySum / float64(accuracyCount)
	}

	// Assign accuracy grade
	comparison.AccuracyGrade = classifyAccuracy(comparison.OverallAccuracy)

	// Determine recommended action
	comparison.RecommendedAction = determineRecommendedAction(comparison)

	return comparison, nil
}

// RunRegressionTests runs comprehensive regression tests
func (hrs *HistoricalReplaySystemImpl) RunRegressionTests(ctx context.Context, config *interfaces.RegressionTestConfig) (*interfaces.RegressionTestResults, error) {
	if config == nil {
		return nil, fmt.Errorf("regression test config cannot be nil")
	}

	results := &interfaces.RegressionTestResults{
		TestID:          generateTestID(),
		StartedAt:       time.Now(),
		Config:          config,
		StrategyResults: make(map[interfaces.StrategyType]*interfaces.StrategyRegressionResult),
		CriticalIssues:  make([]string, 0),
		Warnings:        make([]string, 0),
		Recommendations: make([]string, 0),
	}

	// Run tests for each strategy
	for _, strategy := range config.Strategies {
		strategyResult, err := hrs.runStrategyRegressionTest(ctx, strategy, config)
		if err != nil {
			results.CriticalIssues = append(results.CriticalIssues,
				fmt.Sprintf("Failed to test strategy %s: %v", strategy, err))
			continue
		}

		results.StrategyResults[strategy] = strategyResult
		results.TotalTests += strategyResult.TestedTransactions

		if strategyResult.AccuracyScore >= config.AccuracyThreshold {
			results.PassedTests += strategyResult.TestedTransactions
		} else {
			results.FailedTests += strategyResult.TestedTransactions
		}
	}

	results.CompletedAt = time.Now()
	results.TotalReplayTime = results.CompletedAt.Sub(results.StartedAt)
	results.OverallSuccess = float64(results.PassedTests)/float64(results.TotalTests) >= config.AccuracyThreshold

	// Generate recommendations
	hrs.generateRegressionRecommendations(results)

	return results, nil
}

// GetHistoricalLogs retrieves historical logs based on filter criteria
func (hrs *HistoricalReplaySystemImpl) GetHistoricalLogs(ctx context.Context, filter *interfaces.HistoricalLogFilter) ([]*interfaces.HistoricalTransactionLog, error) {
	// This would typically query the database
	// For now, delegate to the transaction logger
	return hrs.getHistoricalLogsFromDB(ctx, filter)
}

// ArchiveOldLogs archives logs older than the specified duration
func (hrs *HistoricalReplaySystemImpl) ArchiveOldLogs(ctx context.Context, olderThan time.Duration) error {
	cutoffTime := time.Now().Add(-olderThan)

	// Archive to cold storage or delete based on configuration
	query := `UPDATE historical_transaction_logs SET archived = true WHERE created_at < $1 AND archived = false`
	_, err := hrs.db.ExecContext(ctx, query, cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to archive old logs: %w", err)
	}

	return nil
}

// Helper functions

func generateLogID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func generateTestID() string {
	return fmt.Sprintf("regression-test-%d", time.Now().Unix())
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func classifyAccuracy(accuracy float64) interfaces.AccuracyGrade {
	if accuracy >= 0.95 {
		return interfaces.AccuracyExcellent
	} else if accuracy >= 0.85 {
		return interfaces.AccuracyGood
	} else if accuracy >= 0.70 {
		return interfaces.AccuracyFair
	}
	return interfaces.AccuracyPoor
}

func determineRecommendedAction(comparison *interfaces.ProfitabilityComparison) interfaces.RecommendedAction {
	if comparison.OverallAccuracy >= 0.95 {
		return interfaces.ActionNone
	} else if comparison.OverallAccuracy >= 0.85 {
		return interfaces.ActionRecalibrate
	} else if comparison.OverallAccuracy >= 0.70 {
		return interfaces.ActionAdjustThresholds
	} else if comparison.OverallAccuracy >= 0.50 {
		return interfaces.ActionInvestigate
	}
	return interfaces.ActionDisableStrategy
}

// captureCurrentMarketConditions captures current market state
func (hrs *HistoricalReplaySystemImpl) captureCurrentMarketConditions(ctx context.Context) (*interfaces.MarketConditions, error) {
	// This would integrate with price feeds, gas price oracles, etc.
	// For now, return a basic structure
	return &interfaces.MarketConditions{
		Timestamp:         time.Now(),
		GasPrice:          big.NewInt(20000000000), // 20 gwei
		NetworkCongestion: 0.5,                     // 50%
		TokenPrices:       make(map[string]*big.Int),
		PoolLiquidities:   make(map[string]*big.Int),
		VolatilityIndex:   0.3,
	}, nil
}

// runStrategyRegressionTest runs regression test for a specific strategy
func (hrs *HistoricalReplaySystemImpl) runStrategyRegressionTest(ctx context.Context, strategy interfaces.StrategyType, config *interfaces.RegressionTestConfig) (*interfaces.StrategyRegressionResult, error) {
	// Get historical logs for this strategy
	filter := &interfaces.HistoricalLogFilter{
		Strategy:     &strategy,
		OnlyExecuted: !config.SkipFailedOriginals,
		Limit:        config.MaxTransactions,
	}

	if config.TimeWindow > 0 {
		startTime := time.Now().Add(-config.TimeWindow)
		filter.StartTime = &startTime
	}

	logs, err := hrs.GetHistoricalLogs(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical logs: %w", err)
	}

	if len(logs) == 0 {
		return &interfaces.StrategyRegressionResult{
			Strategy:           strategy,
			TestedTransactions: 0,
			AccuracyScore:      0,
			PassRate:           0,
		}, nil
	}

	// Replay transactions
	replayResults, err := hrs.BatchReplayTransactions(ctx, logs)
	if err != nil {
		return nil, fmt.Errorf("failed to replay transactions: %w", err)
	}

	// Analyze results
	result := &interfaces.StrategyRegressionResult{
		Strategy:              strategy,
		TestedTransactions:    len(logs),
		SignificantDeviations: make([]*interfaces.DeviationReport, 0),
		ThresholdViolations:   make([]string, 0),
	}

	accuracySum := 0.0
	passCount := 0

	for _, replayResult := range replayResults {
		if replayResult.Success {
			accuracySum += replayResult.AccuracyScore
			if replayResult.AccuracyScore >= config.AccuracyThreshold {
				passCount++
			}
		}
	}

	result.AccuracyScore = accuracySum / float64(len(replayResults))
	result.PassRate = float64(passCount) / float64(len(replayResults))

	return result, nil
}

// generateRegressionRecommendations generates recommendations based on test results
func (hrs *HistoricalReplaySystemImpl) generateRegressionRecommendations(results *interfaces.RegressionTestResults) {
	// Analyze results and generate recommendations
	totalAccuracy := 0.0
	strategyCount := 0

	for _, strategyResult := range results.StrategyResults {
		totalAccuracy += strategyResult.AccuracyScore
		strategyCount++

		if strategyResult.AccuracyScore < 0.70 {
			results.CriticalIssues = append(results.CriticalIssues,
				fmt.Sprintf("Strategy %s has poor accuracy: %.2f%%", strategyResult.Strategy, strategyResult.AccuracyScore*100))
		}

		if strategyResult.PassRate < 0.80 {
			results.Warnings = append(results.Warnings,
				fmt.Sprintf("Strategy %s has low pass rate: %.2f%%", strategyResult.Strategy, strategyResult.PassRate*100))
		}
	}

	if strategyCount > 0 {
		avgAccuracy := totalAccuracy / float64(strategyCount)
		if avgAccuracy < 0.85 {
			results.Recommendations = append(results.Recommendations,
				"Consider recalibrating profit models - overall accuracy is below 85%")
		}
	}
}

// getHistoricalLogsFromDB retrieves logs from database based on filter
func (hrs *HistoricalReplaySystemImpl) getHistoricalLogsFromDB(ctx context.Context, filter *interfaces.HistoricalLogFilter) ([]*interfaces.HistoricalTransactionLog, error) {
	// This would build and execute the appropriate SQL query
	// For now, return empty slice
	return []*interfaces.HistoricalTransactionLog{}, nil
}
