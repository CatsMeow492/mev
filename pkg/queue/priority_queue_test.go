package queue

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create test transactions
func createTestTransaction(hash string, gasPrice int64, nonce uint64, timestamp time.Time) *types.Transaction {
	return &types.Transaction{
		Hash:      hash,
		From:      common.HexToAddress("0x1234567890123456789012345678901234567890"),
		To:        &common.Address{},
		Value:     big.NewInt(1000),
		GasPrice:  big.NewInt(gasPrice),
		GasLimit:  21000,
		Nonce:     nonce,
		Data:      []byte{},
		Timestamp: timestamp,
		ChainID:   big.NewInt(8453), // Base chain ID
	}
}

func TestPriorityQueue_BasicOperations(t *testing.T) {
	pq := NewPriorityQueue()

	// Test empty queue
	assert.True(t, pq.IsEmpty())
	assert.Equal(t, 0, pq.Size())

	// Test peek on empty queue
	_, err := pq.Peek()
	assert.Error(t, err)

	// Test pop on empty queue
	_, err = pq.Pop()
	assert.Error(t, err)

	// Add transactions
	now := time.Now()
	tx1 := createTestTransaction("0x1", 100, 1, now)
	tx2 := createTestTransaction("0x2", 200, 2, now.Add(time.Second))
	tx3 := createTestTransaction("0x3", 150, 3, now.Add(2*time.Second))

	err = pq.Push(tx1)
	require.NoError(t, err)
	assert.Equal(t, 1, pq.Size())
	assert.False(t, pq.IsEmpty())

	err = pq.Push(tx2)
	require.NoError(t, err)
	assert.Equal(t, 2, pq.Size())

	err = pq.Push(tx3)
	require.NoError(t, err)
	assert.Equal(t, 3, pq.Size())

	// Test peek - should return highest gas price (tx2)
	peeked, err := pq.Peek()
	require.NoError(t, err)
	assert.Equal(t, "0x2", peeked.Hash)
	assert.Equal(t, 3, pq.Size()) // Size should not change

	// Test pop - should return highest gas price (tx2)
	popped, err := pq.Pop()
	require.NoError(t, err)
	assert.Equal(t, "0x2", popped.Hash)
	assert.Equal(t, 2, pq.Size())

	// Next pop should return tx3 (gas price 150)
	popped, err = pq.Pop()
	require.NoError(t, err)
	assert.Equal(t, "0x3", popped.Hash)
	assert.Equal(t, 1, pq.Size())

	// Last pop should return tx1 (gas price 100)
	popped, err = pq.Pop()
	require.NoError(t, err)
	assert.Equal(t, "0x1", popped.Hash)
	assert.Equal(t, 0, pq.Size())
	assert.True(t, pq.IsEmpty())
}

func TestPriorityQueue_GasPriceOrdering(t *testing.T) {
	pq := NewPriorityQueue()
	now := time.Now()

	// Add transactions with different gas prices
	transactions := []*types.Transaction{
		createTestTransaction("0x1", 50, 1, now),
		createTestTransaction("0x2", 300, 2, now),
		createTestTransaction("0x3", 100, 3, now),
		createTestTransaction("0x4", 200, 4, now),
		createTestTransaction("0x5", 150, 5, now),
	}

	for _, tx := range transactions {
		err := pq.Push(tx)
		require.NoError(t, err)
	}

	// Pop all transactions and verify they come out in gas price order (highest first)
	expectedOrder := []string{"0x2", "0x4", "0x5", "0x3", "0x1"}
	for i, expectedHash := range expectedOrder {
		tx, err := pq.Pop()
		require.NoError(t, err, "Failed to pop transaction %d", i)
		assert.Equal(t, expectedHash, tx.Hash, "Transaction %d has wrong hash", i)
	}
}

func TestPriorityQueue_NonceOrdering(t *testing.T) {
	pq := NewPriorityQueue()
	now := time.Now()

	// Add transactions with same gas price but different nonces
	tx1 := createTestTransaction("0x1", 100, 5, now)
	tx2 := createTestTransaction("0x2", 100, 2, now)
	tx3 := createTestTransaction("0x3", 100, 8, now)

	err := pq.Push(tx1)
	require.NoError(t, err)
	err = pq.Push(tx2)
	require.NoError(t, err)
	err = pq.Push(tx3)
	require.NoError(t, err)

	// Should pop in nonce order (lowest first) when gas prices are equal
	tx, err := pq.Pop()
	require.NoError(t, err)
	assert.Equal(t, "0x2", tx.Hash) // nonce 2

	tx, err = pq.Pop()
	require.NoError(t, err)
	assert.Equal(t, "0x1", tx.Hash) // nonce 5

	tx, err = pq.Pop()
	require.NoError(t, err)
	assert.Equal(t, "0x3", tx.Hash) // nonce 8
}

func TestPriorityQueue_GetByHash(t *testing.T) {
	pq := NewPriorityQueue()
	now := time.Now()

	tx1 := createTestTransaction("0x1", 100, 1, now)
	tx2 := createTestTransaction("0x2", 200, 2, now)

	err := pq.Push(tx1)
	require.NoError(t, err)
	err = pq.Push(tx2)
	require.NoError(t, err)

	// Test getting existing transaction
	found, exists := pq.GetByHash("0x1")
	assert.True(t, exists)
	assert.Equal(t, "0x1", found.Hash)

	// Test getting non-existing transaction
	_, exists = pq.GetByHash("0x999")
	assert.False(t, exists)
}

func TestPriorityQueue_RemoveByHash(t *testing.T) {
	pq := NewPriorityQueue()
	now := time.Now()

	tx1 := createTestTransaction("0x1", 100, 1, now)
	tx2 := createTestTransaction("0x2", 200, 2, now)
	tx3 := createTestTransaction("0x3", 150, 3, now)

	err := pq.Push(tx1)
	require.NoError(t, err)
	err = pq.Push(tx2)
	require.NoError(t, err)
	err = pq.Push(tx3)
	require.NoError(t, err)

	assert.Equal(t, 3, pq.Size())

	// Remove middle priority transaction
	removed := pq.RemoveByHash("0x3")
	assert.True(t, removed)
	assert.Equal(t, 2, pq.Size())

	// Verify it's actually removed
	_, exists := pq.GetByHash("0x3")
	assert.False(t, exists)

	// Verify remaining transactions are still in correct order
	tx, err := pq.Pop()
	require.NoError(t, err)
	assert.Equal(t, "0x2", tx.Hash) // highest gas price

	tx, err = pq.Pop()
	require.NoError(t, err)
	assert.Equal(t, "0x1", tx.Hash) // lowest gas price

	// Test removing non-existing transaction
	removed = pq.RemoveByHash("0x999")
	assert.False(t, removed)
}

func TestPriorityQueue_DuplicateTransactions(t *testing.T) {
	pq := NewPriorityQueue()
	now := time.Now()

	tx1 := createTestTransaction("0x1", 100, 1, now)
	tx1Duplicate := createTestTransaction("0x1", 200, 2, now) // Same hash, different data

	err := pq.Push(tx1)
	require.NoError(t, err)

	// Should fail to add duplicate
	err = pq.Push(tx1Duplicate)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	assert.Equal(t, 1, pq.Size())
}

func TestPriorityQueue_Clear(t *testing.T) {
	pq := NewPriorityQueue()
	now := time.Now()

	// Add some transactions
	for i := 0; i < 5; i++ {
		tx := createTestTransaction(fmt.Sprintf("0x%d", i), int64(100+i*10), uint64(i), now)
		err := pq.Push(tx)
		require.NoError(t, err)
	}

	assert.Equal(t, 5, pq.Size())
	assert.False(t, pq.IsEmpty())

	// Clear the queue
	err := pq.Clear()
	require.NoError(t, err)

	assert.Equal(t, 0, pq.Size())
	assert.True(t, pq.IsEmpty())

	// Verify we can't get any transactions
	_, exists := pq.GetByHash("0x0")
	assert.False(t, exists)
}

func TestPriorityQueue_CapacityEviction(t *testing.T) {
	// Create a small capacity queue for testing
	pq := NewPriorityQueueWithCapacity(3)
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

	// Add one more transaction - should evict the oldest (tx1)
	tx4 := createTestTransaction("0x4", 300, 4, now) // newest
	err = pq.Push(tx4)
	require.NoError(t, err)

	assert.Equal(t, 3, pq.Size()) // Size should remain at capacity

	// Verify tx1 was evicted
	_, exists := pq.GetByHash("0x1")
	assert.False(t, exists)

	// Verify other transactions are still there
	_, exists = pq.GetByHash("0x2")
	assert.True(t, exists)
	_, exists = pq.GetByHash("0x3")
	assert.True(t, exists)
	_, exists = pq.GetByHash("0x4")
	assert.True(t, exists)
}

func TestPriorityQueue_Stats(t *testing.T) {
	pqImpl := NewPriorityQueueWithCapacity(5).(*PriorityQueueImpl)
	now := time.Now()

	// Check initial stats
	stats := pqImpl.GetStats()
	assert.Equal(t, 0, stats.CurrentSize)
	assert.Equal(t, 5, stats.MaxSize)
	assert.Equal(t, int64(0), stats.TotalProcessed)
	assert.Equal(t, int64(0), stats.EvictedCount)

	// Add some transactions
	for i := 0; i < 3; i++ {
		tx := createTestTransaction(fmt.Sprintf("0x%d", i), int64(100+i*10), uint64(i), now)
		err := pqImpl.Push(tx)
		require.NoError(t, err)
	}

	stats = pqImpl.GetStats()
	assert.Equal(t, 3, stats.CurrentSize)
	assert.Equal(t, int64(3), stats.TotalProcessed)

	// Force eviction by filling to capacity and adding more
	for i := 3; i < 7; i++ {
		tx := createTestTransaction(fmt.Sprintf("0x%d", i), int64(100+i*10), uint64(i), now.Add(time.Duration(i)*time.Second))
		err := pqImpl.Push(tx)
		require.NoError(t, err)
	}

	stats = pqImpl.GetStats()
	assert.Equal(t, 5, stats.CurrentSize) // Should be at max capacity
	assert.Equal(t, int64(7), stats.TotalProcessed)
	assert.Equal(t, int64(2), stats.EvictedCount) // Should have evicted 2 transactions
	assert.False(t, stats.LastEviction.IsZero())
}

func TestPriorityQueue_ConcurrentAccess(t *testing.T) {
	pq := NewPriorityQueue()
	now := time.Now()

	// Test concurrent pushes and pops
	const numGoroutines = 10
	const transactionsPerGoroutine = 100

	// Channel to collect errors
	errChan := make(chan error, numGoroutines*2)
	doneChan := make(chan bool, numGoroutines*2)

	// Start producer goroutines
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
			}
		}(i)
	}

	// Start consumer goroutines
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { doneChan <- true }()
			for j := 0; j < transactionsPerGoroutine; j++ {
				// Wait a bit to let producers add some transactions
				time.Sleep(time.Microsecond)
				if _, err := pq.Pop(); err != nil {
					// It's okay if queue is empty sometimes
					if err.Error() != "queue is empty" {
						errChan <- err
						return
					}
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
}