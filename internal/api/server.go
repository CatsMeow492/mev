package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/mev-engine/l2-mev-strategy-engine/internal/config"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// Server implements the REST API server
type Server struct {
	config           *config.Config
	server           *http.Server
	handlers         *Handlers
	authService      *AuthService
	rateLimiter      *RateLimiter
	websocketServer  *WebSocketServer
	
	// Dependencies
	strategyEngine   interfaces.StrategyEngine
	metricsCollector interfaces.MetricsCollector
	shutdownManager  interfaces.ShutdownManager
}

// NewServer creates a new API server
func NewServer(
	cfg *config.Config,
	strategyEngine interfaces.StrategyEngine,
	metricsCollector interfaces.MetricsCollector,
	shutdownManager interfaces.ShutdownManager,
) *Server {
	authService := NewAuthService()
	rateLimiter := NewRateLimiter()
	websocketServer := NewWebSocketServer()
	
	handlers := NewHandlers(strategyEngine, metricsCollector, shutdownManager)
	
	server := &Server{
		config:           cfg,
		handlers:         handlers,
		authService:      authService,
		rateLimiter:      rateLimiter,
		websocketServer:  websocketServer,
		strategyEngine:   strategyEngine,
		metricsCollector: metricsCollector,
		shutdownManager:  shutdownManager,
	}
	
	server.setupServer()
	
	return server
}

// Start starts the API server
func (s *Server) Start(ctx context.Context) error {
	log.Printf("Starting API server on %s:%d", s.config.Server.Host, s.config.Server.Port)
	
	// Start WebSocket server
	if err := s.websocketServer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start WebSocket server: %w", err)
	}
	
	// Start rate limiter cleanup routine
	go s.rateLimiterCleanup(ctx)
	
	// Start HTTP server
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("API server error: %v", err)
		}
	}()
	
	log.Println("API server started successfully")
	return nil
}

// Stop stops the API server
func (s *Server) Stop(ctx context.Context) error {
	log.Println("Stopping API server...")
	
	// Stop WebSocket server
	if err := s.websocketServer.Stop(ctx); err != nil {
		log.Printf("Error stopping WebSocket server: %v", err)
	}
	
	// Stop HTTP server
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown API server: %w", err)
	}
	
	log.Println("API server stopped")
	return nil
}

// GetRouter returns the HTTP router
func (s *Server) GetRouter() http.Handler {
	return s.server.Handler
}

// setupServer configures the HTTP server and routes
func (s *Server) setupServer() {
	router := mux.NewRouter()
	
	// Setup CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"}, // TODO: Configure properly for production
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})
	
	// Setup middleware chain
	router.Use(s.loggingMiddleware)
	router.Use(s.rateLimiter.RateLimitMiddleware)
	
	// Public routes (no authentication required)
	router.HandleFunc("/health", s.healthCheck).Methods("GET")
	router.HandleFunc("/metrics", s.handlers.GetPrometheusMetrics).Methods("GET")
	
	// WebSocket endpoint
	router.HandleFunc("/ws", s.websocketServer.HandleWebSocket)
	
	// Protected API routes
	api := router.PathPrefix("/api/v1").Subrouter()
	api.Use(s.authService.AuthMiddleware)
	
	// System status and information
	api.HandleFunc("/status", s.handlers.GetSystemStatus).Methods("GET")
	
	// Opportunities
	api.HandleFunc("/opportunities", s.handlers.GetOpportunities).Methods("GET")
	api.HandleFunc("/opportunities/{id}", s.handlers.GetOpportunityByID).Methods("GET")
	
	// Metrics
	api.HandleFunc("/metrics/profitability", s.handlers.GetMetrics).Methods("GET")
	api.HandleFunc("/metrics/latency/{operation}", s.handlers.GetLatencyMetrics).Methods("GET")
	
	// Strategies (read access)
	api.HandleFunc("/strategies", s.handlers.GetStrategies).Methods("GET")
	
	// Strategy management (operator+ access)
	operatorRoutes := api.PathPrefix("").Subrouter()
	operatorRoutes.Use(RequireRole(interfaces.UserRoleOperator))
	operatorRoutes.HandleFunc("/strategies/{strategy}/config", s.handlers.UpdateStrategyConfig).Methods("PUT")
	operatorRoutes.HandleFunc("/strategies/{strategy}/enable", s.handlers.EnableStrategy).Methods("POST")
	operatorRoutes.HandleFunc("/strategies/{strategy}/disable", s.handlers.DisableStrategy).Methods("POST")
	
	// Admin routes
	adminRoutes := api.PathPrefix("/admin").Subrouter()
	adminRoutes.Use(RequireRole(interfaces.UserRoleAdmin))
	// TODO: Add admin-specific endpoints (user management, system config, etc.)
	
	// Apply CORS wrapper
	handler := c.Handler(router)
	
	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port),
		Handler:      handler,
		ReadTimeout:  s.config.Server.ReadTimeout,
		WriteTimeout: s.config.Server.WriteTimeout,
		IdleTimeout:  s.config.Server.IdleTimeout,
	}
}

// healthCheck provides a simple health check endpoint
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"version":   "1.0.0",
		"websocket_clients": s.websocketServer.GetConnectedClients(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(health); err != nil {
		log.Printf("Error encoding health check response: %v", err)
	}
}

// loggingMiddleware logs HTTP requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Create a response writer wrapper to capture status code
		wrapper := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		next.ServeHTTP(wrapper, r)
		
		duration := time.Since(start)
		
		log.Printf("%s %s %d %v %s",
			r.Method,
			r.RequestURI,
			wrapper.statusCode,
			duration,
			r.RemoteAddr,
		)
	})
}

// rateLimiterCleanup periodically cleans up expired rate limiter entries
func (s *Server) rateLimiterCleanup(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.rateLimiter.CleanupExpiredClients()
		}
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}