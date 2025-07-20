package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// AuthService implements API authentication
type AuthService struct {
	apiKeys map[string]*interfaces.APIKeyInfo
	users   map[string]*interfaces.APIUser
	mutex   sync.RWMutex
}

// NewAuthService creates a new authentication service
func NewAuthService() *AuthService {
	service := &AuthService{
		apiKeys: make(map[string]*interfaces.APIKeyInfo),
		users:   make(map[string]*interfaces.APIUser),
	}
	
	// Create default admin user and API key for development
	adminUser := &interfaces.APIUser{
		ID:        "admin",
		Name:      "Administrator",
		Email:     "admin@mev-engine.local",
		Role:      interfaces.UserRoleAdmin,
		CreatedAt: time.Now(),
	}
	
	service.users[adminUser.ID] = adminUser
	
	// Generate default API key
	defaultKey := "mev_engine_dev_key_" + generateRandomString(32)
	service.apiKeys[defaultKey] = &interfaces.APIKeyInfo{
		KeyID:       "default",
		UserID:      adminUser.ID,
		Name:        "Default Development Key",
		Permissions: []string{"read", "write", "admin"},
		CreatedAt:   time.Now(),
		IsActive:    true,
	}
	
	fmt.Printf("Default API Key: %s\n", defaultKey)
	
	return service
}

// ValidateAPIKey validates an API key and returns the associated user
func (a *AuthService) ValidateAPIKey(apiKey string) (*interfaces.APIUser, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	
	keyInfo, exists := a.apiKeys[apiKey]
	if !exists {
		return nil, fmt.Errorf("invalid API key")
	}
	
	if !keyInfo.IsActive {
		return nil, fmt.Errorf("API key is inactive")
	}
	
	if keyInfo.ExpiresAt != nil && time.Now().After(*keyInfo.ExpiresAt) {
		return nil, fmt.Errorf("API key has expired")
	}
	
	user, exists := a.users[keyInfo.UserID]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}
	
	// Update last used timestamp
	now := time.Now()
	keyInfo.LastUsedAt = &now
	
	return user, nil
}

// GenerateAPIKey generates a new API key for a user
func (a *AuthService) GenerateAPIKey(userID string) (string, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	
	user, exists := a.users[userID]
	if !exists {
		return "", fmt.Errorf("user not found")
	}
	
	apiKey := "mev_engine_" + generateRandomString(48)
	keyID := generateRandomString(16)
	
	keyInfo := &interfaces.APIKeyInfo{
		KeyID:       keyID,
		UserID:      userID,
		Name:        fmt.Sprintf("API Key for %s", user.Name),
		Permissions: getDefaultPermissions(user.Role),
		CreatedAt:   time.Now(),
		IsActive:    true,
	}
	
	a.apiKeys[apiKey] = keyInfo
	
	return apiKey, nil
}

// RevokeAPIKey revokes an API key
func (a *AuthService) RevokeAPIKey(apiKey string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	
	keyInfo, exists := a.apiKeys[apiKey]
	if !exists {
		return fmt.Errorf("API key not found")
	}
	
	keyInfo.IsActive = false
	return nil
}

// GetAPIKeyInfo returns information about an API key
func (a *AuthService) GetAPIKeyInfo(apiKey string) (*interfaces.APIKeyInfo, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	
	keyInfo, exists := a.apiKeys[apiKey]
	if !exists {
		return nil, fmt.Errorf("API key not found")
	}
	
	return keyInfo, nil
}

// AuthMiddleware provides authentication middleware for HTTP handlers
func (a *AuthService) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract API key from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}
		
		// Expected format: "Bearer <api_key>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}
		
		apiKey := parts[1]
		user, err := a.ValidateAPIKey(apiKey)
		if err != nil {
			http.Error(w, fmt.Sprintf("Authentication failed: %v", err), http.StatusUnauthorized)
			return
		}
		
		// Add user to request context
		ctx := context.WithValue(r.Context(), "user", user)
		ctx = context.WithValue(ctx, "api_key", apiKey)
		
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole middleware ensures the user has the required role
func RequireRole(role interfaces.UserRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := r.Context().Value("user").(*interfaces.APIUser)
			if !ok {
				http.Error(w, "User not found in context", http.StatusInternalServerError)
				return
			}
			
			if !hasRequiredRole(user.Role, role) {
				http.Error(w, "Insufficient permissions", http.StatusForbidden)
				return
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

// Helper functions

func generateRandomString(length int) string {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}

func getDefaultPermissions(role interfaces.UserRole) []string {
	switch role {
	case interfaces.UserRoleAdmin:
		return []string{"read", "write", "admin"}
	case interfaces.UserRoleOperator:
		return []string{"read", "write"}
	case interfaces.UserRoleViewer:
		return []string{"read"}
	default:
		return []string{"read"}
	}
}

func hasRequiredRole(userRole, requiredRole interfaces.UserRole) bool {
	roleHierarchy := map[interfaces.UserRole]int{
		interfaces.UserRoleViewer:   1,
		interfaces.UserRoleOperator: 2,
		interfaces.UserRoleAdmin:    3,
	}
	
	userLevel, userExists := roleHierarchy[userRole]
	requiredLevel, requiredExists := roleHierarchy[requiredRole]
	
	if !userExists || !requiredExists {
		return false
	}
	
	return userLevel >= requiredLevel
}