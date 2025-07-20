package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAlertManager is a mock implementation of AlertManager
type MockAlertManager struct {
	mock.Mock
}

func (m *MockAlertManager) SendAlert(ctx context.Context, alert *interfaces.Alert) error {
	args := m.Called(ctx, alert)
	return args.Error(0)
}

func (m *MockAlertManager) RegisterAlertRule(rule *interfaces.AlertRule) error {
	args := m.Called(rule)
	return args.Error(0)
}

func (m *MockAlertManager) GetActiveAlerts() ([]*interfaces.Alert, error) {
	args := m.Called()
	return args.Get(0).([]*interfaces.Alert), args.Error(1)
}

func (m *MockAlertManager) AcknowledgeAlert(alertID string) error {
	args := m.Called(alertID)
	return args.Error(0)
}

func TestNewShutdownManager(t *testing.T) {
	mockCollector := &MockMetricsCollector{}
	mockAlertManager := &MockAlertManager{}
	
	// Test with nil config
	sm := NewShutdownManager(nil, mockCollector, mockAlertManager)
	assert.NotNil(t, sm)
	assert.NotNil(t, sm.config)
	assert.Equal(t, 0.70, sm.config.WarningLossRate)
	assert.Equal(t, 0.80, sm.config.ShutdownLossRate)
	assert.Equal(t, 100, sm.config.WarningWindowSize)
	assert.Equal(t, 50, sm.config.ShutdownWindowSize)
	
	// Test with custom config
	config := &ShutdownManagerConfig{
		WarningLossRate:    0.60,
		ShutdownLossRate:   0.75,
		WarningWindowSize:  200,
		ShutdownWindowSize: 100,
	}
	sm = NewShutdownManager(config, mockCollector, mockAlertManager)
	assert.Equal(t, 0.60, sm.config.WarningLossRate)
	assert.Equal(t, 0.75, sm.config.ShutdownLossRate)
	assert.Equal(t, 200, sm.config.WarningWindowSize)
	assert.Equal(t, 100, sm.config.ShutdownWindowSize)
}

func TestShutdownManager_CheckShutdownConditions_NoShutdown(t *testing.T) {
	mockCollector := &MockMetricsCollector{}
	mockAlertManager := &MockAlertManager{}
	sm := NewShutdownManager(nil, mockCollector, mockAlertManager)
	
	ctx := context.Background()
	
	// Mock performance metrics with acceptable loss rates
	performanceMetrics := &interfaces.PerformanceMetrics{
		TradeMetrics: map[int]*interfaces.ProfitabilityMetrics{
			100: {
				WindowSize:   100,
				TotalTrades:  100,
				LossRate:     0.60, // Below 70% warning threshold
				SuccessRate:  0.40,
				LossTrades:   60,
			},
			50: {
				WindowSize:   50,
				TotalTrades:  50,
				LossRate:     0.70, // Below 80% shutdown threshold
				SuccessRate:  0.30,
				LossTrades:   35,
			},
		},
	}
	
	mockCollector.On("GetPerformanceMetrics").Return(performanceMetrics, nil)
	
	decision, err := sm.CheckShutdownConditions(ctx)
	assert.NoError(t, err)
	assert.False(t, decision.ShouldShutdown)
	assert.Contains(t, decision.Metrics, "warning_loss_rate")
	assert.Contains(t, decision.Metrics, "shutdown_loss_rate")
	assert.Equal(t, 0.60, decision.Metrics["warning_loss_rate"])
	assert.Equal(t, 0.70, decision.Metrics["shutdown_loss_rate"])
	
	mockCollector.AssertExpectations(t)
}

func TestShutdownManager_CheckShutdownConditions_WarningTriggered(t *testing.T) {
	mockCollector := &MockMetricsCollector{}
	mockAlertManager := &MockAlertManager{}
	sm := NewShutdownManager(nil, mockCollector, mockAlertManager)
	
	ctx := context.Background()
	
	// Mock performance metrics with warning-level loss rate
	performanceMetrics := &interfaces.PerformanceMetrics{
		TradeMetrics: map[int]*interfaces.ProfitabilityMetrics{
			100: {
				WindowSize:   100,
				TotalTrades:  100,
				LossRate:     0.75, // Above 70% warning threshold
				SuccessRate:  0.25,
				LossTrades:   75,
			},
			50: {
				WindowSize:   50,
				TotalTrades:  50,
				LossRate:     0.70, // Below 80% shutdown threshold
				SuccessRate:  0.30,
				LossTrades:   35,
			},
		},
	}
	
	mockCollector.On("GetPerformanceMetrics").Return(performanceMetrics, nil)
	mockAlertManager.On("SendAlert", ctx, mock.AnythingOfType("*interfaces.Alert")).Return(nil)
	
	decision, err := sm.CheckShutdownConditions(ctx)
	assert.NoError(t, err)
	assert.False(t, decision.ShouldShutdown)
	assert.True(t, sm.warningTriggered)
	
	mockCollector.AssertExpectations(t)
	mockAlertManager.AssertExpectations(t)
}

func TestShutdownManager_CheckShutdownConditions_ShutdownTriggered(t *testing.T) {
	mockCollector := &MockMetricsCollector{}
	mockAlertManager := &MockAlertManager{}
	sm := NewShutdownManager(nil, mockCollector, mockAlertManager)
	
	ctx := context.Background()
	
	// Mock performance metrics with shutdown-level loss rate
	performanceMetrics := &interfaces.PerformanceMetrics{
		TradeMetrics: map[int]*interfaces.ProfitabilityMetrics{
			100: {
				WindowSize:   100,
				TotalTrades:  100,
				LossRate:     0.75, // Above 70% warning threshold
				SuccessRate:  0.25,
				LossTrades:   75,
			},
			50: {
				WindowSize:   50,
				TotalTrades:  50,
				LossRate:     0.85, // Above 80% shutdown threshold
				SuccessRate:  0.15,
				LossTrades:   42,
			},
		},
	}
	
	mockCollector.On("GetPerformanceMetrics").Return(performanceMetrics, nil)
	mockAlertManager.On("SendAlert", ctx, mock.AnythingOfType("*interfaces.Alert")).Return(nil)
	
	decision, err := sm.CheckShutdownConditions(ctx)
	assert.NoError(t, err)
	assert.True(t, decision.ShouldShutdown)
	assert.Contains(t, decision.Reason, "85.00%")
	assert.Contains(t, decision.Reason, "80.00%")
	
	mockCollector.AssertExpectations(t)
	mockAlertManager.AssertExpectations(t)
}

func TestShutdownManager_CheckShutdownConditions_InsufficientTrades(t *testing.T) {
	mockCollector := &MockMetricsCollector{}
	mockAlertManager := &MockAlertManager{}
	sm := NewShutdownManager(nil, mockCollector, mockAlertManager)
	
	ctx := context.Background()
	
	// Mock performance metrics with insufficient trades
	performanceMetrics := &interfaces.PerformanceMetrics{
		TradeMetrics: map[int]*interfaces.ProfitabilityMetrics{
			100: {
				WindowSize:   100,
				TotalTrades:  25, // Below minimum of 100
				LossRate:     0.90,
				SuccessRate:  0.10,
				LossTrades:   22,
			},
			50: {
				WindowSize:   50,
				TotalTrades:  25, // Below minimum of 50
				LossRate:     0.90,
				SuccessRate:  0.10,
				LossTrades:   22,
			},
		},
	}
	
	mockCollector.On("GetPerformanceMetrics").Return(performanceMetrics, nil)
	
	decision, err := sm.CheckShutdownConditions(ctx)
	assert.NoError(t, err)
	assert.False(t, decision.ShouldShutdown)
	assert.False(t, sm.warningTriggered)
	
	mockCollector.AssertExpectations(t)
}

func TestShutdownManager_InitiateShutdown(t *testing.T) {
	mockCollector := &MockMetricsCollector{}
	mockAlertManager := &MockAlertManager{}
	sm := NewShutdownManager(nil, mockCollector, mockAlertManager)
	
	ctx := context.Background()
	reason := "Test shutdown"
	
	mockAlertManager.On("SendAlert", ctx, mock.AnythingOfType("*interfaces.Alert")).Return(nil)
	
	// Test successful shutdown
	err := sm.InitiateShutdown(ctx, reason)
	assert.NoError(t, err)
	assert.True(t, sm.status.IsShutdown)
	assert.Equal(t, reason, sm.status.ShutdownReason)
	assert.NotNil(t, sm.status.ShutdownTime)
	assert.True(t, sm.status.CanRestart)
	
	// Test shutdown when already shutdown
	err = sm.InitiateShutdown(ctx, "Another reason")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already shutdown")
	
	mockAlertManager.AssertExpectations(t)
}

func TestShutdownManager_GetShutdownStatus(t *testing.T) {
	mockCollector := &MockMetricsCollector{}
	mockAlertManager := &MockAlertManager{}
	sm := NewShutdownManager(nil, mockCollector, mockAlertManager)
	
	// Test initial status
	status, err := sm.GetShutdownStatus()
	assert.NoError(t, err)
	assert.False(t, status.IsShutdown)
	assert.Empty(t, status.ShutdownReason)
	assert.Nil(t, status.ShutdownTime)
	assert.True(t, status.CanRestart)
	assert.False(t, status.ManualOverride)
	
	// Initiate shutdown
	ctx := context.Background()
	mockAlertManager.On("SendAlert", ctx, mock.AnythingOfType("*interfaces.Alert")).Return(nil)
	sm.InitiateShutdown(ctx, "Test shutdown")
	
	// Test shutdown status
	status, err = sm.GetShutdownStatus()
	assert.NoError(t, err)
	assert.True(t, status.IsShutdown)
	assert.Equal(t, "Test shutdown", status.ShutdownReason)
	assert.NotNil(t, status.ShutdownTime)
	assert.True(t, status.CanRestart)
	
	mockAlertManager.AssertExpectations(t)
}

func TestShutdownManager_SetManualOverride(t *testing.T) {
	config := &ShutdownManagerConfig{
		AllowManualOverride: true,
		OverrideTimeout:     time.Hour,
	}
	
	mockCollector := &MockMetricsCollector{}
	mockAlertManager := &MockAlertManager{}
	sm := NewShutdownManager(config, mockCollector, mockAlertManager)
	
	mockAlertManager.On("SendAlert", mock.Anything, mock.AnythingOfType("*interfaces.Alert")).Return(nil)
	
	// Test enabling manual override
	err := sm.SetManualOverride(true)
	assert.NoError(t, err)
	assert.True(t, sm.status.ManualOverride)
	
	// Test disabling manual override
	err = sm.SetManualOverride(false)
	assert.NoError(t, err)
	assert.False(t, sm.status.ManualOverride)
	
	mockAlertManager.AssertExpectations(t)
}

func TestShutdownManager_SetManualOverride_NotAllowed(t *testing.T) {
	config := &ShutdownManagerConfig{
		AllowManualOverride: false,
	}
	
	mockCollector := &MockMetricsCollector{}
	mockAlertManager := &MockAlertManager{}
	sm := NewShutdownManager(config, mockCollector, mockAlertManager)
	
	err := sm.SetManualOverride(true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "manual override is not allowed")
	assert.False(t, sm.status.ManualOverride)
}

func TestShutdownManager_CheckShutdownConditions_ManualOverride(t *testing.T) {
	mockCollector := &MockMetricsCollector{}
	mockAlertManager := &MockAlertManager{}
	sm := NewShutdownManager(nil, mockCollector, mockAlertManager)
	
	ctx := context.Background()
	
	// Enable manual override
	mockAlertManager.On("SendAlert", mock.Anything, mock.AnythingOfType("*interfaces.Alert")).Return(nil)
	sm.SetManualOverride(true)
	
	decision, err := sm.CheckShutdownConditions(ctx)
	assert.NoError(t, err)
	assert.False(t, decision.ShouldShutdown)
	assert.Equal(t, "Manual override active", decision.Reason)
	
	mockAlertManager.AssertExpectations(t)
}

func TestShutdownManager_CheckShutdownConditions_AlreadyShutdown(t *testing.T) {
	mockCollector := &MockMetricsCollector{}
	mockAlertManager := &MockAlertManager{}
	sm := NewShutdownManager(nil, mockCollector, mockAlertManager)
	
	ctx := context.Background()
	
	// Shutdown the system
	mockAlertManager.On("SendAlert", ctx, mock.AnythingOfType("*interfaces.Alert")).Return(nil)
	sm.InitiateShutdown(ctx, "Test shutdown")
	
	decision, err := sm.CheckShutdownConditions(ctx)
	assert.NoError(t, err)
	assert.False(t, decision.ShouldShutdown)
	assert.Equal(t, "System already shutdown", decision.Reason)
	
	mockAlertManager.AssertExpectations(t)
}

func TestShutdownManager_Restart(t *testing.T) {
	mockCollector := &MockMetricsCollector{}
	mockAlertManager := &MockAlertManager{}
	sm := NewShutdownManager(nil, mockCollector, mockAlertManager)
	
	ctx := context.Background()
	
	// Test restart when not shutdown
	err := sm.Restart(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "system is not shutdown")
	
	// Shutdown the system
	mockAlertManager.On("SendAlert", ctx, mock.AnythingOfType("*interfaces.Alert")).Return(nil).Times(2) // shutdown + restart alerts
	sm.InitiateShutdown(ctx, "Test shutdown")
	
	// Test successful restart
	err = sm.Restart(ctx)
	assert.NoError(t, err)
	assert.False(t, sm.status.IsShutdown)
	assert.Empty(t, sm.status.ShutdownReason)
	assert.Nil(t, sm.status.ShutdownTime)
	assert.False(t, sm.warningTriggered)
	assert.Equal(t, CircuitClosed, sm.circuitState)
	assert.Equal(t, 0, sm.failureCount)
	
	mockAlertManager.AssertExpectations(t)
}

func TestShutdownManager_AddShutdownCallback(t *testing.T) {
	mockCollector := &MockMetricsCollector{}
	mockAlertManager := &MockAlertManager{}
	sm := NewShutdownManager(nil, mockCollector, mockAlertManager)
	
	callbackExecuted := false
	callback := func(ctx context.Context, reason string) error {
		callbackExecuted = true
		assert.Equal(t, "Test shutdown", reason)
		return nil
	}
	
	sm.AddShutdownCallback(callback)
	assert.Len(t, sm.shutdownCallbacks, 1)
	
	// Trigger shutdown to test callback execution
	ctx := context.Background()
	mockAlertManager.On("SendAlert", ctx, mock.AnythingOfType("*interfaces.Alert")).Return(nil)
	sm.InitiateShutdown(ctx, "Test shutdown")
	
	assert.True(t, callbackExecuted)
	mockAlertManager.AssertExpectations(t)
}

func TestShutdownManager_CircuitBreaker(t *testing.T) {
	config := &ShutdownManagerConfig{
		FailureThreshold: 2,
		RecoveryTimeout:  time.Second,
	}
	
	mockCollector := &MockMetricsCollector{}
	mockAlertManager := &MockAlertManager{}
	sm := NewShutdownManager(config, mockCollector, mockAlertManager)
	
	ctx := context.Background()
	
	// Simulate failures to open circuit
	mockCollector.On("GetPerformanceMetrics").Return((*interfaces.PerformanceMetrics)(nil), assert.AnError).Times(2)
	
	// First failure
	_, err := sm.CheckShutdownConditions(ctx)
	assert.Error(t, err)
	assert.Equal(t, CircuitClosed, sm.circuitState)
	assert.Equal(t, 1, sm.failureCount)
	
	// Second failure - should open circuit
	_, err = sm.CheckShutdownConditions(ctx)
	assert.Error(t, err)
	assert.Equal(t, CircuitOpen, sm.circuitState)
	assert.Equal(t, 2, sm.failureCount)
	
	// Third call - should return shutdown decision due to open circuit
	decision, err := sm.CheckShutdownConditions(ctx)
	assert.NoError(t, err)
	assert.True(t, decision.ShouldShutdown)
	assert.Equal(t, "Circuit breaker is open", decision.Reason)
	
	mockCollector.AssertExpectations(t)
}

func TestShutdownManager_CircuitBreaker_Recovery(t *testing.T) {
	config := &ShutdownManagerConfig{
		FailureThreshold: 1,
		RecoveryTimeout:  100 * time.Millisecond, // Short timeout for testing
	}
	
	mockCollector := &MockMetricsCollector{}
	mockAlertManager := &MockAlertManager{}
	sm := NewShutdownManager(config, mockCollector, mockAlertManager)
	
	ctx := context.Background()
	
	// Simulate failure to open circuit
	mockCollector.On("GetPerformanceMetrics").Return((*interfaces.PerformanceMetrics)(nil), assert.AnError).Once()
	
	_, err := sm.CheckShutdownConditions(ctx)
	assert.Error(t, err)
	assert.Equal(t, CircuitOpen, sm.circuitState)
	
	// Wait for recovery timeout
	time.Sleep(150 * time.Millisecond)
	
	// Mock successful response for recovery
	performanceMetrics := &interfaces.PerformanceMetrics{
		TradeMetrics: map[int]*interfaces.ProfitabilityMetrics{
			100: {WindowSize: 100, TotalTrades: 10, LossRate: 0.50},
			50:  {WindowSize: 50, TotalTrades: 10, LossRate: 0.50},
		},
	}
	mockCollector.On("GetPerformanceMetrics").Return(performanceMetrics, nil).Once()
	
	// Should transition to half-open and then closed on success
	decision, err := sm.CheckShutdownConditions(ctx)
	assert.NoError(t, err)
	assert.False(t, decision.ShouldShutdown)
	assert.Equal(t, CircuitClosed, sm.circuitState)
	assert.Equal(t, 0, sm.failureCount)
	
	mockCollector.AssertExpectations(t)
}

func TestShutdownManager_GetCircuitBreakerStatus(t *testing.T) {
	mockCollector := &MockMetricsCollector{}
	mockAlertManager := &MockAlertManager{}
	sm := NewShutdownManager(nil, mockCollector, mockAlertManager)
	
	status := sm.GetCircuitBreakerStatus()
	assert.Contains(t, status, "state")
	assert.Contains(t, status, "failure_count")
	assert.Contains(t, status, "last_failure_time")
	assert.Contains(t, status, "last_check")
	assert.Equal(t, CircuitClosed, status["state"])
	assert.Equal(t, 0, status["failure_count"])
}

func TestShutdownManager_StartStop(t *testing.T) {
	mockCollector := &MockMetricsCollector{}
	mockAlertManager := &MockAlertManager{}
	sm := NewShutdownManager(nil, mockCollector, mockAlertManager)
	
	ctx := context.Background()
	
	// Test starting
	err := sm.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, sm.running)
	
	// Test starting again (should fail)
	err = sm.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
	
	// Test stopping
	err = sm.Stop()
	assert.NoError(t, err)
	assert.False(t, sm.running)
	
	// Test stopping again (should fail)
	err = sm.Stop()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}