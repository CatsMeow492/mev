package validation

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/strategy"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

// StrategyValidationFramework provides comprehensive validation for all MEV strategies
type StrategyValidationFramework struct {
	replaySystem      interfaces.HistoricalReplaySystem
	alertManager      interfaces.AlertManager
	metricsCollector  interfaces.MetricsCollector
	strategyDetectors map[interfaces.StrategyType]StrategyDetector
	config            *ValidationConfig
	mu                sync.RWMutex
}

// StrategyDetector wraps strategy-specific validation logic
type StrategyDetector interface {
	ValidateConfiguration() error
	RunDetectionTests(ctx context.Context, testCases []*ValidationTestCase) (*StrategyTestResults, error)
	ValidateThresholds(ctx context.Context, thresholds *interfaces.ProfitThreshold) (*ThresholdValidationResult, error)
	GetValidationMetrics() *StrategyValidationMetrics
}

// ValidationConfig contains configuration for strategy validation
type ValidationConfig struct {
	// Test execution settings
	MaxConcurrentTests int           `json:"max_concurrent_tests"`
	TestTimeout        time.Duration `json:"test_timeout"`
	RetryAttempts      int           `json:"retry_attempts"`

	// Validation thresholds
	MinAccuracyThreshold   float64 `json:"min_accuracy_threshold"`
	MaxRegressionThreshold float64 `json:"max_regression_threshold"`
	AlertAccuracyThreshold float64 `json:"alert_accuracy_threshold"`

	// Historical data settings
	ValidationPeriods []time.Duration `json:"validation_periods"`
	MinSampleSize     int             `json:"min_sample_size"`

	// CI/CD settings
	EnableNightlyRuns bool   `json:"enable_nightly_runs"`
	NightlySchedule   string `json:"nightly_schedule"`
	AlertOnFailure    bool   `json:"alert_on_failure"`

	// Performance criteria
	MaxExecutionTime time.Duration `json:"max_execution_time"`
	MaxMemoryUsage   int64         `json:"max_memory_usage"`
	MaxCPUUsage      float64       `json:"max_cpu_usage"`
}

// ValidationTestCase represents a test case for strategy validation
type ValidationTestCase struct {
	ID               string                       `json:"id"`
	Strategy         interfaces.StrategyType      `json:"strategy"`
	Transaction      *types.Transaction           `json:"transaction"`
	SimulationResult *interfaces.SimulationResult `json:"simulation_result"`
	ExpectedResult   *ExpectedValidationResult    `json:"expected_result"`
	MarketConditions *interfaces.MarketConditions `json:"market_conditions"`
	Metadata         map[string]interface{}       `json:"metadata"`
}

// ExpectedValidationResult defines what we expect from a validation test
type ExpectedValidationResult struct {
	ShouldDetect       bool          `json:"should_detect"`
	ExpectedProfit     *big.Int      `json:"expected_profit"`
	ExpectedAccuracy   float64       `json:"expected_accuracy"`
	ExpectedConfidence float64       `json:"expected_confidence"`
	TolerancePercent   float64       `json:"tolerance_percent"`
	MaxExecutionTime   time.Duration `json:"max_execution_time"`
}

// StrategyTestResults contains results from strategy validation tests
type StrategyTestResults struct {
	Strategy             interfaces.StrategyType       `json:"strategy"`
	TotalTests           int                           `json:"total_tests"`
	PassedTests          int                           `json:"passed_tests"`
	FailedTests          int                           `json:"failed_tests"`
	AverageAccuracy      float64                       `json:"average_accuracy"`
	AverageExecutionTime time.Duration                 `json:"average_execution_time"`
	RegressionDetected   bool                          `json:"regression_detected"`
	FailedTestCases      []*FailedTestCase             `json:"failed_test_cases"`
	PerformanceMetrics   *ValidationPerformanceMetrics `json:"performance_metrics"`
}

// FailedTestCase represents a test case that failed validation
type FailedTestCase struct {
	TestCaseID     string                    `json:"test_case_id"`
	FailureReason  string                    `json:"failure_reason"`
	ExpectedResult *ExpectedValidationResult `json:"expected_result"`
	ActualResult   *ActualValidationResult   `json:"actual_result"`
	AccuracyDelta  float64                   `json:"accuracy_delta"`
	ExecutionTime  time.Duration             `json:"execution_time"`
}

// ActualValidationResult contains the actual results from validation
type ActualValidationResult struct {
	Detected         bool          `json:"detected"`
	ActualProfit     *big.Int      `json:"actual_profit"`
	ActualAccuracy   float64       `json:"actual_accuracy"`
	ActualConfidence float64       `json:"actual_confidence"`
	ExecutionTime    time.Duration `json:"execution_time"`
	ErrorMessage     string        `json:"error_message"`
}

// ThresholdValidationResult contains results from threshold validation
type ThresholdValidationResult struct {
	Strategy          interfaces.StrategyType     `json:"strategy"`
	OriginalThreshold *interfaces.ProfitThreshold `json:"original_threshold"`
	NewThreshold      *interfaces.ProfitThreshold `json:"new_threshold"`
	ImpactAssessment  *ThresholdImpactAssessment  `json:"impact_assessment"`
	Recommendation    ThresholdRecommendation     `json:"recommendation"`
	ValidationPassed  bool                        `json:"validation_passed"`
}

// ThresholdImpactAssessment analyzes the impact of threshold changes
type ThresholdImpactAssessment struct {
	DetectionRateChange float64 `json:"detection_rate_change"`
	FalsePositiveChange float64 `json:"false_positive_change"`
	FalseNegativeChange float64 `json:"false_negative_change"`
	ProfitabilityImpact float64 `json:"profitability_impact"`
	PerformanceImpact   float64 `json:"performance_impact"`
	RiskScore           float64 `json:"risk_score"`
}

// StrategyValidationMetrics contains validation-specific metrics
type StrategyValidationMetrics struct {
	LastValidationTime     time.Time  `json:"last_validation_time"`
	ValidationCount        int        `json:"validation_count"`
	AverageAccuracy        float64    `json:"average_accuracy"`
	RegressionCount        int        `json:"regression_count"`
	LastRegressionTime     *time.Time `json:"last_regression_time"`
	ConfigurationStability float64    `json:"configuration_stability"`
	ThresholdChanges       int        `json:"threshold_changes"`
}

// ValidationPerformanceMetrics tracks performance during validation
type ValidationPerformanceMetrics struct {
	CPUUsage      float64       `json:"cpu_usage"`
	MemoryUsage   int64         `json:"memory_usage"`
	ExecutionTime time.Duration `json:"execution_time"`
	ThroughputTPS float64       `json:"throughput_tps"`
	ErrorRate     float64       `json:"error_rate"`
}

// ThresholdRecommendation provides recommendations for threshold changes
type ThresholdRecommendation string

const (
	RecommendationApprove ThresholdRecommendation = "approve"
	RecommendationReject  ThresholdRecommendation = "reject"
	RecommendationModify  ThresholdRecommendation = "modify"
	RecommendationMonitor ThresholdRecommendation = "monitor"
)

// NewStrategyValidationFramework creates a new validation framework
func NewStrategyValidationFramework(
	replaySystem interfaces.HistoricalReplaySystem,
	alertManager interfaces.AlertManager,
	metricsCollector interfaces.MetricsCollector,
	config *ValidationConfig,
) *StrategyValidationFramework {
	if config == nil {
		config = &ValidationConfig{
			MaxConcurrentTests:     5,
			TestTimeout:            30 * time.Second,
			RetryAttempts:          3,
			MinAccuracyThreshold:   0.85,
			MaxRegressionThreshold: 0.05,
			AlertAccuracyThreshold: 0.75,
			ValidationPeriods:      []time.Duration{24 * time.Hour, 7 * 24 * time.Hour, 30 * 24 * time.Hour},
			MinSampleSize:          50,
			EnableNightlyRuns:      true,
			NightlySchedule:        "0 2 * * *", // 2 AM daily
			AlertOnFailure:         true,
			MaxExecutionTime:       100 * time.Millisecond,
			MaxMemoryUsage:         500 * 1024 * 1024, // 500MB
			MaxCPUUsage:            80.0,              // 80%
		}
	}

	framework := &StrategyValidationFramework{
		replaySystem:      replaySystem,
		alertManager:      alertManager,
		metricsCollector:  metricsCollector,
		strategyDetectors: make(map[interfaces.StrategyType]StrategyDetector),
		config:            config,
	}

	// Initialize strategy detectors
	framework.initializeStrategyDetectors()

	return framework
}

// initializeStrategyDetectors sets up validation for each strategy type
func (svf *StrategyValidationFramework) initializeStrategyDetectors() {
	// Sandwich strategy validator
	svf.strategyDetectors[interfaces.StrategySandwich] = NewSandwichValidator(
		strategy.NewSandwichDetector(nil),
		svf.config,
	)

	// Backrun strategy validator
	svf.strategyDetectors[interfaces.StrategyBackrun] = NewBackrunValidator(
		strategy.NewBackrunDetector(nil),
		svf.config,
	)

	// Frontrun strategy validator
	svf.strategyDetectors[interfaces.StrategyFrontrun] = NewFrontrunValidator(
		strategy.NewFrontrunDetector(nil),
		svf.config,
	)

	// Time bandit strategy validator
	svf.strategyDetectors[interfaces.StrategyTimeBandit] = NewTimeBanditValidator(
		strategy.NewTimeBanditDetector(nil),
		svf.config,
	)

	// Cross layer strategy validator (using concrete type)
	crossLayerDetector := strategy.NewCrossLayerDetector(nil, nil, nil, common.Address{})
	svf.strategyDetectors[interfaces.StrategyCrossLayer] = NewCrossLayerValidator(
		*crossLayerDetector,
		svf.config,
	)
}

// RunComprehensiveValidation runs validation for all strategies
func (svf *StrategyValidationFramework) RunComprehensiveValidation(ctx context.Context) (*ComprehensiveValidationReport, error) {
	startTime := time.Now()

	report := &ComprehensiveValidationReport{
		ValidationID:    generateValidationID(),
		StartTime:       startTime,
		StrategyResults: make(map[interfaces.StrategyType]*StrategyTestResults),
		OverallMetrics:  &OverallValidationMetrics{},
	}

	// Run validation for each strategy concurrently
	var wg sync.WaitGroup
	resultsChan := make(chan *strategyValidationResult, len(svf.strategyDetectors))

	for strategyType, detector := range svf.strategyDetectors {
		wg.Add(1)
		go func(st interfaces.StrategyType, det StrategyDetector) {
			defer wg.Done()

			// Get test cases for this strategy
			testCases, err := svf.generateTestCases(ctx, st)
			if err != nil {
				resultsChan <- &strategyValidationResult{
					Strategy: st,
					Error:    fmt.Errorf("failed to generate test cases: %w", err),
				}
				return
			}

			// Run validation tests
			results, err := det.RunDetectionTests(ctx, testCases)
			if err != nil {
				resultsChan <- &strategyValidationResult{
					Strategy: st,
					Error:    fmt.Errorf("validation tests failed: %w", err),
				}
				return
			}

			resultsChan <- &strategyValidationResult{
				Strategy: st,
				Results:  results,
			}
		}(strategyType, detector)
	}

	wg.Wait()
	close(resultsChan)

	// Collect results
	var totalTests, totalPassed, totalFailed int
	var totalAccuracy float64
	accuracyCount := 0

	for result := range resultsChan {
		if result.Error != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("Strategy %s: %v", result.Strategy, result.Error))
			continue
		}

		report.StrategyResults[result.Strategy] = result.Results
		totalTests += result.Results.TotalTests
		totalPassed += result.Results.PassedTests
		totalFailed += result.Results.FailedTests

		if result.Results.AverageAccuracy > 0 {
			totalAccuracy += result.Results.AverageAccuracy
			accuracyCount++
		}
	}

	// Calculate overall metrics
	report.OverallMetrics.TotalTests = totalTests
	report.OverallMetrics.TotalPassed = totalPassed
	report.OverallMetrics.TotalFailed = totalFailed

	if accuracyCount > 0 {
		report.OverallMetrics.OverallAccuracy = totalAccuracy / float64(accuracyCount)
	}

	if totalTests > 0 {
		report.OverallMetrics.PassRate = float64(totalPassed) / float64(totalTests)
	}

	report.EndTime = time.Now()
	report.Duration = report.EndTime.Sub(startTime)
	report.OverallSuccess = report.OverallMetrics.PassRate >= svf.config.MinAccuracyThreshold

	// Check for regressions and send alerts if needed
	if err := svf.checkRegressions(ctx, report); err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("Regression check failed: %v", err))
	}

	return report, nil
}

// ValidateThresholdChanges validates proposed threshold changes
func (svf *StrategyValidationFramework) ValidateThresholdChanges(ctx context.Context, proposedChanges map[interfaces.StrategyType]*interfaces.ProfitThreshold) (*ThresholdValidationReport, error) {
	report := &ThresholdValidationReport{
		ValidationID:    generateValidationID(),
		ProposedChanges: proposedChanges,
		Results:         make(map[interfaces.StrategyType]*ThresholdValidationResult),
		StartTime:       time.Now(),
	}

	for strategyType, newThreshold := range proposedChanges {
		detector, exists := svf.strategyDetectors[strategyType]
		if !exists {
			report.Errors = append(report.Errors, fmt.Sprintf("No validator for strategy %s", strategyType))
			continue
		}

		result, err := detector.ValidateThresholds(ctx, newThreshold)
		if err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("Threshold validation failed for %s: %v", strategyType, err))
			continue
		}

		report.Results[strategyType] = result
	}

	report.EndTime = time.Now()
	report.Duration = report.EndTime.Sub(report.StartTime)

	// Determine overall recommendation
	report.OverallRecommendation = svf.determineOverallRecommendation(report.Results)

	return report, nil
}

// RunNightlyValidation executes the nightly validation routine
func (svf *StrategyValidationFramework) RunNightlyValidation(ctx context.Context) error {
	if !svf.config.EnableNightlyRuns {
		return nil
	}

	// Run comprehensive validation
	report, err := svf.RunComprehensiveValidation(ctx)
	if err != nil {
		return fmt.Errorf("nightly validation failed: %w", err)
	}

	// Store results
	if err := svf.storeValidationReport(ctx, report); err != nil {
		return fmt.Errorf("failed to store validation report: %w", err)
	}

	// Send alerts if validation failed
	if svf.config.AlertOnFailure && !report.OverallSuccess {
		if err := svf.sendValidationAlert(ctx, report); err != nil {
			return fmt.Errorf("failed to send validation alert: %w", err)
		}
	}

	return nil
}

// Helper types and methods

type strategyValidationResult struct {
	Strategy interfaces.StrategyType
	Results  *StrategyTestResults
	Error    error
}

type ComprehensiveValidationReport struct {
	ValidationID    string                                           `json:"validation_id"`
	StartTime       time.Time                                        `json:"start_time"`
	EndTime         time.Time                                        `json:"end_time"`
	Duration        time.Duration                                    `json:"duration"`
	StrategyResults map[interfaces.StrategyType]*StrategyTestResults `json:"strategy_results"`
	OverallMetrics  *OverallValidationMetrics                        `json:"overall_metrics"`
	OverallSuccess  bool                                             `json:"overall_success"`
	Errors          []string                                         `json:"errors"`
	Warnings        []string                                         `json:"warnings"`
}

type OverallValidationMetrics struct {
	TotalTests      int     `json:"total_tests"`
	TotalPassed     int     `json:"total_passed"`
	TotalFailed     int     `json:"total_failed"`
	PassRate        float64 `json:"pass_rate"`
	OverallAccuracy float64 `json:"overall_accuracy"`
}

type ThresholdValidationReport struct {
	ValidationID          string                                                  `json:"validation_id"`
	StartTime             time.Time                                               `json:"start_time"`
	EndTime               time.Time                                               `json:"end_time"`
	Duration              time.Duration                                           `json:"duration"`
	ProposedChanges       map[interfaces.StrategyType]*interfaces.ProfitThreshold `json:"proposed_changes"`
	Results               map[interfaces.StrategyType]*ThresholdValidationResult  `json:"results"`
	OverallRecommendation ThresholdRecommendation                                 `json:"overall_recommendation"`
	Errors                []string                                                `json:"errors"`
}

// generateValidationID creates a unique identifier for validation runs
func generateValidationID() string {
	return fmt.Sprintf("validation-%d", time.Now().Unix())
}

// generateTestCases creates test cases for a specific strategy using historical data
func (svf *StrategyValidationFramework) generateTestCases(ctx context.Context, strategy interfaces.StrategyType) ([]*ValidationTestCase, error) {
	// Get historical data for the strategy
	filter := &interfaces.HistoricalLogFilter{
		Strategy:       &strategy,
		OnlyProfitable: true,
		Limit:          svf.config.MinSampleSize * 2, // Get more than minimum to have options
	}

	historicalLogs, err := svf.replaySystem.GetHistoricalLogs(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical logs: %w", err)
	}

	if len(historicalLogs) < svf.config.MinSampleSize {
		return nil, fmt.Errorf("insufficient historical data: got %d, need %d", len(historicalLogs), svf.config.MinSampleSize)
	}

	// Convert historical logs to test cases
	testCases := make([]*ValidationTestCase, 0, len(historicalLogs))
	for i, log := range historicalLogs {
		if log.TargetTransaction == nil {
			continue
		}

		testCase := &ValidationTestCase{
			ID:               fmt.Sprintf("%s-test-%d", strategy, i),
			Strategy:         strategy,
			Transaction:      log.TargetTransaction,
			MarketConditions: log.MarketConditions,
			ExpectedResult: &ExpectedValidationResult{
				ShouldDetect:     true, // These are known profitable opportunities
				ExpectedProfit:   log.ExpectedProfit,
				ExpectedAccuracy: 0.85, // Default expected accuracy
				TolerancePercent: 15.0, // 15% tolerance
				MaxExecutionTime: svf.config.MaxExecutionTime,
			},
			Metadata: log.Metadata,
		}

		// Add simulation results if available
		if len(log.OriginalSimResults) > 0 {
			testCase.SimulationResult = log.OriginalSimResults[0]
		}

		testCases = append(testCases, testCase)
	}

	return testCases, nil
}

// checkRegressions analyzes validation results for performance regressions
func (svf *StrategyValidationFramework) checkRegressions(ctx context.Context, report *ComprehensiveValidationReport) error {
	// Get previous validation results for comparison
	// This would typically query a database of historical validation results
	// For now, we'll implement a simplified version

	for strategy, results := range report.StrategyResults {
		if results.AverageAccuracy < svf.config.AlertAccuracyThreshold {
			results.RegressionDetected = true

			// Create alert for accuracy regression
			alert := &interfaces.Alert{
				ID:       fmt.Sprintf("regression-%s-%d", strategy, time.Now().Unix()),
				Type:     "performance_regression",
				Severity: interfaces.AlertSeverityCritical,
				Message:  fmt.Sprintf("Strategy %s accuracy dropped to %.2f%% (threshold: %.2f%%)", strategy, results.AverageAccuracy*100, svf.config.AlertAccuracyThreshold*100),
				Details: map[string]interface{}{
					"strategy":         strategy,
					"current_accuracy": results.AverageAccuracy,
					"threshold":        svf.config.AlertAccuracyThreshold,
					"failed_tests":     results.FailedTests,
					"total_tests":      results.TotalTests,
				},
				CreatedAt: time.Now(),
			}

			if svf.alertManager != nil {
				if err := svf.alertManager.SendAlert(ctx, alert); err != nil {
					return fmt.Errorf("failed to send regression alert: %w", err)
				}
			}
		}
	}

	return nil
}

// determineOverallRecommendation analyzes threshold validation results to provide recommendation
func (svf *StrategyValidationFramework) determineOverallRecommendation(results map[interfaces.StrategyType]*ThresholdValidationResult) ThresholdRecommendation {
	approveCount := 0
	rejectCount := 0
	modifyCount := 0
	monitorCount := 0

	for _, result := range results {
		switch result.Recommendation {
		case RecommendationApprove:
			approveCount++
		case RecommendationReject:
			rejectCount++
		case RecommendationModify:
			modifyCount++
		case RecommendationMonitor:
			monitorCount++
		}
	}

	total := len(results)
	if total == 0 {
		return RecommendationReject
	}

	// If more than 50% recommend reject, overall recommendation is reject
	if float64(rejectCount)/float64(total) > 0.5 {
		return RecommendationReject
	}

	// If more than 70% recommend approve, overall recommendation is approve
	if float64(approveCount)/float64(total) > 0.7 {
		return RecommendationApprove
	}

	// If significant modifications are needed, recommend modify
	if float64(modifyCount)/float64(total) > 0.3 {
		return RecommendationModify
	}

	// Default to monitor for mixed results
	return RecommendationMonitor
}

// storeValidationReport stores validation results for historical analysis
func (svf *StrategyValidationFramework) storeValidationReport(ctx context.Context, report *ComprehensiveValidationReport) error {
	// This would typically store to database or file system
	// For now, just log the report summary
	fmt.Printf("Validation Report %s: Overall Success: %v, Pass Rate: %.2f%%, Duration: %v\n",
		report.ValidationID, report.OverallSuccess, report.OverallMetrics.PassRate*100, report.Duration)
	return nil
}

// sendValidationAlert sends an alert when validation fails
func (svf *StrategyValidationFramework) sendValidationAlert(ctx context.Context, report *ComprehensiveValidationReport) error {
	alert := &interfaces.Alert{
		ID:       fmt.Sprintf("validation-failure-%s", report.ValidationID),
		Type:     "validation_failure",
		Severity: interfaces.AlertSeverityCritical,
		Message:  fmt.Sprintf("Strategy validation failed: Pass rate %.2f%% below threshold %.2f%%", report.OverallMetrics.PassRate*100, svf.config.MinAccuracyThreshold*100),
		Details: map[string]interface{}{
			"validation_id": report.ValidationID,
			"pass_rate":     report.OverallMetrics.PassRate,
			"threshold":     svf.config.MinAccuracyThreshold,
			"total_tests":   report.OverallMetrics.TotalTests,
			"failed_tests":  report.OverallMetrics.TotalFailed,
			"duration":      report.Duration.String(),
		},
		CreatedAt: time.Now(),
	}

	return svf.alertManager.SendAlert(ctx, alert)
}
