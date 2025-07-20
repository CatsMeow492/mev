package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// WebSocketServer implements real-time opportunity streaming
type WebSocketServer struct {
	upgrader websocket.Upgrader
	clients  map[*websocket.Conn]*Client
	mutex    sync.RWMutex
	
	// Channels for broadcasting
	opportunityBroadcast chan *interfaces.MEVOpportunity
	metricsBroadcast     chan *interfaces.PerformanceMetrics
	statusBroadcast      chan *interfaces.SystemStatus
	alertBroadcast       chan *interfaces.Alert
	
	// Control channels
	register   chan *Client
	unregister chan *Client
	shutdown   chan struct{}
}

// Client represents a WebSocket client connection
type Client struct {
	conn     *websocket.Conn
	send     chan *interfaces.WebSocketMessage
	userID   string
	role     interfaces.UserRole
	lastPing time.Time
}

// NewWebSocketServer creates a new WebSocket server
func NewWebSocketServer() *WebSocketServer {
	return &WebSocketServer{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// TODO: Implement proper origin checking
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		clients:              make(map[*websocket.Conn]*Client),
		opportunityBroadcast: make(chan *interfaces.MEVOpportunity, 100),
		metricsBroadcast:     make(chan *interfaces.PerformanceMetrics, 10),
		statusBroadcast:      make(chan *interfaces.SystemStatus, 10),
		alertBroadcast:       make(chan *interfaces.Alert, 50),
		register:             make(chan *Client),
		unregister:           make(chan *Client),
		shutdown:             make(chan struct{}),
	}
}

// Start starts the WebSocket server
func (ws *WebSocketServer) Start(ctx context.Context) error {
	log.Println("Starting WebSocket server...")
	
	go ws.run(ctx)
	
	log.Println("WebSocket server started")
	return nil
}

// Stop stops the WebSocket server
func (ws *WebSocketServer) Stop(ctx context.Context) error {
	log.Println("Stopping WebSocket server...")
	
	close(ws.shutdown)
	
	// Close all client connections
	ws.mutex.Lock()
	for conn, client := range ws.clients {
		close(client.send)
		conn.Close()
	}
	ws.mutex.Unlock()
	
	log.Println("WebSocket server stopped")
	return nil
}

// HandleWebSocket handles WebSocket connection upgrades
func (ws *WebSocketServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	// TODO: Extract user info from authentication
	client := &Client{
		conn:     conn,
		send:     make(chan *interfaces.WebSocketMessage, 256),
		userID:   "anonymous", // TODO: Get from auth
		role:     interfaces.UserRoleViewer, // TODO: Get from auth
		lastPing: time.Now(),
	}

	ws.register <- client

	// Start goroutines for this client
	go ws.writePump(client)
	go ws.readPump(client)
}

// BroadcastOpportunity broadcasts a new MEV opportunity to all clients
func (ws *WebSocketServer) BroadcastOpportunity(opportunity *interfaces.MEVOpportunity) error {
	select {
	case ws.opportunityBroadcast <- opportunity:
		return nil
	default:
		return fmt.Errorf("opportunity broadcast channel full")
	}
}

// BroadcastMetrics broadcasts performance metrics to all clients
func (ws *WebSocketServer) BroadcastMetrics(metrics *interfaces.PerformanceMetrics) error {
	select {
	case ws.metricsBroadcast <- metrics:
		return nil
	default:
		return fmt.Errorf("metrics broadcast channel full")
	}
}

// BroadcastStatus broadcasts system status to all clients
func (ws *WebSocketServer) BroadcastStatus(status *interfaces.SystemStatus) error {
	select {
	case ws.statusBroadcast <- status:
		return nil
	default:
		return fmt.Errorf("status broadcast channel full")
	}
}

// BroadcastAlert broadcasts an alert to all clients
func (ws *WebSocketServer) BroadcastAlert(alert *interfaces.Alert) error {
	select {
	case ws.alertBroadcast <- alert:
		return nil
	default:
		return fmt.Errorf("alert broadcast channel full")
	}
}

// GetConnectedClients returns the number of connected clients
func (ws *WebSocketServer) GetConnectedClients() int {
	ws.mutex.RLock()
	defer ws.mutex.RUnlock()
	return len(ws.clients)
}

// run is the main event loop for the WebSocket server
func (ws *WebSocketServer) run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Ping interval
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ws.shutdown:
			return
		case client := <-ws.register:
			ws.registerClient(client)
		case client := <-ws.unregister:
			ws.unregisterClient(client)
		case opportunity := <-ws.opportunityBroadcast:
			ws.broadcastToClients(&interfaces.WebSocketMessage{
				Type:      interfaces.MessageTypeOpportunity,
				Data:      opportunity,
				Timestamp: time.Now(),
			})
		case metrics := <-ws.metricsBroadcast:
			ws.broadcastToClients(&interfaces.WebSocketMessage{
				Type:      interfaces.MessageTypeMetrics,
				Data:      metrics,
				Timestamp: time.Now(),
			})
		case status := <-ws.statusBroadcast:
			ws.broadcastToClients(&interfaces.WebSocketMessage{
				Type:      interfaces.MessageTypeStatus,
				Data:      status,
				Timestamp: time.Now(),
			})
		case alert := <-ws.alertBroadcast:
			ws.broadcastToClients(&interfaces.WebSocketMessage{
				Type:      interfaces.MessageTypeAlert,
				Data:      alert,
				Timestamp: time.Now(),
			})
		case <-ticker.C:
			ws.pingClients()
		}
	}
}

// registerClient registers a new client
func (ws *WebSocketServer) registerClient(client *Client) {
	ws.mutex.Lock()
	ws.clients[client.conn] = client
	ws.mutex.Unlock()
	
	log.Printf("WebSocket client connected (total: %d)", len(ws.clients))
	
	// Send welcome message
	welcomeMsg := &interfaces.WebSocketMessage{
		Type: interfaces.MessageTypeStatus,
		Data: map[string]interface{}{
			"message": "Connected to MEV Engine WebSocket",
			"user_id": client.userID,
			"role":    client.role,
		},
		Timestamp: time.Now(),
	}
	
	select {
	case client.send <- welcomeMsg:
	default:
		close(client.send)
		delete(ws.clients, client.conn)
	}
}

// unregisterClient unregisters a client
func (ws *WebSocketServer) unregisterClient(client *Client) {
	ws.mutex.Lock()
	if _, ok := ws.clients[client.conn]; ok {
		delete(ws.clients, client.conn)
		close(client.send)
		client.conn.Close()
	}
	ws.mutex.Unlock()
	
	log.Printf("WebSocket client disconnected (total: %d)", len(ws.clients))
}

// broadcastToClients broadcasts a message to all connected clients
func (ws *WebSocketServer) broadcastToClients(message *interfaces.WebSocketMessage) {
	ws.mutex.RLock()
	defer ws.mutex.RUnlock()
	
	for conn, client := range ws.clients {
		select {
		case client.send <- message:
		default:
			close(client.send)
			delete(ws.clients, conn)
		}
	}
}

// pingClients sends ping messages to all clients to keep connections alive
func (ws *WebSocketServer) pingClients() {
	ws.mutex.RLock()
	defer ws.mutex.RUnlock()
	
	for conn, client := range ws.clients {
		if time.Since(client.lastPing) > 60*time.Second {
			// Client hasn't responded to ping in 60 seconds, disconnect
			close(client.send)
			delete(ws.clients, conn)
			continue
		}
		
		if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
			close(client.send)
			delete(ws.clients, conn)
		}
	}
}

// readPump handles incoming messages from a client
func (ws *WebSocketServer) readPump(client *Client) {
	defer func() {
		ws.unregister <- client
	}()

	client.conn.SetReadLimit(512)
	client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.conn.SetPongHandler(func(string) error {
		client.lastPing = time.Now()
		client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
	}
}

// writePump handles outgoing messages to a client
func (ws *WebSocketServer) writePump(client *Client) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		client.conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.conn.WriteJSON(message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}

		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}