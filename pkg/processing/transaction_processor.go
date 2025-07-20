package processing

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

// TransactionProcessorConfig holds configuration for the transaction processor
type TransactionProcessorConfig struct {
	SimulationPoolSize    int           `json:"simulation_pool_size"`
	StrategyPoolSize      int           `json:"strategy_pool_size"`
	MaxConcurrentJobs     int           `json:"max_concurrent_jobs"`
	ProcessingTimeout     time.Duration `json:"processing_timeout"`
	BatchSize             int           `json:"batch_size"`
	PriorityQueueSize     int           `json:"priority_queue_size"`
	EnableLatencyTracking bool          `json:"enable_latency_tracking"`
	MetricsInterval       time.Duration `json:"metrics_interval"`
}

// DefaultTransactionProcessorConfig returns default configuration
func DefaultTransactionProcessorConfig() *TransactionProcessorConfig {
	return &TransactionProcessorConfig{
		SimulationPoolSize:    20,
		StrategyPoolSize:      15,
		MaxConcurrentJobs:     100,
		ProcessingTimeout:     5 * time.Second,
		BatchSize:             50,
		PriorityQueueSize:     1000,
		EnableLatencyTracking: true,
		MetricsInterval:       10 * time.Second,
	}
}

// transactionProcessor implements the TransactionProcessor interface
type transactionProcessor struct {
	config         *TransactionProcessorConfig
	simulationPool interfaces.WorkerPool
	strategyPool   interfaces.WorkerPool
	forkBalancer   interfaces.ForkLoadBalancer
	latencyMonitor interfaces.LatencyMonitor
	mu             sync.RWMutex
	running        bool

	// Dependencies
	forkManager      interfaces.ForkManager
	strategyEngine   interfaces.StrategyEngine
	profitCalculator interfaces.ProfitCalculator

	// Metrics
	stats            *interfaces.ProcessingStats
	totalProcessed   int64
	successProcessed int64
	failedProcessed  int64
	totalLatency     int64
	lastUpdate       time.Time
}

// NewTransactionProcessor creates a new high-performance transaction processor
func NewTransactionProcessor(
	config *TransactionProcessorConfig,
	forkManager interfaces.ForkManager,
	strategyEngine interfaces.StrategyEngine,
	profitCalculator interfaces.ProfitCalculator,
) interfaces.TransactionProcessor {
	if config == nil {
		config = DefaultTransactionProcessorConfig()
	}

	tp := &transactionProcessor{
		config:           config,
		forkManager:      forkManager,
		strategyEngine:   strategyEngine,
		profitCalculator: profitCalculator,
		stats: &interfaces.ProcessingStats{
			CurrentLoad: 0.0,
		},
		lastUpdate: time.Now(),
	}

	// Initialize worker pools
	tp.simulationPool = NewWorkerPool(&WorkerPoolConfig{
		PoolSize:        config.SimulationPoolSize,
		QueueSize:       config.PriorityQueueSize,
		MaxJobTimeout:   config.ProcessingTimeout,
		ShutdownTimeout: 10 * time.Second,
		EnableMetrics:   true,
	})

	tp.strategyPool = NewWorkerPool(&WorkerPoolConfig{
		PoolSize:        config.StrategyPoolSize,
		QueueSize:       config.PriorityQueueSize,
		MaxJobTimeout:   config.ProcessingTimeout,
		ShutdownTimeout: 10 * time.Second,
		EnableMetrics:   true,
	})

	// Initialize fork load balancer
	tp.forkBalancer = NewForkLoadBalancer(forkManager)

	// Initialize latency monitor if enabled
	if config.EnableLatencyTracking {
		tp.latencyMonitor = NewLatencyMonitor()
	}

	return tp
}

// Start starts the transaction processor
func (tp *transactionProcessor) Start(ctx context.Context) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	if tp.running {
		return fmt.Errorf("transaction processor is already running")
	}

	// Start worker pools
	if err := tp.simulationPool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start simulation pool: %w", err)
	}

	if err := tp.strategyPool.Start(ctx); err != nil {
		tp.simulationPool.Stop(ctx)
		return fmt.Errorf("failed to start strategy pool: %w", err)
	}

	// Start metrics collection goroutine
	go tp.collectMetrics(ctx)

	tp.running = true
	return nil
}

// Stop stops the transaction processor gracefully
func (tp *transactionProcessor) Stop(ctx context.Context) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	if !tp.running {
		return fmt.Errorf("transaction processor is not running")
	}

	// Stop worker pools
	var errors []error
	if err := tp.simulationPool.Stop(ctx); err != nil {
		errors = append(errors, fmt.Errorf("simulation pool stop error: %w", err))
	}

	if err := tp.strategyPool.Stop(ctx); err != nil {
		errors = append(errors, fmt.Errorf("strategy pool stop error: %w", err))
	}

	tp.running = false

	if len(errors) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errors)
	}

	return nil
}

// ProcessTransaction processes a single transaction with concurrent simulation and strategy detection
func (tp *transactionProcessor) ProcessTransaction(ctx context.Context, tx *types.Transaction) (*interfaces.ProcessingResult, error) {
	if !tp.running {
		return nil, fmt.Errorf("transaction processor is not running")
	}

	startTime := time.Now()
	defer func() {
		if tp.latencyMonitor != nil {
			tp.latencyMonitor.RecordLatency("process_transaction", time.Since(startTime))
		}
	}()

	// Create result channel
	resultChan := make(chan *interfaces.ProcessingResult, 1)
	errorChan := make(chan error, 1)

	// Create and submit simulation job
	simulationJob := &TransactionSimulationJob{
		ID:          fmt.Sprintf("sim_%s_%d", tx.Hash, time.Now().UnixNano()),
		Transaction: tx,
		Processor:   tp,
		Priority:    tp.calculatePriority(tx),
		Timeout:     tp.config.ProcessingTimeout,
		ResultChan:  resultChan,
		ErrorChan:   errorChan,
	}

	if err := tp.simulationPool.Submit(simulationJob); err != nil {
		atomic.AddInt64(&tp.failedProcessed, 1)
		return nil, fmt.Errorf("failed to submit simulation job: %w", err)
	}

	// Wait for result with timeout
	select {
	case result := <-resultChan:
		atomic.AddInt64(&tp.successProcessed, 1)
		atomic.AddInt64(&tp.totalLatency, int64(result.ProcessingTime))
		return result, nil
	case err := <-errorChan:
		atomic.AddInt64(&tp.failedProcessed, 1)
		return nil, err
	case <-ctx.Done():
		atomic.AddInt64(&tp.failedProcessed, 1)
		return nil, ctx.Err()
	case <-time.After(tp.config.ProcessingTimeout):
		atomic.AddInt64(&tp.failedProcessed, 1)
		return nil, fmt.Errorf("transaction processing timeout")
	}
}

// ProcessBatch processes multiple transactions concurrently
func (tp *transactionProcessor) ProcessBatch(ctx context.Context, txs []*types.Transaction) ([]*interfaces.ProcessingResult, error) {
	if !tp.running {
		return nil, fmt.Errorf("transaction processor is not running")
	}

	if len(txs) == 0 {
		return []*interfaces.ProcessingResult{}, nil
	}

	startTime := time.Now()
	defer func() {
		if tp.latencyMonitor != nil {
			tp.latencyMonitor.RecordLatency("process_batch", time.Since(startTime))
		}
	}()

	// Create result channels
	resultChan := make(chan *interfaces.ProcessingResult, len(txs))
	errorChan := make(chan error, len(txs))

	// Submit all transactions for processing
	submitted := 0
	for _, tx := range txs {
		simulationJob := &TransactionSimulationJob{
			ID:          fmt.Sprintf("batch_sim_%s_%d", tx.Hash, time.Now().UnixNano()),
			Transaction: tx,
			Processor:   tp,
			Priority:    tp.calculatePriority(tx),
			Timeout:     tp.config.ProcessingTimeout,
			ResultChan:  resultChan,
			ErrorChan:   errorChan,
		}

		if err := tp.simulationPool.Submit(simulationJob); err != nil {
			// Send error for this transaction
			go func(tx *types.Transaction, err error) {
				errorChan <- fmt.Errorf("failed to submit job for tx %s: %w", tx.Hash, err)
			}(tx, err)
		} else {
			submitted++
		}
	}

	// Collect results
	results := make([]*interfaces.ProcessingResult, 0, len(txs))
	var errors []error
	completed := 0

	for completed < len(txs) {
		select {
		case result := <-resultChan:
			results = append(results, result)
			atomic.AddInt64(&tp.successProcessed, 1)
			atomic.AddInt64(&tp.totalLatency, int64(result.ProcessingTime))
			completed++
		case err := <-errorChan:
			errors = append(errors, err)
			atomic.AddInt64(&tp.failedProcessed, 1)
			completed++
		case <-ctx.Done():
			return results, ctx.Err()
		case <-time.After(tp.config.ProcessingTimeout * 2):
			return results, fmt.Errorf("batch processing timeout, completed %d/%d", completed, len(txs))
		}
	}

	if len(errors) > 0 {
		return results, fmt.Errorf("batch processing completed with %d errors: %v", len(errors), errors[0])
	}

	return results, nil
}

// GetStats returns current processing statistics
func (tp *transactionProcessor) GetStats() *interfaces.ProcessingStats {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	total := atomic.LoadInt64(&tp.totalProcessed)
	success := atomic.LoadInt64(&tp.successProcessed)
	failed := atomic.LoadInt64(&tp.failedProcessed)
	totalLatency := atomic.LoadInt64(&tp.totalLatency)

	stats := &interfaces.ProcessingStats{
		TotalProcessed:      total,
		SuccessfulProcessed: success,
		FailedProcessed:     failed,
		QueueDepth:          tp.getQueueDepth(),
	}

	// Calculate average latency
	if total > 0 {
		stats.AverageLatency = time.Duration(totalLatency / total)
	}

	// Calculate throughput (TPS)
	timeDiff := time.Since(tp.lastUpdate).Seconds()
	if timeDiff > 0 {
		stats.ThroughputTPS = float64(total) / timeDiff
	}

	// Calculate current load
	simulationStats := tp.simulationPool.GetStats()
	strategyStats := tp.strategyPool.GetStats()
	stats.CurrentLoad = (simulationStats.Utilization + strategyStats.Utilization) / 2.0

	return stats
}

// calculatePriority calculates job priority based on transaction characteristics
func (tp *transactionProcessor) calculatePriority(tx *types.Transaction) int {
	priority := 0

	// Higher priority for larger value transactions
	if tx.Value != nil && tx.Value.Sign() > 0 {
		// Simple heuristic: priority increases with transaction value
		priority += int(tx.Value.Int64() / 1e18) // Priority per ETH
	}

	// Higher priority for higher gas price (likely more profitable)
	if tx.GasPrice != nil && tx.GasPrice.Sign() > 0 {
		priority += int(tx.GasPrice.Int64() / 1e9) // Priority per Gwei
	}

	// Cap priority to prevent overflow
	if priority > 1000 {
		priority = 1000
	}

	return priority
}

// getQueueDepth returns the total queue depth across all pools
func (tp *transactionProcessor) getQueueDepth() int {
	simulationStats := tp.simulationPool.GetStats()
	strategyStats := tp.strategyPool.GetStats()
	return simulationStats.QueuedJobs + strategyStats.QueuedJobs
}

// collectMetrics periodically updates processing metrics
func (tp *transactionProcessor) collectMetrics(ctx context.Context) {
	ticker := time.NewTicker(tp.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tp.mu.Lock()
			tp.lastUpdate = time.Now()
			atomic.StoreInt64(&tp.totalProcessed,
				atomic.LoadInt64(&tp.successProcessed)+atomic.LoadInt64(&tp.failedProcessed))
			tp.mu.Unlock()
		}
	}
}
