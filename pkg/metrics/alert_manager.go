package metrics

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// AlertManager implements the AlertManager interface
type AlertManager struct {
	mu sync.RWMutex
	
	// Alert storage
	alerts     map[string]*interfaces.Alert
	alertRules map[string]*interfaces.AlertRule
	
	// Configuration
	config *AlertManagerConfig
	
	// Metrics collector for checking conditions
	metricsCollector interfaces.MetricsCollector
	
	// Alert channels
	alertChan chan *interfaces.Alert
	stopChan  chan struct{}
	
	// Running state
	running bool
}

// AlertManagerConfig contains configuration for the alert manager
type AlertManagerConfig struct {
	// Alert retention
	MaxAlerts        int
	AlertRetention   time.Duration
	
	// Notification settings
	EnableLogging    bool
	EnableWebhooks   bool
	WebhookURL       string
	WebhookTimeout   time.Duration
	
	// Check intervals
	CheckInterval    time.Duration
	CleanupInterval  time.Duration
}

// NewAlertManager creates a new alert manager
func NewAlertManager(config *AlertManagerConfig, metricsCollector interfaces.MetricsCollector) *AlertManager {
	if config == nil {
		config = &AlertManagerConfig{
			MaxAlerts:       1000,
			AlertRetention:  24 * time.Hour,
			EnableLogging:   true,
			EnableWebhooks:  false,
			CheckInterval:   10 * time.Second,
			CleanupInterval: time.Hour,
		}
	}
	
	return &AlertManager{
		alerts:           make(map[string]*interfaces.Alert),
		alertRules:       make(map[string]*interfaces.AlertRule),
		config:           config,
		metricsCollector: metricsCollector,
		alertChan:        make(chan *interfaces.Alert, 100),
		stopChan:         make(chan struct{}),
	}
}

// Start starts the alert manager background processes
func (am *AlertManager) Start(ctx context.Context) error {
	am.mu.Lock()
	if am.running {
		am.mu.Unlock()
		return fmt.Errorf("alert manager is already running")
	}
	am.running = true
	am.mu.Unlock()
	
	// Start alert processing goroutine
	go am.processAlerts(ctx)
	
	// Start rule checking goroutine
	go am.checkRules(ctx)
	
	// Start cleanup goroutine
	go am.cleanup(ctx)
	
	return nil
}

// Stop stops the alert manager
func (am *AlertManager) Stop() error {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	if !am.running {
		return fmt.Errorf("alert manager is not running")
	}
	
	close(am.stopChan)
	am.running = false
	
	return nil
}

// SendAlert sends an alert
func (am *AlertManager) SendAlert(ctx context.Context, alert *interfaces.Alert) error {
	if alert == nil {
		return fmt.Errorf("alert cannot be nil")
	}
	
	// Generate ID if not provided
	if alert.ID == "" {
		alert.ID = fmt.Sprintf("alert_%d", time.Now().UnixNano())
	}
	
	// Set creation time if not provided
	if alert.CreatedAt.IsZero() {
		alert.CreatedAt = time.Now()
	}
	
	// Store alert
	am.mu.Lock()
	am.alerts[alert.ID] = alert
	
	// Maintain max alerts limit
	if len(am.alerts) > am.config.MaxAlerts {
		am.evictOldestAlert()
	}
	am.mu.Unlock()
	
	// Send to processing channel
	select {
	case am.alertChan <- alert:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("alert channel is full")
	}
}

// RegisterAlertRule registers an alert rule
func (am *AlertManager) RegisterAlertRule(rule *interfaces.AlertRule) error {
	if rule == nil {
		return fmt.Errorf("alert rule cannot be nil")
	}
	
	if rule.ID == "" {
		return fmt.Errorf("alert rule ID cannot be empty")
	}
	
	// Set creation time if not provided
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = time.Now()
	}
	
	am.mu.Lock()
	am.alertRules[rule.ID] = rule
	am.mu.Unlock()
	
	return nil
}

// GetActiveAlerts returns all active (unresolved) alerts
func (am *AlertManager) GetActiveAlerts() ([]*interfaces.Alert, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	var activeAlerts []*interfaces.Alert
	for _, alert := range am.alerts {
		if alert.ResolvedAt == nil {
			activeAlerts = append(activeAlerts, alert)
		}
	}
	
	return activeAlerts, nil
}

// AcknowledgeAlert acknowledges an alert
func (am *AlertManager) AcknowledgeAlert(alertID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	alert, exists := am.alerts[alertID]
	if !exists {
		return fmt.Errorf("alert with ID %s not found", alertID)
	}
	
	now := time.Now()
	alert.AcknowledgedAt = &now
	
	return nil
}

// ResolveAlert resolves an alert
func (am *AlertManager) ResolveAlert(alertID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	alert, exists := am.alerts[alertID]
	if !exists {
		return fmt.Errorf("alert with ID %s not found", alertID)
	}
	
	now := time.Now()
	alert.ResolvedAt = &now
	
	return nil
}

// processAlerts processes incoming alerts
func (am *AlertManager) processAlerts(ctx context.Context) {
	for {
		select {
		case alert := <-am.alertChan:
			am.handleAlert(ctx, alert)
		case <-am.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// handleAlert handles a single alert
func (am *AlertManager) handleAlert(ctx context.Context, alert *interfaces.Alert) {
	// Log alert if enabled
	if am.config.EnableLogging {
		log.Printf("ALERT [%s] %s: %s", alert.Severity, alert.Type, alert.Message)
		if alert.Details != nil {
			log.Printf("Alert details: %+v", alert.Details)
		}
	}
	
	// Send webhook if enabled
	if am.config.EnableWebhooks && am.config.WebhookURL != "" {
		go am.sendWebhook(ctx, alert)
	}
}

// sendWebhook sends alert to webhook endpoint
func (am *AlertManager) sendWebhook(ctx context.Context, alert *interfaces.Alert) {
	// This would implement webhook sending logic
	// For now, just log that we would send a webhook
	if am.config.EnableLogging {
		log.Printf("Would send webhook for alert %s to %s", alert.ID, am.config.WebhookURL)
	}
}

// checkRules periodically checks alert rules
func (am *AlertManager) checkRules(ctx context.Context) {
	ticker := time.NewTicker(am.config.CheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			am.evaluateRules(ctx)
		case <-am.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// evaluateRules evaluates all active alert rules
func (am *AlertManager) evaluateRules(ctx context.Context) {
	am.mu.RLock()
	rules := make([]*interfaces.AlertRule, 0, len(am.alertRules))
	for _, rule := range am.alertRules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}
	am.mu.RUnlock()
	
	for _, rule := range rules {
		am.evaluateRule(ctx, rule)
	}
}

// evaluateRule evaluates a single alert rule
func (am *AlertManager) evaluateRule(ctx context.Context, rule *interfaces.AlertRule) {
	switch rule.Type {
	case interfaces.AlertTypeProfitability:
		am.evaluateProfitabilityRule(ctx, rule)
	case interfaces.AlertTypeLatency:
		am.evaluateLatencyRule(ctx, rule)
	case interfaces.AlertTypeSystem:
		am.evaluateSystemRule(ctx, rule)
	case interfaces.AlertTypeConnection:
		am.evaluateConnectionRule(ctx, rule)
	}
}

// evaluateProfitabilityRule evaluates profitability-based alert rules
func (am *AlertManager) evaluateProfitabilityRule(ctx context.Context, rule *interfaces.AlertRule) {
	if am.metricsCollector == nil {
		return
	}
	
	// Get profitability metrics for the specified window size
	metrics, err := am.metricsCollector.GetProfitabilityMetrics(rule.WindowSize)
	if err != nil {
		return
	}
	
	var currentValue float64
	var alertMessage string
	
	switch rule.Condition {
	case "loss_rate":
		currentValue = metrics.LossRate
		alertMessage = fmt.Sprintf("Loss rate %.2f%% exceeds threshold %.2f%% over %d trades", 
			currentValue*100, rule.Threshold*100, rule.WindowSize)
	case "success_rate":
		currentValue = metrics.SuccessRate
		alertMessage = fmt.Sprintf("Success rate %.2f%% below threshold %.2f%% over %d trades", 
			currentValue*100, rule.Threshold*100, rule.WindowSize)
	default:
		return
	}
	
	// Check if threshold is exceeded
	shouldAlert := false
	if rule.Condition == "loss_rate" && currentValue > rule.Threshold {
		shouldAlert = true
	} else if rule.Condition == "success_rate" && currentValue < rule.Threshold {
		shouldAlert = true
	}
	
	if shouldAlert {
		alert := &interfaces.Alert{
			Type:     interfaces.AlertTypeProfitability,
			Severity: am.getSeverityForThreshold(currentValue, rule.Threshold),
			Message:  alertMessage,
			Details: map[string]interface{}{
				"rule_id":       rule.ID,
				"condition":     rule.Condition,
				"current_value": currentValue,
				"threshold":     rule.Threshold,
				"window_size":   rule.WindowSize,
				"total_trades":  metrics.TotalTrades,
			},
		}
		
		am.SendAlert(ctx, alert)
	}
}

// evaluateLatencyRule evaluates latency-based alert rules
func (am *AlertManager) evaluateLatencyRule(ctx context.Context, rule *interfaces.AlertRule) {
	if am.metricsCollector == nil {
		return
	}
	
	// This would evaluate latency rules - implementation depends on specific latency metrics
	// For now, just a placeholder
}

// evaluateSystemRule evaluates system-based alert rules
func (am *AlertManager) evaluateSystemRule(ctx context.Context, rule *interfaces.AlertRule) {
	if am.metricsCollector == nil {
		return
	}
	
	// This would evaluate system rules - implementation depends on specific system metrics
	// For now, just a placeholder
}

// evaluateConnectionRule evaluates connection-based alert rules
func (am *AlertManager) evaluateConnectionRule(ctx context.Context, rule *interfaces.AlertRule) {
	// This would evaluate connection rules - implementation depends on connection status
	// For now, just a placeholder
}

// getSeverityForThreshold determines alert severity based on how much threshold is exceeded
func (am *AlertManager) getSeverityForThreshold(currentValue, threshold float64) interfaces.AlertSeverity {
	ratio := currentValue / threshold
	
	if ratio >= 2.0 {
		return interfaces.AlertSeverityCritical
	} else if ratio >= 1.5 {
		return interfaces.AlertSeverityError
	} else if ratio >= 1.2 {
		return interfaces.AlertSeverityWarning
	}
	
	return interfaces.AlertSeverityInfo
}

// cleanup periodically cleans up old alerts
func (am *AlertManager) cleanup(ctx context.Context) {
	ticker := time.NewTicker(am.config.CleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			am.cleanupOldAlerts()
		case <-am.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// cleanupOldAlerts removes old alerts based on retention policy
func (am *AlertManager) cleanupOldAlerts() {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	cutoff := time.Now().Add(-am.config.AlertRetention)
	
	for id, alert := range am.alerts {
		if alert.CreatedAt.Before(cutoff) {
			delete(am.alerts, id)
		}
	}
}

// evictOldestAlert removes the oldest alert to maintain size limit
func (am *AlertManager) evictOldestAlert() {
	var oldestID string
	var oldestTime time.Time
	
	for id, alert := range am.alerts {
		if oldestID == "" || alert.CreatedAt.Before(oldestTime) {
			oldestID = id
			oldestTime = alert.CreatedAt
		}
	}
	
	if oldestID != "" {
		delete(am.alerts, oldestID)
	}
}

// CreateProfitabilityAlertRule creates a standard profitability alert rule
func (am *AlertManager) CreateProfitabilityAlertRule(id, condition string, threshold float64, windowSize int) *interfaces.AlertRule {
	return &interfaces.AlertRule{
		ID:         id,
		Name:       fmt.Sprintf("Profitability Alert - %s", condition),
		Type:       interfaces.AlertTypeProfitability,
		Condition:  condition,
		Threshold:  threshold,
		WindowSize: windowSize,
		Enabled:    true,
		CreatedAt:  time.Now(),
	}
}