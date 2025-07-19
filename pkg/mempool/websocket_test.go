package mempool

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock WebSocket server for testing
type mockWebSocketServer struct {
	server   *httptest.Server
	upgrader websocket.Upgrader
	conn     *websocket.Conn
	messages [][]byte
}

func newMockWebSocketServer() *mockWebSocketServer {
	mock := &mockWebSocketServer{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		messages: make([][]byte, 0),
	}

	mock.server = httptest.NewServer(http.HandlerFunc(mock.handleWebSocket))
	return mock
}

func (m *mockWebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := m.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	m.conn = conn
	writeMutex := &sync.Mutex{}

	// Handle ping messages
	conn.SetPingHandler(func(appData string) error {
		writeMutex.Lock()
		defer writeMutex.Unlock()
		return conn.WriteMessage(websocket.PongMessage, []byte(appData))
	})

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}
		m.messages = append(m.messages, message)

		// Handle subscription requests
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err == nil {
			if method, ok := msg["method"].(string); ok && method == "eth_subscribe" {
				// Send subscription confirmation
				response := map[string]interface{}{
					"id":     msg["id"],
					"result": "0x123456789",
				}
				respBytes, _ := json.Marshal(response)
				
				writeMutex.Lock()
				conn.WriteMessage(websocket.TextMessage, respBytes)
				writeMutex.Unlock()

				// Send a mock notification after a short delay
				go func() {
					time.Sleep(100 * time.Millisecond)
					notification := map[string]interface{}{
						"method": "eth_subscription",
						"params": map[string]interface{}{
							"subscription": "0x123456789",
							"result":       "0xmocktransactionhash",
						},
					}
					notifBytes, _ := json.Marshal(notification)
					
					writeMutex.Lock()
					conn.WriteMessage(websocket.TextMessage, notifBytes)
					writeMutex.Unlock()
				}()
			}
		}
	}
}

func (m *mockWebSocketServer) getWebSocketURL() string {
	return "ws" + strings.TrimPrefix(m.server.URL, "http")
}

func (m *mockWebSocketServer) close() {
	if m.conn != nil {
		m.conn.Close()
	}
	m.server.Close()
}

func TestWebSocketConnection_Connect(t *testing.T) {
	server := newMockWebSocketServer()
	defer server.close()

	conn := NewWebSocketConnection()
	ctx := context.Background()

	// Test successful connection
	err := conn.Connect(ctx, server.getWebSocketURL())
	assert.NoError(t, err)
	assert.True(t, conn.IsConnected())

	health := conn.GetConnectionHealth()
	assert.True(t, health.IsHealthy)
	assert.Equal(t, 0, health.ErrorCount)

	// Clean up
	err = conn.Close()
	assert.NoError(t, err)
	assert.False(t, conn.IsConnected())
}

func TestWebSocketConnection_ConnectInvalidURL(t *testing.T) {
	conn := NewWebSocketConnection()
	ctx := context.Background()

	// Test connection to invalid URL
	err := conn.Connect(ctx, "ws://invalid-url:99999")
	assert.Error(t, err)
	assert.False(t, conn.IsConnected())

	health := conn.GetConnectionHealth()
	assert.False(t, health.IsHealthy)
	assert.Greater(t, health.ErrorCount, 0)
	assert.NotNil(t, health.LastError)
}

func TestWebSocketConnection_Subscribe(t *testing.T) {
	server := newMockWebSocketServer()
	defer server.close()

	conn := NewWebSocketConnection()
	ctx := context.Background()

	// Connect first
	err := conn.Connect(ctx, server.getWebSocketURL())
	require.NoError(t, err)

	// Test subscription
	msgChan, err := conn.Subscribe(ctx, "newPendingTransactions")
	assert.NoError(t, err)
	assert.NotNil(t, msgChan)

	// Wait for subscription confirmation and notification
	select {
	case msg := <-msgChan:
		assert.NotNil(t, msg)
		
		var notification map[string]interface{}
		err := json.Unmarshal(msg, &notification)
		assert.NoError(t, err)
		assert.Equal(t, "eth_subscription", notification["method"])
		
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for subscription message")
	}

	// Clean up
	conn.Close()
}

func TestWebSocketConnection_SubscribeWithoutConnection(t *testing.T) {
	conn := NewWebSocketConnection()
	ctx := context.Background()

	// Test subscription without connection
	_, err := conn.Subscribe(ctx, "newPendingTransactions")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection not established")
}

func TestWebSocketConnection_HealthMonitoring(t *testing.T) {
	server := newMockWebSocketServer()
	defer server.close()

	conn := NewWebSocketConnection()
	ctx := context.Background()

	// Connect
	err := conn.Connect(ctx, server.getWebSocketURL())
	require.NoError(t, err)

	// Wait for ping routine to start and send at least one ping
	time.Sleep(100 * time.Millisecond)

	health := conn.GetConnectionHealth()
	assert.True(t, health.IsHealthy)

	// The ping time should be set (not zero time) if ping has been sent
	if !health.LastPingTime.IsZero() {
		assert.WithinDuration(t, time.Now(), health.LastPingTime, 5*time.Second)
	}

	conn.Close()
}

func TestWebSocketConnection_Reconnection(t *testing.T) {
	server := newMockWebSocketServer()
	defer server.close()

	conn := NewWebSocketConnection()
	ctx := context.Background()

	// Connect
	err := conn.Connect(ctx, server.getWebSocketURL())
	require.NoError(t, err)
	assert.True(t, conn.IsConnected())

	// Close the server connection to simulate network failure
	server.conn.Close()

	// Wait for connection to be detected as unhealthy
	time.Sleep(200 * time.Millisecond)

	// Connection should be marked as disconnected
	assert.False(t, conn.IsConnected())

	conn.Close()
}

func TestWebSocketConnection_ConcurrentOperations(t *testing.T) {
	server := newMockWebSocketServer()
	defer server.close()

	conn := NewWebSocketConnection()
	ctx := context.Background()

	// Connect
	err := conn.Connect(ctx, server.getWebSocketURL())
	require.NoError(t, err)

	// Test concurrent subscriptions
	const numSubscriptions = 10
	channels := make([]<-chan []byte, numSubscriptions)

	for i := 0; i < numSubscriptions; i++ {
		ch, err := conn.Subscribe(ctx, "newPendingTransactions")
		assert.NoError(t, err)
		channels[i] = ch
	}

	// Test concurrent health checks
	for i := 0; i < 5; i++ {
		go func() {
			health := conn.GetConnectionHealth()
			assert.NotNil(t, health)
		}()
	}

	// Test concurrent connection status checks
	for i := 0; i < 5; i++ {
		go func() {
			isConnected := conn.IsConnected()
			assert.True(t, isConnected)
		}()
	}

	// Wait a bit for concurrent operations
	time.Sleep(100 * time.Millisecond)

	conn.Close()
}

func TestWebSocketConnection_MessageBuffering(t *testing.T) {
	server := newMockWebSocketServer()
	defer server.close()

	conn := NewWebSocketConnection()
	ctx := context.Background()

	// Connect
	err := conn.Connect(ctx, server.getWebSocketURL())
	require.NoError(t, err)

	// Subscribe
	msgChan, err := conn.Subscribe(ctx, "newPendingTransactions")
	require.NoError(t, err)

	// The channel should be buffered and not block
	select {
	case <-msgChan:
		// Expected to receive at least one message
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for buffered message")
	}

	conn.Close()
}