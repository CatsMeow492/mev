package api

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	clients map[string]*ClientBucket
	mutex   sync.RWMutex
	
	// Default limits
	defaultLimit *interfaces.RateLimit
}

// ClientBucket represents a token bucket for a specific client
type ClientBucket struct {
	tokens     int
	lastRefill time.Time
	limit      *interfaces.RateLimit
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		clients: make(map[string]*ClientBucket),
		defaultLimit: &interfaces.RateLimit{
			RequestsPerMinute: 100,
			BurstSize:         20,
			WindowSize:        time.Minute,
		},
	}
}

// Allow checks if a request should be allowed for the given client
func (rl *RateLimiter) Allow(clientID string) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	
	bucket, exists := rl.clients[clientID]
	if !exists {
		bucket = &ClientBucket{
			tokens:     rl.defaultLimit.BurstSize,
			lastRefill: time.Now(),
			limit:      rl.defaultLimit,
		}
		rl.clients[clientID] = bucket
	}
	
	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill)
	
	if elapsed >= bucket.limit.WindowSize {
		// Full refill
		bucket.tokens = bucket.limit.BurstSize
		bucket.lastRefill = now
	} else {
		// Partial refill based on elapsed time
		tokensToAdd := int(elapsed.Seconds() * float64(bucket.limit.RequestsPerMinute) / 60.0)
		bucket.tokens += tokensToAdd
		if bucket.tokens > bucket.limit.BurstSize {
			bucket.tokens = bucket.limit.BurstSize
		}
		bucket.lastRefill = now
	}
	
	// Check if request can be allowed
	if bucket.tokens > 0 {
		bucket.tokens--
		return true
	}
	
	return false
}

// GetLimits returns the current rate limit status for a client
func (rl *RateLimiter) GetLimits(clientID string) *interfaces.RateLimitInfo {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()
	
	bucket, exists := rl.clients[clientID]
	if !exists {
		return &interfaces.RateLimitInfo{
			Limit:      rl.defaultLimit.RequestsPerMinute,
			Remaining:  rl.defaultLimit.BurstSize,
			ResetTime:  time.Now().Add(rl.defaultLimit.WindowSize),
			WindowSize: rl.defaultLimit.WindowSize,
		}
	}
	
	return &interfaces.RateLimitInfo{
		Limit:      bucket.limit.RequestsPerMinute,
		Remaining:  bucket.tokens,
		ResetTime:  bucket.lastRefill.Add(bucket.limit.WindowSize),
		WindowSize: bucket.limit.WindowSize,
	}
}

// SetCustomLimit sets a custom rate limit for a specific client
func (rl *RateLimiter) SetCustomLimit(clientID string, limit *interfaces.RateLimit) error {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	
	bucket, exists := rl.clients[clientID]
	if !exists {
		bucket = &ClientBucket{
			tokens:     limit.BurstSize,
			lastRefill: time.Now(),
			limit:      limit,
		}
		rl.clients[clientID] = bucket
	} else {
		bucket.limit = limit
		// Reset tokens to new burst size
		bucket.tokens = limit.BurstSize
		bucket.lastRefill = time.Now()
	}
	
	return nil
}

// RateLimitMiddleware provides rate limiting middleware for HTTP handlers
func (rl *RateLimiter) RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get client ID from API key or IP address
		clientID := getClientID(r)
		
		if !rl.Allow(clientID) {
			// Add rate limit headers
			limits := rl.GetLimits(clientID)
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limits.Limit))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", limits.Remaining))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", limits.ResetTime.Unix()))
			
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		
		// Add rate limit headers to successful requests
		limits := rl.GetLimits(clientID)
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limits.Limit))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", limits.Remaining))
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", limits.ResetTime.Unix()))
		
		next.ServeHTTP(w, r)
	})
}

// getClientID extracts a client identifier from the request
func getClientID(r *http.Request) string {
	// Try to get API key from context first
	if apiKey, ok := r.Context().Value("api_key").(string); ok {
		return apiKey
	}
	
	// Fall back to IP address
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	
	// Extract just the IP part (remove port)
	if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
		clientIP = clientIP[:idx]
	}
	
	return clientIP
}

// CleanupExpiredClients removes expired client buckets (should be called periodically)
func (rl *RateLimiter) CleanupExpiredClients() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	
	now := time.Now()
	for clientID, bucket := range rl.clients {
		// Remove clients that haven't been active for 1 hour
		if now.Sub(bucket.lastRefill) > time.Hour {
			delete(rl.clients, clientID)
		}
	}
}