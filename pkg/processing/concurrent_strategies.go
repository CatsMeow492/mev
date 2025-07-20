package processing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"math/big"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

// ConcurrentStrategyProcessor handles parallel strategy detection across multiple opportunities
type ConcurrentStrategyProcessor struct {
	workerPool         interfaces.WorkerPool
	sandwichDetector   interfaces.SandwichDetector
	backrunDetector    interfaces.BackrunDetector
	frontrunDetector   interfaces.FrontrunDetector
	timeBanditDetector interfaces.TimeBanditDetector
	latencyMonitor     interfaces.LatencyMonitor
	mu                 sync.RWMutex
	running            bool
}

// ConcurrentStrategyConfig holds configuration for concurrent strategy processing
type ConcurrentStrategyConfig struct {
	WorkerPoolSize    int                       `json:"worker_pool_size"`
	QueueSize         int                       `json:"queue_size"`
	ProcessingTimeout time.Duration             `json:"processing_timeout"`
	EnabledStrategies []interfaces.StrategyType `json:"enabled_strategies"`
	MaxConcurrentOps  int                       `json:"max_concurrent_ops"`
}

// DefaultConcurrentStrategyConfig returns default configuration
func DefaultConcurrentStrategyConfig() *ConcurrentStrategyConfig {
	return &ConcurrentStrategyConfig{
		WorkerPoolSize:    15,
		QueueSize:         500,
		ProcessingTimeout: 3 * time.Second,
		EnabledStrategies: []interfaces.StrategyType{
			interfaces.StrategySandwich,
			interfaces.StrategyBackrun,
			interfaces.StrategyFrontrun,
			interfaces.StrategyTimeBandit,
		},
		MaxConcurrentOps: 100,
	}
}

// NewConcurrentStrategyProcessor creates a new concurrent strategy processor
func NewConcurrentStrategyProcessor(
	config *ConcurrentStrategyConfig,
	sandwichDetector interfaces.SandwichDetector,
	backrunDetector interfaces.BackrunDetector,
	frontrunDetector interfaces.FrontrunDetector,
	timeBanditDetector interfaces.TimeBanditDetector,
	latencyMonitor interfaces.LatencyMonitor,
) *ConcurrentStrategyProcessor {
	if config == nil {
		config = DefaultConcurrentStrategyConfig()
	}

	csp := &ConcurrentStrategyProcessor{
		sandwichDetector:   sandwichDetector,
		backrunDetector:    backrunDetector,
		frontrunDetector:   frontrunDetector,
		timeBanditDetector: timeBanditDetector,
		latencyMonitor:     latencyMonitor,
	}

	// Initialize worker pool for strategy processing
	csp.workerPool = NewWorkerPool(&WorkerPoolConfig{
		PoolSize:        config.WorkerPoolSize,
		QueueSize:       config.QueueSize,
		MaxJobTimeout:   config.ProcessingTimeout,
		ShutdownTimeout: 10 * time.Second,
		EnableMetrics:   true,
	})

	return csp
}

// Start starts the concurrent strategy processor
func (csp *ConcurrentStrategyProcessor) Start(ctx context.Context) error {
	csp.mu.Lock()
	defer csp.mu.Unlock()

	if csp.running {
		return fmt.Errorf("concurrent strategy processor is already running")
	}

	if err := csp.workerPool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}

	csp.running = true
	return nil
}

// Stop stops the concurrent strategy processor
func (csp *ConcurrentStrategyProcessor) Stop(ctx context.Context) error {
	csp.mu.Lock()
	defer csp.mu.Unlock()

	if !csp.running {
		return fmt.Errorf("concurrent strategy processor is not running")
	}

	if err := csp.workerPool.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop worker pool: %w", err)
	}

	csp.running = false
	return nil
}

// ProcessOpportunities processes multiple transactions concurrently for MEV opportunities
func (csp *ConcurrentStrategyProcessor) ProcessOpportunities(
	ctx context.Context,
	transactions []*types.Transaction,
	simResults []*interfaces.SimulationResult,
) ([]*interfaces.MEVOpportunity, error) {
	if !csp.running {
		return nil, fmt.Errorf("concurrent strategy processor is not running")
	}

	if len(transactions) != len(simResults) {
		return nil, fmt.Errorf("transactions and simulation results count mismatch")
	}

	startTime := time.Now()
	defer func() {
		if csp.latencyMonitor != nil {
			csp.latencyMonitor.RecordLatency("process_opportunities_batch", time.Since(startTime))
		}
	}()

	// Channel to collect all opportunities
	opportunityChan := make(chan []*interfaces.MEVOpportunity, len(transactions))
	errorChan := make(chan error, len(transactions))

	// Submit jobs for each transaction
	for i, tx := range transactions {
		if i >= len(simResults) {
			break
		}

		job := &ConcurrentStrategyJob{
			ID:               fmt.Sprintf("strategy_batch_%s_%d", tx.Hash, time.Now().UnixNano()),
			Transaction:      tx,
			SimulationResult: simResults[i],
			Processor:        csp,
			Priority:         csp.calculatePriority(tx),
			Timeout:          3 * time.Second,
			OpportunityChan:  opportunityChan,
			ErrorChan:        errorChan,
		}

		if err := csp.workerPool.Submit(job); err != nil {
			errorChan <- fmt.Errorf("failed to submit strategy job for tx %s: %w", tx.Hash, err)
		}
	}

	// Collect results
	var allOpportunities []*interfaces.MEVOpportunity
	var errors []error
	completed := 0

	for completed < len(transactions) {
		select {
		case opportunities := <-opportunityChan:
			allOpportunities = append(allOpportunities, opportunities...)
			completed++
		case err := <-errorChan:
			errors = append(errors, err)
			completed++
		case <-ctx.Done():
			return allOpportunities, ctx.Err()
		case <-time.After(10 * time.Second):
			return allOpportunities, fmt.Errorf("timeout processing opportunities, completed %d/%d", completed, len(transactions))
		}
	}

	if len(errors) > 0 {
		return allOpportunities, fmt.Errorf("strategy processing completed with %d errors: %v", len(errors), errors[0])
	}

	return allOpportunities, nil
}

// DetectStrategiesConcurrently detects all enabled strategies for a transaction concurrently
func (csp *ConcurrentStrategyProcessor) DetectStrategiesConcurrently(
	ctx context.Context,
	tx *types.Transaction,
	simResult *interfaces.SimulationResult,
) ([]*interfaces.MEVOpportunity, error) {
	startTime := time.Now()
	defer func() {
		if csp.latencyMonitor != nil {
			csp.latencyMonitor.RecordLatency("detect_strategies_concurrent", time.Since(startTime))
		}
	}()

	// Channel to collect opportunities from all strategies
	opportunityChan := make(chan *interfaces.MEVOpportunity, 10)
	errorChan := make(chan error, 4) // Max 4 strategies
	var wg sync.WaitGroup

	// Launch goroutines for each strategy type
	strategies := []struct {
		name     string
		detector func(context.Context, *types.Transaction, *interfaces.SimulationResult) (*interfaces.MEVOpportunity, error)
	}{
		{"sandwich", csp.detectSandwich},
		{"backrun", csp.detectBackrun},
		{"frontrun", csp.detectFrontrun},
	}

	for _, strategy := range strategies {
		wg.Add(1)
		go func(name string, detector func(context.Context, *types.Transaction, *interfaces.SimulationResult) (*interfaces.MEVOpportunity, error)) {
			defer wg.Done()

			strategyStart := time.Now()
			opportunity, err := detector(ctx, tx, simResult)
			if csp.latencyMonitor != nil {
				csp.latencyMonitor.RecordLatency(fmt.Sprintf("detect_%s", name), time.Since(strategyStart))
			}

			if err != nil {
				select {
				case errorChan <- fmt.Errorf("%s detection failed: %w", name, err):
				case <-ctx.Done():
				}
				return
			}

			if opportunity != nil {
				select {
				case opportunityChan <- opportunity:
				case <-ctx.Done():
				}
			}
		}(strategy.name, strategy.detector)
	}

	// Close channels when all strategies complete
	go func() {
		wg.Wait()
		close(opportunityChan)
		close(errorChan)
	}()

	// Collect opportunities and errors
	var opportunities []*interfaces.MEVOpportunity
	var errors []error

	for {
		select {
		case opportunity, ok := <-opportunityChan:
			if !ok {
				// Channel closed, we're done
				if len(errors) > 0 {
					return opportunities, fmt.Errorf("strategy detection had %d errors: %v", len(errors), errors[0])
				}
				return opportunities, nil
			}
			opportunities = append(opportunities, opportunity)

		case err, ok := <-errorChan:
			if !ok {
				// Channel closed
				continue
			}
			errors = append(errors, err)

		case <-ctx.Done():
			return opportunities, ctx.Err()
		}
	}
}

// Strategy detection methods

func (csp *ConcurrentStrategyProcessor) detectSandwich(
	ctx context.Context,
	tx *types.Transaction,
	simResult *interfaces.SimulationResult,
) (*interfaces.MEVOpportunity, error) {
	if csp.sandwichDetector == nil {
		return nil, nil
	}

	opportunity, err := csp.sandwichDetector.DetectOpportunity(ctx, tx, simResult)
	if err != nil || opportunity == nil {
		return nil, err
	}

	return &interfaces.MEVOpportunity{
		ID:             fmt.Sprintf("sandwich_%s_%d", tx.Hash, time.Now().UnixNano()),
		Strategy:       interfaces.StrategySandwich,
		TargetTx:       tx.Hash,
		ExpectedProfit: opportunity.ExpectedProfit,
		GasCost:        big.NewInt(0), // Would need to calculate
		NetProfit:      opportunity.ExpectedProfit,
		Confidence:     0.8, // Default confidence
		Status:         interfaces.StatusDetected,
		CreatedAt:      time.Now(),
		Metadata: map[string]interface{}{
			"sandwich_opportunity": opportunity,
		},
	}, nil
}

func (csp *ConcurrentStrategyProcessor) detectBackrun(
	ctx context.Context,
	tx *types.Transaction,
	simResult *interfaces.SimulationResult,
) (*interfaces.MEVOpportunity, error) {
	if csp.backrunDetector == nil {
		return nil, nil
	}

	opportunity, err := csp.backrunDetector.DetectOpportunity(ctx, tx, simResult)
	if err != nil || opportunity == nil {
		return nil, err
	}

	return &interfaces.MEVOpportunity{
		ID:             fmt.Sprintf("backrun_%s_%d", tx.Hash, time.Now().UnixNano()),
		Strategy:       interfaces.StrategyBackrun,
		TargetTx:       tx.Hash,
		ExpectedProfit: opportunity.ExpectedProfit,
		GasCost:        big.NewInt(0), // Would need to calculate
		NetProfit:      opportunity.ExpectedProfit,
		Confidence:     0.8, // Default confidence
		Status:         interfaces.StatusDetected,
		CreatedAt:      time.Now(),
		Metadata: map[string]interface{}{
			"backrun_opportunity": opportunity,
		},
	}, nil
}

func (csp *ConcurrentStrategyProcessor) detectFrontrun(
	ctx context.Context,
	tx *types.Transaction,
	simResult *interfaces.SimulationResult,
) (*interfaces.MEVOpportunity, error) {
	if csp.frontrunDetector == nil {
		return nil, nil
	}

	opportunity, err := csp.frontrunDetector.DetectOpportunity(ctx, tx, simResult)
	if err != nil || opportunity == nil {
		return nil, err
	}

	return &interfaces.MEVOpportunity{
		ID:             fmt.Sprintf("frontrun_%s_%d", tx.Hash, time.Now().UnixNano()),
		Strategy:       interfaces.StrategyFrontrun,
		TargetTx:       tx.Hash,
		ExpectedProfit: opportunity.ExpectedProfit,
		GasCost:        big.NewInt(0), // Would need to calculate
		NetProfit:      opportunity.ExpectedProfit,
		Confidence:     0.8, // Default confidence
		Status:         interfaces.StatusDetected,
		CreatedAt:      time.Now(),
		Metadata: map[string]interface{}{
			"frontrun_opportunity": opportunity,
		},
	}, nil
}

// calculatePriority calculates the priority for strategy processing
func (csp *ConcurrentStrategyProcessor) calculatePriority(tx *types.Transaction) int {
	priority := 0

	// Higher priority for larger value transactions
	if tx.Value != nil && tx.Value.Sign() > 0 {
		priority += int(tx.Value.Int64() / 1e18) // Priority per ETH
	}

	// Higher priority for higher gas price
	if tx.GasPrice != nil && tx.GasPrice.Sign() > 0 {
		priority += int(tx.GasPrice.Int64() / 1e9) // Priority per Gwei
	}

	// Cap priority
	if priority > 1000 {
		priority = 1000
	}

	return priority
}

// GetStats returns worker pool statistics
func (csp *ConcurrentStrategyProcessor) GetStats() *interfaces.WorkerPoolStats {
	csp.mu.RLock()
	defer csp.mu.RUnlock()

	if csp.workerPool == nil {
		return &interfaces.WorkerPoolStats{}
	}

	return csp.workerPool.GetStats()
}

// ConcurrentStrategyJob implements the Job interface for concurrent strategy detection
type ConcurrentStrategyJob struct {
	ID               string
	Transaction      *types.Transaction
	SimulationResult *interfaces.SimulationResult
	Processor        *ConcurrentStrategyProcessor
	Priority         int
	Timeout          time.Duration
	OpportunityChan  chan<- []*interfaces.MEVOpportunity
	ErrorChan        chan<- error
}

// Execute implements the Job interface
func (job *ConcurrentStrategyJob) Execute(ctx context.Context) (interface{}, error) {
	opportunities, err := job.Processor.DetectStrategiesConcurrently(ctx, job.Transaction, job.SimulationResult)

	if err != nil {
		select {
		case job.ErrorChan <- err:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return nil, err
	}

	select {
	case job.OpportunityChan <- opportunities:
		return opportunities, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetPriority returns the job priority
func (job *ConcurrentStrategyJob) GetPriority() int {
	return job.Priority
}

// GetID returns the job ID
func (job *ConcurrentStrategyJob) GetID() string {
	return job.ID
}

// GetTimeout returns the job timeout
func (job *ConcurrentStrategyJob) GetTimeout() time.Duration {
	return job.Timeout
}
