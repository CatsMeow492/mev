package simulation

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

func TestNewForkManager(t *testing.T) {
	config := &ForkManagerConfig{
		MaxForks:        5,
		MinForks:        2,
		ForkURL:         "http://localhost:8545", // Use local test URL
		AnvilPath:       "echo", // Use echo command for testing
		BasePort:        9000,
		HealthCheckInterval: 1 * time.Second,
		ForkTimeout:     5 * time.Second,
	}

	fm := NewForkManager(config)
	require.NotNil(t, fm)

	// Cleanup
	err := fm.CleanupForks()
	assert.NoError(t, err)
}

func TestDefaultForkManagerConfig(t *testing.T) {
	config := DefaultForkManagerConfig()
	
	assert.Equal(t, 10, config.MaxForks)
	assert.Equal(t, 2, config.MinForks)
	assert.Equal(t, "https://mainnet.base.org", config.ForkURL)
	assert.Equal(t, "anvil", config.AnvilPath)
	assert.Equal(t, 8545, config.BasePort)
	assert.Equal(t, 30*time.Second, config.HealthCheckInterval)
	assert.Equal(t, 10*time.Second, config.ForkTimeout)
}

func TestForkManagerCreateFork(t *testing.T) {
	config := &ForkManagerConfig{
		MaxForks:        2,
		MinForks:        0,
		ForkURL:         "http://localhost:8545",
		AnvilPath:       "echo", // Use echo for testing
		BasePort:        9001,
		HealthCheckInterval: 1 * time.Second,
		ForkTimeout:     5 * time.Second,
	}

	fm := NewForkManager(config)
	defer fm.CleanupForks()

	ctx := context.Background()

	// Test creating a fork - this will fail with echo command but we can test the logic
	_, err := fm.CreateFork(ctx, "http://localhost:8545")
	// We expect this to fail since we're using echo instead of anvil
	assert.Error(t, err)
}

func TestForkManagerMaxForksLimit(t *testing.T) {
	config := &ForkManagerConfig{
		MaxForks:        1,
		MinForks:        0,
		ForkURL:         "http://localhost:8545",
		AnvilPath:       "echo",
		BasePort:        9002,
		HealthCheckInterval: 1 * time.Second,
		ForkTimeout:     5 * time.Second,
	}

	fm := NewForkManager(config)
	defer fm.CleanupForks()

	// Manually add a fork to the manager to simulate reaching the limit
	forkManager := fm.(*forkManager)
	forkManager.mu.Lock()
	forkManager.forks["test-fork"] = &anvilFork{id: "test-fork"}
	forkManager.mu.Unlock()

	ctx := context.Background()

	// Now try to create another fork - should fail due to limit
	_, err := fm.CreateFork(ctx, "http://localhost:8545")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum number of forks")
}

func TestForkManagerGetForkPoolStats(t *testing.T) {
	config := &ForkManagerConfig{
		MaxForks:        5,
		MinForks:        0,
		ForkURL:         "http://localhost:8545",
		AnvilPath:       "echo",
		BasePort:        9003,
		HealthCheckInterval: 1 * time.Second,
		ForkTimeout:     5 * time.Second,
	}

	fm := NewForkManager(config)
	defer fm.CleanupForks()

	stats := fm.GetForkPoolStats()
	assert.GreaterOrEqual(t, stats.TotalForks, 0)
	assert.GreaterOrEqual(t, stats.AvailableForks, 0)
	assert.GreaterOrEqual(t, stats.BusyForks, 0)
	assert.GreaterOrEqual(t, stats.FailedForks, 0)
}

func TestForkManagerCleanupForks(t *testing.T) {
	config := &ForkManagerConfig{
		MaxForks:        3,
		MinForks:        0,
		ForkURL:         "http://localhost:8545",
		AnvilPath:       "echo",
		BasePort:        9004,
		HealthCheckInterval: 1 * time.Second,
		ForkTimeout:     5 * time.Second,
	}

	fm := NewForkManager(config)

	// Cleanup should not error even with no forks
	err := fm.CleanupForks()
	assert.NoError(t, err)

	// Stats should be reset
	stats := fm.GetForkPoolStats()
	assert.Equal(t, 0, stats.TotalForks)
	assert.Equal(t, 0, stats.AvailableForks)
	assert.Equal(t, 0, stats.BusyForks)
}

func TestForkManagerGetAvailableForkTimeout(t *testing.T) {
	config := &ForkManagerConfig{
		MaxForks:        1,
		MinForks:        0,
		ForkURL:         "http://localhost:8545",
		AnvilPath:       "echo",
		BasePort:        9005,
		HealthCheckInterval: 1 * time.Second,
		ForkTimeout:     100 * time.Millisecond, // Very short timeout
	}

	fm := NewForkManager(config)
	defer fm.CleanupForks()

	ctx := context.Background()

	// Should timeout since no forks are available
	_, err := fm.GetAvailableFork(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestForkManagerContextCancellation(t *testing.T) {
	config := &ForkManagerConfig{
		MaxForks:        1,
		MinForks:        0,
		ForkURL:         "http://localhost:8545",
		AnvilPath:       "echo",
		BasePort:        9006,
		HealthCheckInterval: 1 * time.Second,
		ForkTimeout:     5 * time.Second,
	}

	fm := NewForkManager(config)
	defer fm.CleanupForks()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Should return context error
	_, err := fm.GetAvailableFork(ctx)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

// Mock fork for testing ReleaseFork functionality
type mockFork struct {
	id      string
	healthy bool
	resetErr error
}

func (m *mockFork) GetID() string {
	return m.id
}

func (m *mockFork) ExecuteTransaction(ctx context.Context, tx *types.Transaction) (*interfaces.SimulationResult, error) {
	return nil, nil
}

func (m *mockFork) GetBlockNumber() (*big.Int, error) {
	return nil, nil
}

func (m *mockFork) GetBalance(address common.Address) (*big.Int, error) {
	return nil, nil
}

func (m *mockFork) Reset() error {
	return m.resetErr
}

func (m *mockFork) Close() error {
	return nil
}

func (m *mockFork) IsHealthy() bool {
	return m.healthy
}

func TestForkManagerReleaseForkInvalidType(t *testing.T) {
	config := DefaultForkManagerConfig()
	fm := NewForkManager(config)
	defer fm.CleanupForks()

	mockFork := &mockFork{id: "test-fork", healthy: true}
	
	err := fm.ReleaseFork(mockFork)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid fork type")
}

func TestForkManagerConcurrentAccess(t *testing.T) {
	config := &ForkManagerConfig{
		MaxForks:        3,
		MinForks:        0,
		ForkURL:         "http://localhost:8545",
		AnvilPath:       "echo",
		BasePort:        9007,
		HealthCheckInterval: 1 * time.Second,
		ForkTimeout:     1 * time.Second,
	}

	fm := NewForkManager(config)
	defer fm.CleanupForks()

	// Test concurrent access to stats
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			stats := fm.GetForkPoolStats()
			assert.GreaterOrEqual(t, stats.TotalForks, 0)
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}