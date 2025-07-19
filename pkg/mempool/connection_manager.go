package mempool

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// Endpoint represents a WebSocket endpoint with priority and connection
type Endpoint struct {
	URL        string
	Priority   int
	Connection interfaces.WebSocketConnection
	LastFailed time.Time
	FailCount  int
}

// ConnectionManagerImpl implements the ConnectionManager interface
type ConnectionManagerImpl struct {
	endpoints         []*Endpoint
	mu                sync.RWMutex
	maxRetries        int
	baseRetryDelay    time.Duration
	maxRetryDelay     time.Duration
	connectionTimeout time.Duration
	healthCheckInterval time.Duration
	stopHealthCheck   chan struct{}
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(maxRetries int, baseRetryDelay, maxRetryDelay, connectionTimeout time.Duration) interfaces.ConnectionManager {
	cm := &ConnectionManagerImpl{
		endpoints:           make([]*Endpoint, 0),
		maxRetries:          maxRetries,
		baseRetryDelay:      baseRetryDelay,
		maxRetryDelay:       maxRetryDelay,
		connectionTimeout:   connectionTimeout,
		healthCheckInterval: 30 * time.Second,
		stopHealthCheck:     make(chan struct{}),
	}

	// Start health check routine
	go cm.healthCheckRoutine()

	return cm
}

// AddEndpoint adds a new WebSocket endpoint with the specified priority
func (cm *ConnectionManagerImpl) AddEndpoint(url string, priority int) error {
	if url == "" {
		return fmt.Errorf("endpoint URL cannot be empty")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if endpoint already exists
	for _, ep := range cm.endpoints {
		if ep.URL == url {
			return fmt.Errorf("endpoint %s already exists", url)
		}
	}

	endpoint := &Endpoint{
		URL:        url,
		Priority:   priority,
		Connection: NewWebSocketConnection(),
		LastFailed: time.Time{},
		FailCount:  0,
	}

	cm.endpoints = append(cm.endpoints, endpoint)

	// Sort endpoints by priority (lower number = higher priority)
	sort.Slice(cm.endpoints, func(i, j int) bool {
		return cm.endpoints[i].Priority < cm.endpoints[j].Priority
	})

	return nil
}

// GetConnection returns a healthy WebSocket connection, attempting failover if needed
func (cm *ConnectionManagerImpl) GetConnection(ctx context.Context) (interfaces.WebSocketConnection, error) {
	cm.mu.RLock()
	endpoints := make([]*Endpoint, len(cm.endpoints))
	copy(endpoints, cm.endpoints)
	cm.mu.RUnlock()

	if len(endpoints) == 0 {
		return nil, fmt.Errorf("no endpoints configured")
	}

	// Try to get a healthy connection from existing endpoints
	for _, ep := range endpoints {
		if ep.Connection.IsConnected() && ep.Connection.GetConnectionHealth().IsHealthy {
			return ep.Connection, nil
		}
	}

	// No healthy connections found, try to establish new connections
	for _, ep := range endpoints {
		if cm.shouldRetryEndpoint(ep) {
			if err := cm.connectToEndpoint(ctx, ep); err == nil {
				return ep.Connection, nil
			}
		}
	}

	return nil, fmt.Errorf("no healthy connections available")
}

// HandleConnectionFailure handles a connection failure and implements exponential backoff
func (cm *ConnectionManagerImpl) HandleConnectionFailure(conn interfaces.WebSocketConnection) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Find the endpoint for this connection
	var failedEndpoint *Endpoint
	for _, ep := range cm.endpoints {
		if ep.Connection == conn {
			failedEndpoint = ep
			break
		}
	}

	if failedEndpoint == nil {
		return fmt.Errorf("connection not found in managed endpoints")
	}

	// Update failure information
	failedEndpoint.LastFailed = time.Now()
	failedEndpoint.FailCount++

	// Close the failed connection
	if err := failedEndpoint.Connection.Close(); err != nil {
		// Log error but don't return it as the main goal is to handle the failure
	}

	// Create a new connection instance for future retry attempts
	failedEndpoint.Connection = NewWebSocketConnection()

	return nil
}

// GetHealthyConnections returns all currently healthy connections
func (cm *ConnectionManagerImpl) GetHealthyConnections() []interfaces.WebSocketConnection {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var healthyConns []interfaces.WebSocketConnection
	for _, ep := range cm.endpoints {
		if ep.Connection.IsConnected() && ep.Connection.GetConnectionHealth().IsHealthy {
			healthyConns = append(healthyConns, ep.Connection)
		}
	}

	return healthyConns
}

// shouldRetryEndpoint determines if an endpoint should be retried based on exponential backoff
func (cm *ConnectionManagerImpl) shouldRetryEndpoint(ep *Endpoint) bool {
	if ep.FailCount == 0 {
		return true
	}

	if ep.FailCount >= cm.maxRetries {
		// Reset fail count after a longer period to allow eventual retry
		if time.Since(ep.LastFailed) > cm.maxRetryDelay*10 {
			ep.FailCount = 0
			return true
		}
		return false
	}

	// Calculate exponential backoff delay
	backoffDelay := time.Duration(math.Pow(2, float64(ep.FailCount-1))) * cm.baseRetryDelay
	if backoffDelay > cm.maxRetryDelay {
		backoffDelay = cm.maxRetryDelay
	}

	return time.Since(ep.LastFailed) >= backoffDelay
}

// connectToEndpoint attempts to connect to a specific endpoint
func (cm *ConnectionManagerImpl) connectToEndpoint(ctx context.Context, ep *Endpoint) error {
	// Create context with timeout
	connectCtx, cancel := context.WithTimeout(ctx, cm.connectionTimeout)
	defer cancel()

	err := ep.Connection.Connect(connectCtx, ep.URL)
	if err != nil {
		ep.LastFailed = time.Now()
		ep.FailCount++
		return fmt.Errorf("failed to connect to %s: %w", ep.URL, err)
	}

	// Reset failure count on successful connection
	ep.FailCount = 0
	ep.LastFailed = time.Time{}

	return nil
}

// healthCheckRoutine periodically checks the health of all connections
func (cm *ConnectionManagerImpl) healthCheckRoutine() {
	ticker := time.NewTicker(cm.healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cm.performHealthCheck()
		case <-cm.stopHealthCheck:
			return
		}
	}
}

// performHealthCheck checks the health of all connections and handles failures
func (cm *ConnectionManagerImpl) performHealthCheck() {
	cm.mu.RLock()
	endpoints := make([]*Endpoint, len(cm.endpoints))
	copy(endpoints, cm.endpoints)
	cm.mu.RUnlock()

	for _, ep := range endpoints {
		if ep.Connection.IsConnected() {
			health := ep.Connection.GetConnectionHealth()
			
			// Consider connection unhealthy if:
			// - It's marked as unhealthy
			// - Last ping was more than 2 minutes ago
			// - Error count is too high
			if !health.IsHealthy || 
			   time.Since(health.LastPingTime) > 2*time.Minute ||
			   health.ErrorCount > 5 {
				
				// Handle the connection failure
				cm.HandleConnectionFailure(ep.Connection)
			}
		}
	}
}

// Close shuts down the connection manager and all connections
func (cm *ConnectionManagerImpl) Close() error {
	// Stop health check routine
	close(cm.stopHealthCheck)

	cm.mu.Lock()
	defer cm.mu.Unlock()

	var lastErr error
	for _, ep := range cm.endpoints {
		if err := ep.Connection.Close(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}