package processing

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// LatencyMonitorConfig holds configuration for the latency monitor
type LatencyMonitorConfig struct {
	WindowSize        int                      `json:"window_size"`
	CleanupInterval   time.Duration            `json:"cleanup_interval"`
	RetentionPeriod   time.Duration            `json:"retention_period"`
	AlertThresholds   map[string]time.Duration `json:"alert_thresholds"`
	EnablePercentiles bool                     `json:"enable_percentiles"`
}

// DefaultLatencyMonitorConfig returns default configuration
func DefaultLatencyMonitorConfig() *LatencyMonitorConfig {
	return &LatencyMonitorConfig{
		WindowSize:        1000,
		CleanupInterval:   5 * time.Minute,
		RetentionPeriod:   1 * time.Hour,
		EnablePercentiles: true,
		AlertThresholds: map[string]time.Duration{
			"process_transaction":  100 * time.Millisecond,
			"simulate_transaction": 50 * time.Millisecond,
			"detect_opportunities": 25 * time.Millisecond,
		},
	}
}

// latencyMonitor implements the LatencyMonitor interface
type latencyMonitor struct {
	config *LatencyMonitorConfig
	mu     sync.RWMutex

	// Operation tracking
	operations map[string]*operationTracker

	// Alert tracking
	alerts          []interfaces.PerformanceAlert
	alertsGenerated int64
}

// operationTracker tracks latency data for a specific operation
type operationTracker struct {
	name       string
	samples    []latencySample
	totalCount int64
	totalTime  time.Duration
	minLatency time.Duration
	maxLatency time.Duration
	lastUpdate time.Time
}

// latencySample represents a single latency measurement
type latencySample struct {
	timestamp time.Time
	duration  time.Duration
}

// NewLatencyMonitor creates a new latency monitor
func NewLatencyMonitor() interfaces.LatencyMonitor {
	config := DefaultLatencyMonitorConfig()

	lm := &latencyMonitor{
		config:     config,
		operations: make(map[string]*operationTracker),
		alerts:     make([]interfaces.PerformanceAlert, 0),
	}

	return lm
}

// RecordLatency records a latency measurement for an operation
func (lm *latencyMonitor) RecordLatency(operation string, duration time.Duration) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	tracker, exists := lm.operations[operation]
	if !exists {
		tracker = &operationTracker{
			name:       operation,
			samples:    make([]latencySample, 0, lm.config.WindowSize),
			minLatency: duration,
			maxLatency: duration,
			lastUpdate: time.Now(),
		}
		lm.operations[operation] = tracker
	}

	// Add new sample
	sample := latencySample{
		timestamp: time.Now(),
		duration:  duration,
	}

	tracker.samples = append(tracker.samples, sample)
	tracker.totalCount++
	tracker.totalTime += duration
	tracker.lastUpdate = time.Now()

	// Update min/max
	if duration < tracker.minLatency {
		tracker.minLatency = duration
	}
	if duration > tracker.maxLatency {
		tracker.maxLatency = duration
	}

	// Maintain window size
	if len(tracker.samples) > lm.config.WindowSize {
		// Remove oldest samples
		removeCount := len(tracker.samples) - lm.config.WindowSize
		tracker.samples = tracker.samples[removeCount:]
	}

	// Cleanup old samples based on retention period
	lm.cleanupOldSamples(tracker)
}

// GetAverageLatency returns the average latency for an operation
func (lm *latencyMonitor) GetAverageLatency(operation string) time.Duration {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	tracker, exists := lm.operations[operation]
	if !exists || len(tracker.samples) == 0 {
		return 0
	}

	return lm.calculateAverage(tracker.samples)
}

// GetP95Latency returns the 95th percentile latency for an operation
func (lm *latencyMonitor) GetP95Latency(operation string) time.Duration {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	tracker, exists := lm.operations[operation]
	if !exists || len(tracker.samples) == 0 {
		return 0
	}

	return lm.calculatePercentile(tracker.samples, 0.95)
}

// GetP99Latency returns the 99th percentile latency for an operation
func (lm *latencyMonitor) GetP99Latency(operation string) time.Duration {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	tracker, exists := lm.operations[operation]
	if !exists || len(tracker.samples) == 0 {
		return 0
	}

	return lm.calculatePercentile(tracker.samples, 0.99)
}

// CheckThresholds checks if any operations exceed their alert thresholds
func (lm *latencyMonitor) CheckThresholds() []interfaces.PerformanceAlert {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	var newAlerts []interfaces.PerformanceAlert

	for operation, threshold := range lm.config.AlertThresholds {
		tracker, exists := lm.operations[operation]
		if !exists || len(tracker.samples) == 0 {
			continue
		}

		// Check average latency
		avgLatency := lm.calculateAverage(tracker.samples)
		if avgLatency > threshold {
			alert := interfaces.PerformanceAlert{
				Operation: operation,
				Metric:    "average_latency",
				Threshold: threshold,
				Current:   avgLatency,
				Timestamp: time.Now(),
				Severity:  lm.determineSeverity(avgLatency, threshold),
			}
			newAlerts = append(newAlerts, alert)
			lm.alerts = append(lm.alerts, alert)
			lm.alertsGenerated++
		}

		// Check P95 latency if enabled
		if lm.config.EnablePercentiles {
			p95Latency := lm.calculatePercentile(tracker.samples, 0.95)
			p95Threshold := time.Duration(float64(threshold) * 1.5) // 50% higher threshold for P95

			if p95Latency > p95Threshold {
				alert := interfaces.PerformanceAlert{
					Operation: operation,
					Metric:    "p95_latency",
					Threshold: p95Threshold,
					Current:   p95Latency,
					Timestamp: time.Now(),
					Severity:  lm.determineSeverity(p95Latency, p95Threshold),
				}
				newAlerts = append(newAlerts, alert)
				lm.alerts = append(lm.alerts, alert)
				lm.alertsGenerated++
			}
		}
	}

	return newAlerts
}

// GetMetrics returns comprehensive latency metrics
func (lm *latencyMonitor) GetMetrics() *interfaces.LatencyMetrics {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	// Calculate overall metrics from all operations
	var allSamples []latencySample
	var totalSampleCount int

	for _, tracker := range lm.operations {
		allSamples = append(allSamples, tracker.samples...)
		totalSampleCount += len(tracker.samples)
	}

	if len(allSamples) == 0 {
		return &interfaces.LatencyMetrics{
			Operation:   "overall",
			WindowSize:  lm.config.WindowSize,
			SampleCount: 0,
			LastUpdated: time.Now(),
		}
	}

	// Calculate overall metrics
	avgLatency := lm.calculateAverage(allSamples)
	medianLatency := lm.calculatePercentile(allSamples, 0.50)
	p95Latency := lm.calculatePercentile(allSamples, 0.95)
	p99Latency := lm.calculatePercentile(allSamples, 0.99)

	// Find min/max across all samples
	minLatency := allSamples[0].duration
	maxLatency := allSamples[0].duration
	for _, sample := range allSamples {
		if sample.duration < minLatency {
			minLatency = sample.duration
		}
		if sample.duration > maxLatency {
			maxLatency = sample.duration
		}
	}

	return &interfaces.LatencyMetrics{
		Operation:      "overall",
		WindowSize:     lm.config.WindowSize,
		SampleCount:    totalSampleCount,
		AverageLatency: avgLatency,
		MedianLatency:  medianLatency,
		P95Latency:     p95Latency,
		P99Latency:     p99Latency,
		MinLatency:     minLatency,
		MaxLatency:     maxLatency,
		LastUpdated:    time.Now(),
	}
}

// GetOperationMetrics returns latency metrics for a specific operation
func (lm *latencyMonitor) GetOperationMetrics(operation string) *interfaces.LatencyMetrics {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	tracker, exists := lm.operations[operation]
	if !exists || len(tracker.samples) == 0 {
		return &interfaces.LatencyMetrics{
			Operation:   operation,
			WindowSize:  lm.config.WindowSize,
			SampleCount: 0,
			LastUpdated: time.Now(),
		}
	}

	return &interfaces.LatencyMetrics{
		Operation:      operation,
		WindowSize:     lm.config.WindowSize,
		SampleCount:    len(tracker.samples),
		AverageLatency: lm.calculateAverage(tracker.samples),
		MedianLatency:  lm.calculatePercentile(tracker.samples, 0.50),
		P95Latency:     lm.calculatePercentile(tracker.samples, 0.95),
		P99Latency:     lm.calculatePercentile(tracker.samples, 0.99),
		MinLatency:     tracker.minLatency,
		MaxLatency:     tracker.maxLatency,
		LastUpdated:    tracker.lastUpdate,
	}
}

// calculateAverage calculates the average duration from samples
func (lm *latencyMonitor) calculateAverage(samples []latencySample) time.Duration {
	if len(samples) == 0 {
		return 0
	}

	var total time.Duration
	for _, sample := range samples {
		total += sample.duration
	}

	return total / time.Duration(len(samples))
}

// calculatePercentile calculates the specified percentile from samples
func (lm *latencyMonitor) calculatePercentile(samples []latencySample, percentile float64) time.Duration {
	if len(samples) == 0 {
		return 0
	}

	// Copy samples for sorting
	durations := make([]time.Duration, len(samples))
	for i, sample := range samples {
		durations[i] = sample.duration
	}

	// Sort durations
	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	// Calculate percentile index
	index := int(math.Ceil(percentile*float64(len(durations)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(durations) {
		index = len(durations) - 1
	}

	return durations[index]
}

// determineSeverity determines the alert severity based on how much the current value exceeds the threshold
func (lm *latencyMonitor) determineSeverity(current, threshold time.Duration) interfaces.AlertSeverity {
	ratio := float64(current) / float64(threshold)

	if ratio >= 3.0 {
		return interfaces.AlertSeverityCritical
	} else if ratio >= 2.0 {
		return interfaces.AlertSeverityError
	} else if ratio >= 1.5 {
		return interfaces.AlertSeverityWarning
	} else {
		return interfaces.AlertSeverityInfo
	}
}

// cleanupOldSamples removes samples older than the retention period
func (lm *latencyMonitor) cleanupOldSamples(tracker *operationTracker) {
	if lm.config.RetentionPeriod <= 0 {
		return
	}

	cutoff := time.Now().Add(-lm.config.RetentionPeriod)

	// Find first sample within retention period
	startIndex := 0
	for i, sample := range tracker.samples {
		if sample.timestamp.After(cutoff) {
			startIndex = i
			break
		}
	}

	// Remove old samples
	if startIndex > 0 {
		tracker.samples = tracker.samples[startIndex:]
	}
}

// CleanupExpiredData removes expired tracking data
func (lm *latencyMonitor) CleanupExpiredData() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	cutoff := time.Now().Add(-lm.config.RetentionPeriod)

	// Remove operations that haven't been updated recently
	for name, tracker := range lm.operations {
		if tracker.lastUpdate.Before(cutoff) {
			delete(lm.operations, name)
		} else {
			lm.cleanupOldSamples(tracker)
		}
	}

	// Cleanup old alerts
	validAlerts := make([]interfaces.PerformanceAlert, 0, len(lm.alerts))
	for _, alert := range lm.alerts {
		if alert.Timestamp.After(cutoff) {
			validAlerts = append(validAlerts, alert)
		}
	}
	lm.alerts = validAlerts
}
