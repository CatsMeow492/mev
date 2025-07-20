package interfaces

import (
	"context"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

// TransactionProcessor handles high-frequency transaction processing
type TransactionProcessor interface {
	ProcessTransaction(ctx context.Context, tx *types.Transaction) (*ProcessingResult, error)
	ProcessBatch(ctx context.Context, txs []*types.Transaction) ([]*ProcessingResult, error)
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	GetStats() *ProcessingStats
}

// WorkerPool manages a pool of worker goroutines for parallel processing
type WorkerPool interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Submit(job Job) error
	GetStats() *WorkerPoolStats
	Resize(newSize int) error
}

// Job represents a unit of work to be processed by worker pool
type Job interface {
	Execute(ctx context.Context) (interface{}, error)
	GetPriority() int
	GetID() string
	GetTimeout() time.Duration
}

// ForkLoadBalancer distributes work across multiple fork instances
type ForkLoadBalancer interface {
	GetFork(ctx context.Context) (Fork, error)
	ReleaseFork(fork Fork) error
	GetBestFork(ctx context.Context) (Fork, error)
	GetStats() *LoadBalancerStats
}

// LatencyMonitor tracks processing latencies and performance metrics
type LatencyMonitor interface {
	RecordLatency(operation string, duration time.Duration)
	GetAverageLatency(operation string) time.Duration
	GetP95Latency(operation string) time.Duration
	GetP99Latency(operation string) time.Duration
	CheckThresholds() []PerformanceAlert
	GetMetrics() *LatencyMetrics
}

// ProcessingResult contains the result of transaction processing
type ProcessingResult struct {
	Transaction      *types.Transaction
	SimulationResult *SimulationResult
	Opportunities    []*MEVOpportunity
	ProcessingTime   time.Duration
	Success          bool
	Error            error
}

// ProcessingStats provides statistics about transaction processing
type ProcessingStats struct {
	TotalProcessed      int64
	SuccessfulProcessed int64
	FailedProcessed     int64
	AverageLatency      time.Duration
	ThroughputTPS       float64
	CurrentLoad         float64
	QueueDepth          int
}

// WorkerPoolStats provides statistics about worker pool performance
type WorkerPoolStats struct {
	PoolSize       int
	ActiveWorkers  int
	QueuedJobs     int
	CompletedJobs  int64
	FailedJobs     int64
	AverageLatency time.Duration
	Utilization    float64
}

// LoadBalancerStats provides statistics about fork load balancing
type LoadBalancerStats struct {
	TotalForks       int
	HealthyForks     int
	LoadDistribution map[string]int
	AverageLatency   time.Duration
	FailoverCount    int64
}

// OperationMetrics contains metrics for a specific operation
type OperationMetrics struct {
	Count           int64
	TotalDuration   time.Duration
	AverageDuration time.Duration
	P50Duration     time.Duration
	P95Duration     time.Duration
	P99Duration     time.Duration
	MaxDuration     time.Duration
	MinDuration     time.Duration
}

// PerformanceAlert represents a performance threshold alert
type PerformanceAlert struct {
	Operation string
	Metric    string
	Threshold time.Duration
	Current   time.Duration
	Timestamp time.Time
	Severity  AlertSeverity
}

// TransactionJob implements the Job interface for transaction processing
type TransactionJob struct {
	ID          string
	Transaction *types.Transaction
	Priority    int
	Timeout     time.Duration
	Callback    func(*ProcessingResult)
}

// StrategyDetectionJob implements the Job interface for strategy detection
type StrategyDetectionJob struct {
	ID               string
	Transaction      *types.Transaction
	SimulationResult *SimulationResult
	Priority         int
	Timeout          time.Duration
	Callback         func([]*MEVOpportunity)
}
