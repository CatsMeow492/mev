package validation

import (
	"context"
	"fmt"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// SandwichValidator implements StrategyDetector for sandwich attack strategies
type SandwichValidator struct {
	detector interfaces.SandwichDetector
	config   *ValidationConfig
	metrics  *StrategyValidationMetrics
}

// NewSandwichValidator creates a new sandwich strategy validator
func NewSandwichValidator(detector interfaces.SandwichDetector, config *ValidationConfig) *SandwichValidator {
	return &SandwichValidator{
		detector: detector,
		config:   config,
		metrics: &StrategyValidationMetrics{
			LastValidationTime:     time.Now(),
			ConfigurationStability: 1.0,
		},
	}
}

// ValidateConfiguration validates the sandwich detector configuration
func (sv *SandwichValidator) ValidateConfiguration() error {
	config := sv.detector.GetConfiguration()

	// Validate minimum swap amount
	if config.MinSwapAmount == nil || config.MinSwapAmount.Sign() <= 0 {
		return fmt.Errorf("minimum swap amount must be positive")
	}

	// Validate maximum slippage
	if config.MaxSlippage <= 0 || config.MaxSlippage > 1.0 {
		return fmt.Errorf("maximum slippage must be between 0 and 1.0")
	}

	// Validate gas premium percentage
	if config.GasPremiumPercent <= 0 || config.GasPremiumPercent > 1.0 {
		return fmt.Errorf("gas premium percentage must be between 0 and 1.0")
	}

	// Validate minimum profit threshold
	if config.MinProfitThreshold == nil || config.MinProfitThreshold.Sign() <= 0 {
		return fmt.Errorf("minimum profit threshold must be positive")
	}

	return nil
}

// RunDetectionTests runs comprehensive detection tests for sandwich strategy
func (sv *SandwichValidator) RunDetectionTests(ctx context.Context, testCases []*ValidationTestCase) (*StrategyTestResults, error) {
	startTime := time.Now()

	results := &StrategyTestResults{
		Strategy:           interfaces.StrategySandwich,
		TotalTests:         len(testCases),
		FailedTestCases:    make([]*FailedTestCase, 0),
		PerformanceMetrics: &ValidationPerformanceMetrics{},
	}

	var totalAccuracy float64
	var totalExecutionTime time.Duration
	accuracyCount := 0

	for _, testCase := range testCases {
		if testCase.Strategy != interfaces.StrategySandwich {
			continue
		}

		// Run individual test case
		testResult, err := sv.runSingleTest(ctx, testCase)
		if err != nil {
			results.FailedTests++
			results.FailedTestCases = append(results.FailedTestCases, &FailedTestCase{
				TestCaseID:     testCase.ID,
				FailureReason:  err.Error(),
				ExpectedResult: testCase.ExpectedResult,
			})
			continue
		}

		// Evaluate test result
		if sv.evaluateTestResult(testCase, testResult) {
			results.PassedTests++
			if testResult.ActualAccuracy > 0 {
				totalAccuracy += testResult.ActualAccuracy
				accuracyCount++
			}
		} else {
			results.FailedTests++

			failedCase := &FailedTestCase{
				TestCaseID:     testCase.ID,
				FailureReason:  "Test result did not meet expected criteria",
				ExpectedResult: testCase.ExpectedResult,
				ActualResult:   testResult,
				ExecutionTime:  testResult.ExecutionTime,
			}

			// Calculate accuracy delta
			if testCase.ExpectedResult.ExpectedAccuracy > 0 {
				failedCase.AccuracyDelta = testResult.ActualAccuracy - testCase.ExpectedResult.ExpectedAccuracy
			}

			results.FailedTestCases = append(results.FailedTestCases, failedCase)
		}

		totalExecutionTime += testResult.ExecutionTime
	}

	// Calculate averages
	if accuracyCount > 0 {
		results.AverageAccuracy = totalAccuracy / float64(accuracyCount)
	}

	if len(testCases) > 0 {
		results.AverageExecutionTime = totalExecutionTime / time.Duration(len(testCases))
	}

	// Check for regression
	results.RegressionDetected = sv.detectRegression(results)

	// Update metrics
	sv.updateValidationMetrics(results)

	results.PerformanceMetrics.ExecutionTime = time.Since(startTime)
	results.PerformanceMetrics.ThroughputTPS = float64(len(testCases)) / time.Since(startTime).Seconds()
	results.PerformanceMetrics.ErrorRate = float64(results.FailedTests) / float64(results.TotalTests)

	return results, nil
}

// ValidateThresholds validates proposed threshold changes for sandwich strategy
func (sv *SandwichValidator) ValidateThresholds(ctx context.Context, newThreshold *interfaces.ProfitThreshold) (*ThresholdValidationResult, error) {
	// Create threshold validation result
	result := &ThresholdValidationResult{
		Strategy:         interfaces.StrategySandwich,
		NewThreshold:     newThreshold,
		ImpactAssessment: &ThresholdImpactAssessment{},
	}

	// Simulate impact of threshold changes
	impactAssessment, err := sv.assessThresholdImpact(ctx, newThreshold)
	if err != nil {
		return nil, fmt.Errorf("failed to assess threshold impact: %w", err)
	}

	result.ImpactAssessment = impactAssessment

	// Determine recommendation based on impact
	result.Recommendation = sv.determineThresholdRecommendation(impactAssessment)
	result.ValidationPassed = result.Recommendation == RecommendationApprove || result.Recommendation == RecommendationMonitor

	return result, nil
}

// GetValidationMetrics returns current validation metrics
func (sv *SandwichValidator) GetValidationMetrics() *StrategyValidationMetrics {
	return sv.metrics
}

// runSingleTest executes a single sandwich detection test
func (sv *SandwichValidator) runSingleTest(ctx context.Context, testCase *ValidationTestCase) (*ActualValidationResult, error) {
	startTime := time.Now()

	result := &ActualValidationResult{
		ExecutionTime: time.Since(startTime),
	}

	// Run sandwich detection
	opportunity, err := sv.detector.DetectOpportunity(ctx, testCase.Transaction, testCase.SimulationResult)
	if err != nil {
		result.ErrorMessage = err.Error()
		return result, fmt.Errorf("detection failed: %w", err)
	}

	result.Detected = opportunity != nil
	result.ExecutionTime = time.Since(startTime)

	if opportunity != nil {
		result.ActualProfit = opportunity.ExpectedProfit

		// Validate the opportunity
		if err := sv.detector.ValidateOpportunity(ctx, opportunity); err != nil {
			result.ErrorMessage = fmt.Sprintf("opportunity validation failed: %v", err)
			result.Detected = false
		} else {
			// Calculate accuracy based on expected vs actual profit
			if testCase.ExpectedResult.ExpectedProfit != nil && testCase.ExpectedResult.ExpectedProfit.Sign() > 0 {
				expectedFloat, _ := testCase.ExpectedResult.ExpectedProfit.Float64()
				actualFloat, _ := result.ActualProfit.Float64()

				if expectedFloat > 0 {
					result.ActualAccuracy = 1.0 - (abs(actualFloat-expectedFloat) / expectedFloat)
					if result.ActualAccuracy < 0 {
						result.ActualAccuracy = 0
					}
				}
			}

			// Set confidence based on opportunity parameters
			result.ActualConfidence = sv.calculateOpportunityConfidence(opportunity)
		}
	}

	return result, nil
}

// evaluateTestResult determines if a test result meets expectations
func (sv *SandwichValidator) evaluateTestResult(testCase *ValidationTestCase, result *ActualValidationResult) bool {
	expected := testCase.ExpectedResult

	// Check detection expectation
	if expected.ShouldDetect && !result.Detected {
		return false
	}

	if !expected.ShouldDetect && result.Detected {
		return false
	}

	// Check execution time
	if result.ExecutionTime > expected.MaxExecutionTime {
		return false
	}

	// If detection was expected, validate accuracy and confidence
	if expected.ShouldDetect && result.Detected {
		// Check accuracy within tolerance
		if expected.ExpectedAccuracy > 0 {
			toleranceRange := expected.ExpectedAccuracy * (expected.TolerancePercent / 100.0)
			minAccuracy := expected.ExpectedAccuracy - toleranceRange
			if result.ActualAccuracy < minAccuracy {
				return false
			}
		}

		// Check confidence
		if expected.ExpectedConfidence > 0 && result.ActualConfidence < expected.ExpectedConfidence {
			return false
		}
	}

	return true
}

// detectRegression checks if current results indicate a regression
func (sv *SandwichValidator) detectRegression(results *StrategyTestResults) bool {
	// Check accuracy regression
	if results.AverageAccuracy < sv.config.AlertAccuracyThreshold {
		return true
	}

	// Check failure rate regression
	failureRate := float64(results.FailedTests) / float64(results.TotalTests)
	if failureRate > sv.config.MaxRegressionThreshold {
		return true
	}

	// Check execution time regression (simplified)
	if results.AverageExecutionTime > sv.config.MaxExecutionTime*2 {
		return true
	}

	return false
}

// updateValidationMetrics updates internal validation metrics
func (sv *SandwichValidator) updateValidationMetrics(results *StrategyTestResults) {
	sv.metrics.LastValidationTime = time.Now()
	sv.metrics.ValidationCount++

	// Update running average of accuracy
	if results.AverageAccuracy > 0 {
		if sv.metrics.AverageAccuracy == 0 {
			sv.metrics.AverageAccuracy = results.AverageAccuracy
		} else {
			// Exponential moving average
			alpha := 0.1
			sv.metrics.AverageAccuracy = alpha*results.AverageAccuracy + (1-alpha)*sv.metrics.AverageAccuracy
		}
	}

	// Track regressions
	if results.RegressionDetected {
		sv.metrics.RegressionCount++
		now := time.Now()
		sv.metrics.LastRegressionTime = &now
	}
}

// assessThresholdImpact simulates the impact of threshold changes
func (sv *SandwichValidator) assessThresholdImpact(ctx context.Context, newThreshold *interfaces.ProfitThreshold) (*ThresholdImpactAssessment, error) {
	assessment := &ThresholdImpactAssessment{}

	// This would typically run historical data through both old and new thresholds
	// For now, we'll provide a simplified assessment based on threshold changes

	currentConfig := sv.detector.GetConfiguration()

	// Assess detection rate changes based on profit threshold changes
	if newThreshold.MinNetProfit != nil && currentConfig.MinProfitThreshold != nil {
		oldFloat, _ := currentConfig.MinProfitThreshold.Float64()
		newFloat, _ := newThreshold.MinNetProfit.Float64()

		if newFloat > oldFloat {
			// Higher threshold = fewer detections but higher quality
			assessment.DetectionRateChange = -0.1 // Estimated 10% reduction
			assessment.FalsePositiveChange = -0.2 // Estimated 20% reduction in false positives
			assessment.ProfitabilityImpact = 0.05 // Estimated 5% improvement in profitability
		} else if newFloat < oldFloat {
			// Lower threshold = more detections but potentially lower quality
			assessment.DetectionRateChange = 0.15  // Estimated 15% increase
			assessment.FalsePositiveChange = 0.1   // Estimated 10% increase in false positives
			assessment.ProfitabilityImpact = -0.03 // Estimated 3% decrease in profitability
		}
	}

	// Assess success probability impact
	if newThreshold.MinSuccessProbability > 0 {
		if newThreshold.MinSuccessProbability > 0.8 {
			assessment.RiskScore = 0.2 // Low risk
		} else if newThreshold.MinSuccessProbability > 0.6 {
			assessment.RiskScore = 0.5 // Medium risk
		} else {
			assessment.RiskScore = 0.8 // High risk
		}
	}

	return assessment, nil
}

// determineThresholdRecommendation provides recommendation based on impact assessment
func (sv *SandwichValidator) determineThresholdRecommendation(assessment *ThresholdImpactAssessment) ThresholdRecommendation {
	// If profitability impact is significantly negative, reject
	if assessment.ProfitabilityImpact < -0.1 {
		return RecommendationReject
	}

	// If risk score is too high, reject
	if assessment.RiskScore > 0.7 {
		return RecommendationReject
	}

	// If false positive rate increases significantly, modify
	if assessment.FalsePositiveChange > 0.2 {
		return RecommendationModify
	}

	// If profitability improves and risk is manageable, approve
	if assessment.ProfitabilityImpact > 0.02 && assessment.RiskScore < 0.5 {
		return RecommendationApprove
	}

	// Default to monitor for small changes
	return RecommendationMonitor
}

// calculateOpportunityConfidence calculates confidence score for detected opportunity
func (sv *SandwichValidator) calculateOpportunityConfidence(opportunity *interfaces.SandwichOpportunity) float64 {
	confidence := 0.5 // Base confidence

	// Increase confidence based on profit margin
	if opportunity.ExpectedProfit != nil && opportunity.ExpectedProfit.Sign() > 0 {
		profitFloat, _ := opportunity.ExpectedProfit.Float64()
		if profitFloat > 1000000 { // > $1000 equivalent
			confidence += 0.2
		} else if profitFloat > 100000 { // > $100 equivalent
			confidence += 0.1
		}
	}

	// Adjust confidence based on slippage tolerance
	if opportunity.SlippageTolerance < 0.01 { // < 1%
		confidence += 0.2
	} else if opportunity.SlippageTolerance < 0.02 { // < 2%
		confidence += 0.1
	} else if opportunity.SlippageTolerance > 0.05 { // > 5%
		confidence -= 0.2
	}

	// Ensure confidence is within bounds
	if confidence > 1.0 {
		confidence = 1.0
	} else if confidence < 0.0 {
		confidence = 0.0
	}

	return confidence
}

// Helper function for absolute value
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
