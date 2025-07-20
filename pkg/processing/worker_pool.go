package processing

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// WorkerPoolConfig holds configuration for the worker pool
type WorkerPoolConfig struct {
	PoolSize        int           `json:"pool_size"`
	QueueSize       int           `json:"queue_size"`
	MaxJobTimeout   time.Duration `json:"max_job_timeout"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
	EnableMetrics   bool          `json:"enable_metrics"`
}

// DefaultWorkerPoolConfig returns default configuration
func DefaultWorkerPoolConfig() *WorkerPoolConfig {
	return &WorkerPoolConfig{
		PoolSize:        10,
		QueueSize:       1000,
		MaxJobTimeout:   30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		EnableMetrics:   true,
	}
}

// workerPool implements the WorkerPool interface
type workerPool struct {
	config   *WorkerPoolConfig
	jobQueue chan interfaces.Job
	workers  []*worker
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.RWMutex
	running  bool

	// Metrics
	stats         *interfaces.WorkerPoolStats
	completedJobs int64
	failedJobs    int64
	totalLatency  int64
	jobCount      int64
}

// worker represents a single worker goroutine
type worker struct {
	id       int
	pool     *workerPool
	jobQueue chan interfaces.Job
	quit     chan bool
}

// NewWorkerPool creates a new worker pool instance
func NewWorkerPool(config *WorkerPoolConfig) interfaces.WorkerPool {
	if config == nil {
		config = DefaultWorkerPoolConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	wp := &workerPool{
		config:   config,
		jobQueue: make(chan interfaces.Job, config.QueueSize),
		workers:  make([]*worker, config.PoolSize),
		ctx:      ctx,
		cancel:   cancel,
		stats: &interfaces.WorkerPoolStats{
			PoolSize: config.PoolSize,
		},
	}

	return wp
}

// Start starts the worker pool
func (wp *workerPool) Start(ctx context.Context) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if wp.running {
		return fmt.Errorf("worker pool is already running")
	}

	// Create and start workers
	for i := 0; i < wp.config.PoolSize; i++ {
		worker := &worker{
			id:       i,
			pool:     wp,
			jobQueue: wp.jobQueue,
			quit:     make(chan bool),
		}
		wp.workers[i] = worker

		wp.wg.Add(1)
		go worker.start()
	}

	wp.running = true
	return nil
}

// Stop stops the worker pool gracefully
func (wp *workerPool) Stop(ctx context.Context) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if !wp.running {
		return fmt.Errorf("worker pool is not running")
	}

	// Cancel the context to signal workers to stop
	wp.cancel()

	// Close the job queue
	close(wp.jobQueue)

	// Stop all workers
	for _, worker := range wp.workers {
		worker.stop()
	}

	// Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All workers finished gracefully
	case <-time.After(wp.config.ShutdownTimeout):
		// Timeout reached, force shutdown
		return fmt.Errorf("worker pool shutdown timeout")
	case <-ctx.Done():
		return ctx.Err()
	}

	wp.running = false
	return nil
}

// Submit submits a job to the worker pool
func (wp *workerPool) Submit(job interfaces.Job) error {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	if !wp.running {
		return fmt.Errorf("worker pool is not running")
	}

	select {
	case wp.jobQueue <- job:
		return nil
	default:
		return fmt.Errorf("job queue is full")
	}
}

// GetStats returns current worker pool statistics
func (wp *workerPool) GetStats() *interfaces.WorkerPoolStats {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	completed := atomic.LoadInt64(&wp.completedJobs)
	failed := atomic.LoadInt64(&wp.failedJobs)
	totalLatency := atomic.LoadInt64(&wp.totalLatency)
	jobCount := atomic.LoadInt64(&wp.jobCount)

	stats := &interfaces.WorkerPoolStats{
		PoolSize:      wp.config.PoolSize,
		ActiveWorkers: wp.getActiveWorkerCount(),
		QueuedJobs:    len(wp.jobQueue),
		CompletedJobs: completed,
		FailedJobs:    failed,
	}

	if jobCount > 0 {
		stats.AverageLatency = time.Duration(totalLatency / jobCount)
	}

	if wp.config.PoolSize > 0 {
		stats.Utilization = float64(stats.ActiveWorkers) / float64(wp.config.PoolSize)
	}

	return stats
}

// Resize changes the pool size (not implemented for this version)
func (wp *workerPool) Resize(newSize int) error {
	return fmt.Errorf("dynamic resizing not implemented")
}

// getActiveWorkerCount returns the number of currently active workers
func (wp *workerPool) getActiveWorkerCount() int {
	// This is a simplified implementation
	// In a production system, you'd track active workers more precisely
	activeCount := 0
	for _, worker := range wp.workers {
		if worker != nil {
			activeCount++
		}
	}
	return activeCount
}

// worker methods

// start starts the worker goroutine
func (w *worker) start() {
	defer w.pool.wg.Done()

	for {
		select {
		case job := <-w.jobQueue:
			if job == nil {
				return // Channel closed
			}
			w.processJob(job)

		case <-w.quit:
			return

		case <-w.pool.ctx.Done():
			return
		}
	}
}

// stop stops the worker
func (w *worker) stop() {
	close(w.quit)
}

// processJob executes a job with timeout and metrics tracking
func (w *worker) processJob(job interfaces.Job) {
	startTime := time.Now()

	// Create timeout context for the job
	timeout := job.GetTimeout()
	if timeout == 0 {
		timeout = w.pool.config.MaxJobTimeout
	}

	ctx, cancel := context.WithTimeout(w.pool.ctx, timeout)
	defer cancel()

	// Execute the job
	_, err := job.Execute(ctx)

	// Update metrics
	duration := time.Since(startTime)
	atomic.AddInt64(&w.pool.jobCount, 1)
	atomic.AddInt64(&w.pool.totalLatency, int64(duration))

	if err != nil {
		atomic.AddInt64(&w.pool.failedJobs, 1)
	} else {
		atomic.AddInt64(&w.pool.completedJobs, 1)
	}
}
