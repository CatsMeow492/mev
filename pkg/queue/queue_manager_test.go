package queue

import (
	"fmt"
	"testing"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueueManager_ManageCapacity(t *testing.T) {
	// Create a small capacity queue and manager
	pq := NewPriorityQueueWithCapacity(3)
	qm := NewQueueManagerWithConfig(3, 5*time.Minute)

	now := time.Now()

	// Add transactions up to capacity
	tx1 := createTestTransaction("0x1", 100, 1, now.Add(-3*time.Second)) // oldest
	tx2 := createTestTransaction("0x2", 200, 2, now.Add(-2*time.Second))
	tx3 := createTestTransaction("0x3", 150, 3, now.Add(-1*time.Second))

	err := pq.Push(tx1)
	require.NoError(t, err)
	err = pq.Push(tx2)
	require.NoError(t, err)
	err = pq.Push(tx3)
	require.NoError(t, err)

	assert.Equal(t, 3, pq.Size())

	// Queue is at capacity, manage capacity should update stats
	err = qm.ManageCapacity(pq)
	require.NoError(t, err)
	assert.Equal(t, 3, pq.Size())

	// Add one more transaction to exceed capacity (queue handles eviction internally)
	tx4 := createTestTransaction("0x4", 300, 4, now)
	err = pq.Push(tx4)
	require.NoError(t, err)

	// Queue should have automatically evicted to maintain capacity
	assert.Equal(t, 3, pq.Size()) // Should be at max capacity

	// Update manager stats
	err = qm.ManageCapacity(pq)
	require.NoError(t, err)

	// Check stats
	stats := qm.GetQueueStats()
	assert.Equal(t, 3, stats.CurrentSize)
	assert.Equal(t, 3, stats.MaxSize)
	assert.Equal(t, int64(1), stats.EvictedCount)
	assert.False(t, stats.LastEviction.IsZero())
}

func TestQueueManager_EvictOldTransactions(t *testing.T) {
	pq := NewPriorityQueue()
	qm := NewQueueManager()

	now := time.Now()
	maxAge := 2 * time.Second

	// Add transactions with different ages
	tx1 := createTestTransaction("0x1", 100, 1, now.Add(-5*time.Second)) // too old
	tx2 := createTestTransaction("0x2", 200, 2, now.Add(-3*time.Second)) // too old
	tx3 := createTestTransaction("0x3", 150, 3, now.Add(-1*time.Second)) // recent
	tx4 := createTestTransaction("0x4", 300, 4, now)                     // recent

	err := pq.Push(tx1)
	require.NoError(t, err)
	err = pq.Push(tx2)
	require.NoError(t, err)
	err = pq.Push(tx3)
	require.NoError(t, err)
	err = pq.Push(tx4)
	require.NoError(t, err)

	assert.Equal(t, 4, pq.Size())

	// Evict old transactions
	err = qm.EvictOldTransactions(pq, maxAge)
	require.NoError(t, err)

	assert.Equal(t, 2, pq.Size()) // Should have 2 recent transactions left

	// Verify old transactions were removed
	_, exists := pq.GetByHash("0x1")
	assert.False(t, exists)
	_, exists = pq.GetByHash("0x2")
	assert.False(t, exists)

	// Verify recent transactions remain
	_, exists = pq.GetByHash("0x3")
	assert.True(t, exists)
	_, exists = pq.GetByHash("0x4")
	assert.True(t, exists)

	// Check stats
	stats := qm.GetQueueStats()
	assert.Equal(t, int64(2), stats.EvictedCount)
	assert.False(t, stats.LastEviction.IsZero())
}

func TestQueueManager_GetQueueStats(t *testing.T) {
	qm := NewQueueManagerWithConfig(100, 10*time.Minute)

	stats := qm.GetQueueStats()
	assert.Equal(t, 0, stats.CurrentSize)
	assert.Equal(t, 100, stats.MaxSize)
	assert.Equal(t, int64(0), stats.EvictedCount)
	assert.True(t, stats.LastEviction.IsZero())
}

func TestQueueManager_SetMaxCapacity(t *testing.T) {
	qm := NewQueueManager().(*QueueManagerImpl)

	// Initial capacity should be default
	stats := qm.GetQueueStats()
	assert.Equal(t, DefaultMaxCapacity, stats.MaxSize)

	// Update capacity
	newCapacity := 5000
	qm.SetMaxCapacity(newCapacity)

	stats = qm.GetQueueStats()
	assert.Equal(t, newCapacity, stats.MaxSize)
}

func TestQueueManager_SetMaxAge(t *testing.T) {
	qm := NewQueueManager().(*QueueManagerImpl)

	// Initial max age should be default
	assert.Equal(t, DefaultMaxAge, qm.maxAge)

	// Update max age
	newMaxAge := 10 * time.Minute
	qm.SetMaxAge(newMaxAge)

	assert.Equal(t, newMaxAge, qm.maxAge)
}

func TestQueueManager_EmptyQueueEviction(t *testing.T) {
	pq := NewPriorityQueue()
	qm := NewQueueManager()

	// Try to manage capacity on empty queue
	err := qm.ManageCapacity(pq)
	require.NoError(t, err)

	// Try to evict old transactions from empty queue
	err = qm.EvictOldTransactions(pq, time.Minute)
	require.NoError(t, err)

	stats := qm.GetQueueStats()
	assert.Equal(t, int64(0), stats.EvictedCount)
}

func TestQueueManager_MultipleEvictions(t *testing.T) {
	// Create a very small capacity queue
	pq := NewPriorityQueueWithCapacity(2)
	qm := NewQueueManagerWithConfig(2, 5*time.Minute)

	now := time.Now()

	// Add transactions that will require multiple evictions
	transactions := []*types.Transaction{
		createTestTransaction("0x1", 100, 1, now.Add(-5*time.Second)), // oldest
		createTestTransaction("0x2", 200, 2, now.Add(-4*time.Second)),
		createTestTransaction("0x3", 150, 3, now.Add(-3*time.Second)),
		createTestTransaction("0x4", 300, 4, now.Add(-2*time.Second)),
		createTestTransaction("0x5", 250, 5, now.Add(-1*time.Second)), // newest
	}

	// Add all transactions (this will trigger evictions automatically)
	for _, tx := range transactions {
		err := pq.Push(tx)
		require.NoError(t, err)
	}

	// Update manager stats
	err := qm.ManageCapacity(pq)
	require.NoError(t, err)

	assert.Equal(t, 2, pq.Size()) // Should be at max capacity

	// Check stats show multiple evictions
	stats := qm.GetQueueStats()
	assert.Equal(t, 2, stats.CurrentSize)
	assert.True(t, stats.EvictedCount >= 3) // Should have evicted at least 3 transactions
}

func TestQueueManager_ConcurrentAccess(t *testing.T) {
	pq := NewPriorityQueue()
	qm := NewQueueManager()

	now := time.Now()
	const numGoroutines = 5
	const transactionsPerGoroutine = 20

	// Channel to collect errors
	errChan := make(chan error, numGoroutines*2)
	doneChan := make(chan bool, numGoroutines*2)

	// Start goroutines that add transactions and manage capacity
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { doneChan <- true }()
			for j := 0; j < transactionsPerGoroutine; j++ {
				tx := createTestTransaction(
					fmt.Sprintf("0x%d_%d", id, j),
					int64(100+j),
					uint64(j),
					now.Add(time.Duration(j)*time.Millisecond),
				)
				if err := pq.Push(tx); err != nil {
					errChan <- err
					return
				}

				// Occasionally manage capacity
				if j%5 == 0 {
					if err := qm.ManageCapacity(pq); err != nil {
						errChan <- err
						return
					}
				}
			}
		}(i)
	}

	// Start goroutines that evict old transactions
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { doneChan <- true }()
			for j := 0; j < transactionsPerGoroutine; j++ {
				time.Sleep(time.Millisecond) // Small delay
				if err := qm.EvictOldTransactions(pq, time.Second); err != nil {
					errChan <- err
					return
				}
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines*2; i++ {
		select {
		case err := <-errChan:
			t.Fatalf("Concurrent access error: %v", err)
		case <-doneChan:
			// Goroutine completed successfully
		case <-time.After(10 * time.Second):
			t.Fatal("Test timed out")
		}
	}

	// Check that no errors occurred
	select {
	case err := <-errChan:
		t.Fatalf("Unexpected error: %v", err)
	default:
		// No errors, test passed
	}

	// Update final stats and verify consistency
	err := qm.ManageCapacity(pq)
	require.NoError(t, err)
	
	stats := qm.GetQueueStats()
	assert.Equal(t, pq.Size(), stats.CurrentSize)
}