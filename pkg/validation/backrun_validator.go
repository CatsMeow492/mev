package validation

import (
	"context"
	"fmt"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// BackrunValidator implements StrategyDetector for backrun arbitrage strategies
type BackrunValidator struct {
	detector interfaces.BackrunDetector
	config   *ValidationConfig
	metrics  *StrategyValidationMetrics
}

// NewBackrunValidator creates a new backrun strategy validator
func NewBackrunValidator(detector interfaces.BackrunDetector, config *ValidationConfig) *BackrunValidator {
	return &BackrunValidator{
		detector: detector,
		config:   config,
		metrics: &StrategyValidationMetrics{
			LastValidationTime:     time.Now(),
			ConfigurationStability: 1.0,
		},
	}
}

// ValidateConfiguration validates the backrun detector configuration
func (bv *BackrunValidator) ValidateConfiguration() error {
	config := bv.detector.GetConfiguration()

	// Validate minimum price gap
	if config.MinPriceGap == nil || config.MinPriceGap.Sign() <= 0 {
		return fmt.Errorf("minimum price gap must be positive")
	}

	// Validate maximum trade size
	if config.MaxTradeSize == nil || config.MaxTradeSize.Sign() <= 0 {
		return fmt.Errorf("maximum trade size must be positive")
	}

	// Validate minimum profit threshold
	if config.MinProfitThreshold == nil || config.MinProfitThreshold.Sign() <= 0 {
		return fmt.Errorf("minimum profit threshold must be positive")
	}

	// Validate supported pools
	if len(config.SupportedPools) == 0 {
		return fmt.Errorf("at least one supported pool must be configured")
	}

	return nil
}

// RunDetectionTests runs comprehensive detection tests for backrun strategy
func (bv *BackrunValidator) RunDetectionTests(ctx context.Context, testCases []*ValidationTestCase) (*StrategyTestResults, error) {
	startTime := time.Now()

	results := &StrategyTestResults{
		Strategy:           interfaces.StrategyBackrun,
		TotalTests:         len(testCases),
		FailedTestCases:    make([]*FailedTestCase, 0),
		PerformanceMetrics: &ValidationPerformanceMetrics{},
	}

	var totalAccuracy float64
	var totalExecutionTime time.Duration
	accuracyCount := 0

	for _, testCase := range testCases {
		if testCase.Strategy != interfaces.StrategyBackrun {
			continue
		}

		// Run individual test case
		testResult, err := bv.runSingleTest(ctx, testCase)
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
		if bv.evaluateTestResult(testCase, testResult) {
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
	results.RegressionDetected = bv.detectRegression(results)

	// Update metrics
	bv.updateValidationMetrics(results)

	results.PerformanceMetrics.ExecutionTime = time.Since(startTime)
	results.PerformanceMetrics.ThroughputTPS = float64(len(testCases)) / time.Since(startTime).Seconds()
	results.PerformanceMetrics.ErrorRate = float64(results.FailedTests) / float64(results.TotalTests)

	return results, nil
}

// ValidateThresholds validates proposed threshold changes for backrun strategy
func (bv *BackrunValidator) ValidateThresholds(ctx context.Context, newThreshold *interfaces.ProfitThreshold) (*ThresholdValidationResult, error) {
	// Create threshold validation result
	result := &ThresholdValidationResult{
		Strategy:         interfaces.StrategyBackrun,
		NewThreshold:     newThreshold,
		ImpactAssessment: &ThresholdImpactAssessment{},
	}

	// Simulate impact of threshold changes
	impactAssessment, err := bv.assessThresholdImpact(ctx, newThreshold)
	if err != nil {
		return nil, fmt.Errorf("failed to assess threshold impact: %w", err)
	}

	result.ImpactAssessment = impactAssessment

	// Determine recommendation based on impact
	result.Recommendation = bv.determineThresholdRecommendation(impactAssessment)
	result.ValidationPassed = result.Recommendation == RecommendationApprove || result.Recommendation == RecommendationMonitor

	return result, nil
}

// GetValidationMetrics returns current validation metrics
func (bv *BackrunValidator) GetValidationMetrics() *StrategyValidationMetrics {
	return bv.metrics
}

// runSingleTest executes a single backrun detection test
func (bv *BackrunValidator) runSingleTest(ctx context.Context, testCase *ValidationTestCase) (*ActualValidationResult, error) {
	startTime := time.Now()

	result := &ActualValidationResult{
		ExecutionTime: time.Since(startTime),
	}

	// Run backrun detection
	opportunity, err := bv.detector.DetectOpportunity(ctx, testCase.Transaction, testCase.SimulationResult)
	if err != nil {
		result.ErrorMessage = err.Error()
		return result, fmt.Errorf("detection failed: %w", err)
	}

	result.Detected = opportunity != nil
	result.ExecutionTime = time.Since(startTime)

	if opportunity != nil {
		result.ActualProfit = opportunity.ExpectedProfit

		// Validate the opportunity
		if err := bv.detector.ValidateArbitrage(ctx, opportunity); err != nil {
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
			result.ActualConfidence = bv.calculateOpportunityConfidence(opportunity)
		}
	}

	return result, nil
}

// evaluateTestResult determines if a test result meets expectations
func (bv *BackrunValidator) evaluateTestResult(testCase *ValidationTestCase, result *ActualValidationResult) bool {
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
func (bv *BackrunValidator) detectRegression(results *StrategyTestResults) bool {
	// Check accuracy regression
	if results.AverageAccuracy < bv.config.AlertAccuracyThreshold {
		return true
	}

	// Check failure rate regression
	failureRate := float64(results.FailedTests) / float64(results.TotalTests)
	if failureRate > bv.config.MaxRegressionThreshold {
		return true
	}

	// Check execution time regression
	if results.AverageExecutionTime > bv.config.MaxExecutionTime*2 {
		return true
	}

	return false
}

// updateValidationMetrics updates internal validation metrics
func (bv *BackrunValidator) updateValidationMetrics(results *StrategyTestResults) {
	bv.metrics.LastValidationTime = time.Now()
	bv.metrics.ValidationCount++

	// Update running average of accuracy
	if results.AverageAccuracy > 0 {
		if bv.metrics.AverageAccuracy == 0 {
			bv.metrics.AverageAccuracy = results.AverageAccuracy
		} else {
			// Exponential moving average
			alpha := 0.1
			bv.metrics.AverageAccuracy = alpha*results.AverageAccuracy + (1-alpha)*bv.metrics.AverageAccuracy
		}
	}

	// Track regressions
	if results.RegressionDetected {
		bv.metrics.RegressionCount++
		now := time.Now()
		bv.metrics.LastRegressionTime = &now
	}
}

// assessThresholdImpact simulates the impact of threshold changes
func (bv *BackrunValidator) assessThresholdImpact(ctx context.Context, newThreshold *interfaces.ProfitThreshold) (*ThresholdImpactAssessment, error) {
	assessment := &ThresholdImpactAssessment{}

	currentConfig := bv.detector.GetConfiguration()

	// Assess detection rate changes based on profit threshold changes
	if newThreshold.MinNetProfit != nil && currentConfig.MinProfitThreshold != nil {
		oldFloat, _ := currentConfig.MinProfitThreshold.Float64()
		newFloat, _ := newThreshold.MinNetProfit.Float64()

		if newFloat > oldFloat {
			// Higher threshold = fewer detections but higher quality
			assessment.DetectionRateChange = -0.08 // Estimated 8% reduction
			assessment.FalsePositiveChange = -0.15 // Estimated 15% reduction in false positives
			assessment.ProfitabilityImpact = 0.06  // Estimated 6% improvement in profitability
		} else if newFloat < oldFloat {
			// Lower threshold = more detections but potentially lower quality
			assessment.DetectionRateChange = 0.12  // Estimated 12% increase
			assessment.FalsePositiveChange = 0.08  // Estimated 8% increase in false positives
			assessment.ProfitabilityImpact = -0.04 // Estimated 4% decrease in profitability
		}
	}

	// Note: Price gap thresholds are strategy-specific and handled separately

	// Calculate overall risk score
	assessment.RiskScore = bv.calculateRiskScore(assessment)

	return assessment, nil
}

// determineThresholdRecommendation provides recommendation based on impact assessment
func (bv *BackrunValidator) determineThresholdRecommendation(assessment *ThresholdImpactAssessment) ThresholdRecommendation {
	// If profitability impact is significantly negative, reject
	if assessment.ProfitabilityImpact < -0.08 {
		return RecommendationReject
	}

	// If risk score is too high, reject
	if assessment.RiskScore > 0.7 {
		return RecommendationReject
	}

	// If false positive rate increases significantly, modify
	if assessment.FalsePositiveChange > 0.15 {
		return RecommendationModify
	}

	// If profitability improves and risk is manageable, approve
	if assessment.ProfitabilityImpact > 0.03 && assessment.RiskScore < 0.4 {
		return RecommendationApprove
	}

	// Default to monitor for small changes
	return RecommendationMonitor
}

// calculateOpportunityConfidence calculates confidence score for detected opportunity
func (bv *BackrunValidator) calculateOpportunityConfidence(opportunity *interfaces.BackrunOpportunity) float64 {
	confidence := 0.6 // Base confidence for backrun (higher than sandwich due to clearer arbitrage)

	// Increase confidence based on profit margin
	if opportunity.ExpectedProfit != nil && opportunity.ExpectedProfit.Sign() > 0 {
		profitFloat, _ := opportunity.ExpectedProfit.Float64()
		if profitFloat > 500000 { // > $500 equivalent
			confidence += 0.2
		} else if profitFloat > 100000 { // > $100 equivalent
			confidence += 0.1
		}
	}

	// Adjust confidence based on price gap
	if opportunity.PriceGap != nil && opportunity.PriceGap.Sign() > 0 {
		priceGapFloat, _ := opportunity.PriceGap.Float64()
		if priceGapFloat > 100 { // > 1% in basis points
			confidence += 0.15
		} else if priceGapFloat > 50 { // > 0.5% in basis points
			confidence += 0.05
		}
	}

	// Ensure confidence is within bounds
	if confidence > 1.0 {
		confidence = 1.0
	} else if confidence < 0.0 {
		confidence = 0.0
	}

	return confidence
}

// calculateRiskScore calculates overall risk score for threshold changes
func (bv *BackrunValidator) calculateRiskScore(assessment *ThresholdImpactAssessment) float64 {
	riskScore := 0.3 // Base risk

	// Increase risk if detection rate changes significantly
	if abs(assessment.DetectionRateChange) > 0.2 {
		riskScore += 0.2
	}

	// Increase risk if false positive rate increases
	if assessment.FalsePositiveChange > 0.1 {
		riskScore += 0.3
	}

	// Decrease risk if profitability improves
	if assessment.ProfitabilityImpact > 0.05 {
		riskScore -= 0.1
	}

	// Ensure within bounds
	if riskScore > 1.0 {
		riskScore = 1.0
	} else if riskScore < 0.0 {
		riskScore = 0.0
	}

	return riskScore
}
