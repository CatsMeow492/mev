package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// Handlers contains all HTTP handlers for the API
type Handlers struct {
	strategyEngine interfaces.StrategyEngine
	metricsCollector interfaces.MetricsCollector
	shutdownManager interfaces.ShutdownManager
}

// NewHandlers creates a new handlers instance
func NewHandlers(
	strategyEngine interfaces.StrategyEngine,
	metricsCollector interfaces.MetricsCollector,
	shutdownManager interfaces.ShutdownManager,
) *Handlers {
	return &Handlers{
		strategyEngine:   strategyEngine,
		metricsCollector: metricsCollector,
		shutdownManager:  shutdownManager,
	}
}

// GetSystemStatus returns the current system status
func (h *Handlers) GetSystemStatus(w http.ResponseWriter, r *http.Request) {
	performanceMetrics, err := h.metricsCollector.GetPerformanceMetrics()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get performance metrics: %v", err), http.StatusInternalServerError)
		return
	}

	systemMetrics, err := h.metricsCollector.GetSystemMetrics()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get system metrics: %v", err), http.StatusInternalServerError)
		return
	}

	shutdownStatus, err := h.shutdownManager.GetShutdownStatus()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get shutdown status: %v", err), http.StatusInternalServerError)
		return
	}

	status := &interfaces.SystemStatus{
		Status:             getSystemStatusString(shutdownStatus, performanceMetrics),
		Version:            "1.0.0", // TODO: Get from build info
		Uptime:             time.Since(time.Now().Add(-24 * time.Hour)), // TODO: Track actual uptime
		ConnectedToMempool: true, // TODO: Get from mempool watcher
		ActiveStrategies:   h.strategyEngine.GetActiveStrategies(),
		QueueSize:          0, // TODO: Get from queue manager
		ActiveForks:        0, // TODO: Get from simulation engine
		PerformanceMetrics: performanceMetrics,
		SystemMetrics:      systemMetrics,
		LastUpdated:        time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// GetOpportunities returns MEV opportunities with optional filtering
func (h *Handlers) GetOpportunities(w http.ResponseWriter, r *http.Request) {
	filter := parseOpportunityFilter(r)
	
	// TODO: Implement opportunity storage and retrieval
	// For now, return empty response
	response := &interfaces.OpportunityResponse{
		Opportunities: []*interfaces.MEVOpportunity{},
		Total:         0,
		Limit:         filter.Limit,
		Offset:        filter.Offset,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetOpportunityByID returns a specific MEV opportunity
func (h *Handlers) GetOpportunityByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	opportunityID := vars["id"]

	// TODO: Implement opportunity retrieval by ID
	// For now, return 404
	http.Error(w, fmt.Sprintf("Opportunity %s not found", opportunityID), http.StatusNotFound)
}

// GetMetrics returns performance metrics
func (h *Handlers) GetMetrics(w http.ResponseWriter, r *http.Request) {
	windowSizeStr := r.URL.Query().Get("window_size")
	windowSize := 100 // default
	if windowSizeStr != "" {
		if ws, err := strconv.Atoi(windowSizeStr); err == nil && ws > 0 {
			windowSize = ws
		}
	}

	profitabilityMetrics, err := h.metricsCollector.GetProfitabilityMetrics(windowSize)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get profitability metrics: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profitabilityMetrics)
}

// GetLatencyMetrics returns latency metrics for a specific operation
func (h *Handlers) GetLatencyMetrics(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	operation := vars["operation"]

	windowSizeStr := r.URL.Query().Get("window_size")
	windowSize := 100 // default
	if windowSizeStr != "" {
		if ws, err := strconv.Atoi(windowSizeStr); err == nil && ws > 0 {
			windowSize = ws
		}
	}

	latencyMetrics, err := h.metricsCollector.GetLatencyMetrics(operation, windowSize)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get latency metrics: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(latencyMetrics)
}

// GetStrategies returns active strategies and their configurations
func (h *Handlers) GetStrategies(w http.ResponseWriter, r *http.Request) {
	activeStrategies := h.strategyEngine.GetActiveStrategies()
	
	response := map[string]interface{}{
		"active_strategies": activeStrategies,
		"total_count":      len(activeStrategies),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateStrategyConfig updates configuration for a specific strategy
func (h *Handlers) UpdateStrategyConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	strategyStr := vars["strategy"]
	
	strategy := interfaces.StrategyType(strategyStr)
	
	var config interface{}
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if err := h.strategyEngine.UpdateStrategyConfig(strategy, config); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update strategy config: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// EnableStrategy enables a specific strategy
func (h *Handlers) EnableStrategy(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	strategyStr := vars["strategy"]
	
	strategy := interfaces.StrategyType(strategyStr)
	
	if err := h.strategyEngine.EnableStrategy(strategy); err != nil {
		http.Error(w, fmt.Sprintf("Failed to enable strategy: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "enabled"})
}

// DisableStrategy disables a specific strategy
func (h *Handlers) DisableStrategy(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	strategyStr := vars["strategy"]
	
	strategy := interfaces.StrategyType(strategyStr)
	
	if err := h.strategyEngine.DisableStrategy(strategy); err != nil {
		http.Error(w, fmt.Sprintf("Failed to disable strategy: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "disabled"})
}

// GetPrometheusMetrics returns metrics in Prometheus format
func (h *Handlers) GetPrometheusMetrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := h.metricsCollector.GetPrometheusMetrics()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get Prometheus metrics: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	
	// Convert metrics map to Prometheus format
	for name, value := range metrics {
		fmt.Fprintf(w, "# HELP %s MEV Engine metric\n", name)
		fmt.Fprintf(w, "# TYPE %s gauge\n", name)
		fmt.Fprintf(w, "%s %v\n", name, value)
	}
}

// Helper functions

func parseOpportunityFilter(r *http.Request) *interfaces.OpportunityFilter {
	filter := &interfaces.OpportunityFilter{}
	
	if strategy := r.URL.Query().Get("strategy"); strategy != "" {
		s := interfaces.StrategyType(strategy)
		filter.Strategy = &s
	}
	
	if status := r.URL.Query().Get("status"); status != "" {
		s := interfaces.OpportunityStatus(status)
		filter.Status = &s
	}
	
	if minProfit := r.URL.Query().Get("min_profit"); minProfit != "" {
		filter.MinProfit = &minProfit
	}
	
	if maxProfit := r.URL.Query().Get("max_profit"); maxProfit != "" {
		filter.MaxProfit = &maxProfit
	}
	
	if startTime := r.URL.Query().Get("start_time"); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			filter.StartTime = &t
		}
	}
	
	if endTime := r.URL.Query().Get("end_time"); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			filter.EndTime = &t
		}
	}
	
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			filter.Limit = l
		}
	}
	if filter.Limit == 0 {
		filter.Limit = 50 // default
	}
	
	if offset := r.URL.Query().Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			filter.Offset = o
		}
	}
	
	return filter
}

func getSystemStatusString(shutdownStatus *interfaces.ShutdownStatus, performanceMetrics *interfaces.PerformanceMetrics) string {
	if shutdownStatus.IsShutdown {
		return "shutdown"
	}
	
	if performanceMetrics.ShutdownPending {
		return "shutdown_pending"
	}
	
	if performanceMetrics.WarningMode {
		return "warning"
	}
	
	if performanceMetrics.IsHealthy {
		return "healthy"
	}
	
	return "unknown"
}