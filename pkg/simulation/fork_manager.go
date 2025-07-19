package simulation

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// ForkManagerConfig holds configuration for the fork manager
type ForkManagerConfig struct {
	MaxForks        int           `json:"max_forks"`
	MinForks        int           `json:"min_forks"`
	ForkURL         string        `json:"fork_url"`
	AnvilPath       string        `json:"anvil_path"`
	BasePort        int           `json:"base_port"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	ForkTimeout     time.Duration `json:"fork_timeout"`
}

// DefaultForkManagerConfig returns default configuration
func DefaultForkManagerConfig() *ForkManagerConfig {
	return &ForkManagerConfig{
		MaxForks:        10,
		MinForks:        2,
		ForkURL:         "https://mainnet.base.org",
		AnvilPath:       "anvil",
		BasePort:        8545,
		HealthCheckInterval: 30 * time.Second,
		ForkTimeout:     10 * time.Second,
	}
}

// forkManager implements the ForkManager interface
type forkManager struct {
	config       *ForkManagerConfig
	forks        map[string]*anvilFork
	availableForks chan *anvilFork
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	nextPort     int
	stats        interfaces.ForkPoolStats
}

// NewForkManager creates a new fork manager instance
func NewForkManager(config *ForkManagerConfig) interfaces.ForkManager {
	if config == nil {
		config = DefaultForkManagerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())
	
	fm := &forkManager{
		config:         config,
		forks:          make(map[string]*anvilFork),
		availableForks: make(chan *anvilFork, config.MaxForks),
		ctx:            ctx,
		cancel:         cancel,
		nextPort:       config.BasePort,
	}

	// Initialize minimum number of forks
	fm.initializeForks()
	
	// Start health check routine
	fm.wg.Add(1)
	go fm.healthCheckRoutine()

	return fm
}

// CreateFork creates a new Anvil fork instance
func (fm *forkManager) CreateFork(ctx context.Context, forkURL string) (interfaces.Fork, error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if len(fm.forks) >= fm.config.MaxForks {
		return nil, fmt.Errorf("maximum number of forks (%d) reached", fm.config.MaxForks)
	}

	port := fm.nextPort
	fm.nextPort++

	fork, err := fm.createAnvilFork(ctx, forkURL, port)
	if err != nil {
		return nil, fmt.Errorf("failed to create fork: %w", err)
	}

	fm.forks[fork.GetID()] = fork
	fm.updateStats()

	return fork, nil
}

// GetAvailableFork returns an available fork from the pool
func (fm *forkManager) GetAvailableFork(ctx context.Context) (interfaces.Fork, error) {
	select {
	case fork := <-fm.availableForks:
		if fork.IsHealthy() {
			fm.mu.Lock()
			fm.stats.BusyForks++
			fm.stats.AvailableForks--
			fm.mu.Unlock()
			return fork, nil
		}
		// Fork is unhealthy, try to replace it
		fm.replaceFork(fork)
		return fm.GetAvailableFork(ctx)
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(fm.config.ForkTimeout):
		return nil, fmt.Errorf("timeout waiting for available fork")
	}
}

// ReleaseFork returns a fork to the available pool
func (fm *forkManager) ReleaseFork(fork interfaces.Fork) error {
	anvilFork, ok := fork.(*anvilFork)
	if !ok {
		return fmt.Errorf("invalid fork type")
	}

	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, exists := fm.forks[fork.GetID()]; !exists {
		return fmt.Errorf("fork not managed by this manager")
	}

	// Reset fork state before returning to pool
	if err := anvilFork.Reset(); err != nil {
		// If reset fails, remove the fork and create a new one
		fm.removeFork(anvilFork)
		go fm.ensureMinimumForks()
		return fmt.Errorf("failed to reset fork, removed from pool: %w", err)
	}

	select {
	case fm.availableForks <- anvilFork:
		fm.stats.BusyForks--
		fm.stats.AvailableForks++
	default:
		// Pool is full, this shouldn't happen but handle gracefully
		return fmt.Errorf("fork pool is full")
	}

	return nil
}

// CleanupForks shuts down all fork instances
func (fm *forkManager) CleanupForks() error {
	fm.cancel()
	fm.wg.Wait()

	fm.mu.Lock()
	defer fm.mu.Unlock()

	var errors []error
	for _, fork := range fm.forks {
		if err := fork.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	fm.forks = make(map[string]*anvilFork)
	
	// Drain the available forks channel
	for len(fm.availableForks) > 0 {
		<-fm.availableForks
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during cleanup: %v", errors)
	}

	return nil
}

// GetForkPoolStats returns current statistics about the fork pool
func (fm *forkManager) GetForkPoolStats() interfaces.ForkPoolStats {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return fm.stats
}

// initializeForks creates the minimum number of forks
func (fm *forkManager) initializeForks() {
	for i := 0; i < fm.config.MinForks; i++ {
		fork, err := fm.CreateFork(fm.ctx, fm.config.ForkURL)
		if err != nil {
			continue
		}
		fm.availableForks <- fork.(*anvilFork)
	}
}

// createAnvilFork creates a new Anvil fork instance
func (fm *forkManager) createAnvilFork(ctx context.Context, forkURL string, port int) (*anvilFork, error) {
	forkID := fmt.Sprintf("fork-%d-%d", port, time.Now().Unix())
	
	// Start Anvil process
	cmd := exec.CommandContext(ctx, fm.config.AnvilPath,
		"--fork-url", forkURL,
		"--port", strconv.Itoa(port),
		"--host", "127.0.0.1",
		"--silent",
	)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start anvil: %w", err)
	}

	// Wait for Anvil to be ready
	rpcURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	client, err := fm.waitForAnvil(ctx, rpcURL)
	if err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("anvil failed to start: %w", err)
	}

	fork := &anvilFork{
		id:      forkID,
		port:    port,
		rpcURL:  rpcURL,
		client:  client,
		cmd:     cmd,
		healthy: true,
	}

	return fork, nil
}

// waitForAnvil waits for Anvil to be ready and returns an eth client
func (fm *forkManager) waitForAnvil(ctx context.Context, rpcURL string) (*ethclient.Client, error) {
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for anvil to start")
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			client, err := ethclient.Dial(rpcURL)
			if err != nil {
				continue
			}

			// Test the connection
			_, err = client.BlockNumber(ctx)
			if err != nil {
				client.Close()
				continue
			}

			return client, nil
		}
	}
}

// healthCheckRoutine periodically checks fork health
func (fm *forkManager) healthCheckRoutine() {
	defer fm.wg.Done()
	
	ticker := time.NewTicker(fm.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-fm.ctx.Done():
			return
		case <-ticker.C:
			fm.performHealthCheck()
		}
	}
}

// performHealthCheck checks the health of all forks
func (fm *forkManager) performHealthCheck() {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	var unhealthyForks []*anvilFork
	
	for _, fork := range fm.forks {
		if !fork.IsHealthy() {
			unhealthyForks = append(unhealthyForks, fork)
		}
	}

	// Replace unhealthy forks
	for _, fork := range unhealthyForks {
		fm.replaceFork(fork)
	}

	// Ensure minimum number of forks
	go fm.ensureMinimumForks()
}

// replaceFork replaces an unhealthy fork with a new one
func (fm *forkManager) replaceFork(oldFork *anvilFork) {
	fm.removeFork(oldFork)
	
	// Try to create a replacement fork
	newFork, err := fm.createAnvilFork(fm.ctx, fm.config.ForkURL, fm.nextPort)
	if err != nil {
		fm.stats.FailedForks++
		return
	}
	
	fm.nextPort++
	fm.forks[newFork.GetID()] = newFork
	
	// Add to available pool if there's space
	select {
	case fm.availableForks <- newFork:
		fm.stats.AvailableForks++
	default:
		// Pool is full
	}
	
	fm.updateStats()
}

// removeFork removes a fork from management
func (fm *forkManager) removeFork(fork *anvilFork) {
	delete(fm.forks, fork.GetID())
	fork.Close()
	fm.updateStats()
}

// ensureMinimumForks ensures we have at least the minimum number of forks
func (fm *forkManager) ensureMinimumForks() {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	currentCount := len(fm.forks)
	if currentCount < fm.config.MinForks {
		needed := fm.config.MinForks - currentCount
		for i := 0; i < needed; i++ {
			fork, err := fm.createAnvilFork(fm.ctx, fm.config.ForkURL, fm.nextPort)
			if err != nil {
				continue
			}
			fm.nextPort++
			fm.forks[fork.GetID()] = fork
			
			select {
			case fm.availableForks <- fork:
				fm.stats.AvailableForks++
			default:
				// Pool is full
			}
		}
		fm.updateStats()
	}
}

// updateStats updates the fork pool statistics
func (fm *forkManager) updateStats() {
	fm.stats.TotalForks = len(fm.forks)
	fm.stats.AvailableForks = len(fm.availableForks)
	fm.stats.BusyForks = fm.stats.TotalForks - fm.stats.AvailableForks
}