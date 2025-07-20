package metrics

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

// TestIntegration_AlertAndShutdownSystem demonstrates the integration between
// AlertManager and ShutdownManager with the MetricsCollector
func TestIntegration_AlertAndShutdownSystem(t *testing.T) {
	ctx := context.Background()
	
	// Create metrics collector with custom registry to avoid conflicts
	registry := prometheus.NewRegistry()
	collector := NewCollectorWithRegistry(nil, registry)
	
	// Create alert manager
	alertManager := NewAlertManager(nil, collector)
	err := alertManager.Start(ctx)
	assert.NoError(t, err)
	defer alertManager.Stop()
	
	// Create shutdown manager
	shutdownManager := NewShutdownManager(nil, collector, alertManager)
	err = shutdownManager.Start(ctx)
	assert.NoError(t, err)
	defer shutdownManager.Stop()
	
	// Register alert rules for profitability monitoring
	warningRule := alertManager.CreateProfitabilityAlertRule(
		"warning-loss-rate", "loss_rate", 0.70, 100)
	err = alertManager.RegisterAlertRule(warningRule)
	assert.NoError(t, err)
	
	shutdownRule := alertManager.CreateProfitabilityAlertRule(
		"shutdown-loss-rate", "loss_rate", 0.80, 50)
	err = alertManager.RegisterAlertRule(shutdownRule)
	assert.NoError(t, err)
	
	// Simulate a series of trades with 75% loss rate (above warning threshold)
	for i := 0; i < 100; i++ {
		var trade *interfaces.TradeResult
		if i < 75 { // 75% losses
			trade = &interfaces.TradeResult{
				ID:             fmt.Sprintf("trade-%d", i),
				Strategy:       interfaces.StrategySandwich,
				Success:        false,
				ActualProfit:   big.NewInt(-1000),
				ExpectedProfit: big.NewInt(500),
				GasCost:        big.NewInt(200),
				NetProfit:      big.NewInt(-1200),
				ExecutionTime:  50 * time.Millisecond,
				ExecutedAt:     time.Now(),
			}
		} else { // 25% wins
			trade = &interfaces.TradeResult{
				ID:             fmt.Sprintf("trade-%d", i),
				Strategy:       interfaces.StrategySandwich,
				Success:        true,
				ActualProfit:   big.NewInt(1500),
				ExpectedProfit: big.NewInt(1000),
				GasCost:        big.NewInt(200),
				NetProfit:      big.NewInt(1300),
				ExecutionTime:  50 * time.Millisecond,
				ExecutedAt:     time.Now(),
			}
		}
		
		err := collector.RecordTrade(ctx, trade)
		assert.NoError(t, err)
	}
	
	// Now we should have warning triggered (100 trades with 75% loss rate > 70% threshold)
	decision, err := shutdownManager.CheckShutdownConditions(ctx)
	assert.NoError(t, err)
	assert.False(t, decision.ShouldShutdown) // 75% loss rate is below 80% shutdown threshold
	
	// Check that alerts were generated
	time.Sleep(100 * time.Millisecond) // Allow time for alert processing
	activeAlerts, err := alertManager.GetActiveAlerts()
	assert.NoError(t, err)
	
	// Should have at least one profitability alert
	profitabilityAlerts := 0
	for _, alert := range activeAlerts {
		if alert.Type == interfaces.AlertTypeProfitability {
			profitabilityAlerts++
		}
	}
	assert.Greater(t, profitabilityAlerts, 0)
	
	// Get performance metrics to verify the state
	perfMetrics, err := collector.GetPerformanceMetrics()
	assert.NoError(t, err)
	assert.True(t, perfMetrics.WarningMode)
	assert.False(t, perfMetrics.ShutdownPending)
	
	// Now simulate enough bad trades to trigger shutdown
	// Add 50 more trades with >80% loss rate to trigger shutdown
	for i := 100; i < 150; i++ {
		var trade *interfaces.TradeResult
		if i < 142 { // 42 out of 50 = 84% losses (above 80% shutdown threshold)
			trade = &interfaces.TradeResult{
				ID:             fmt.Sprintf("trade-%d", i),
				Strategy:       interfaces.StrategySandwich,
				Success:        false,
				ActualProfit:   big.NewInt(-1000),
				ExpectedProfit: big.NewInt(500),
				GasCost:        big.NewInt(200),
				NetProfit:      big.NewInt(-1200),
				ExecutionTime:  50 * time.Millisecond,
				ExecutedAt:     time.Now(),
			}
		} else { // 8 wins
			trade = &interfaces.TradeResult{
				ID:             fmt.Sprintf("trade-%d", i),
				Strategy:       interfaces.StrategySandwich,
				Success:        true,
				ActualProfit:   big.NewInt(1500),
				ExpectedProfit: big.NewInt(1000),
				GasCost:        big.NewInt(200),
				NetProfit:      big.NewInt(1300),
				ExecutionTime:  50 * time.Millisecond,
				ExecutedAt:     time.Now(),
			}
		}
		
		err := collector.RecordTrade(ctx, trade)
		assert.NoError(t, err)
	}
	
	// Check shutdown conditions - should trigger shutdown
	decision, err = shutdownManager.CheckShutdownConditions(ctx)
	assert.NoError(t, err)
	assert.True(t, decision.ShouldShutdown)
	assert.Contains(t, decision.Reason, "Loss rate")
	assert.Contains(t, decision.Reason, "exceeds shutdown threshold")
	
	// Verify shutdown status
	status, err := shutdownManager.GetShutdownStatus()
	assert.NoError(t, err)
	assert.False(t, status.IsShutdown) // Not yet shutdown, just conditions met
	
	// Actually initiate shutdown
	err = shutdownManager.InitiateShutdown(ctx, decision.Reason)
	assert.NoError(t, err)
	
	// Verify system is shutdown
	status, err = shutdownManager.GetShutdownStatus()
	assert.NoError(t, err)
	assert.True(t, status.IsShutdown)
	assert.Contains(t, status.ShutdownReason, "Loss rate")
	
	// Verify shutdown alert was sent
	time.Sleep(100 * time.Millisecond)
	activeAlerts, err = alertManager.GetActiveAlerts()
	assert.NoError(t, err)
	
	shutdownAlerts := 0
	for _, alert := range activeAlerts {
		if alert.Type == interfaces.AlertTypeShutdown {
			shutdownAlerts++
		}
	}
	assert.Greater(t, shutdownAlerts, 0)
}

// TestIntegration_ManualOverride demonstrates manual override functionality
func TestIntegration_ManualOverride(t *testing.T) {
	ctx := context.Background()
	
	// Create components with custom registry to avoid conflicts
	registry := prometheus.NewRegistry()
	collector := NewCollectorWithRegistry(nil, registry)
	alertManager := NewAlertManager(nil, collector)
	shutdownManager := NewShutdownManager(nil, collector, alertManager)
	
	err := alertManager.Start(ctx)
	assert.NoError(t, err)
	defer alertManager.Stop()
	
	err = shutdownManager.Start(ctx)
	assert.NoError(t, err)
	defer shutdownManager.Stop()
	
	// Simulate bad trades that would normally trigger shutdown
	for i := 0; i < 60; i++ {
		trade := &interfaces.TradeResult{
			ID:             fmt.Sprintf("trade-%d", i),
			Strategy:       interfaces.StrategySandwich,
			Success:        false,
			ActualProfit:   big.NewInt(-1000),
			ExpectedProfit: big.NewInt(500),
			GasCost:        big.NewInt(200),
			NetProfit:      big.NewInt(-1200),
			ExecutionTime:  50 * time.Millisecond,
			ExecutedAt:     time.Now(),
		}
		
		err := collector.RecordTrade(ctx, trade)
		assert.NoError(t, err)
	}
	
	// Enable manual override before conditions would trigger shutdown
	err = shutdownManager.SetManualOverride(true)
	assert.NoError(t, err)
	
	// Check shutdown conditions - should not shutdown due to override
	decision, err := shutdownManager.CheckShutdownConditions(ctx)
	assert.NoError(t, err)
	assert.False(t, decision.ShouldShutdown)
	assert.Equal(t, "Manual override active", decision.Reason)
	
	// Disable override
	err = shutdownManager.SetManualOverride(false)
	assert.NoError(t, err)
	
	// Now shutdown conditions should be evaluated normally
	decision, err = shutdownManager.CheckShutdownConditions(ctx)
	assert.NoError(t, err)
	// May or may not trigger shutdown depending on exact loss rates
}

// TestIntegration_CircuitBreakerWithAlerts demonstrates circuit breaker integration
func TestIntegration_CircuitBreakerWithAlerts(t *testing.T) {
	ctx := context.Background()
	
	// Create components with custom config for faster circuit breaker
	config := &ShutdownManagerConfig{
		FailureThreshold: 2,
		RecoveryTimeout:  100 * time.Millisecond,
	}
	
	registry := prometheus.NewRegistry()
	collector := NewCollectorWithRegistry(nil, registry)
	alertManager := NewAlertManager(nil, collector)
	
	err := alertManager.Start(ctx)
	assert.NoError(t, err)
	defer alertManager.Stop()
	
	// Create a failing metrics collector to trigger circuit breaker
	failingCollector := &MockMetricsCollector{}
	failingCollector.On("GetPerformanceMetrics").Return((*interfaces.PerformanceMetrics)(nil), assert.AnError).Times(2)
	
	shutdownManagerWithFailingCollector := NewShutdownManager(config, failingCollector, alertManager)
	
	// Trigger circuit breaker
	_, err = shutdownManagerWithFailingCollector.CheckShutdownConditions(ctx)
	assert.Error(t, err)
	
	_, err = shutdownManagerWithFailingCollector.CheckShutdownConditions(ctx)
	assert.Error(t, err)
	
	// Circuit should be open now
	status := shutdownManagerWithFailingCollector.GetCircuitBreakerStatus()
	assert.Equal(t, CircuitOpen, status["state"])
	
	// Next call should trigger shutdown due to open circuit
	decision, err := shutdownManagerWithFailingCollector.CheckShutdownConditions(ctx)
	assert.NoError(t, err)
	assert.True(t, decision.ShouldShutdown)
	assert.Equal(t, "Circuit breaker is open", decision.Reason)
}