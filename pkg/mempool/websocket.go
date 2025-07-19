package mempool

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// WebSocketConnectionImpl implements the WebSocketConnection interface
type WebSocketConnectionImpl struct {
	conn         *websocket.Conn
	url          string
	isConnected  bool
	health       interfaces.ConnectionHealth
	mu           sync.RWMutex
	pingTicker   *time.Ticker
	stopPing     chan struct{}
	subscriptions map[string]chan []byte
	subMu        sync.RWMutex
}

// NewWebSocketConnection creates a new WebSocket connection
func NewWebSocketConnection() interfaces.WebSocketConnection {
	return &WebSocketConnectionImpl{
		subscriptions: make(map[string]chan []byte),
		health: interfaces.ConnectionHealth{
			IsHealthy:    false,
			ErrorCount:   0,
			LastPingTime: time.Time{},
		},
	}
}

// Connect establishes a WebSocket connection to the specified URL
func (w *WebSocketConnectionImpl) Connect(ctx context.Context, url string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.url = url
	
	dialer := websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
		ReadBufferSize:   1024 * 16, // 16KB buffer
		WriteBufferSize:  1024 * 16, // 16KB buffer
	}

	conn, _, err := dialer.DialContext(ctx, url, http.Header{
		"User-Agent": []string{"MEV-Engine/1.0"},
	})
	if err != nil {
		w.health.LastError = err
		w.health.ErrorCount++
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	w.conn = conn
	w.isConnected = true
	w.health.IsHealthy = true
	w.health.ErrorCount = 0
	w.health.LastError = nil

	// Start ping routine for connection health monitoring
	w.startPingRoutine()

	// Start message reading routine
	go w.readMessages()

	return nil
}

// Subscribe subscribes to a WebSocket method and returns a channel for messages
func (w *WebSocketConnectionImpl) Subscribe(ctx context.Context, method string, params ...interface{}) (<-chan []byte, error) {
	w.mu.RLock()
	if !w.isConnected || w.conn == nil {
		w.mu.RUnlock()
		return nil, fmt.Errorf("connection not established")
	}
	w.mu.RUnlock()

	// Generate subscription ID
	subID := fmt.Sprintf("%s_%d", method, rand.Int63())

	// Create subscription message
	subMsg := map[string]interface{}{
		"id":     subID,
		"method": "eth_subscribe",
		"params": append([]interface{}{method}, params...),
	}

	msgBytes, err := json.Marshal(subMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal subscription message: %w", err)
	}

	// Create response channel
	respChan := make(chan []byte, 100) // Buffered channel to prevent blocking

	w.subMu.Lock()
	w.subscriptions[subID] = respChan
	w.subMu.Unlock()

	// Send subscription message
	w.mu.Lock()
	err = w.conn.WriteMessage(websocket.TextMessage, msgBytes)
	w.mu.Unlock()

	if err != nil {
		w.subMu.Lock()
		delete(w.subscriptions, subID)
		w.subMu.Unlock()
		close(respChan)
		return nil, fmt.Errorf("failed to send subscription message: %w", err)
	}

	return respChan, nil
}

// Close closes the WebSocket connection
func (w *WebSocketConnectionImpl) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Stop ping routine first
	if w.stopPing != nil {
		select {
		case <-w.stopPing:
			// Already closed
		default:
			close(w.stopPing)
		}
		w.stopPing = nil
	}

	if w.pingTicker != nil {
		w.pingTicker.Stop()
		w.pingTicker = nil
	}

	// Close all subscription channels
	w.subMu.Lock()
	for _, ch := range w.subscriptions {
		select {
		case <-ch:
			// Channel already closed
		default:
			close(ch)
		}
	}
	w.subscriptions = make(map[string]chan []byte)
	w.subMu.Unlock()

	var err error
	if w.conn != nil {
		err = w.conn.Close()
		w.conn = nil
	}
	
	w.isConnected = false
	w.health.IsHealthy = false
	return err
}

// IsConnected returns whether the connection is active
func (w *WebSocketConnectionImpl) IsConnected() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.isConnected && w.conn != nil
}

// GetConnectionHealth returns the current connection health status
func (w *WebSocketConnectionImpl) GetConnectionHealth() interfaces.ConnectionHealth {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.health
}

// startPingRoutine starts a routine to ping the server for health monitoring
func (w *WebSocketConnectionImpl) startPingRoutine() {
	// Stop existing ping routine if running
	if w.stopPing != nil {
		select {
		case <-w.stopPing:
			// Already closed
		default:
			close(w.stopPing)
		}
	}
	
	if w.pingTicker != nil {
		w.pingTicker.Stop()
	}
	
	w.pingTicker = time.NewTicker(30 * time.Second)
	w.stopPing = make(chan struct{})

	go func() {
		defer func() {
			// Recover from any panics
			if r := recover(); r != nil {
				// Log the panic but don't crash
			}
			
			// Clean up ticker
			w.mu.Lock()
			if w.pingTicker != nil {
				w.pingTicker.Stop()
				w.pingTicker = nil
			}
			w.mu.Unlock()
		}()
		
		for {
			select {
			case <-w.pingTicker.C:
				// Check if we should still be running
				w.mu.RLock()
				shouldContinue := w.isConnected && w.conn != nil
				w.mu.RUnlock()
				
				if shouldContinue {
					w.sendPing()
				} else {
					return
				}
			case <-w.stopPing:
				return
			}
		}
	}()
}

// sendPing sends a ping message and measures response time
func (w *WebSocketConnectionImpl) sendPing() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.isConnected || w.conn == nil {
		w.health.IsHealthy = false
		return
	}

	start := time.Now()
	w.health.LastPingTime = start

	err := w.conn.WriteMessage(websocket.PingMessage, []byte{})
	if err != nil {
		w.health.LastError = err
		w.health.ErrorCount++
		w.health.IsHealthy = false
		return
	}

	// Set pong handler to measure response time
	w.conn.SetPongHandler(func(appData string) error {
		w.mu.Lock()
		defer w.mu.Unlock()
		w.health.ResponseTime = time.Since(start)
		w.health.IsHealthy = true
		return nil
	})
}

// readMessages continuously reads messages from the WebSocket connection
func (w *WebSocketConnectionImpl) readMessages() {
	defer func() {
		w.mu.Lock()
		w.isConnected = false
		w.health.IsHealthy = false
		w.mu.Unlock()
	}()

	for {
		w.mu.RLock()
		conn := w.conn
		w.mu.RUnlock()

		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			w.mu.Lock()
			w.health.LastError = err
			w.health.ErrorCount++
			w.health.IsHealthy = false
			w.mu.Unlock()
			return
		}

		// Parse message to determine subscription
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			continue // Skip malformed messages
		}

		// Handle subscription notifications
		if method, ok := msg["method"].(string); ok && method == "eth_subscription" {
			// Send to all subscription channels for simplicity in tests
			w.subMu.RLock()
			for _, ch := range w.subscriptions {
				select {
				case ch <- message:
				default:
					// Channel is full, skip message to prevent blocking
				}
			}
			w.subMu.RUnlock()
		}
	}
}