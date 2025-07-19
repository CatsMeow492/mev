package queue

import (
	"container/heap"
	"fmt"
	"sync"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

const (
	DefaultMaxCapacity = 10000
	DefaultMaxAge      = 5 * time.Minute
)

// TransactionHeap implements heap.Interface for transaction priority queue
type TransactionHeap []*types.Transaction

func (h TransactionHeap) Len() int { return len(h) }

func (h TransactionHeap) Less(i, j int) bool {
	// Higher gas price has higher priority (max heap)
	gasCompare := h[i].GasPrice.Cmp(h[j].GasPrice)
	if gasCompare != 0 {
		return gasCompare > 0
	}
	
	// If gas prices are equal, lower nonce has higher priority
	return h[i].Nonce < h[j].Nonce
}

func (h TransactionHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *TransactionHeap) Push(x interface{}) {
	*h = append(*h, x.(*types.Transaction))
}

func (h *TransactionHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

// PriorityQueueImpl implements the PriorityQueue interface
type PriorityQueueImpl struct {
	heap        *TransactionHeap
	hashIndex   map[string]int // Maps transaction hash to heap index
	maxCapacity int
	mutex       sync.RWMutex
	stats       interfaces.QueueStats
}

// NewPriorityQueue creates a new priority queue with default capacity
func NewPriorityQueue() interfaces.PriorityQueue {
	return NewPriorityQueueWithCapacity(DefaultMaxCapacity)
}

// NewPriorityQueueWithCapacity creates a new priority queue with specified capacity
func NewPriorityQueueWithCapacity(capacity int) interfaces.PriorityQueue {
	h := &TransactionHeap{}
	heap.Init(h)
	
	return &PriorityQueueImpl{
		heap:        h,
		hashIndex:   make(map[string]int),
		maxCapacity: capacity,
		stats: interfaces.QueueStats{
			MaxSize: capacity,
		},
	}
}

// Push adds a transaction to the priority queue
func (pq *PriorityQueueImpl) Push(tx *types.Transaction) error {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	// Check if transaction already exists
	if _, exists := pq.hashIndex[tx.Hash]; exists {
		return fmt.Errorf("transaction %s already exists in queue", tx.Hash)
	}

	// Check capacity and evict if necessary
	if pq.heap.Len() >= pq.maxCapacity {
		if err := pq.evictLRU(); err != nil {
			return fmt.Errorf("failed to evict transaction: %w", err)
		}
	}

	// Add to heap
	heap.Push(pq.heap, tx)
	pq.hashIndex[tx.Hash] = pq.heap.Len() - 1
	pq.rebuildIndex()

	// Update stats
	pq.stats.CurrentSize = pq.heap.Len()
	pq.stats.TotalProcessed++

	return nil
}

// Pop removes and returns the highest priority transaction
func (pq *PriorityQueueImpl) Pop() (*types.Transaction, error) {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	if pq.heap.Len() == 0 {
		return nil, fmt.Errorf("queue is empty")
	}

	tx := heap.Pop(pq.heap).(*types.Transaction)
	delete(pq.hashIndex, tx.Hash)
	pq.rebuildIndex()

	// Update stats
	pq.stats.CurrentSize = pq.heap.Len()

	return tx, nil
}

// Peek returns the highest priority transaction without removing it
func (pq *PriorityQueueImpl) Peek() (*types.Transaction, error) {
	pq.mutex.RLock()
	defer pq.mutex.RUnlock()

	if pq.heap.Len() == 0 {
		return nil, fmt.Errorf("queue is empty")
	}

	return (*pq.heap)[0], nil
}

// Size returns the current number of transactions in the queue
func (pq *PriorityQueueImpl) Size() int {
	pq.mutex.RLock()
	defer pq.mutex.RUnlock()
	return pq.heap.Len()
}

// IsEmpty returns true if the queue is empty
func (pq *PriorityQueueImpl) IsEmpty() bool {
	pq.mutex.RLock()
	defer pq.mutex.RUnlock()
	return pq.heap.Len() == 0
}

// Clear removes all transactions from the queue
func (pq *PriorityQueueImpl) Clear() error {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	*pq.heap = (*pq.heap)[:0]
	pq.hashIndex = make(map[string]int)
	pq.stats.CurrentSize = 0

	return nil
}

// GetByHash retrieves a transaction by its hash
func (pq *PriorityQueueImpl) GetByHash(hash string) (*types.Transaction, bool) {
	pq.mutex.RLock()
	defer pq.mutex.RUnlock()

	index, exists := pq.hashIndex[hash]
	if !exists || index >= pq.heap.Len() {
		return nil, false
	}

	return (*pq.heap)[index], true
}

// RemoveByHash removes a transaction by its hash
func (pq *PriorityQueueImpl) RemoveByHash(hash string) bool {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()
	return pq.removeByHashUnsafe(hash)
}

// removeByHashUnsafe removes a transaction by hash without locking (internal use)
func (pq *PriorityQueueImpl) removeByHashUnsafe(hash string) bool {
	index, exists := pq.hashIndex[hash]
	if !exists || index >= pq.heap.Len() {
		return false
	}

	// Use heap.Remove to properly maintain heap invariant
	heap.Remove(pq.heap, index)
	delete(pq.hashIndex, hash)
	pq.rebuildIndex()

	// Update stats
	pq.stats.CurrentSize = pq.heap.Len()

	return true
}

// evictLRU removes the oldest transaction (LRU eviction)
func (pq *PriorityQueueImpl) evictLRU() error {
	if pq.heap.Len() == 0 {
		return fmt.Errorf("cannot evict from empty queue")
	}

	// Find the transaction with the oldest timestamp
	oldestIndex := 0
	oldestTime := (*pq.heap)[0].Timestamp

	for i, tx := range *pq.heap {
		if tx.Timestamp.Before(oldestTime) {
			oldestTime = tx.Timestamp
			oldestIndex = i
		}
	}

	// Remove the oldest transaction
	oldestTx := (*pq.heap)[oldestIndex]
	
	// Use heap.Remove to properly maintain heap invariant
	heap.Remove(pq.heap, oldestIndex)
	delete(pq.hashIndex, oldestTx.Hash)
	pq.rebuildIndex()

	// Update stats
	pq.stats.EvictedCount++
	pq.stats.LastEviction = time.Now()

	return nil
}

// rebuildIndex rebuilds the hash index after heap operations
func (pq *PriorityQueueImpl) rebuildIndex() {
	pq.hashIndex = make(map[string]int)
	for i, tx := range *pq.heap {
		pq.hashIndex[tx.Hash] = i
	}
}

// GetStats returns current queue statistics
func (pq *PriorityQueueImpl) GetStats() interfaces.QueueStats {
	pq.mutex.RLock()
	defer pq.mutex.RUnlock()
	
	stats := pq.stats
	stats.CurrentSize = pq.heap.Len()
	
	return stats
}