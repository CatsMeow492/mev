package metrics

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// ShutdownManager implements the ShutdownManager interface with circuit breaker pattern
type ShutdownManager struct {
	mu sync.RWMutex
	
	// Dependencies
	metricsCollector interfaces.MetricsCollector
	alertManager     interfaces.AlertManager
	
	// Configuration
	config *ShutdownManagerConfig
	
	// State
	status           *interfaces.ShutdownStatus
	lastCheck        time.Time
	warningTriggered bool
	
	// Circuit breaker state
	circuitState     CircuitState
	failureCount     int
	lastFailureTime  time.Time
	
	// Shutdown callbacks
	shutdownCallbacks []ShutdownCallback
	
	// Background processing
	stopChan chan struct{}
	running  bool
}

// ShutdownManagerConfig contains configuration for the shutdown manager
type ShutdownManagerConfig struct {
	// Shutdown thresholds
	WarningLossRate    float64 // 0.70 (70%)
	ShutdownLossRate   float64 // 0.80 (80%)
	WarningWindowSize  int     // 100 trades
	ShutdownWindowSize int     // 50 trades
	
	// Minimum trades required before checking
	MinTradesForWarning  int // 100
	MinTradesForShutdown int // 50
	
	// Check intervals
	CheckInterval    time.Duration
	AlertCooldown    time.Duration
	
	// Circuit breaker settings
	FailureThreshold    int           // Number of consecutive failures before opening circuit
	RecoveryTimeout     time.Duration // Time to wait before attempting recovery
	HalfOpenMaxRetries  int           // Max retries in half-open state
	
	// Manual override settings
	AllowManualOverride bool
	OverrideTimeout     time.Duration
}

// CircuitState represents the state of the circuit breaker
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// ShutdownCallback is called when shutdown is initiated
type ShutdownCallback func(ctx context.Context, reason string) error

// NewShutdownManager creates a new shutdown manager
func NewShutdownManager(config *ShutdownManagerConfig, metricsCollector interfaces.MetricsCollector, alertManager interfaces.AlertManager) *ShutdownManager {
	if config == nil {
		config = &ShutdownManagerConfig{
			WarningLossRate:     0.70,
			ShutdownLossRate:    0.80,
			WarningWindowSize:   100,
			ShutdownWindowSize:  50,
			MinTradesForWarning: 100,
			MinTradesForShutdown: 50,
			CheckInterval:       30 * time.Second,
			AlertCooldown:       5 * time.Minute,
			FailureThreshold:    3,
			RecoveryTimeout:     5 * time.Minute,
			HalfOpenMaxRetries:  3,
			AllowManualOverride: true,
			OverrideTimeout:     time.Hour,
		}
	}
	
	return &ShutdownManager{
		metricsCollector: metricsCollector,
		alertManager:     alertManager,
		config:           config,
		status: &interfaces.ShutdownStatus{
			IsShutdown:     false,
			CanRestart:     true,
			ManualOverride: false,
		},
		circuitState:      CircuitClosed,
		shutdownCallbacks: make([]ShutdownCallback, 0),
		stopChan:          make(chan struct{}),
	}
}

// Start starts the shutdown manager background monitoring
func (sm *ShutdownManager) Start(ctx context.Context) error {
	sm.mu.Lock()
	if sm.running {
		sm.mu.Unlock()
		return fmt.Errorf("shutdown manager is already running")
	}
	sm.running = true
	sm.mu.Unlock()
	
	// Start monitoring goroutine
	go sm.monitor(ctx)
	
	return nil
}

// Stop stops the shutdown manager
func (sm *ShutdownManager) Stop() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if !sm.running {
		return fmt.Errorf("shutdown manager is not running")
	}
	
	close(sm.stopChan)
	sm.running = false
	
	return nil
}

// CheckShutdownConditions checks if shutdown conditions are met
func (sm *ShutdownManager) CheckShutdownConditions(ctx context.Context) (*interfaces.ShutdownDecision, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	decision := &interfaces.ShutdownDecision{
		ShouldShutdown: false,
		Metrics:        make(map[string]float64),
		Timestamp:      time.Now(),
	}
	
	// Skip check if already shutdown
	if sm.status.IsShutdown {
		decision.Reason = "System already shutdown"
		return decision, nil
	}
	
	// Skip check if manual override is active
	if sm.status.ManualOverride {
		decision.Reason = "Manual override active"
		return decision, nil
	}
	
	// Check circuit breaker state
	if sm.circuitState == CircuitOpen {
		if time.Since(sm.lastFailureTime) < sm.config.RecoveryTimeout {
			decision.ShouldShutdown = true
			decision.Reason = "Circuit breaker is open"
			return decision, nil
		} else {
			// Transition to half-open
			sm.circuitState = CircuitHalfOpen
			sm.failureCount = 0
		}
	}
	
	// Get performance metrics
	if sm.metricsCollector == nil {
		return decision, fmt.Errorf("metrics collector not available")
	}
	
	performanceMetrics, err := sm.metricsCollector.GetPerformanceMetrics()
	if err != nil {
		sm.recordFailure()
		return decision, fmt.Errorf("failed to get performance metrics: %w", err)
	}
	
	// Check warning conditions (70% loss rate over 100 trades)
	warningMetrics, exists := performanceMetrics.TradeMetrics[sm.config.WarningWindowSize]
	if exists && warningMetrics.TotalTrades >= sm.config.MinTradesForWarning {
		decision.Metrics["warning_loss_rate"] = warningMetrics.LossRate
		decision.Metrics["warning_total_trades"] = float64(warningMetrics.TotalTrades)
		
		if warningMetrics.LossRate > sm.config.WarningLossRate {
			if !sm.warningTriggered {
				sm.warningTriggered = true
				sm.sendWarningAlert(ctx, warningMetrics)
			}
		} else {
			sm.warningTriggered = false
		}
	}
	
	// Check shutdown conditions (80% loss rate over 50 trades)
	shutdownMetrics, exists := performanceMetrics.TradeMetrics[sm.config.ShutdownWindowSize]
	if exists && shutdownMetrics.TotalTrades >= sm.config.MinTradesForShutdown {
		decision.Metrics["shutdown_loss_rate"] = shutdownMetrics.LossRate
		decision.Metrics["shutdown_total_trades"] = float64(shutdownMetrics.TotalTrades)
		
		if shutdownMetrics.LossRate > sm.config.ShutdownLossRate {
			decision.ShouldShutdown = true
			decision.Reason = fmt.Sprintf("Loss rate %.2f%% exceeds shutdown threshold %.2f%% over %d trades",
				shutdownMetrics.LossRate*100, sm.config.ShutdownLossRate*100, sm.config.ShutdownWindowSize)
		}
	}
	
	sm.lastCheck = time.Now()
	
	// Record success if we got here without errors
	if sm.circuitState == CircuitHalfOpen {
		sm.circuitState = CircuitClosed
		sm.failureCount = 0
	}
	
	return decision, nil
}

// InitiateShutdown initiates system shutdown
func (sm *ShutdownManager) InitiateShutdown(ctx context.Context, reason string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if sm.status.IsShutdown {
		return fmt.Errorf("system is already shutdown")
	}
	
	log.Printf("SHUTDOWN INITIATED: %s", reason)
	
	// Update status
	now := time.Now()
	sm.status.IsShutdown = true
	sm.status.ShutdownReason = reason
	sm.status.ShutdownTime = &now
	sm.status.CanRestart = true
	
	// Send shutdown alert
	sm.sendShutdownAlert(ctx, reason)
	
	// Execute shutdown callbacks
	for i, callback := range sm.shutdownCallbacks {
		if err := callback(ctx, reason); err != nil {
			log.Printf("Shutdown callback %d failed: %v", i, err)
		}
	}
	
	return nil
}

// GetShutdownStatus returns current shutdown status
func (sm *ShutdownManager) GetShutdownStatus() (*interfaces.ShutdownStatus, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	// Return a copy to prevent external modification
	status := &interfaces.ShutdownStatus{
		IsShutdown:     sm.status.IsShutdown,
		ShutdownReason: sm.status.ShutdownReason,
		ManualOverride: sm.status.ManualOverride,
		CanRestart:     sm.status.CanRestart,
	}
	
	if sm.status.ShutdownTime != nil {
		shutdownTime := *sm.status.ShutdownTime
		status.ShutdownTime = &shutdownTime
	}
	
	return status, nil
}

// SetManualOverride enables or disables manual override
func (sm *ShutdownManager) SetManualOverride(enabled bool) error {
	if !sm.config.AllowManualOverride {
		return fmt.Errorf("manual override is not allowed")
	}
	
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.status.ManualOverride = enabled
	
	if enabled {
		log.Printf("MANUAL OVERRIDE ENABLED - Automatic shutdown disabled")
		sm.sendManualOverrideAlert(context.Background(), true)
		
		// Set timeout for manual override if configured
		if sm.config.OverrideTimeout > 0 {
			go sm.scheduleOverrideTimeout()
		}
	} else {
		log.Printf("MANUAL OVERRIDE DISABLED - Automatic shutdown re-enabled")
		sm.sendManualOverrideAlert(context.Background(), false)
	}
	
	return nil
}

// AddShutdownCallback adds a callback to be executed during shutdown
func (sm *ShutdownManager) AddShutdownCallback(callback ShutdownCallback) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.shutdownCallbacks = append(sm.shutdownCallbacks, callback)
}

// Restart attempts to restart the system after shutdown
func (sm *ShutdownManager) Restart(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if !sm.status.IsShutdown {
		return fmt.Errorf("system is not shutdown")
	}
	
	if !sm.status.CanRestart {
		return fmt.Errorf("system cannot be restarted")
	}
	
	log.Printf("SYSTEM RESTART INITIATED")
	
	// Reset status
	sm.status.IsShutdown = false
	sm.status.ShutdownReason = ""
	sm.status.ShutdownTime = nil
	sm.warningTriggered = false
	
	// Reset circuit breaker
	sm.circuitState = CircuitClosed
	sm.failureCount = 0
	
	sm.sendRestartAlert(ctx)
	
	return nil
}

// monitor runs the background monitoring loop
func (sm *ShutdownManager) monitor(ctx context.Context) {
	ticker := time.NewTicker(sm.config.CheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			sm.performCheck(ctx)
		case <-sm.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// performCheck performs a shutdown condition check
func (sm *ShutdownManager) performCheck(ctx context.Context) {
	decision, err := sm.CheckShutdownConditions(ctx)
	if err != nil {
		log.Printf("Error checking shutdown conditions: %v", err)
		return
	}
	
	if decision.ShouldShutdown {
		if err := sm.InitiateShutdown(ctx, decision.Reason); err != nil {
			log.Printf("Error initiating shutdown: %v", err)
		}
	}
}

// recordFailure records a failure for circuit breaker logic
func (sm *ShutdownManager) recordFailure() {
	sm.failureCount++
	sm.lastFailureTime = time.Now()
	
	if sm.failureCount >= sm.config.FailureThreshold {
		sm.circuitState = CircuitOpen
		log.Printf("Circuit breaker opened after %d failures", sm.failureCount)
	}
}

// sendWarningAlert sends a warning alert
func (sm *ShutdownManager) sendWarningAlert(ctx context.Context, metrics *interfaces.ProfitabilityMetrics) {
	if sm.alertManager == nil {
		return
	}
	
	alert := &interfaces.Alert{
		Type:     interfaces.AlertTypeProfitability,
		Severity: interfaces.AlertSeverityWarning,
		Message: fmt.Sprintf("WARNING: Loss rate %.2f%% exceeds warning threshold %.2f%% over %d trades",
			metrics.LossRate*100, sm.config.WarningLossRate*100, sm.config.WarningWindowSize),
		Details: map[string]interface{}{
			"loss_rate":     metrics.LossRate,
			"threshold":     sm.config.WarningLossRate,
			"window_size":   sm.config.WarningWindowSize,
			"total_trades":  metrics.TotalTrades,
			"warning_mode":  true,
		},
	}
	
	sm.alertManager.SendAlert(ctx, alert)
}

// sendShutdownAlert sends a shutdown alert
func (sm *ShutdownManager) sendShutdownAlert(ctx context.Context, reason string) {
	if sm.alertManager == nil {
		return
	}
	
	alert := &interfaces.Alert{
		Type:     interfaces.AlertTypeShutdown,
		Severity: interfaces.AlertSeverityCritical,
		Message:  fmt.Sprintf("SYSTEM SHUTDOWN: %s", reason),
		Details: map[string]interface{}{
			"shutdown_reason": reason,
			"shutdown_time":   time.Now(),
			"can_restart":     sm.status.CanRestart,
		},
	}
	
	sm.alertManager.SendAlert(ctx, alert)
}

// sendManualOverrideAlert sends a manual override alert
func (sm *ShutdownManager) sendManualOverrideAlert(ctx context.Context, enabled bool) {
	if sm.alertManager == nil {
		return
	}
	
	var message string
	var severity interfaces.AlertSeverity
	
	if enabled {
		message = "Manual override ENABLED - Automatic shutdown disabled"
		severity = interfaces.AlertSeverityWarning
	} else {
		message = "Manual override DISABLED - Automatic shutdown re-enabled"
		severity = interfaces.AlertSeverityInfo
	}
	
	alert := &interfaces.Alert{
		Type:     interfaces.AlertTypeSystem,
		Severity: severity,
		Message:  message,
		Details: map[string]interface{}{
			"manual_override": enabled,
			"timestamp":       time.Now(),
		},
	}
	
	sm.alertManager.SendAlert(ctx, alert)
}

// sendRestartAlert sends a restart alert
func (sm *ShutdownManager) sendRestartAlert(ctx context.Context) {
	if sm.alertManager == nil {
		return
	}
	
	alert := &interfaces.Alert{
		Type:     interfaces.AlertTypeSystem,
		Severity: interfaces.AlertSeverityInfo,
		Message:  "System restart initiated",
		Details: map[string]interface{}{
			"restart_time": time.Now(),
		},
	}
	
	sm.alertManager.SendAlert(ctx, alert)
}

// scheduleOverrideTimeout schedules automatic override timeout
func (sm *ShutdownManager) scheduleOverrideTimeout() {
	time.Sleep(sm.config.OverrideTimeout)
	
	sm.mu.Lock()
	if sm.status.ManualOverride {
		sm.status.ManualOverride = false
		log.Printf("MANUAL OVERRIDE TIMEOUT - Automatic shutdown re-enabled")
		sm.sendManualOverrideAlert(context.Background(), false)
	}
	sm.mu.Unlock()
}

// GetCircuitBreakerStatus returns the current circuit breaker status
func (sm *ShutdownManager) GetCircuitBreakerStatus() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	return map[string]interface{}{
		"state":             sm.circuitState,
		"failure_count":     sm.failureCount,
		"last_failure_time": sm.lastFailureTime,
		"last_check":        sm.lastCheck,
	}
}