package profit

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

func TestNewCalibrationSystem(t *testing.T) {
	cs := NewCalibrationSystem()
	
	if cs == nil {
		t.Fatal("NewCalibrationSystem returned nil")
	}
	
	if cs.historicalResults == nil {
		t.Error("historicalResults map not initialized")
	}
	
	if cs.modelParameters == nil {
		t.Error("modelParameters map not initialized")
	}
}

func TestAddHistoricalResult(t *testing.T) {
	cs := NewCalibrationSystem()
	
	// Test nil result
	err := cs.AddHistoricalResult(nil)
	if err == nil {
		t.Error("Expected error for nil result")
	}
	
	// Test valid result
	result := &HistoricalResult{
		Timestamp:        time.Now(),
		Strategy:         interfaces.StrategySandwich,
		PredictedProfit:  big.NewInt(1e18),
		ActualProfit:     big.NewInt(8e17),
		GasCosts:         big.NewInt(1e17),
		SlippageCosts:    big.NewInt(5e16),
		ExecutionSuccess: true,
		BlockNumber:      12345,
		TransactionHash:  "0x123",
	}
	
	err = cs.AddHistoricalResult(result)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	// Check that result was added
	cs.mu.RLock()
	results, exists := cs.historicalResults[interfaces.StrategySandwich]
	cs.mu.RUnlock()
	
	if !exists {
		t.Error("Strategy not found in historical results")
	}
	
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	
	if results[0].TransactionHash != "0x123" {
		t.Errorf("Expected transaction hash 0x123, got %s", results[0].TransactionHash)
	}
}

func TestAddHistoricalResult_Limit(t *testing.T) {
	cs := NewCalibrationSystem()
	
	// Add more than 1000 results
	for i := 0; i < 1100; i++ {
		result := &HistoricalResult{
			Timestamp:        time.Now().Add(-time.Duration(i) * time.Minute),
			Strategy:         interfaces.StrategySandwich,
			PredictedProfit:  big.NewInt(1e18),
			ActualProfit:     big.NewInt(8e17),
			GasCosts:         big.NewInt(1e17),
			SlippageCosts:    big.NewInt(5e16),
			ExecutionSuccess: true,
			BlockNumber:      uint64(12345 + i),
			TransactionHash:  fmt.Sprintf("0x%d", i),
		}
		
		err := cs.AddHistoricalResult(result)
		if err != nil {
			t.Errorf("Unexpected error on result %d: %v", i, err)
		}
	}
	
	// Check that only 1000 results are kept
	cs.mu.RLock()
	results := cs.historicalResults[interfaces.StrategySandwich]
	cs.mu.RUnlock()
	
	if len(results) != 1000 {
		t.Errorf("Expected 1000 results, got %d", len(results))
	}
}

func TestCalibrateModels(t *testing.T) {
	cs := NewCalibrationSystem()
	
	// Add sufficient historical data
	now := time.Now()
	for i := 0; i < 60; i++ {
		result := &HistoricalResult{
			Timestamp:        now.Add(-time.Duration(i) * time.Hour),
			Strategy:         interfaces.StrategySandwich,
			PredictedProfit:  big.NewInt(1e18),
			ActualProfit:     big.NewInt(int64(8e17 + int64(i)*1e15)), // Varying actual profit
			GasCosts:         big.NewInt(1e17),
			SlippageCosts:    big.NewInt(5e16),
			ExecutionSuccess: i%10 != 0, // 90% success rate
			BlockNumber:      uint64(12345 + i),
			TransactionHash:  fmt.Sprintf("0x%d", i),
		}
		
		cs.AddHistoricalResult(result)
	}
	
	// Test calibration with custom config to ensure recalibration
	config := &CalibrationConfig{
		MinSampleSize:       50,
		CalibrationWindow:   7 * 24 * time.Hour, // 7 days
		RecalibrationPeriod: 1 * time.Hour,      // Force recalibration
		OutlierThreshold:    2.5,
	}
	
	// Force last calibration to be old
	cs.lastCalibration = time.Now().Add(-2 * time.Hour)
	
	err := cs.CalibrateModels(config)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	// Check that model was calibrated
	params, exists := cs.GetModelParameters(interfaces.StrategySandwich)
	if !exists {
		t.Error("Model parameters not found after calibration")
		return
	}
	
	if params == nil {
		t.Error("Model parameters are nil")
		return
	}
	
	if params.SampleSize == 0 {
		t.Error("Sample size should be greater than 0")
	}
	
	if params.SuccessRate < 0 || params.SuccessRate > 1 {
		t.Errorf("Success rate should be between 0 and 1, got %f", params.SuccessRate)
	}
	
	// Success rate should be around 0.9 (90%)
	if params.SuccessRate < 0.8 || params.SuccessRate > 1.0 {
		t.Errorf("Expected success rate around 0.9, got %f", params.SuccessRate)
	}
}

func TestCalibrateModels_InsufficientData(t *testing.T) {
	cs := NewCalibrationSystem()
	
	// Add insufficient data (less than 50 samples)
	for i := 0; i < 10; i++ {
		result := &HistoricalResult{
			Timestamp:        time.Now().Add(-time.Duration(i) * time.Hour),
			Strategy:         interfaces.StrategySandwich,
			PredictedProfit:  big.NewInt(1e18),
			ActualProfit:     big.NewInt(8e17),
			GasCosts:         big.NewInt(1e17),
			SlippageCosts:    big.NewInt(5e16),
			ExecutionSuccess: true,
			BlockNumber:      uint64(12345 + i),
			TransactionHash:  fmt.Sprintf("0x%d", i),
		}
		
		cs.AddHistoricalResult(result)
	}
	
	// Calibration should not create parameters due to insufficient data
	config := &CalibrationConfig{
		MinSampleSize:       50,
		CalibrationWindow:   24 * time.Hour,
		RecalibrationPeriod: 1 * time.Hour,
		OutlierThreshold:    2.5,
	}
	
	err := cs.CalibrateModels(config)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	// Should not have calibrated parameters
	_, exists := cs.GetModelParameters(interfaces.StrategySandwich)
	if exists {
		t.Error("Model parameters should not exist with insufficient data")
	}
}

func TestRemoveOutliers(t *testing.T) {
	cs := NewCalibrationSystem()
	
	// Create results with some outliers
	results := []*HistoricalResult{}
	
	// Normal results
	for i := 0; i < 20; i++ {
		result := &HistoricalResult{
			Timestamp:       time.Now().Add(-time.Duration(i) * time.Hour),
			Strategy:        interfaces.StrategySandwich,
			PredictedProfit: big.NewInt(1e18),
			ActualProfit:    big.NewInt(int64(8e17 + int64(i)*1e15)), // Normal variation
			ExecutionSuccess: true,
		}
		results = append(results, result)
	}
	
	// Add outliers
	outlier1 := &HistoricalResult{
		Timestamp:       time.Now().Add(-25 * time.Hour),
		Strategy:        interfaces.StrategySandwich,
		PredictedProfit: big.NewInt(1e18),
		ActualProfit:    big.NewInt(5e18), // 5x predicted (outlier)
		ExecutionSuccess: true,
	}
	results = append(results, outlier1)
	
	outlier2 := &HistoricalResult{
		Timestamp:       time.Now().Add(-26 * time.Hour),
		Strategy:        interfaces.StrategySandwich,
		PredictedProfit: big.NewInt(1e18),
		ActualProfit:    big.NewInt(1e16), // 0.1x predicted (outlier)
		ExecutionSuccess: true,
	}
	results = append(results, outlier2)
	
	// Remove outliers
	cleaned := cs.removeOutliers(results, 2.0) // 2 standard deviations
	
	// Should have fewer results after outlier removal
	if len(cleaned) >= len(results) {
		t.Error("Outlier removal should reduce the number of results")
	}
	
	// Should have removed the extreme outliers
	if len(cleaned) < 15 { // Should keep most normal results
		t.Errorf("Too many results removed, expected at least 15, got %d", len(cleaned))
	}
}

func TestApplyCalibration(t *testing.T) {
	cs := NewCalibrationSystem()
	
	// Set up calibrated parameters
	params := &ModelParameters{
		GasMultiplier:      1.2,
		SlippageMultiplier: 1.3,
		SuccessRate:        0.8,
		ProfitAccuracy:     0.75,
		LastCalibrated:     time.Now(),
		SampleSize:         100,
		ConfidenceInterval: 0.05,
	}
	
	cs.mu.Lock()
	cs.modelParameters[interfaces.StrategySandwich] = params
	cs.mu.Unlock()
	
	// Create original estimate
	original := &interfaces.ProfitEstimate{
		GrossProfit:        big.NewInt(1e18),
		GasCosts:          big.NewInt(1e17),
		SlippageCosts:     big.NewInt(5e16),
		NetProfit:         big.NewInt(8e17),
		ProfitMargin:      0.8,
		SuccessProbability: 0.9,
		RiskScore:         0.1,
		Confidence:        0.6,
	}
	
	// Apply calibration
	calibrated := cs.ApplyCalibration(interfaces.StrategySandwich, original)
	
	// Check that costs were adjusted
	expectedGasCosts := int64(float64(1e17) * 1.2)
	if calibrated.GasCosts.Int64() != expectedGasCosts {
		t.Errorf("Expected gas costs %d, got %d", expectedGasCosts, calibrated.GasCosts.Int64())
	}
	
	expectedSlippageCosts := int64(float64(5e16) * 1.3)
	if calibrated.SlippageCosts.Int64() != expectedSlippageCosts {
		t.Errorf("Expected slippage costs %d, got %d", expectedSlippageCosts, calibrated.SlippageCosts.Int64())
	}
	
	// Check that success probability was updated
	if calibrated.SuccessProbability != params.SuccessRate {
		t.Errorf("Expected success probability %f, got %f", params.SuccessRate, calibrated.SuccessProbability)
	}
	
	// Check that confidence was updated
	if calibrated.Confidence != params.ProfitAccuracy {
		t.Errorf("Expected confidence %f, got %f", params.ProfitAccuracy, calibrated.Confidence)
	}
	
	// Net profit should be recalculated
	expectedNetProfit := big.NewInt(1e18)
	expectedNetProfit.Sub(expectedNetProfit, calibrated.GasCosts)
	expectedNetProfit.Sub(expectedNetProfit, calibrated.SlippageCosts)
	
	if calibrated.NetProfit.Cmp(expectedNetProfit) != 0 {
		t.Errorf("Expected net profit %s, got %s", expectedNetProfit.String(), calibrated.NetProfit.String())
	}
}

func TestApplyCalibration_NoParameters(t *testing.T) {
	cs := NewCalibrationSystem()
	
	original := &interfaces.ProfitEstimate{
		GrossProfit:        big.NewInt(1e18),
		GasCosts:          big.NewInt(1e17),
		SlippageCosts:     big.NewInt(5e16),
		NetProfit:         big.NewInt(8e17),
		ProfitMargin:      0.8,
		SuccessProbability: 0.9,
		RiskScore:         0.1,
		Confidence:        0.6,
	}
	
	// Apply calibration without parameters
	calibrated := cs.ApplyCalibration(interfaces.StrategySandwich, original)
	
	// Should return original estimate unchanged
	if calibrated.GasCosts.Cmp(original.GasCosts) != 0 {
		t.Error("Gas costs should be unchanged without calibration parameters")
	}
	
	if calibrated.SuccessProbability != original.SuccessProbability {
		t.Error("Success probability should be unchanged without calibration parameters")
	}
}

func TestGetCalibrationStats(t *testing.T) {
	cs := NewCalibrationSystem()
	
	// Add some historical data
	for i := 0; i < 30; i++ {
		result := &HistoricalResult{
			Timestamp:        time.Now().Add(-time.Duration(i) * time.Hour),
			Strategy:         interfaces.StrategySandwich,
			PredictedProfit:  big.NewInt(1e18),
			ActualProfit:     big.NewInt(8e17),
			GasCosts:         big.NewInt(1e17),
			SlippageCosts:    big.NewInt(5e16),
			ExecutionSuccess: true,
			BlockNumber:      uint64(12345 + i),
			TransactionHash:  fmt.Sprintf("0x%d", i),
		}
		
		cs.AddHistoricalResult(result)
	}
	
	// Add calibrated parameters
	params := &ModelParameters{
		GasMultiplier:      1.2,
		SlippageMultiplier: 1.3,
		SuccessRate:        0.8,
		ProfitAccuracy:     0.75,
		LastCalibrated:     time.Now(),
		SampleSize:         30,
		ConfidenceInterval: 0.05,
	}
	
	cs.mu.Lock()
	cs.modelParameters[interfaces.StrategySandwich] = params
	cs.mu.Unlock()
	
	// Get stats
	stats := cs.GetCalibrationStats()
	
	// Check basic stats
	if stats["total_historical_results"].(int) != 30 {
		t.Errorf("Expected 30 historical results, got %d", stats["total_historical_results"].(int))
	}
	
	if stats["calibrated_strategies"].(int) != 1 {
		t.Errorf("Expected 1 calibrated strategy, got %d", stats["calibrated_strategies"].(int))
	}
	
	// Check strategy-specific stats
	strategyStats, ok := stats["strategy_parameters"].(map[string]interface{})
	if !ok {
		t.Error("Strategy parameters not found in stats")
	}
	
	sandwichStats, ok := strategyStats["sandwich"].(map[string]interface{})
	if !ok {
		t.Error("Sandwich strategy stats not found")
	}
	
	if sandwichStats["success_rate"].(float64) != 0.8 {
		t.Errorf("Expected success rate 0.8, got %f", sandwichStats["success_rate"].(float64))
	}
}

func TestValidateCalibration(t *testing.T) {
	cs := NewCalibrationSystem()
	
	// Test validation without parameters
	_, err := cs.ValidateCalibration(interfaces.StrategySandwich)
	if err == nil {
		t.Error("Expected error for strategy without calibration parameters")
	}
	
	// Add historical data and parameters
	for i := 0; i < 100; i++ {
		result := &HistoricalResult{
			Timestamp:        time.Now().Add(-time.Duration(i) * time.Hour),
			Strategy:         interfaces.StrategySandwich,
			PredictedProfit:  big.NewInt(1e18),
			ActualProfit:     big.NewInt(8e17),
			GasCosts:         big.NewInt(1e17),
			SlippageCosts:    big.NewInt(5e16),
			ExecutionSuccess: true,
			BlockNumber:      uint64(12345 + i),
			TransactionHash:  fmt.Sprintf("0x%d", i),
		}
		
		cs.AddHistoricalResult(result)
	}
	
	params := &ModelParameters{
		GasMultiplier:      1.2,
		SlippageMultiplier: 1.3,
		SuccessRate:        0.8,
		ProfitAccuracy:     0.75,
		LastCalibrated:     time.Now(),
		SampleSize:         100,
		ConfidenceInterval: 0.05,
	}
	
	cs.mu.Lock()
	cs.modelParameters[interfaces.StrategySandwich] = params
	cs.mu.Unlock()
	
	// Validate calibration
	validation, err := cs.ValidateCalibration(interfaces.StrategySandwich)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	if validation == nil {
		t.Error("Validation result is nil")
	}
	
	if validation.Strategy != interfaces.StrategySandwich {
		t.Errorf("Expected strategy %s, got %s", interfaces.StrategySandwich, validation.Strategy)
	}
	
	if validation.Quality != "high" {
		t.Errorf("Expected high quality calibration, got %s", validation.Quality)
	}
	
	if validation.SampleSize != 100 {
		t.Errorf("Expected sample size 100, got %d", validation.SampleSize)
	}
}

func TestExportHistoricalData(t *testing.T) {
	cs := NewCalibrationSystem()
	
	// Add historical data with different timestamps
	now := time.Now()
	for i := 0; i < 10; i++ {
		result := &HistoricalResult{
			Timestamp:        now.Add(-time.Duration(i) * time.Hour),
			Strategy:         interfaces.StrategySandwich,
			PredictedProfit:  big.NewInt(1e18),
			ActualProfit:     big.NewInt(8e17),
			GasCosts:         big.NewInt(1e17),
			SlippageCosts:    big.NewInt(5e16),
			ExecutionSuccess: true,
			BlockNumber:      uint64(12345 + i),
			TransactionHash:  fmt.Sprintf("0x%d", i),
		}
		
		cs.AddHistoricalResult(result)
	}
	
	// Export last 5 hours of data
	exported := cs.ExportHistoricalData(interfaces.StrategySandwich, 5*time.Hour)
	
	// Should return around 6 results (0-5 hours ago), but timing might be slightly off
	expectedCount := 6
	if len(exported) < expectedCount-1 || len(exported) > expectedCount {
		t.Errorf("Expected around %d exported results, got %d", expectedCount, len(exported))
	}
	
	// Results should be sorted by timestamp (oldest first)
	for i := 1; i < len(exported); i++ {
		if exported[i].Timestamp.Before(exported[i-1].Timestamp) {
			t.Error("Exported results should be sorted by timestamp")
		}
	}
	
	// Test non-existent strategy
	exported = cs.ExportHistoricalData(interfaces.StrategyBackrun, 24*time.Hour)
	if len(exported) != 0 {
		t.Errorf("Expected 0 results for non-existent strategy, got %d", len(exported))
	}
}