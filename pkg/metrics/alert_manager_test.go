package metrics

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockMetricsCollector is a mock implementation of MetricsCollector
type MockMetricsCollector struct {
	mock.Mock
}

func (m *MockMetricsCollector) RecordTrade(ctx context.Context, trade *interfaces.TradeResult) error {
	args := m.Called(ctx, trade)
	return args.Error(0)
}

func (m *MockMetricsCollector) RecordLatency(ctx context.Context, operation string, duration time.Duration) error {
	args := m.Called(ctx, operation, duration)
	return args.Error(0)
}

func (m *MockMetricsCollector) RecordOpportunity(ctx context.Context, opportunity *interfaces.MEVOpportunity) error {
	args := m.Called(ctx, opportunity)
	return args.Error(0)
}

func (m *MockMetricsCollector) GetTradeSuccessRate(windowSize int) (float64, error) {
	args := m.Called(windowSize)
	return args.Get(0).(float64), args.Error(1)
}

func (m *MockMetricsCollector) GetProfitabilityMetrics(windowSize int) (*interfaces.ProfitabilityMetrics, error) {
	args := m.Called(windowSize)
	return args.Get(0).(*interfaces.ProfitabilityMetrics), args.Error(1)
}

func (m *MockMetricsCollector) GetLatencyMetrics(operation string, windowSize int) (*interfaces.LatencyMetrics, error) {
	args := m.Called(operation, windowSize)
	return args.Get(0).(*interfaces.LatencyMetrics), args.Error(1)
}

func (m *MockMetricsCollector) GetPerformanceMetrics() (*interfaces.PerformanceMetrics, error) {
	args := m.Called()
	return args.Get(0).(*interfaces.PerformanceMetrics), args.Error(1)
}

func (m *MockMetricsCollector) GetSystemMetrics() (*interfaces.SystemMetrics, error) {
	args := m.Called()
	return args.Get(0).(*interfaces.SystemMetrics), args.Error(1)
}

func (m *MockMetricsCollector) GetPrometheusMetrics() (map[string]interface{}, error) {
	args := m.Called()
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockMetricsCollector) RegisterPrometheusCollectors() error {
	args := m.Called()
	return args.Error(0)
}

func TestNewAlertManager(t *testing.T) {
	mockCollector := &MockMetricsCollector{}
	
	// Test with nil config
	am := NewAlertManager(nil, mockCollector)
	assert.NotNil(t, am)
	assert.NotNil(t, am.config)
	assert.Equal(t, 1000, am.config.MaxAlerts)
	assert.Equal(t, 24*time.Hour, am.config.AlertRetention)
	
	// Test with custom config
	config := &AlertManagerConfig{
		MaxAlerts:      500,
		AlertRetention: 12 * time.Hour,
		EnableLogging:  false,
	}
	am = NewAlertManager(config, mockCollector)
	assert.Equal(t, 500, am.config.MaxAlerts)
	assert.Equal(t, 12*time.Hour, am.config.AlertRetention)
	assert.False(t, am.config.EnableLogging)
}

func TestAlertManager_SendAlert(t *testing.T) {
	mockCollector := &MockMetricsCollector{}
	am := NewAlertManager(nil, mockCollector)
	
	ctx := context.Background()
	
	// Test sending nil alert
	err := am.SendAlert(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "alert cannot be nil")
	
	// Test sending valid alert
	alert := &interfaces.Alert{
		Type:     interfaces.AlertTypeProfitability,
		Severity: interfaces.AlertSeverityWarning,
		Message:  "Test alert",
	}
	
	err = am.SendAlert(ctx, alert)
	assert.NoError(t, err)
	assert.NotEmpty(t, alert.ID)
	assert.False(t, alert.CreatedAt.IsZero())
	
	// Verify alert is stored
	am.mu.RLock()
	storedAlert, exists := am.alerts[alert.ID]
	am.mu.RUnlock()
	assert.True(t, exists)
	assert.Equal(t, alert.Message, storedAlert.Message)
}

func TestAlertManager_RegisterAlertRule(t *testing.T) {
	mockCollector := &MockMetricsCollector{}
	am := NewAlertManager(nil, mockCollector)
	
	// Test registering nil rule
	err := am.RegisterAlertRule(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "alert rule cannot be nil")
	
	// Test registering rule without ID
	rule := &interfaces.AlertRule{
		Name: "Test Rule",
		Type: interfaces.AlertTypeProfitability,
	}
	err = am.RegisterAlertRule(rule)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "alert rule ID cannot be empty")
	
	// Test registering valid rule
	rule.ID = "test-rule-1"
	err = am.RegisterAlertRule(rule)
	assert.NoError(t, err)
	assert.False(t, rule.CreatedAt.IsZero())
	
	// Verify rule is stored
	am.mu.RLock()
	storedRule, exists := am.alertRules[rule.ID]
	am.mu.RUnlock()
	assert.True(t, exists)
	assert.Equal(t, rule.Name, storedRule.Name)
}

func TestAlertManager_GetActiveAlerts(t *testing.T) {
	mockCollector := &MockMetricsCollector{}
	am := NewAlertManager(nil, mockCollector)
	
	ctx := context.Background()
	
	// Add some alerts
	alert1 := &interfaces.Alert{
		ID:       "alert-1",
		Type:     interfaces.AlertTypeProfitability,
		Severity: interfaces.AlertSeverityWarning,
		Message:  "Alert 1",
	}
	
	alert2 := &interfaces.Alert{
		ID:       "alert-2",
		Type:     interfaces.AlertTypeLatency,
		Severity: interfaces.AlertSeverityError,
		Message:  "Alert 2",
	}
	
	// Resolve alert2
	now := time.Now()
	alert2.ResolvedAt = &now
	
	am.SendAlert(ctx, alert1)
	am.SendAlert(ctx, alert2)
	
	// Get active alerts
	activeAlerts, err := am.GetActiveAlerts()
	assert.NoError(t, err)
	assert.Len(t, activeAlerts, 1)
	assert.Equal(t, "alert-1", activeAlerts[0].ID)
}

func TestAlertManager_AcknowledgeAlert(t *testing.T) {
	mockCollector := &MockMetricsCollector{}
	am := NewAlertManager(nil, mockCollector)
	
	ctx := context.Background()
	
	// Test acknowledging non-existent alert
	err := am.AcknowledgeAlert("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "alert with ID non-existent not found")
	
	// Add an alert
	alert := &interfaces.Alert{
		ID:       "test-alert",
		Type:     interfaces.AlertTypeProfitability,
		Severity: interfaces.AlertSeverityWarning,
		Message:  "Test alert",
	}
	am.SendAlert(ctx, alert)
	
	// Acknowledge the alert
	err = am.AcknowledgeAlert("test-alert")
	assert.NoError(t, err)
	
	// Verify acknowledgment
	am.mu.RLock()
	storedAlert := am.alerts["test-alert"]
	am.mu.RUnlock()
	assert.NotNil(t, storedAlert.AcknowledgedAt)
}

func TestAlertManager_EvaluateProfitabilityRule(t *testing.T) {
	mockCollector := &MockMetricsCollector{}
	am := NewAlertManager(nil, mockCollector)
	
	ctx := context.Background()
	
	// Create a loss rate rule
	rule := &interfaces.AlertRule{
		ID:         "loss-rate-rule",
		Type:       interfaces.AlertTypeProfitability,
		Condition:  "loss_rate",
		Threshold:  0.70, // 70%
		WindowSize: 100,
		Enabled:    true,
	}
	
	// Mock metrics that exceed threshold
	mockMetrics := &interfaces.ProfitabilityMetrics{
		WindowSize:   100,
		TotalTrades:  100,
		LossRate:     0.75, // 75% - exceeds 70% threshold
		SuccessRate:  0.25,
		LossTrades:   75,
	}
	
	mockCollector.On("GetProfitabilityMetrics", 100).Return(mockMetrics, nil)
	
	// Start alert manager to process alerts
	am.Start(ctx)
	defer am.Stop()
	
	// Evaluate the rule
	am.evaluateProfitabilityRule(ctx, rule)
	
	// Give some time for alert processing
	time.Sleep(100 * time.Millisecond)
	
	// Check that an alert was generated
	activeAlerts, err := am.GetActiveAlerts()
	assert.NoError(t, err)
	assert.Len(t, activeAlerts, 1)
	assert.Equal(t, interfaces.AlertTypeProfitability, activeAlerts[0].Type)
	assert.Contains(t, activeAlerts[0].Message, "75.00%")
	assert.Contains(t, activeAlerts[0].Message, "70.00%")
	
	mockCollector.AssertExpectations(t)
}

func TestAlertManager_CreateProfitabilityAlertRule(t *testing.T) {
	mockCollector := &MockMetricsCollector{}
	am := NewAlertManager(nil, mockCollector)
	
	rule := am.CreateProfitabilityAlertRule("test-rule", "loss_rate", 0.80, 50)
	
	assert.Equal(t, "test-rule", rule.ID)
	assert.Equal(t, "Profitability Alert - loss_rate", rule.Name)
	assert.Equal(t, interfaces.AlertTypeProfitability, rule.Type)
	assert.Equal(t, "loss_rate", rule.Condition)
	assert.Equal(t, 0.80, rule.Threshold)
	assert.Equal(t, 50, rule.WindowSize)
	assert.True(t, rule.Enabled)
	assert.False(t, rule.CreatedAt.IsZero())
}

func TestAlertManager_GetSeverityForThreshold(t *testing.T) {
	mockCollector := &MockMetricsCollector{}
	am := NewAlertManager(nil, mockCollector)
	
	tests := []struct {
		currentValue float64
		threshold    float64
		expected     interfaces.AlertSeverity
	}{
		{0.80, 0.70, interfaces.AlertSeverityInfo},    // 1.14 ratio
		{0.85, 0.70, interfaces.AlertSeverityWarning}, // 1.21 ratio
		{1.05, 0.70, interfaces.AlertSeverityError},   // 1.5 ratio
		{1.40, 0.70, interfaces.AlertSeverityCritical}, // 2.0 ratio
	}
	
	for _, test := range tests {
		severity := am.getSeverityForThreshold(test.currentValue, test.threshold)
		assert.Equal(t, test.expected, severity, 
			"Expected %v for currentValue=%f, threshold=%f", 
			test.expected, test.currentValue, test.threshold)
	}
}

func TestAlertManager_CleanupOldAlerts(t *testing.T) {
	config := &AlertManagerConfig{
		MaxAlerts:      1000,
		AlertRetention: time.Hour,
		EnableLogging:  false,
	}
	
	mockCollector := &MockMetricsCollector{}
	am := NewAlertManager(config, mockCollector)
	
	ctx := context.Background()
	
	// Add old alert
	oldAlert := &interfaces.Alert{
		ID:        "old-alert",
		Type:      interfaces.AlertTypeProfitability,
		Severity:  interfaces.AlertSeverityWarning,
		Message:   "Old alert",
		CreatedAt: time.Now().Add(-2 * time.Hour), // 2 hours ago
	}
	
	// Add recent alert
	recentAlert := &interfaces.Alert{
		ID:        "recent-alert",
		Type:      interfaces.AlertTypeProfitability,
		Severity:  interfaces.AlertSeverityWarning,
		Message:   "Recent alert",
		CreatedAt: time.Now().Add(-30 * time.Minute), // 30 minutes ago
	}
	
	am.SendAlert(ctx, oldAlert)
	am.SendAlert(ctx, recentAlert)
	
	// Verify both alerts exist
	assert.Len(t, am.alerts, 2)
	
	// Run cleanup
	am.cleanupOldAlerts()
	
	// Verify only recent alert remains
	assert.Len(t, am.alerts, 1)
	_, exists := am.alerts["recent-alert"]
	assert.True(t, exists)
	_, exists = am.alerts["old-alert"]
	assert.False(t, exists)
}

func TestAlertManager_MaxAlertsLimit(t *testing.T) {
	config := &AlertManagerConfig{
		MaxAlerts:     2, // Very small limit for testing
		EnableLogging: false,
	}
	
	mockCollector := &MockMetricsCollector{}
	am := NewAlertManager(config, mockCollector)
	
	ctx := context.Background()
	
	// Add alerts beyond the limit
	for i := 0; i < 5; i++ {
		alert := &interfaces.Alert{
			Type:     interfaces.AlertTypeProfitability,
			Severity: interfaces.AlertSeverityWarning,
			Message:  fmt.Sprintf("Alert %d", i),
		}
		am.SendAlert(ctx, alert)
		time.Sleep(time.Millisecond) // Ensure different timestamps
	}
	
	// Should only have 2 alerts (the limit)
	assert.Len(t, am.alerts, 2)
}

func TestAlertManager_StartStop(t *testing.T) {
	mockCollector := &MockMetricsCollector{}
	am := NewAlertManager(nil, mockCollector)
	
	ctx := context.Background()
	
	// Test starting
	err := am.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, am.running)
	
	// Test starting again (should fail)
	err = am.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
	
	// Test stopping
	err = am.Stop()
	assert.NoError(t, err)
	assert.False(t, am.running)
	
	// Test stopping again (should fail)
	err = am.Stop()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}