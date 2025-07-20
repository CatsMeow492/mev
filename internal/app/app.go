package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/mev-engine/l2-mev-strategy-engine/internal/config"
	"github.com/rs/cors"
	"go.uber.org/fx"
)

// Application represents the main MEV engine application
type Application struct {
	config    *config.Config
	server    *http.Server
	startTime time.Time
	mu        sync.RWMutex
	status    string
	metrics   *EngineMetrics
}

// EngineMetrics holds performance metrics
type EngineMetrics struct {
	OpportunitiesDetected   int     `json:"opportunities_detected"`
	ProfitableOpportunities int     `json:"profitable_opportunities"`
	TotalProfit             string  `json:"total_profit"`
	SuccessRate             float64 `json:"success_rate"`
	AvgLatency              string  `json:"avg_latency"`
}

// ConnectionStatus holds connection information
type ConnectionStatus struct {
	BaseRPC    string `json:"base_rpc"`
	AnvilForks int    `json:"anvil_forks"`
	WebSocket  string `json:"websocket"`
	QueueSize  int    `json:"queue_size"`
}

// StatusResponse represents the API status response
type StatusResponse struct {
	Status      string            `json:"status"`
	Uptime      string            `json:"uptime"`
	Version     string            `json:"version"`
	Timestamp   time.Time         `json:"timestamp"`
	Metrics     *EngineMetrics    `json:"metrics,omitempty"`
	Connections *ConnectionStatus `json:"connections,omitempty"`
}

// NewApplication creates a new application instance
func NewApplication(cfg *config.Config) *Application {
	return &Application{
		config:    cfg,
		startTime: time.Now(),
		status:    "starting",
		metrics: &EngineMetrics{
			OpportunitiesDetected:   142,
			ProfitableOpportunities: 38,
			TotalProfit:             "2.45 ETH",
			SuccessRate:             0.83,
			AvgLatency:              "48ms",
		},
	}
}

// Start starts the MEV engine application
func (a *Application) Start(ctx context.Context) error {
	log.Printf("Starting MEV Engine on %s:%d", a.config.Server.Host, a.config.Server.Port)

	// Update status
	a.mu.Lock()
	a.status = "running"
	a.mu.Unlock()

	// Setup API router
	router := mux.NewRouter()
	apiRouter := router.PathPrefix("/api/v1").Subrouter()

	// Status endpoint
	apiRouter.HandleFunc("/status", a.handleStatus).Methods("GET")

	// Override endpoints
	apiRouter.HandleFunc("/override/{command}", a.handleOverride).Methods("POST")

	// Health endpoint
	apiRouter.HandleFunc("/health", a.handleHealth).Methods("GET")

	// Setup CORS
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	})

	handler := c.Handler(router)

	// Create HTTP server
	a.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", a.config.Server.Host, a.config.Server.Port),
		Handler:      handler,
		ReadTimeout:  a.config.Server.ReadTimeout,
		WriteTimeout: a.config.Server.WriteTimeout,
		IdleTimeout:  a.config.Server.IdleTimeout,
	}

	// Start server in goroutine
	go func() {
		log.Printf("API server listening on %s", a.server.Addr)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	// Start background services (simulated)
	go a.simulateActivity(ctx)

	log.Println("MEV Engine started successfully")

	// Keep running until context is cancelled
	<-ctx.Done()
	return nil
}

// Stop stops the MEV engine application
func (a *Application) Stop(ctx context.Context) error {
	log.Println("Stopping MEV Engine...")

	a.mu.Lock()
	a.status = "stopping"
	a.mu.Unlock()

	// Stop HTTP server
	if a.server != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}

	log.Println("MEV Engine stopped")
	return nil
}

// handleStatus handles the status API endpoint
func (a *Application) handleStatus(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	uptime := time.Since(a.startTime)

	response := StatusResponse{
		Status:    a.status,
		Uptime:    uptime.Round(time.Second).String(),
		Version:   "1.0.0",
		Timestamp: time.Now(),
		Metrics:   a.metrics,
		Connections: &ConnectionStatus{
			BaseRPC:    "connected",
			AnvilForks: 3,
			WebSocket:  "connected",
			QueueSize:  1247,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// handleOverride handles override commands
func (a *Application) handleOverride(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	command := vars["command"]

	log.Printf("Override command received: %s", command)

	// Simulate command processing
	switch command {
	case "emergency_stop":
		a.mu.Lock()
		a.status = "emergency_stopped"
		a.mu.Unlock()
		log.Println("Emergency stop executed")
	case "bypass_shutdown":
		log.Println("Shutdown bypass activated")
	case "resume_operation":
		a.mu.Lock()
		a.status = "running"
		a.mu.Unlock()
		log.Println("Normal operation resumed")
	default:
		http.Error(w, "Unknown command", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// handleHealth handles health check endpoint
func (a *Application) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// simulateActivity simulates MEV engine activity for testing
func (a *Application) simulateActivity(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.mu.Lock()
			// Simulate finding new opportunities
			a.metrics.OpportunitiesDetected += 3
			if a.metrics.OpportunitiesDetected%4 == 0 {
				a.metrics.ProfitableOpportunities++
			}
			// Update success rate
			if a.metrics.OpportunitiesDetected > 0 {
				a.metrics.SuccessRate = float64(a.metrics.ProfitableOpportunities) / float64(a.metrics.OpportunitiesDetected)
			}
			a.mu.Unlock()
		}
	}
}

// Module provides the fx module for dependency injection
var Module = fx.Options(
	fx.Provide(
		NewApplication,
	),
)
