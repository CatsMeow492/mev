package replay

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// PerformanceValidatorImpl implements the PerformanceValidator interface
type PerformanceValidatorImpl struct {
	db                *sql.DB
	transactionLogger interfaces.TransactionLogger
}

// NewPerformanceValidator creates a new performance validator
func NewPerformanceValidator(db *sql.DB, transactionLogger interfaces.TransactionLogger) interfaces.PerformanceValidator {
	return &PerformanceValidatorImpl{
		db:                db,
		transactionLogger: transactionLogger,
	}
}

// ValidateStrategy validates a strategy's performance over a time window
func (pv *PerformanceValidatorImpl) ValidateStrategy(ctx context.Context, strategy interfaces.StrategyType, timeWindow time.Duration) (*interfaces.StrategyValidationResult, error) {
	startTime := time.Now().Add(-timeWindow)

	// Get historical logs for this strategy
	logs, err := pv.transactionLogger.GetLogsByTimeRange(ctx, startTime, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to get historical logs: %w", err)
	}

	// Filter logs for the specific strategy
	strategyLogs := make([]*interfaces.HistoricalTransactionLog, 0)
	for _, log := range logs {
		if log.Strategy == strategy {
			strategyLogs = append(strategyLogs, log)
		}
	}

	result := &interfaces.StrategyValidationResult{
		Strategy:             strategy,
		ValidationPeriod:     timeWindow,
		TotalOpportunities:   len(strategyLogs),
		SuccessfulReplays:    0,
		FailedReplays:        0,
		ThresholdAdjustments: make(map[string]float64),
		AccuracyDistribution: make(map[interfaces.AccuracyGrade]int),
		TimeSeriesData:       make([]*interfaces.TimeSeriesPoint, 0),
	}

	if len(strategyLogs) == 0 {
		result.OverallAccuracy = 0
		result.StrategyStatus = interfaces.StatusWarning
		return result, nil
	}

	// Simulate replays to calculate accuracy metrics
	totalProfitAccuracy := 0.0
	totalGasCostAccuracy := 0.0
	totalSlippageAccuracy := 0.0
	validAccuracyCount := 0

	for _, log := range strategyLogs {
		// Calculate accuracy based on actual vs expected results
		if log.ActualTradeResult != nil {
			profitAccuracy := pv.calculateProfitAccuracy(log)
			gasCostAccuracy := pv.calculateGasCostAccuracy(log)
			slippageAccuracy := pv.calculateSlippageAccuracy(log)

			if profitAccuracy >= 0 {
				totalProfitAccuracy += profitAccuracy
				validAccuracyCount++
				result.SuccessfulReplays++
			} else {
				result.FailedReplays++
			}

			totalGasCostAccuracy += gasCostAccuracy
			totalSlippageAccuracy += slippageAccuracy

			// Classify accuracy
			overallAccuracy := (profitAccuracy + gasCostAccuracy + slippageAccuracy) / 3.0
			grade := classifyAccuracy(overallAccuracy)
			result.AccuracyDistribution[grade]++
		}
	}

	// Calculate average accuracies
	if validAccuracyCount > 0 {
		result.AverageProfitAccuracy = totalProfitAccuracy / float64(validAccuracyCount)
		result.AverageGasCostAccuracy = totalGasCostAccuracy / float64(validAccuracyCount)
		result.AverageSlippageAccuracy = totalSlippageAccuracy / float64(validAccuracyCount)
		result.OverallAccuracy = (result.AverageProfitAccuracy + result.AverageGasCostAccuracy + result.AverageSlippageAccuracy) / 3.0
	}

	// Determine trends
	result.ProfitabilityTrend = pv.calculateTrend(strategyLogs, "profitability")
	result.AccuracyTrend = pv.calculateTrend(strategyLogs, "accuracy")

	// Determine strategy status
	result.StrategyStatus = pv.determineStrategyStatus(result)

	// Generate recommendations
	pv.generateValidationRecommendations(result)

	return result, nil
}

// CheckThresholdChanges validates the impact of threshold changes
func (pv *PerformanceValidatorImpl) CheckThresholdChanges(ctx context.Context, oldThresholds, newThresholds map[interfaces.StrategyType]*interfaces.ProfitThreshold) (*interfaces.ThresholdValidationResult, error) {
	result := &interfaces.ThresholdValidationResult{
		OriginalThresholds: oldThresholds,
		NewThresholds:      newThresholds,
		ValidationResults:  make(map[interfaces.StrategyType]*interfaces.ThresholdValidation),
		Recommendations:    make([]string, 0),
	}

	positiveImpacts := 0
	neutralImpacts := 0
	negativeImpacts := 0

	// Validate each strategy threshold change
	for strategy, newThreshold := range newThresholds {
		oldThreshold, exists := oldThresholds[strategy]
		if !exists {
			continue
		}

		validation, err := pv.validateSingleThresholdChange(ctx, strategy, oldThreshold, newThreshold)
		if err != nil {
			continue
		}

		result.ValidationResults[strategy] = validation

		// Assess impact
		if validation.ProfitabilityImpact > 0.05 { // 5% improvement
			positiveImpacts++
		} else if validation.ProfitabilityImpact < -0.05 { // 5% degradation
			negativeImpacts++
		} else {
			neutralImpacts++
		}
	}

	// Determine overall impact
	if positiveImpacts > negativeImpacts {
		result.OverallImpact = interfaces.ImpactPositive
	} else if negativeImpacts > positiveImpacts {
		result.OverallImpact = interfaces.ImpactNegative
	} else {
		result.OverallImpact = interfaces.ImpactNeutral
	}

	return result, nil
}

// GeneratePerformanceReport generates a comprehensive performance report
func (pv *PerformanceValidatorImpl) GeneratePerformanceReport(ctx context.Context, strategy interfaces.StrategyType) (*interfaces.PerformanceReport, error) {
	reportPeriod := 30 * 24 * time.Hour // 30 days
	startTime := time.Now().Add(-reportPeriod)

	// Get historical data
	logs, err := pv.transactionLogger.GetLogsByTimeRange(ctx, startTime, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to get historical logs: %w", err)
	}

	// Filter for strategy
	strategyLogs := make([]*interfaces.HistoricalTransactionLog, 0)
	for _, log := range logs {
		if log.Strategy == strategy {
			strategyLogs = append(strategyLogs, log)
		}
	}

	report := &interfaces.PerformanceReport{
		Strategy:            strategy,
		ReportPeriod:        reportPeriod,
		GeneratedAt:         time.Now(),
		TotalOpportunities:  len(strategyLogs),
		TotalExpectedProfit: big.NewInt(0),
		TotalActualProfit:   big.NewInt(0),
		TotalLosses:         big.NewInt(0),
		Insights:            make([]string, 0),
		ActionItems:         make([]string, 0),
	}

	executedCount := 0
	profitableCount := 0

	for _, log := range strategyLogs {
		if log.ExpectedProfit != nil {
			report.TotalExpectedProfit.Add(report.TotalExpectedProfit, log.ExpectedProfit)
		}

		if log.ActualTradeResult != nil {
			executedCount++
			if log.ActualTradeResult.NetProfit != nil {
				if log.ActualTradeResult.NetProfit.Sign() > 0 {
					profitableCount++
					report.TotalActualProfit.Add(report.TotalActualProfit, log.ActualTradeResult.NetProfit)
				} else {
					loss := new(big.Int).Neg(log.ActualTradeResult.NetProfit)
					report.TotalLosses.Add(report.TotalLosses, loss)
				}
			}
		}
	}

	report.ExecutedOpportunities = executedCount
	report.ProfitableExecutions = profitableCount

	// Calculate ROI
	if report.TotalExpectedProfit.Sign() > 0 {
		actualFloat, _ := report.TotalActualProfit.Float64()
		expectedFloat, _ := report.TotalExpectedProfit.Float64()
		report.ROI = actualFloat / expectedFloat
	}

	// Calculate net profitability
	report.NetProfitability = new(big.Int).Sub(report.TotalActualProfit, report.TotalLosses)

	// Generate insights
	pv.generateReportInsights(report)

	return report, nil
}

// Helper methods

func (pv *PerformanceValidatorImpl) calculateProfitAccuracy(log *interfaces.HistoricalTransactionLog) float64 {
	if log.ExpectedProfit == nil || log.ActualTradeResult == nil || log.ActualTradeResult.NetProfit == nil {
		return -1 // Invalid
	}

	expectedFloat, _ := log.ExpectedProfit.Float64()
	actualFloat, _ := log.ActualTradeResult.NetProfit.Float64()

	if expectedFloat == 0 {
		return 1.0 // Perfect if both are zero
	}

	diff := abs(actualFloat - expectedFloat)
	return 1.0 - (diff / expectedFloat)
}

func (pv *PerformanceValidatorImpl) calculateGasCostAccuracy(log *interfaces.HistoricalTransactionLog) float64 {
	if log.EstimatedGasCost == nil || log.ActualTradeResult == nil || log.ActualTradeResult.GasCost == nil {
		return 0.8 // Default reasonable accuracy
	}

	expectedFloat, _ := log.EstimatedGasCost.Float64()
	actualFloat, _ := log.ActualTradeResult.GasCost.Float64()

	if expectedFloat == 0 {
		return 1.0
	}

	diff := abs(actualFloat - expectedFloat)
	return 1.0 - (diff / expectedFloat)
}

func (pv *PerformanceValidatorImpl) calculateSlippageAccuracy(log *interfaces.HistoricalTransactionLog) float64 {
	// Simplified slippage accuracy calculation
	// In reality, this would compare estimated vs actual slippage
	return 0.85 // Default reasonable accuracy
}

func (pv *PerformanceValidatorImpl) calculateTrend(logs []*interfaces.HistoricalTransactionLog, metric string) interfaces.TrendDirection {
	if len(logs) < 10 {
		return interfaces.TrendUnknown
	}

	// Simple trend calculation - compare first half vs second half
	midpoint := len(logs) / 2
	firstHalf := logs[:midpoint]
	secondHalf := logs[midpoint:]

	var firstAvg, secondAvg float64

	switch metric {
	case "profitability":
		firstAvg = pv.calculateAverageProfitability(firstHalf)
		secondAvg = pv.calculateAverageProfitability(secondHalf)
	case "accuracy":
		firstAvg = pv.calculateAverageAccuracy(firstHalf)
		secondAvg = pv.calculateAverageAccuracy(secondHalf)
	default:
		return interfaces.TrendUnknown
	}

	diff := secondAvg - firstAvg
	if diff > 0.05 { // 5% improvement
		return interfaces.TrendImproving
	} else if diff < -0.05 { // 5% decline
		return interfaces.TrendDeclining
	}
	return interfaces.TrendStable
}

func (pv *PerformanceValidatorImpl) calculateAverageProfitability(logs []*interfaces.HistoricalTransactionLog) float64 {
	totalProfit := 0.0
	count := 0

	for _, log := range logs {
		if log.ActualTradeResult != nil && log.ActualTradeResult.NetProfit != nil {
			profit, _ := log.ActualTradeResult.NetProfit.Float64()
			totalProfit += profit
			count++
		}
	}

	if count == 0 {
		return 0
	}
	return totalProfit / float64(count)
}

func (pv *PerformanceValidatorImpl) calculateAverageAccuracy(logs []*interfaces.HistoricalTransactionLog) float64 {
	totalAccuracy := 0.0
	count := 0

	for _, log := range logs {
		accuracy := pv.calculateProfitAccuracy(log)
		if accuracy >= 0 {
			totalAccuracy += accuracy
			count++
		}
	}

	if count == 0 {
		return 0
	}
	return totalAccuracy / float64(count)
}

func (pv *PerformanceValidatorImpl) determineStrategyStatus(result *interfaces.StrategyValidationResult) interfaces.StrategyStatus {
	if result.OverallAccuracy >= 0.90 && result.AverageProfitAccuracy >= 0.85 {
		return interfaces.StatusHealthy
	} else if result.OverallAccuracy >= 0.75 && result.AverageProfitAccuracy >= 0.70 {
		return interfaces.StatusWarning
	} else if result.OverallAccuracy >= 0.50 {
		return interfaces.StatusCritical
	}
	return interfaces.StatusDisabled
}

func (pv *PerformanceValidatorImpl) generateValidationRecommendations(result *interfaces.StrategyValidationResult) {
	if result.AverageProfitAccuracy < 0.80 {
		result.ModelRecalibration = true
		result.ThresholdAdjustments["min_profit_margin"] = 0.05 // Increase by 5%
	}

	if result.OverallAccuracy < 0.75 {
		result.ThresholdAdjustments["min_success_probability"] = 0.1 // Increase by 10%
	}
}

func (pv *PerformanceValidatorImpl) validateSingleThresholdChange(ctx context.Context, strategy interfaces.StrategyType, oldThreshold, newThreshold *interfaces.ProfitThreshold) (*interfaces.ThresholdValidation, error) {
	validation := &interfaces.ThresholdValidation{
		Strategy:     strategy,
		OldThreshold: oldThreshold,
		NewThreshold: newThreshold,
	}

	// Get recent historical data to test threshold changes
	logs, err := pv.transactionLogger.GetLogsByStrategy(ctx, strategy, 100)
	if err != nil {
		return validation, err
	}

	detectedOld := 0
	detectedNew := 0

	for _, log := range logs {
		// Check if opportunity would have been detected under old thresholds
		if pv.wouldDetectWithThreshold(log, oldThreshold) {
			detectedOld++
		}

		// Check if opportunity would have been detected under new thresholds
		if pv.wouldDetectWithThreshold(log, newThreshold) {
			detectedNew++
		}
	}

	validation.WouldHaveDetected = detectedNew
	validation.WouldHaveMissed = detectedOld - detectedNew

	// Calculate impact
	if detectedOld > 0 {
		validation.VolumeImpact = float64(detectedNew-detectedOld) / float64(detectedOld)
	}

	return validation, nil
}

func (pv *PerformanceValidatorImpl) wouldDetectWithThreshold(log *interfaces.HistoricalTransactionLog, threshold *interfaces.ProfitThreshold) bool {
	// Simplified threshold checking
	// In reality, this would apply the full strategy detection logic

	if log.ExpectedProfit == nil {
		return false
	}

	// Check minimum profit threshold
	if log.ExpectedProfit.Cmp(threshold.MinNetProfit) < 0 {
		return false
	}

	// Check confidence threshold
	if log.Confidence < threshold.MinSuccessProbability {
		return false
	}

	return true
}

func (pv *PerformanceValidatorImpl) generateReportInsights(report *interfaces.PerformanceReport) {
	if report.ROI > 1.2 {
		report.Insights = append(report.Insights, "Strategy is performing well above expectations with ROI > 120%")
	} else if report.ROI < 0.8 {
		report.Insights = append(report.Insights, "Strategy is underperforming with ROI < 80%")
		report.ActionItems = append(report.ActionItems, "Consider recalibrating profit estimation models")
	}

	if report.ProfitableExecutions > 0 {
		successRate := float64(report.ProfitableExecutions) / float64(report.ExecutedOpportunities)
		if successRate < 0.70 {
			report.ActionItems = append(report.ActionItems, "Success rate is below 70% - review strategy thresholds")
		}
	}

	if report.ExecutedOpportunities < report.TotalOpportunities/10 {
		report.Insights = append(report.Insights, "Low execution rate - many opportunities not being executed")
		report.ActionItems = append(report.ActionItems, "Review execution constraints and gas price strategies")
	}
}
