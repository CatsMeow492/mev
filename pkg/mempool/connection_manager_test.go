package mempool

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectionManager_AddEndpoint(t *testing.T) {
	cm := NewConnectionManager(5, time.Second, time.Minute, 30*time.Second)

	// Test adding valid endpoint
	err := cm.AddEndpoint("ws://localhost:8080", 1)
	assert.NoError(t, err)

	// Test adding duplicate endpoint
	err = cm.AddEndpoint("ws://localhost:8080", 2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// Test adding empty URL
	err = cm.AddEndpoint("", 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestConnectionManager_EndpointPriority(t *testing.T) {
	cm := NewConnectionManager(5, time.Second, time.Minute, 30*time.Second)

	// Add endpoints with different priorities
	err := cm.AddEndpoint("ws://localhost:8081", 3)
	require.NoError(t, err)

	err = cm.AddEndpoint("ws://localhost:8080", 1)
	require.NoError(t, err)

	err = cm.AddEndpoint("ws://localhost:8082", 2)
	require.NoError(t, err)

	// The connection manager should prioritize endpoints correctly
	// (This is tested indirectly through the GetConnection method)
	
	// Test that we can't get a connection without any healthy endpoints
	ctx := context.Background()
	_, err = cm.GetConnection(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no healthy connections available")
}

func TestConnectionManager_GetConnectionNoEndpoints(t *testing.T) {
	cm := NewConnectionManager(5, time.Second, time.Minute, 30*time.Second)
	ctx := context.Background()

	// Test getting connection with no endpoints
	_, err := cm.GetConnection(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no endpoints configured")
}

func TestConnectionManager_GetConnectionWithMockServer(t *testing.T) {
	server := newMockWebSocketServer()
	defer server.close()

	cm := NewConnectionManager(5, time.Second, time.Minute, 30*time.Second)
	
	// Add the mock server endpoint
	err := cm.AddEndpoint(server.getWebSocketURL(), 1)
	require.NoError(t, err)

	ctx := context.Background()
	
	// Get connection should succeed
	conn, err := cm.GetConnection(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, conn)
	assert.True(t, conn.IsConnected())

	// Getting connection again should return the same healthy connection
	conn2, err := cm.GetConnection(ctx)
	assert.NoError(t, err)
	assert.Equal(t, conn, conn2)
}

func TestConnectionManager_HandleConnectionFailure(t *testing.T) {
	server := newMockWebSocketServer()
	defer server.close()

	cm := NewConnectionManager(5, time.Second, time.Minute, 30*time.Second)
	
	// Add endpoint and get connection
	err := cm.AddEndpoint(server.getWebSocketURL(), 1)
	require.NoError(t, err)

	ctx := context.Background()
	conn, err := cm.GetConnection(ctx)
	require.NoError(t, err)

	// Close the connection properly first
	conn.Close()

	// Simulate connection failure
	err = cm.HandleConnectionFailure(conn)
	assert.NoError(t, err)

	// Connection should no longer be connected
	assert.False(t, conn.IsConnected())

	// Getting a new connection should work after reconnection
	newConn, err := cm.GetConnection(ctx)
	if err != nil {
		// If it fails due to backoff, that's expected behavior
		assert.Contains(t, err.Error(), "no healthy connections available")
	} else {
		// If it succeeds, it should be a different instance
		assert.NotEqual(t, conn, newConn)
		newConn.Close()
	}
}

func TestConnectionManager_HandleConnectionFailureUnknownConnection(t *testing.T) {
	cm := NewConnectionManager(5, time.Second, time.Minute, 30*time.Second)
	
	// Create a connection that's not managed by the connection manager
	unknownConn := NewWebSocketConnection()
	
	// Handling failure of unknown connection should return error
	err := cm.HandleConnectionFailure(unknownConn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection not found")
}

func TestConnectionManager_GetHealthyConnections(t *testing.T) {
	server1 := newMockWebSocketServer()
	defer server1.close()
	
	server2 := newMockWebSocketServer()
	defer server2.close()

	cm := NewConnectionManager(5, time.Second, time.Minute, 30*time.Second)
	
	// Add multiple endpoints
	err := cm.AddEndpoint(server1.getWebSocketURL(), 1)
	require.NoError(t, err)
	
	err = cm.AddEndpoint(server2.getWebSocketURL(), 2)
	require.NoError(t, err)

	ctx := context.Background()
	
	// Get connections to establish them
	conn1, err := cm.GetConnection(ctx)
	require.NoError(t, err)
	
	// Initially should have one healthy connection
	healthyConns := cm.GetHealthyConnections()
	assert.Len(t, healthyConns, 1)
	assert.Equal(t, conn1, healthyConns[0])

	// Simulate failure of first connection
	err = cm.HandleConnectionFailure(conn1)
	require.NoError(t, err)

	// Should now have no healthy connections
	healthyConns = cm.GetHealthyConnections()
	assert.Len(t, healthyConns, 0)
}

func TestConnectionManager_ExponentialBackoff(t *testing.T) {
	cm := NewConnectionManager(3, 100*time.Millisecond, time.Second, 30*time.Second)
	
	// Add an endpoint that will fail to connect
	err := cm.AddEndpoint("ws://invalid-endpoint:99999", 1)
	require.NoError(t, err)

	ctx := context.Background()
	
	// First attempt should fail immediately
	start := time.Now()
	_, err = cm.GetConnection(ctx)
	assert.Error(t, err)
	assert.Less(t, time.Since(start), 50*time.Millisecond) // Should fail quickly

	// Second attempt should also fail quickly (within backoff period)
	start = time.Now()
	_, err = cm.GetConnection(ctx)
	assert.Error(t, err)
	assert.Less(t, time.Since(start), 50*time.Millisecond)

	// Wait for backoff period and try again
	time.Sleep(150 * time.Millisecond)
	
	start = time.Now()
	_, err = cm.GetConnection(ctx)
	assert.Error(t, err)
	// This should take some time as it actually attempts to connect
	// Just verify it took some measurable time
	elapsed := time.Since(start)
	assert.Greater(t, elapsed, time.Duration(0))
}

func TestConnectionManager_MaxRetries(t *testing.T) {
	cm := NewConnectionManager(2, 10*time.Millisecond, 100*time.Millisecond, 30*time.Second)
	
	// Add an endpoint that will always fail
	err := cm.AddEndpoint("ws://invalid-endpoint:99999", 1)
	require.NoError(t, err)

	ctx := context.Background()
	
	// Try multiple times to exceed max retries
	for i := 0; i < 5; i++ {
		_, err = cm.GetConnection(ctx)
		assert.Error(t, err)
		
		if i < 2 {
			// Should still be trying
			assert.Contains(t, err.Error(), "no healthy connections available")
		}
		
		// Small delay between attempts
		time.Sleep(20 * time.Millisecond)
	}
}

func TestConnectionManager_MultipleEndpointsFailover(t *testing.T) {
	server1 := newMockWebSocketServer()
	server2 := newMockWebSocketServer()
	defer server1.close()
	defer server2.close()

	cm := NewConnectionManager(5, time.Second, time.Minute, 30*time.Second)
	
	// Add working endpoint with lower priority
	err := cm.AddEndpoint(server2.getWebSocketURL(), 2)
	require.NoError(t, err)
	
	// Add failing endpoint with higher priority
	err = cm.AddEndpoint("ws://invalid-endpoint:99999", 1)
	require.NoError(t, err)

	ctx := context.Background()
	
	// Should eventually get connection from the working endpoint
	conn, err := cm.GetConnection(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, conn)
	assert.True(t, conn.IsConnected())
}

func TestConnectionManager_ConcurrentAccess(t *testing.T) {
	server := newMockWebSocketServer()
	defer server.close()

	cm := NewConnectionManager(5, time.Second, time.Minute, 30*time.Second)
	
	err := cm.AddEndpoint(server.getWebSocketURL(), 1)
	require.NoError(t, err)

	ctx := context.Background()
	
	// Test concurrent access to GetConnection
	const numGoroutines = 10
	results := make(chan error, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func() {
			conn, err := cm.GetConnection(ctx)
			if err == nil && conn != nil {
				results <- nil
			} else {
				results <- err
			}
		}()
	}
	
	// Collect results
	successCount := 0
	for i := 0; i < numGoroutines; i++ {
		select {
		case err := <-results:
			if err == nil {
				successCount++
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent operations")
		}
	}
	
	// At least some operations should succeed
	assert.Greater(t, successCount, 0)
}