package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAPIIntegration tests the complete API integration
func TestAPIIntegration(t *testing.T) {
	// Setup test server
	server, mockStrategy, mockMetrics, mockShutdown := setupTestServer(t)

	// Setup mocks for system status
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

	// Test 1: Health check (no auth required)
	t.Run("HealthCheck", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		server.GetRouter().ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var health map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &health)
		require.NoError(t, err)
		assert.Equal(t, "healthy", health["status"])
	})

	// Test 2: System status (auth required)
	t.Run("SystemStatus", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/status", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()

		server.GetRouter().ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var status interfaces.SystemStatus
		err := json.Unmarshal(w.Body.Bytes(), &status)
		require.NoError(t, err)
		assert.Equal(t, "healthy", status.Status)
		assert.Len(t, status.ActiveStrategies, 2)
	})

	// Test 3: Opportunities endpoint
	t.Run("Opportunities", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/opportunities", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()

		server.GetRouter().ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response interfaces.OpportunityResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, 50, response.Limit) // Default limit
	})

	// Test 4: Strategy management
	t.Run("StrategyManagement", func(t *testing.T) {
		// Enable strategy
		mockStrategy.On("EnableStrategy", interfaces.StrategySandwich).Return(nil)

		req := httptest.NewRequest("POST", "/api/v1/strategies/sandwich/enable", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()

		server.GetRouter().ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Disable strategy
		mockStrategy.On("DisableStrategy", interfaces.StrategySandwich).Return(nil)

		req = httptest.NewRequest("POST", "/api/v1/strategies/sandwich/disable", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w = httptest.NewRecorder()

		server.GetRouter().ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	// Test 5: WebSocket server functionality
	t.Run("WebSocketServer", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := server.websocketServer.Start(ctx)
		require.NoError(t, err)

		// Test broadcasting
		opportunity := &interfaces.MEVOpportunity{
			ID:       "test_opportunity",
			Strategy: interfaces.StrategySandwich,
		}

		err = server.websocketServer.BroadcastOpportunity(opportunity)
		assert.NoError(t, err)

		assert.Equal(t, 0, server.websocketServer.GetConnectedClients())
	})

	// Test 6: Rate limiting
	t.Run("RateLimiting", func(t *testing.T) {
		// Set low rate limit
		server.rateLimiter.SetCustomLimit("integration_test", &interfaces.RateLimit{
			RequestsPerMinute: 1,
			BurstSize:         1,
			WindowSize:        time.Minute,
		})

		// First request should succeed
		req := httptest.NewRequest("GET", "/health", nil)
		req.RemoteAddr = "integration_test:12345"
		w := httptest.NewRecorder()

		server.GetRouter().ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Second request should be rate limited
		req = httptest.NewRequest("GET", "/health", nil)
		req.RemoteAddr = "integration_test:12345"
		w = httptest.NewRecorder()

		server.GetRouter().ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
	})

	// Verify all mocks were called as expected
	mockStrategy.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
	mockShutdown.AssertExpectations(t)
}

// TestAPIServerLifecycle tests the server start/stop lifecycle
func TestAPIServerLifecycle(t *testing.T) {
	server, _, _, _ := setupTestServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test server start
	err := server.Start(ctx)
	assert.NoError(t, err)

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Test server stop
	err = server.Stop(ctx)
	assert.NoError(t, err)
}

// TestAuthenticationFlow tests the complete authentication flow
func TestAuthenticationFlow(t *testing.T) {
	authService := NewAuthService()

	// Test API key validation
	apiKey := getTestAPIKey(authService)
	user, err := authService.ValidateAPIKey(apiKey)
	require.NoError(t, err)
	assert.Equal(t, "admin", user.ID)
	assert.Equal(t, interfaces.UserRoleAdmin, user.Role)

	// Test invalid API key
	_, err = authService.ValidateAPIKey("invalid_key")
	assert.Error(t, err)

	// Test API key generation
	newKey, err := authService.GenerateAPIKey("admin")
	require.NoError(t, err)
	assert.NotEmpty(t, newKey)

	// Test new key validation
	user, err = authService.ValidateAPIKey(newKey)
	require.NoError(t, err)
	assert.Equal(t, "admin", user.ID)

	// Test API key revocation
	err = authService.RevokeAPIKey(newKey)
	require.NoError(t, err)

	// Test revoked key validation
	_, err = authService.ValidateAPIKey(newKey)
	assert.Error(t, err)
}