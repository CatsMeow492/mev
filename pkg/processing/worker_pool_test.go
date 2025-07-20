package processing

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SimpleTestJob implements the Job interface for basic testing
type SimpleTestJob struct {
	ID       string
	Duration time.Duration
	Priority int
	executed bool
	result   string
	mu       sync.Mutex
}

func (j *SimpleTestJob) Execute(ctx context.Context) (interface{}, error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.executed {
		return nil, fmt.Errorf("job already executed")
	}

	// Simulate work
	time.Sleep(j.Duration)
	j.executed = true
	j.result = fmt.Sprintf("job-%s-completed", j.ID)

	return j.result, nil
}

func (j *SimpleTestJob) GetPriority() int {
	return j.Priority
}

func (j *SimpleTestJob) GetID() string {
	return j.ID
}

func (j *SimpleTestJob) GetTimeout() time.Duration {
	return 5 * time.Second
}

func (j *SimpleTestJob) IsExecuted() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.executed
}

func (j *SimpleTestJob) GetResult() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.result
}

// TestWorkerPoolBasic tests basic worker pool functionality
func TestWorkerPoolBasic(t *testing.T) {
	config := &WorkerPoolConfig{
		PoolSize:        3,
		QueueSize:       10,
		MaxJobTimeout:   2 * time.Second,
		ShutdownTimeout: 5 * time.Second,
		EnableMetrics:   true,
	}

	pool := NewWorkerPool(config)
	ctx := context.Background()

	// Test start
	err := pool.Start(ctx)
	require.NoError(t, err)

	// Submit a simple job
	job := &SimpleTestJob{
		ID:       "test-job-1",
		Duration: 50 * time.Millisecond,
		Priority: 1,
	}

	err = pool.Submit(job)
	require.NoError(t, err)

	// Wait for job to complete
	time.Sleep(200 * time.Millisecond)

	// Check job was executed
	assert.True(t, job.IsExecuted())
	assert.Equal(t, "job-test-job-1-completed", job.GetResult())

	// Check stats
	stats := pool.GetStats()
	assert.Greater(t, stats.CompletedJobs, int64(0))

	// Test stop
	err = pool.Stop(ctx)
	require.NoError(t, err)
}

// TestWorkerPoolConcurrency tests concurrent job submission
func TestWorkerPoolConcurrency(t *testing.T) {
	config := &WorkerPoolConfig{
		PoolSize:        5,
		QueueSize:       50,
		MaxJobTimeout:   3 * time.Second,
		ShutdownTimeout: 5 * time.Second,
		EnableMetrics:   true,
	}

	pool := NewWorkerPool(config)
	ctx := context.Background()

	err := pool.Start(ctx)
	require.NoError(t, err)
	defer pool.Stop(ctx)

	// Submit multiple jobs concurrently
	numJobs := 20
	jobs := make([]*SimpleTestJob, numJobs)
	var wg sync.WaitGroup

	for i := 0; i < numJobs; i++ {
		jobs[i] = &SimpleTestJob{
			ID:       fmt.Sprintf("concurrent-job-%d", i),
			Duration: 10 * time.Millisecond,
			Priority: i % 5, // Varying priorities
		}

		wg.Add(1)
		go func(j *SimpleTestJob) {
			defer wg.Done()
			err := pool.Submit(j)
			assert.NoError(t, err)
		}(jobs[i])
	}

	wg.Wait()

	// Wait for all jobs to complete
	time.Sleep(1 * time.Second)

	// Check all jobs were executed
	completedCount := 0
	for _, job := range jobs {
		if job.IsExecuted() {
			completedCount++
		}
	}

	assert.Equal(t, numJobs, completedCount, "All jobs should be completed")

	// Check stats
	stats := pool.GetStats()
	assert.GreaterOrEqual(t, stats.CompletedJobs, int64(numJobs))
	assert.Greater(t, stats.Utilization, 0.0)
}

// TestLatencyMonitorBasic tests basic latency monitoring functionality
func TestLatencyMonitorBasic(t *testing.T) {
	monitor := NewLatencyMonitor()

	// Record some latencies
	monitor.RecordLatency("test_operation", 50*time.Millisecond)
	monitor.RecordLatency("test_operation", 75*time.Millisecond)
	monitor.RecordLatency("test_operation", 100*time.Millisecond)

	// Get average latency
	avgLatency := monitor.GetAverageLatency("test_operation")
	assert.Greater(t, avgLatency, 50*time.Millisecond)
	assert.Less(t, avgLatency, 100*time.Millisecond)

	// Get P95 latency
	p95Latency := monitor.GetP95Latency("test_operation")
	assert.GreaterOrEqual(t, p95Latency, avgLatency)

	// Get metrics
	metrics := monitor.GetMetrics()
	assert.Equal(t, "overall", metrics.Operation)
	assert.Equal(t, 3, metrics.SampleCount)
	assert.Greater(t, metrics.AverageLatency, 50*time.Millisecond)
}

// TestLatencyMonitorThresholds tests alert threshold functionality
func TestLatencyMonitorThresholds(t *testing.T) {
	monitor := NewLatencyMonitor()

	// Record latencies that exceed thresholds
	monitor.RecordLatency("process_transaction", 150*time.Millisecond) // Exceeds 100ms threshold
	monitor.RecordLatency("process_transaction", 200*time.Millisecond)

	// Check thresholds
	alerts := monitor.CheckThresholds()
	assert.Greater(t, len(alerts), 0, "Should generate alerts for exceeded thresholds")

	for _, alert := range alerts {
		assert.Equal(t, "process_transaction", alert.Operation)
		assert.Greater(t, alert.Current, alert.Threshold)
	}
}

// TestForkLoadBalancerBasic tests basic fork load balancer functionality
func TestForkLoadBalancerBasic(t *testing.T) {
	// Create a simple mock fork manager
	mockManager := &SimpleMockForkManager{}
	balancer := NewForkLoadBalancer(mockManager)

	ctx := context.Background()

	// Test getting a fork
	fork, err := balancer.GetFork(ctx)
	require.NoError(t, err)
	require.NotNil(t, fork)

	// Test releasing a fork
	err = balancer.ReleaseFork(fork)
	require.NoError(t, err)

	// Test stats
	stats := balancer.GetStats()
	assert.GreaterOrEqual(t, int64(stats.TotalForks), int64(0))
}

// SimpleMockForkManager for basic testing
type SimpleMockForkManager struct{}

func (m *SimpleMockForkManager) GetAvailableFork(ctx context.Context) (interfaces.Fork, error) {
	return &SimpleMockFork{id: "test-fork"}, nil
}

func (m *SimpleMockForkManager) ReleaseFork(fork interfaces.Fork) error {
	return nil
}

func (m *SimpleMockForkManager) GetForkPoolStats() interfaces.ForkPoolStats {
	return interfaces.ForkPoolStats{
		TotalForks:  2,
		FailedForks: 0,
	}
}

func (m *SimpleMockForkManager) CleanupForks() error {
	return nil
}

func (m *SimpleMockForkManager) CreateFork(ctx context.Context, forkID string) (interfaces.Fork, error) {
	return &SimpleMockFork{id: forkID}, nil
}

// SimpleMockFork for basic testing
type SimpleMockFork struct {
	id string
}

func (f *SimpleMockFork) GetID() string {
	return f.id
}

func (f *SimpleMockFork) ExecuteTransaction(ctx context.Context, tx *types.Transaction) (*interfaces.SimulationResult, error) {
	return &interfaces.SimulationResult{
		Success:       true,
		GasUsed:       21000,
		ExecutionTime: 10 * time.Millisecond,
	}, nil
}

func (f *SimpleMockFork) GetBlockNumber() (*big.Int, error) {
	return big.NewInt(1000000), nil
}

func (f *SimpleMockFork) GetBalance(address common.Address) (*big.Int, error) {
	return big.NewInt(1000000000000000000), nil
}

func (f *SimpleMockFork) Reset() error {
	return nil
}

func (f *SimpleMockFork) Close() error {
	return nil
}

func (f *SimpleMockFork) IsHealthy() bool {
	return true
}
