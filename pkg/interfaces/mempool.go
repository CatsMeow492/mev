package interfaces

import (
	"context"
	"time"

	mevtypes "github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

// WebSocketConnection manages WebSocket connections to RPC endpoints
type WebSocketConnection interface {
	Connect(ctx context.Context, url string) error
	Subscribe(ctx context.Context, method string, params ...interface{}) (<-chan []byte, error)
	Close() error
	IsConnected() bool
	GetConnectionHealth() ConnectionHealth
}

// ConnectionHealth represents the health status of a connection
type ConnectionHealth struct {
	IsHealthy     bool
	LastPingTime  time.Time
	ResponseTime  time.Duration
	ErrorCount    int
	LastError     error
}

// TransactionStream processes incoming transaction data from WebSocket
type TransactionStream interface {
	ProcessTransaction(ctx context.Context, rawTx []byte) (*mevtypes.Transaction, error)
	FilterTransaction(tx *mevtypes.Transaction) bool
	ValidateTransaction(tx *mevtypes.Transaction) error
}

// ConnectionManager handles connection pooling and failover
type ConnectionManager interface {
	AddEndpoint(url string, priority int) error
	GetConnection(ctx context.Context) (WebSocketConnection, error)
	HandleConnectionFailure(conn WebSocketConnection) error
	GetHealthyConnections() []WebSocketConnection
}