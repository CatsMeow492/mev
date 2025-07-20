package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/internal/config"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock implementations for testing
type MockStrategyEngine struct {
	mock.Mock
}

func (m *MockStrategyEngine) AnalyzeTransaction(ctx context.Context, tx *types.Transaction, simResult *interfaces.SimulationResult) ([]*interfaces.MEVOpportunity, error) {
	args := m.Called(ctx, tx, simResult)
	return args.Get(0).([]*interfaces.MEVOpportunity), args.Error(1)
}

func (m *MockStrategyEngine) GetActiveStrategies() []interfaces.StrategyType {
	args := m.Called()
	return args.Get(0).([]interfaces.StrategyType)
}

func (m *MockStrategyEngine) EnableStrategy(strategy interfaces.StrategyType) error {
	args := m.Called(strategy)
	return args.Error(0)
}

func (m *MockStrategyEngine) DisableStrategy(strategy interfaces.StrategyType) error {
	args := m.Called(strategy)
	return args.Error(0)
}

func (m *MockStrategyEngine) UpdateStrategyConfig(strategy interfaces.StrategyType, config interface{}) error {
	args := m.Called(strategy, config)
	return args.Error(0)
}

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

type MockShutdownManager struct {
	mock.Mock
}

func (m *MockShutdownManager) CheckShutdownConditions(ctx context.Context) (*interfaces.ShutdownDecision, error) {
	args := m.Called(ctx)
	return args.Get(0).(*interfaces.ShutdownDecision), args.Error(1)
}

func (m *MockShutdownManager) InitiateShutdown(ctx context.Context, reason string) error {
	args := m.Called(ctx, reason)
	return args.Error(0)
}

func (m *MockShutdownManager) GetShutdownStatus() (*interfaces.ShutdownStatus, error) {
	args := m.Called()
	return args.Get(0).(*interfaces.ShutdownStatus), args.Error(1)
}

func (m *MockShutdownManager) SetManualOverride(enabled bool) error {
	args := m.Called(enabled)
	return args.Error(0)
}

// Test setup helper
func setupTestServer(t *testing.T) (*Server, *MockStrategyEngine, *MockMetricsCollector, *MockShutdownManager) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:         "localhost",
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
	}

	mockStrategy := &MockStrategyEngine{}
	mockMetrics := &MockMetricsCollector{}
	mockShutdown := &MockShutdownManager{}

	server := NewServer(cfg, mockStrategy, mockMetrics, mockShutdown)
	
	return server, mockStrategy, mockMetrics, mockShutdown
}

func TestHealthCheck(t *testing.T) {
	server, _, _, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	server.GetRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Equal(t, "healthy", response["status"])
	assert.Contains(t, response, "timestamp")
	assert.Contains(t, response, "version")
}

func TestGetSystemStatus(t *testing.T) {
	server, mockStrategy, mockMetrics, mockShutdown := setupTestServer(t)

	// Setup mocks
	mockStrategy.On("GetActiveStrategies").Return([]interfaces.StrategyType{
		interfaces.StrategySandwich,
		interfaces.StrategyBackrun,
	})

	mockMetrics.On("GetPerformanceMetrics").Return(&interfaces.PerformanceMetrics{
		IsHealthy:       true,
		WarningMode:     false,
		ShutdownPending: false,
		LastUpdated:     time.Now(),
	}, nil)

	mockMetrics.On("GetSystemMetrics").Return(&interfaces.SystemMetrics{
		CPUUsage:    25.5,
		MemoryUsage: 45.2,
		LastUpdated: time.Now(),
	}, nil)

	mockShutdown.On("GetShutdownStatus").Return(&interfaces.ShutdownStatus{
		IsShutdown: false,
	}, nil)

	// Get API key for authentication
	apiKey := getTestAPIKey(server.authService)

	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()

	server.GetRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	
	var response interfaces.SystemStatus
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Equal(t, "healthy", response.Status)
	assert.Len(t, response.ActiveStrategies, 2)
	assert.Contains(t, response.ActiveStrategies, interfaces.StrategySandwich)
	assert.Contains(t, response.ActiveStrategies, interfaces.StrategyBackrun)

	mockStrategy.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
	mockShutdown.AssertExpectations(t)
}

func TestGetOpportunities(t *testing.T) {
	server, _, _, _ := setupTestServer(t)

	apiKey := getTestAPIKey(server.authService)

	req := httptest.NewRequest("GET", "/api/v1/opportunities?limit=10&strategy=sandwich", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()

	server.GetRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	
	var response interfaces.OpportunityResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Equal(t, 0, response.Total) // Empty for now
	assert.Equal(t, 10, response.Limit)
	assert.Equal(t, 0, response.Offset)
}

func TestGetMetrics(t *testing.T) {
	server, _, mockMetrics, _ := setupTestServer(t)

	mockMetrics.On("GetProfitabilityMetrics", 100).Return(&interfaces.ProfitabilityMetrics{
		WindowSize:       100,
		TotalTrades:      50,
		ProfitableTrades: 35,
		SuccessRate:      0.7,
		LastUpdated:      time.Now(),
	}, nil)

	apiKey := getTestAPIKey(server.authService)

	req := httptest.NewRequest("GET", "/api/v1/metrics/profitability?window_size=100", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()

	server.GetRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	
	var response interfaces.ProfitabilityMetrics
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Equal(t, 100, response.WindowSize)
	assert.Equal(t, 50, response.TotalTrades)
	assert.Equal(t, 35, response.ProfitableTrades)
	assert.Equal(t, 0.7, response.SuccessRate)

	mockMetrics.AssertExpectations(t)
}

func TestStrategyManagement(t *testing.T) {
	server, mockStrategy, _, _ := setupTestServer(t)

	apiKey := getTestAPIKey(server.authService)

	// Test enable strategy
	mockStrategy.On("EnableStrategy", interfaces.StrategySandwich).Return(nil)

	req := httptest.NewRequest("POST", "/api/v1/strategies/sandwich/enable", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()

	server.GetRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Test disable strategy
	mockStrategy.On("DisableStrategy", interfaces.StrategySandwich).Return(nil)

	req = httptest.NewRequest("POST", "/api/v1/strategies/sandwich/disable", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w = httptest.NewRecorder()

	server.GetRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Test update strategy config
	config := map[string]interface{}{
		"min_swap_amount": "1000000000000000000",
		"max_slippage":    0.02,
	}
	configJSON, _ := json.Marshal(config)

	mockStrategy.On("UpdateStrategyConfig", interfaces.StrategySandwich, mock.Anything).Return(nil)

	req = httptest.NewRequest("PUT", "/api/v1/strategies/sandwich/config", bytes.NewBuffer(configJSON))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	server.GetRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	mockStrategy.AssertExpectations(t)
}

func TestAuthentication(t *testing.T) {
	server, _, _, _ := setupTestServer(t)

	// Test without API key
	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	w := httptest.NewRecorder()

	server.GetRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Test with invalid API key
	req = httptest.NewRequest("GET", "/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer invalid_key")
	w = httptest.NewRecorder()

	server.GetRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRateLimiting(t *testing.T) {
	server, _, _, _ := setupTestServer(t)

	clientID := "test_client"
	
	// Set a very low rate limit for testing
	server.rateLimiter.SetCustomLimit(clientID, &interfaces.RateLimit{
		RequestsPerMinute: 2,
		BurstSize:         2,
		WindowSize:        time.Minute,
	})

	// Make requests that should exceed the rate limit
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/health", nil)
		req.RemoteAddr = clientID + ":12345"
		w := httptest.NewRecorder()

		server.GetRouter().ServeHTTP(w, req)

		if i < 2 {
			assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i)
		} else {
			assert.Equal(t, http.StatusTooManyRequests, w.Code, "Request %d should be rate limited", i)
		}
	}
}

func TestWebSocketServer(t *testing.T) {
	server, _, _, _ := setupTestServer(t)

	// Start WebSocket server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	err := server.websocketServer.Start(ctx)
	require.NoError(t, err)

	// Test that server starts with 0 clients
	assert.Equal(t, 0, server.websocketServer.GetConnectedClients())

	// Test broadcasting (should not error even with no clients)
	testOpportunity := &interfaces.MEVOpportunity{
		ID:       "test_opportunity",
		Strategy: interfaces.StrategySandwich,
	}

	err = server.websocketServer.BroadcastOpportunity(testOpportunity)
	require.NoError(t, err)

	// Test broadcasting metrics
	testMetrics := &interfaces.PerformanceMetrics{
		IsHealthy:   true,
		LastUpdated: time.Now(),
	}

	err = server.websocketServer.BroadcastMetrics(testMetrics)
	require.NoError(t, err)

	// Test broadcasting status
	testStatus := &interfaces.SystemStatus{
		Status:      "healthy",
		LastUpdated: time.Now(),
	}

	err = server.websocketServer.BroadcastStatus(testStatus)
	require.NoError(t, err)

	// Test broadcasting alert
	testAlert := &interfaces.Alert{
		ID:        "test_alert",
		Type:      interfaces.AlertTypeProfitability,
		Severity:  interfaces.AlertSeverityWarning,
		Message:   "Test alert",
		CreatedAt: time.Now(),
	}

	err = server.websocketServer.BroadcastAlert(testAlert)
	require.NoError(t, err)
}

func TestPrometheusMetrics(t *testing.T) {
	server, _, mockMetrics, _ := setupTestServer(t)

	mockMetrics.On("GetPrometheusMetrics").Return(map[string]interface{}{
		"mev_engine_opportunities_total":     100,
		"mev_engine_profit_total":           "1500000000000000000",
		"mev_engine_latency_simulation_avg": 0.05,
	}, nil)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	server.GetRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))

	body := w.Body.String()
	assert.Contains(t, body, "mev_engine_opportunities_total 100")
	assert.Contains(t, body, "mev_engine_profit_total 1500000000000000000")
	assert.Contains(t, body, "mev_engine_latency_simulation_avg 0.05")

	mockMetrics.AssertExpectations(t)
}

// Helper function to get a test API key
func getTestAPIKey(authService *AuthService) string {
	// Return the default development API key
	for apiKey := range authService.apiKeys {
		return apiKey
	}
	return ""
}