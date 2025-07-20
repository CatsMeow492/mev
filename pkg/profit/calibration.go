package profit

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// CalibrationSystem manages profit model calibration using historical performance
type CalibrationSystem struct {
	historicalResults map[interfaces.StrategyType][]*HistoricalResult
	modelParameters   map[interfaces.StrategyType]*ModelParameters
	mu                sync.RWMutex
	lastCalibration   time.Time
}

// HistoricalResult represents a historical MEV execution result
type HistoricalResult struct {
	Timestamp        time.Time
	Strategy         interfaces.StrategyType
	PredictedProfit  *big.Int
	ActualProfit     *big.Int
	GasCosts         *big.Int
	SlippageCosts    *big.Int
	ExecutionSuccess bool
	BlockNumber      uint64
	TransactionHash  string
}

// ModelParameters contains calibrated parameters for profit prediction models
type ModelParameters struct {
	GasMultiplier       float64   // Multiplier for gas cost predictions
	SlippageMultiplier  float64   // Multiplier for slippage predictions
	SuccessRate         float64   // Historical success rate
	ProfitAccuracy      float64   // Accuracy of profit predictions (0-1)
	LastCalibrated      time.Time
	SampleSize          int
	ConfidenceInterval  float64   // 95% confidence interval
}

// CalibrationConfig defines parameters for model calibration
type CalibrationConfig struct {
	MinSampleSize       int           // Minimum samples needed for calibration
	CalibrationWindow   time.Duration // Time window for historical data
	RecalibrationPeriod time.Duration // How often to recalibrate
	OutlierThreshold    float64       // Threshold for outlier detection (z-score)
}

// NewCalibrationSystem creates a new calibration system
func NewCalibrationSystem() *CalibrationSystem {
	return &CalibrationSystem{
		historicalResults: make(map[interfaces.StrategyType][]*HistoricalResult),
		modelParameters:   make(map[interfaces.StrategyType]*ModelParameters),
		lastCalibration:   time.Now(),
	}
}

// AddHistoricalResult adds a new historical execution result
func (cs *CalibrationSystem) AddHistoricalResult(result *HistoricalResult) error {
	if result == nil {
		return fmt.Errorf("result cannot be nil")
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Add to historical results
	if _, exists := cs.historicalResults[result.Strategy]; !exists {
		cs.historicalResults[result.Strategy] = make([]*HistoricalResult, 0)
	}

	cs.historicalResults[result.Strategy] = append(cs.historicalResults[result.Strategy], result)

	// Keep only last 1000 results per strategy
	if len(cs.historicalResults[result.Strategy]) > 1000 {
		cs.historicalResults[result.Strategy] = cs.historicalResults[result.Strategy][1:]
	}

	return nil
}

// CalibrateModels recalibrates all profit prediction models
func (cs *CalibrationSystem) CalibrateModels(config *CalibrationConfig) error {
	if config == nil {
		config = &CalibrationConfig{
			MinSampleSize:       50,
			CalibrationWindow:   7 * 24 * time.Hour, // 7 days
			RecalibrationPeriod: 24 * time.Hour,     // Daily
			OutlierThreshold:    2.5,                // 2.5 standard deviations
		}
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Check if recalibration is needed
	if time.Since(cs.lastCalibration) < config.RecalibrationPeriod {
		return nil
	}

	cutoff := time.Now().Add(-config.CalibrationWindow)

	for strategy, results := range cs.historicalResults {
		// Filter recent results
		recentResults := cs.filterRecentResults(results, cutoff)
		
		if len(recentResults) < config.MinSampleSize {
			continue
		}

		// Remove outliers
		cleanResults := cs.removeOutliers(recentResults, config.OutlierThreshold)
		
		if len(cleanResults) < config.MinSampleSize {
			continue
		}

		// Calibrate model parameters
		params, err := cs.calibrateStrategy(strategy, cleanResults)
		if err != nil {
			continue
		}

		cs.modelParameters[strategy] = params
	}

	cs.lastCalibration = time.Now()
	return nil
}

// filterRecentResults filters results within the calibration window
func (cs *CalibrationSystem) filterRecentResults(results []*HistoricalResult, cutoff time.Time) []*HistoricalResult {
	var filtered []*HistoricalResult
	for _, result := range results {
		if result.Timestamp.After(cutoff) {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

// removeOutliers removes statistical outliers from the dataset
func (cs *CalibrationSystem) removeOutliers(results []*HistoricalResult, threshold float64) []*HistoricalResult {
	if len(results) < 10 {
		return results // Not enough data for outlier detection
	}

	// Calculate profit ratios (actual/predicted)
	ratios := make([]float64, 0, len(results))
	for _, result := range results {
		if result.PredictedProfit.Cmp(big.NewInt(0)) > 0 {
			predicted, _ := result.PredictedProfit.Float64()
			actual, _ := result.ActualProfit.Float64()
			if predicted > 0 {
				ratios = append(ratios, actual/predicted)
			}
		}
	}

	if len(ratios) < 10 {
		return results
	}

	// Calculate mean and standard deviation
	mean := cs.calculateMean(ratios)
	stdDev := cs.calculateStdDev(ratios, mean)

	// Filter out outliers
	var filtered []*HistoricalResult
	ratioIndex := 0
	
	for _, result := range results {
		if result.PredictedProfit.Cmp(big.NewInt(0)) > 0 {
			predicted, _ := result.PredictedProfit.Float64()
			if predicted > 0 {
				ratio := ratios[ratioIndex]
				zScore := math.Abs(ratio-mean) / stdDev
				
				if zScore <= threshold {
					filtered = append(filtered, result)
				}
				ratioIndex++
			}
		} else {
			filtered = append(filtered, result)
		}
	}

	return filtered
}

// calibrateStrategy calibrates model parameters for a specific strategy
func (cs *CalibrationSystem) calibrateStrategy(strategy interfaces.StrategyType, results []*HistoricalResult) (*ModelParameters, error) {
	if len(results) == 0 {
		return nil, fmt.Errorf("no results for strategy %s", strategy)
	}

	params := &ModelParameters{
		LastCalibrated: time.Now(),
		SampleSize:     len(results),
	}

	// Calculate success rate
	successCount := 0
	for _, result := range results {
		if result.ExecutionSuccess && result.ActualProfit.Cmp(big.NewInt(0)) > 0 {
			successCount++
		}
	}
	params.SuccessRate = float64(successCount) / float64(len(results))

	// Calculate profit ratios for accuracy assessment
	profitRatios := make([]float64, 0)

	for _, result := range results {
		// Profit accuracy (actual/predicted)
		if result.PredictedProfit.Cmp(big.NewInt(0)) > 0 {
			predicted, _ := result.PredictedProfit.Float64()
			actual, _ := result.ActualProfit.Float64()
			if predicted > 0 {
				profitRatios = append(profitRatios, actual/predicted)
			}
		}
	}

	// Set default multipliers if we don't have enough data
	params.GasMultiplier = 1.1    // 10% buffer for gas costs
	params.SlippageMultiplier = 1.2 // 20% buffer for slippage

	// Calculate profit accuracy
	if len(profitRatios) > 0 {
		mean := cs.calculateMean(profitRatios)
		stdDev := cs.calculateStdDev(profitRatios, mean)
		
		// Accuracy is inverse of coefficient of variation
		if mean > 0 {
			cv := stdDev / mean
			params.ProfitAccuracy = math.Max(0, 1.0-cv)
		}

		// Calculate 95% confidence interval
		params.ConfidenceInterval = 1.96 * stdDev / math.Sqrt(float64(len(profitRatios)))
	}

	return params, nil
}

// GetModelParameters returns calibrated parameters for a strategy
func (cs *CalibrationSystem) GetModelParameters(strategy interfaces.StrategyType) (*ModelParameters, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	params, exists := cs.modelParameters[strategy]
	return params, exists
}

// GetCalibrationStats returns statistics about the calibration system
func (cs *CalibrationSystem) GetCalibrationStats() map[string]interface{} {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	stats := make(map[string]interface{})
	
	totalResults := 0
	for _, results := range cs.historicalResults {
		totalResults += len(results)
	}

	stats["total_historical_results"] = totalResults
	stats["calibrated_strategies"] = len(cs.modelParameters)
	stats["last_calibration"] = cs.lastCalibration

	// Strategy-specific stats
	strategyStats := make(map[string]interface{})
	for strategy, params := range cs.modelParameters {
		strategyStats[string(strategy)] = map[string]interface{}{
			"success_rate":         params.SuccessRate,
			"profit_accuracy":      params.ProfitAccuracy,
			"gas_multiplier":       params.GasMultiplier,
			"slippage_multiplier":  params.SlippageMultiplier,
			"sample_size":          params.SampleSize,
			"last_calibrated":      params.LastCalibrated,
			"confidence_interval":  params.ConfidenceInterval,
		}
	}
	stats["strategy_parameters"] = strategyStats

	return stats
}

// ApplyCalibration applies calibrated parameters to a profit estimate
func (cs *CalibrationSystem) ApplyCalibration(strategy interfaces.StrategyType, estimate *interfaces.ProfitEstimate) *interfaces.ProfitEstimate {
	cs.mu.RLock()
	params, exists := cs.modelParameters[strategy]
	cs.mu.RUnlock()

	if !exists {
		return estimate // No calibration available
	}

	// Create calibrated estimate
	calibrated := &interfaces.ProfitEstimate{
		GrossProfit:        new(big.Int).Set(estimate.GrossProfit),
		GasCosts:          cs.applyMultiplier(estimate.GasCosts, params.GasMultiplier),
		SlippageCosts:     cs.applyMultiplier(estimate.SlippageCosts, params.SlippageMultiplier),
		ProfitMargin:      estimate.ProfitMargin,
		SuccessProbability: params.SuccessRate,
		RiskScore:         estimate.RiskScore,
		Confidence:        params.ProfitAccuracy,
	}

	// Recalculate net profit with calibrated costs
	calibrated.NetProfit = new(big.Int).Sub(calibrated.GrossProfit, calibrated.GasCosts)
	calibrated.NetProfit.Sub(calibrated.NetProfit, calibrated.SlippageCosts)

	// Recalculate profit margin
	if calibrated.GrossProfit.Cmp(big.NewInt(0)) > 0 {
		netFloat, _ := calibrated.NetProfit.Float64()
		grossFloat, _ := calibrated.GrossProfit.Float64()
		calibrated.ProfitMargin = netFloat / grossFloat
	}

	return calibrated
}

// applyMultiplier applies a multiplier to a big.Int value
func (cs *CalibrationSystem) applyMultiplier(value *big.Int, multiplier float64) *big.Int {
	if value == nil {
		return big.NewInt(0)
	}

	valueFloat, _ := value.Float64()
	adjustedFloat := valueFloat * multiplier
	
	result := big.NewInt(int64(adjustedFloat))
	return result
}

// calculateMean calculates the mean of a slice of float64 values
func (cs *CalibrationSystem) calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// calculateStdDev calculates the standard deviation of a slice of float64 values
func (cs *CalibrationSystem) calculateStdDev(values []float64, mean float64) float64 {
	if len(values) <= 1 {
		return 0
	}

	sumSquaredDiff := 0.0
	for _, v := range values {
		diff := v - mean
		sumSquaredDiff += diff * diff
	}

	variance := sumSquaredDiff / float64(len(values)-1)
	return math.Sqrt(variance)
}

// ExportHistoricalData exports historical results for external analysis
func (cs *CalibrationSystem) ExportHistoricalData(strategy interfaces.StrategyType, timeWindow time.Duration) []*HistoricalResult {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	results, exists := cs.historicalResults[strategy]
	if !exists {
		return []*HistoricalResult{}
	}

	cutoff := time.Now().Add(-timeWindow)
	var exported []*HistoricalResult

	for _, result := range results {
		if result.Timestamp.After(cutoff) {
			exported = append(exported, result)
		}
	}

	// Sort by timestamp
	sort.Slice(exported, func(i, j int) bool {
		return exported[i].Timestamp.Before(exported[j].Timestamp)
	})

	return exported
}

// ValidateCalibration validates the quality of calibration for a strategy
func (cs *CalibrationSystem) ValidateCalibration(strategy interfaces.StrategyType) (*CalibrationValidation, error) {
	cs.mu.RLock()
	params, exists := cs.modelParameters[strategy]
	results, hasResults := cs.historicalResults[strategy]
	cs.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no calibration parameters for strategy %s", strategy)
	}

	if !hasResults || len(results) == 0 {
		return nil, fmt.Errorf("no historical results for strategy %s", strategy)
	}

	validation := &CalibrationValidation{
		Strategy:       strategy,
		SampleSize:     params.SampleSize,
		DataQuality:    cs.assessDataQuality(results),
		ModelAccuracy:  params.ProfitAccuracy,
		Confidence:     params.ConfidenceInterval,
		LastValidated:  time.Now(),
	}

	// Assess calibration quality
	if params.SampleSize >= 100 && params.ProfitAccuracy >= 0.7 {
		validation.Quality = "high"
	} else if params.SampleSize >= 50 && params.ProfitAccuracy >= 0.5 {
		validation.Quality = "medium"
	} else {
		validation.Quality = "low"
	}

	return validation, nil
}

// CalibrationValidation contains validation results for a calibrated model
type CalibrationValidation struct {
	Strategy      interfaces.StrategyType
	Quality       string    // "high", "medium", "low"
	SampleSize    int
	DataQuality   float64   // 0-1 score
	ModelAccuracy float64   // 0-1 score
	Confidence    float64   // Confidence interval
	LastValidated time.Time
}

// assessDataQuality assesses the quality of historical data
func (cs *CalibrationSystem) assessDataQuality(results []*HistoricalResult) float64 {
	if len(results) == 0 {
		return 0
	}

	score := 0.0
	factors := 0

	// Factor 1: Data completeness
	completeResults := 0
	for _, result := range results {
		if result.PredictedProfit != nil && result.ActualProfit != nil &&
			result.GasCosts != nil && result.SlippageCosts != nil {
			completeResults++
		}
	}
	completeness := float64(completeResults) / float64(len(results))
	score += completeness
	factors++

	// Factor 2: Data recency
	now := time.Now()
	recentCount := 0
	for _, result := range results {
		if now.Sub(result.Timestamp) <= 7*24*time.Hour { // Last 7 days
			recentCount++
		}
	}
	recency := float64(recentCount) / float64(len(results))
	score += recency
	factors++

	// Factor 3: Success rate variability (not too high, not too low)
	successCount := 0
	for _, result := range results {
		if result.ExecutionSuccess {
			successCount++
		}
	}
	successRate := float64(successCount) / float64(len(results))
	// Optimal success rate is around 0.7, penalize extremes
	variability := 1.0 - math.Abs(successRate-0.7)/0.3
	if variability < 0 {
		variability = 0
	}
	score += variability
	factors++

	return score / float64(factors)
}