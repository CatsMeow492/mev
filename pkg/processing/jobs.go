package processing

import (
	"context"
	"fmt"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

// TransactionSimulationJob handles transaction simulation in worker pools
type TransactionSimulationJob struct {
	ID          string
	Transaction *types.Transaction
	Processor   *transactionProcessor
	Priority    int
	Timeout     time.Duration
	ResultChan  chan<- *interfaces.ProcessingResult
	ErrorChan   chan<- error
}

// Execute implements the Job interface for transaction simulation
func (job *TransactionSimulationJob) Execute(ctx context.Context) (interface{}, error) {
	startTime := time.Now()

	// Get a fork for simulation
	fork, err := job.Processor.forkBalancer.GetFork(ctx)
	if err != nil {
		job.ErrorChan <- fmt.Errorf("failed to get fork: %w", err)
		return nil, err
	}
	defer job.Processor.forkBalancer.ReleaseFork(fork)

	// Simulate the transaction
	simResult, err := fork.ExecuteTransaction(ctx, job.Transaction)
	if err != nil {
		job.ErrorChan <- fmt.Errorf("simulation failed: %w", err)
		return nil, err
	}

	// If simulation successful, detect MEV opportunities using strategy pool
	var opportunities []*interfaces.MEVOpportunity
	if simResult.Success {
		opportunities, err = job.detectOpportunities(ctx, simResult)
		if err != nil {
			// Log error but don't fail the job - simulation was successful
			opportunities = []*interfaces.MEVOpportunity{}
		}
	}

	// Create processing result
	result := &interfaces.ProcessingResult{
		Transaction:      job.Transaction,
		SimulationResult: simResult,
		Opportunities:    opportunities,
		ProcessingTime:   time.Since(startTime),
		Success:          simResult.Success,
		Error:            nil,
	}

	// Send result
	select {
	case job.ResultChan <- result:
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// detectOpportunities detects MEV opportunities using the strategy engine
func (job *TransactionSimulationJob) detectOpportunities(ctx context.Context, simResult *interfaces.SimulationResult) ([]*interfaces.MEVOpportunity, error) {
	// Create strategy detection job
	strategyJobChan := make(chan []*interfaces.MEVOpportunity, 1)
	strategyErrorChan := make(chan error, 1)

	strategyJob := &StrategyDetectionJob{
		ID:               fmt.Sprintf("strategy_%s", job.ID),
		Transaction:      job.Transaction,
		SimulationResult: simResult,
		Priority:         job.Priority,
		Timeout:          job.Timeout / 2, // Give half the remaining time
		ResultChan:       strategyJobChan,
		ErrorChan:        strategyErrorChan,
		StrategyEngine:   job.Processor.strategyEngine,
	}

	// Submit to strategy pool
	if err := job.Processor.strategyPool.Submit(strategyJob); err != nil {
		return nil, fmt.Errorf("failed to submit strategy job: %w", err)
	}

	// Wait for strategy detection result
	select {
	case opportunities := <-strategyJobChan:
		return opportunities, nil
	case err := <-strategyErrorChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(job.Timeout / 2):
		return nil, fmt.Errorf("strategy detection timeout")
	}
}

// GetPriority returns the job priority
func (job *TransactionSimulationJob) GetPriority() int {
	return job.Priority
}

// GetID returns the job ID
func (job *TransactionSimulationJob) GetID() string {
	return job.ID
}

// GetTimeout returns the job timeout
func (job *TransactionSimulationJob) GetTimeout() time.Duration {
	return job.Timeout
}

// StrategyDetectionJob handles MEV strategy detection in worker pools
type StrategyDetectionJob struct {
	ID               string
	Transaction      *types.Transaction
	SimulationResult *interfaces.SimulationResult
	Priority         int
	Timeout          time.Duration
	ResultChan       chan<- []*interfaces.MEVOpportunity
	ErrorChan        chan<- error
	StrategyEngine   interfaces.StrategyEngine
}

// Execute implements the Job interface for strategy detection
func (job *StrategyDetectionJob) Execute(ctx context.Context) (interface{}, error) {
	// Analyze transaction for MEV opportunities
	opportunities, err := job.StrategyEngine.AnalyzeTransaction(ctx, job.Transaction, job.SimulationResult)
	if err != nil {
		job.ErrorChan <- fmt.Errorf("strategy analysis failed: %w", err)
		return nil, err
	}

	// Filter and validate opportunities
	validOpportunities := job.filterOpportunities(opportunities)

	// Send result
	select {
	case job.ResultChan <- validOpportunities:
		return validOpportunities, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// filterOpportunities filters and validates detected opportunities
func (job *StrategyDetectionJob) filterOpportunities(opportunities []*interfaces.MEVOpportunity) []*interfaces.MEVOpportunity {
	if len(opportunities) == 0 {
		return opportunities
	}

	filtered := make([]*interfaces.MEVOpportunity, 0, len(opportunities))
	for _, opp := range opportunities {
		// Basic validation
		if opp == nil {
			continue
		}

		// Check if opportunity has positive expected profit
		if opp.ExpectedProfit != nil && opp.ExpectedProfit.Sign() > 0 {
			// Check if net profit is positive after gas costs
			if opp.NetProfit != nil && opp.NetProfit.Sign() > 0 {
				// Check confidence threshold
				if opp.Confidence >= 0.5 { // 50% minimum confidence
					filtered = append(filtered, opp)
				}
			}
		}
	}

	return filtered
}

// GetPriority returns the job priority
func (job *StrategyDetectionJob) GetPriority() int {
	return job.Priority
}

// GetID returns the job ID
func (job *StrategyDetectionJob) GetID() string {
	return job.ID
}

// GetTimeout returns the job timeout
func (job *StrategyDetectionJob) GetTimeout() time.Duration {
	return job.Timeout
}

// BatchProcessingJob handles batch processing of multiple transactions
type BatchProcessingJob struct {
	ID           string
	Transactions []*types.Transaction
	Processor    *transactionProcessor
	Priority     int
	Timeout      time.Duration
	ResultChan   chan<- []*interfaces.ProcessingResult
	ErrorChan    chan<- error
}

// Execute implements the Job interface for batch processing
func (job *BatchProcessingJob) Execute(ctx context.Context) (interface{}, error) {
	results := make([]*interfaces.ProcessingResult, 0, len(job.Transactions))

	// Process transactions in smaller batches to improve concurrency
	batchSize := job.Processor.config.BatchSize
	if batchSize <= 0 {
		batchSize = 10
	}

	for i := 0; i < len(job.Transactions); i += batchSize {
		end := i + batchSize
		if end > len(job.Transactions) {
			end = len(job.Transactions)
		}

		batch := job.Transactions[i:end]
		batchResults, err := job.processBatch(ctx, batch)
		if err != nil {
			job.ErrorChan <- fmt.Errorf("batch processing failed: %w", err)
			return nil, err
		}

		results = append(results, batchResults...)
	}

	// Send results
	select {
	case job.ResultChan <- results:
		return results, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// processBatch processes a small batch of transactions concurrently
func (job *BatchProcessingJob) processBatch(ctx context.Context, transactions []*types.Transaction) ([]*interfaces.ProcessingResult, error) {
	resultChan := make(chan *interfaces.ProcessingResult, len(transactions))
	errorChan := make(chan error, len(transactions))

	// Submit all transactions in the batch
	for _, tx := range transactions {
		simJob := &TransactionSimulationJob{
			ID:          fmt.Sprintf("%s_tx_%s", job.ID, tx.Hash),
			Transaction: tx,
			Processor:   job.Processor,
			Priority:    job.Priority,
			Timeout:     job.Timeout / time.Duration(len(transactions)+1), // Distribute timeout
			ResultChan:  resultChan,
			ErrorChan:   errorChan,
		}

		if err := job.Processor.simulationPool.Submit(simJob); err != nil {
			return nil, fmt.Errorf("failed to submit simulation job: %w", err)
		}
	}

	// Collect results
	results := make([]*interfaces.ProcessingResult, 0, len(transactions))
	var errors []error
	completed := 0

	for completed < len(transactions) {
		select {
		case result := <-resultChan:
			results = append(results, result)
			completed++
		case err := <-errorChan:
			errors = append(errors, err)
			completed++
		case <-ctx.Done():
			return results, ctx.Err()
		}
	}

	if len(errors) > 0 {
		return results, fmt.Errorf("batch had %d errors: %v", len(errors), errors[0])
	}

	return results, nil
}

// GetPriority returns the job priority
func (job *BatchProcessingJob) GetPriority() int {
	return job.Priority
}

// GetID returns the job ID
func (job *BatchProcessingJob) GetID() string {
	return job.ID
}

// GetTimeout returns the job timeout
func (job *BatchProcessingJob) GetTimeout() time.Duration {
	return job.Timeout
}
