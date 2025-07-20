package processing

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// ForkLoadBalancerConfig holds configuration for the fork load balancer
type ForkLoadBalancerConfig struct {
	MaxRetries          int           `json:"max_retries"`
	RetryDelay          time.Duration `json:"retry_delay"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	LoadBalanceStrategy string        `json:"load_balance_strategy"` // "round_robin", "least_loaded", "fastest"
	LatencyWindow       int           `json:"latency_window"`
}

// DefaultForkLoadBalancerConfig returns default configuration
func DefaultForkLoadBalancerConfig() *ForkLoadBalancerConfig {
	return &ForkLoadBalancerConfig{
		MaxRetries:          3,
		RetryDelay:          100 * time.Millisecond,
		HealthCheckInterval: 30 * time.Second,
		LoadBalanceStrategy: "least_loaded",
		LatencyWindow:       100,
	}
}

// forkLoadBalancer implements the ForkLoadBalancer interface
type forkLoadBalancer struct {
	config      *ForkLoadBalancerConfig
	forkManager interfaces.ForkManager
	mu          sync.RWMutex

	// Load balancing state
	forkLoads   map[string]int           // fork ID -> current load count
	forkLatency map[string]time.Duration // fork ID -> average latency
	roundRobin  int                      // index for round-robin

	// Metrics
	stats         *interfaces.LoadBalancerStats
	requestCount  int64
	failoverCount int64

	// Background tasks
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewForkLoadBalancer creates a new fork load balancer
func NewForkLoadBalancer(forkManager interfaces.ForkManager) interfaces.ForkLoadBalancer {
	config := DefaultForkLoadBalancerConfig()

	ctx, cancel := context.WithCancel(context.Background())

	flb := &forkLoadBalancer{
		config:      config,
		forkManager: forkManager,
		forkLoads:   make(map[string]int),
		forkLatency: make(map[string]time.Duration),
		stats: &interfaces.LoadBalancerStats{
			LoadDistribution: make(map[string]int),
		},
		ctx:    ctx,
		cancel: cancel,
	}

	// Start background health monitoring
	flb.wg.Add(1)
	go flb.monitorHealth()

	return flb
}

// GetFork gets an available fork using the configured load balancing strategy
func (flb *forkLoadBalancer) GetFork(ctx context.Context) (interfaces.Fork, error) {
	atomic.AddInt64(&flb.requestCount, 1)

	var fork interfaces.Fork
	var err error

	// Try to get a fork with retries
	for attempt := 0; attempt < flb.config.MaxRetries; attempt++ {
		fork, err = flb.selectFork(ctx)
		if err == nil {
			// Successfully got a fork
			flb.recordForkAssignment(fork.GetID())
			return fork, nil
		}

		// If this is not the last attempt, wait before retrying
		if attempt < flb.config.MaxRetries-1 {
			select {
			case <-time.After(flb.config.RetryDelay):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	atomic.AddInt64(&flb.failoverCount, 1)
	return nil, fmt.Errorf("failed to get fork after %d attempts: %w", flb.config.MaxRetries, err)
}

// ReleaseFork releases a fork back to the pool
func (flb *forkLoadBalancer) ReleaseFork(fork interfaces.Fork) error {
	flb.recordForkRelease(fork.GetID())
	return flb.forkManager.ReleaseFork(fork)
}

// GetBestFork gets the best fork based on current metrics
func (flb *forkLoadBalancer) GetBestFork(ctx context.Context) (interfaces.Fork, error) {
	// Temporarily override strategy to get the best performing fork
	originalStrategy := flb.config.LoadBalanceStrategy
	flb.config.LoadBalanceStrategy = "fastest"
	defer func() {
		flb.config.LoadBalanceStrategy = originalStrategy
	}()

	return flb.GetFork(ctx)
}

// GetStats returns current load balancer statistics
func (flb *forkLoadBalancer) GetStats() *interfaces.LoadBalancerStats {
	flb.mu.RLock()
	defer flb.mu.RUnlock()

	poolStats := flb.forkManager.GetForkPoolStats()

	stats := &interfaces.LoadBalancerStats{
		TotalForks:       poolStats.TotalForks,
		HealthyForks:     poolStats.TotalForks - poolStats.FailedForks,
		LoadDistribution: make(map[string]int),
		FailoverCount:    atomic.LoadInt64(&flb.failoverCount),
	}

	// Copy load distribution
	for forkID, load := range flb.forkLoads {
		stats.LoadDistribution[forkID] = load
	}

	// Calculate average latency
	if len(flb.forkLatency) > 0 {
		var totalLatency time.Duration
		for _, latency := range flb.forkLatency {
			totalLatency += latency
		}
		stats.AverageLatency = totalLatency / time.Duration(len(flb.forkLatency))
	}

	return stats
}

// selectFork selects a fork based on the configured strategy
func (flb *forkLoadBalancer) selectFork(ctx context.Context) (interfaces.Fork, error) {
	switch flb.config.LoadBalanceStrategy {
	case "round_robin":
		return flb.selectRoundRobin(ctx)
	case "least_loaded":
		return flb.selectLeastLoaded(ctx)
	case "fastest":
		return flb.selectFastest(ctx)
	default:
		return flb.selectLeastLoaded(ctx) // Default to least loaded
	}
}

// selectRoundRobin selects a fork using round-robin strategy
func (flb *forkLoadBalancer) selectRoundRobin(ctx context.Context) (interfaces.Fork, error) {
	// For round-robin, we just get the next available fork
	// The fork manager handles the actual round-robin logic
	return flb.forkManager.GetAvailableFork(ctx)
}

// selectLeastLoaded selects the fork with the least current load
func (flb *forkLoadBalancer) selectLeastLoaded(ctx context.Context) (interfaces.Fork, error) {
	flb.mu.Lock()
	defer flb.mu.Unlock()

	// Try to get a fork and track its load
	fork, err := flb.forkManager.GetAvailableFork(ctx)
	if err != nil {
		return nil, err
	}

	// Initialize load tracking for new forks
	if _, exists := flb.forkLoads[fork.GetID()]; !exists {
		flb.forkLoads[fork.GetID()] = 0
	}

	return fork, nil
}

// selectFastest selects the fork with the lowest average latency
func (flb *forkLoadBalancer) selectFastest(ctx context.Context) (interfaces.Fork, error) {
	flb.mu.Lock()
	defer flb.mu.Unlock()

	// Get available fork
	fork, err := flb.forkManager.GetAvailableFork(ctx)
	if err != nil {
		return nil, err
	}

	// Initialize latency tracking for new forks
	if _, exists := flb.forkLatency[fork.GetID()]; !exists {
		flb.forkLatency[fork.GetID()] = 0
	}

	return fork, nil
}

// recordForkAssignment records when a fork is assigned to a job
func (flb *forkLoadBalancer) recordForkAssignment(forkID string) {
	flb.mu.Lock()
	defer flb.mu.Unlock()

	flb.forkLoads[forkID]++
	flb.stats.LoadDistribution[forkID]++
}

// recordForkRelease records when a fork is released from a job
func (flb *forkLoadBalancer) recordForkRelease(forkID string) {
	flb.mu.Lock()
	defer flb.mu.Unlock()

	if load, exists := flb.forkLoads[forkID]; exists && load > 0 {
		flb.forkLoads[forkID]--
	}
}

// recordForkLatency records latency for a fork operation
func (flb *forkLoadBalancer) recordForkLatency(forkID string, latency time.Duration) {
	flb.mu.Lock()
	defer flb.mu.Unlock()

	// Simple moving average
	if currentLatency, exists := flb.forkLatency[forkID]; exists {
		flb.forkLatency[forkID] = (currentLatency + latency) / 2
	} else {
		flb.forkLatency[forkID] = latency
	}
}

// monitorHealth monitors fork health and updates metrics
func (flb *forkLoadBalancer) monitorHealth() {
	defer flb.wg.Done()

	ticker := time.NewTicker(flb.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-flb.ctx.Done():
			return
		case <-ticker.C:
			flb.performHealthCheck()
		}
	}
}

// performHealthCheck checks the health of all tracked forks
func (flb *forkLoadBalancer) performHealthCheck() {
	flb.mu.Lock()
	defer flb.mu.Unlock()

	// Clean up tracking for forks that no longer exist
	// This is a simplified implementation - in practice you'd check with the fork manager
	poolStats := flb.forkManager.GetForkPoolStats()

	// Reset metrics if no forks are available
	if poolStats.TotalForks == 0 {
		flb.forkLoads = make(map[string]int)
		flb.forkLatency = make(map[string]time.Duration)
		flb.stats.LoadDistribution = make(map[string]int)
	}
}

// Shutdown gracefully shuts down the load balancer
func (flb *forkLoadBalancer) Shutdown() {
	flb.cancel()
	flb.wg.Wait()
}

// ForkSelector is a helper struct for selecting the best fork
type ForkSelector struct {
	forkID  string
	load    int
	latency time.Duration
	healthy bool
}

// selectBestForkFromCandidates selects the best fork from a list of candidates
func (flb *forkLoadBalancer) selectBestForkFromCandidates(candidates []ForkSelector) string {
	if len(candidates) == 0 {
		return ""
	}

	// Filter healthy forks
	healthyCandidates := make([]ForkSelector, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.healthy {
			healthyCandidates = append(healthyCandidates, candidate)
		}
	}

	if len(healthyCandidates) == 0 {
		// No healthy forks, return the first available
		return candidates[0].forkID
	}

	// Sort based on strategy
	switch flb.config.LoadBalanceStrategy {
	case "least_loaded":
		sort.Slice(healthyCandidates, func(i, j int) bool {
			return healthyCandidates[i].load < healthyCandidates[j].load
		})
	case "fastest":
		sort.Slice(healthyCandidates, func(i, j int) bool {
			return healthyCandidates[i].latency < healthyCandidates[j].latency
		})
	default:
		// Default to least loaded
		sort.Slice(healthyCandidates, func(i, j int) bool {
			return healthyCandidates[i].load < healthyCandidates[j].load
		})
	}

	return healthyCandidates[0].forkID
}
