package interfaces

import (
	"context"
	"net/http"
	"time"
)

// APIServer defines the interface for the REST API server
type APIServer interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	GetRouter() http.Handler
}

// WebSocketServer defines the interface for real-time opportunity streaming
type WebSocketServer interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	BroadcastOpportunity(opportunity *MEVOpportunity) error
	BroadcastMetrics(metrics *PerformanceMetrics) error
	GetConnectedClients() int
}

// AuthService defines the interface for API authentication
type AuthService interface {
	ValidateAPIKey(apiKey string) (*APIUser, error)
	GenerateAPIKey(userID string) (string, error)
	RevokeAPIKey(apiKey string) error
	GetAPIKeyInfo(apiKey string) (*APIKeyInfo, error)
}

// RateLimiter defines the interface for API rate limiting
type RateLimiter interface {
	Allow(clientID string) bool
	GetLimits(clientID string) *RateLimitInfo
	SetCustomLimit(clientID string, limit *RateLimit) error
}

// APIUser represents an authenticated API user
type APIUser struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	Role        UserRole  `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// APIKeyInfo contains information about an API key
type APIKeyInfo struct {
	KeyID       string    `json:"key_id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	IsActive    bool      `json:"is_active"`
}

// RateLimitInfo contains current rate limit status
type RateLimitInfo struct {
	Limit       int       `json:"limit"`
	Remaining   int       `json:"remaining"`
	ResetTime   time.Time `json:"reset_time"`
	WindowSize  time.Duration `json:"window_size"`
}

// RateLimit defines rate limiting configuration
type RateLimit struct {
	RequestsPerMinute int           `json:"requests_per_minute"`
	BurstSize         int           `json:"burst_size"`
	WindowSize        time.Duration `json:"window_size"`
}

// SystemStatus represents the overall system status
type SystemStatus struct {
	Status              string                 `json:"status"`
	Version             string                 `json:"version"`
	Uptime              time.Duration          `json:"uptime"`
	ConnectedToMempool  bool                   `json:"connected_to_mempool"`
	ActiveStrategies    []StrategyType         `json:"active_strategies"`
	QueueSize           int                    `json:"queue_size"`
	ActiveForks         int                    `json:"active_forks"`
	PerformanceMetrics  *PerformanceMetrics    `json:"performance_metrics"`
	SystemMetrics       *SystemMetrics         `json:"system_metrics"`
	LastUpdated         time.Time              `json:"last_updated"`
}

// OpportunityFilter defines filters for opportunity queries
type OpportunityFilter struct {
	Strategy    *StrategyType      `json:"strategy,omitempty"`
	Status      *OpportunityStatus `json:"status,omitempty"`
	MinProfit   *string            `json:"min_profit,omitempty"`
	MaxProfit   *string            `json:"max_profit,omitempty"`
	StartTime   *time.Time         `json:"start_time,omitempty"`
	EndTime     *time.Time         `json:"end_time,omitempty"`
	Limit       int                `json:"limit,omitempty"`
	Offset      int                `json:"offset,omitempty"`
}

// OpportunityResponse represents the API response for opportunities
type OpportunityResponse struct {
	Opportunities []*MEVOpportunity `json:"opportunities"`
	Total         int               `json:"total"`
	Limit         int               `json:"limit"`
	Offset        int               `json:"offset"`
}

// WebSocketMessage represents a WebSocket message
type WebSocketMessage struct {
	Type      MessageType `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// Enums
type UserRole string

const (
	UserRoleAdmin    UserRole = "admin"
	UserRoleOperator UserRole = "operator"
	UserRoleViewer   UserRole = "viewer"
)

type MessageType string

const (
	MessageTypeOpportunity MessageType = "opportunity"
	MessageTypeMetrics     MessageType = "metrics"
	MessageTypeStatus      MessageType = "status"
	MessageTypeAlert       MessageType = "alert"
)