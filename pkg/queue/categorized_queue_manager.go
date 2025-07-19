package queue

import (
	"fmt"
	"sync"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

// CategorizedQueueManager manages separate queues for different transaction categories
type CategorizedQueueManager struct {
	queues map[types.TransactionType]interfaces.PriorityQueue
	filter interfaces.TransactionFilter
	mutex  sync.RWMutex
	stats  map[types.TransactionType]interfaces.QueueStats
}

// NewCategorizedQueueManager creates a new categorized queue manager
func NewCategorizedQueueManager(filter interfaces.TransactionFilter) *CategorizedQueueManager {
	cqm := &CategorizedQueueManager{
		queues: make(map[types.TransactionType]interfaces.PriorityQueue),
		filter: filter,
		stats:  make(map[types.TransactionType]interfaces.QueueStats),
	}
	
	// Initialize queues for each transaction type
	transactionTypes := []types.TransactionType{
		types.TxTypeSwap,
		types.TxTypeTransfer,
		types.TxTypeLiquidity,
		types.TxTypeBridge,
		types.TxTypeContract,
		types.TxTypeUnknown,
	}
	
	for _, txType := range transactionTypes {
		cqm.queues[txType] = NewPriorityQueue()
		cqm.stats[txType] = interfaces.QueueStats{
			MaxSize: DefaultMaxCapacity,
		}
	}
	
	return cqm
}

// AddTransaction adds a transaction to the appropriate category queue after filtering
func (cqm *CategorizedQueueManager) AddTransaction(tx *types.Transaction) error {
	// Apply filter first
	if !cqm.filter.ShouldProcess(tx) {
		return fmt.Errorf("transaction %s filtered out", tx.Hash)
	}
	
	// Determine transaction category
	txType := tx.GetTransactionType()
	
	cqm.mutex.Lock()
	defer cqm.mutex.Unlock()
	
	queue, exists := cqm.queues[txType]
	if !exists {
		return fmt.Errorf("no queue found for transaction type %s", txType)
	}
	
	// Add to appropriate queue
	err := queue.Push(tx)
	if err != nil {
		return fmt.Errorf("failed to add transaction to %s queue: %w", txType, err)
	}
	
	// Update stats
	stats := cqm.stats[txType]
	stats.CurrentSize = queue.Size()
	stats.TotalProcessed++
	cqm.stats[txType] = stats
	
	return nil
}

// GetTransaction retrieves the highest priority transaction from a specific category
func (cqm *CategorizedQueueManager) GetTransaction(txType types.TransactionType) (*types.Transaction, error) {
	cqm.mutex.RLock()
	defer cqm.mutex.RUnlock()
	
	queue, exists := cqm.queues[txType]
	if !exists {
		return nil, fmt.Errorf("no queue found for transaction type %s", txType)
	}
	
	tx, err := queue.Pop()
	if err != nil {
		return nil, err
	}
	
	// Update stats
	stats := cqm.stats[txType]
	stats.CurrentSize = queue.Size()
	cqm.stats[txType] = stats
	
	return tx, nil
}

// GetNextTransaction retrieves the highest priority transaction across all categories
// Prioritizes MEV-relevant transaction types
func (cqm *CategorizedQueueManager) GetNextTransaction() (*types.Transaction, error) {
	cqm.mutex.RLock()
	defer cqm.mutex.RUnlock()
	
	// Priority order for MEV strategies
	priorityOrder := []types.TransactionType{
		types.TxTypeSwap,      // Highest priority for sandwich/backrun
		types.TxTypeLiquidity, // Liquidity changes affect prices
		types.TxTypeBridge,    // Cross-layer arbitrage opportunities
		types.TxTypeTransfer,  // High-value transfers for frontrunning
		types.TxTypeContract,  // Other contract interactions
		types.TxTypeUnknown,   // Lowest priority
	}
	
	for _, txType := range priorityOrder {
		queue := cqm.queues[txType]
		if !queue.IsEmpty() {
			tx, err := queue.Pop()
			if err != nil {
				continue // Try next queue
			}
			
			// Update stats
			stats := cqm.stats[txType]
			stats.CurrentSize = queue.Size()
			cqm.stats[txType] = stats
			
			return tx, nil
		}
	}
	
	return nil, fmt.Errorf("no transactions available in any queue")
}

// PeekTransaction returns the highest priority transaction from a specific category without removing it
func (cqm *CategorizedQueueManager) PeekTransaction(txType types.TransactionType) (*types.Transaction, error) {
	cqm.mutex.RLock()
	defer cqm.mutex.RUnlock()
	
	queue, exists := cqm.queues[txType]
	if !exists {
		return nil, fmt.Errorf("no queue found for transaction type %s", txType)
	}
	
	return queue.Peek()
}

// GetQueueSize returns the size of a specific category queue
func (cqm *CategorizedQueueManager) GetQueueSize(txType types.TransactionType) int {
	cqm.mutex.RLock()
	defer cqm.mutex.RUnlock()
	
	queue, exists := cqm.queues[txType]
	if !exists {
		return 0
	}
	
	return queue.Size()
}

// GetTotalSize returns the total number of transactions across all queues
func (cqm *CategorizedQueueManager) GetTotalSize() int {
	cqm.mutex.RLock()
	defer cqm.mutex.RUnlock()
	
	total := 0
	for _, queue := range cqm.queues {
		total += queue.Size()
	}
	return total
}

// GetQueueStats returns statistics for a specific category queue
func (cqm *CategorizedQueueManager) GetQueueStats(txType types.TransactionType) (interfaces.QueueStats, error) {
	cqm.mutex.RLock()
	defer cqm.mutex.RUnlock()
	
	stats, exists := cqm.stats[txType]
	if !exists {
		return interfaces.QueueStats{}, fmt.Errorf("no stats found for transaction type %s", txType)
	}
	
	return stats, nil
}

// GetAllQueueStats returns statistics for all category queues
func (cqm *CategorizedQueueManager) GetAllQueueStats() map[types.TransactionType]interfaces.QueueStats {
	cqm.mutex.RLock()
	defer cqm.mutex.RUnlock()
	
	// Create a copy to avoid race conditions
	statsCopy := make(map[types.TransactionType]interfaces.QueueStats)
	for txType, stats := range cqm.stats {
		statsCopy[txType] = stats
	}
	
	return statsCopy
}

// ClearQueue clears all transactions from a specific category queue
func (cqm *CategorizedQueueManager) ClearQueue(txType types.TransactionType) error {
	cqm.mutex.Lock()
	defer cqm.mutex.Unlock()
	
	queue, exists := cqm.queues[txType]
	if !exists {
		return fmt.Errorf("no queue found for transaction type %s", txType)
	}
	
	err := queue.Clear()
	if err != nil {
		return err
	}
	
	// Reset stats
	stats := cqm.stats[txType]
	stats.CurrentSize = 0
	cqm.stats[txType] = stats
	
	return nil
}

// ClearAllQueues clears all transactions from all category queues
func (cqm *CategorizedQueueManager) ClearAllQueues() error {
	cqm.mutex.Lock()
	defer cqm.mutex.Unlock()
	
	for txType, queue := range cqm.queues {
		err := queue.Clear()
		if err != nil {
			return fmt.Errorf("failed to clear %s queue: %w", txType, err)
		}
		
		// Reset stats
		stats := cqm.stats[txType]
		stats.CurrentSize = 0
		cqm.stats[txType] = stats
	}
	
	return nil
}

// UpdateFilter updates the transaction filter
func (cqm *CategorizedQueueManager) UpdateFilter(filter interfaces.TransactionFilter) {
	cqm.mutex.Lock()
	defer cqm.mutex.Unlock()
	cqm.filter = filter
}

// GetFilter returns the current transaction filter
func (cqm *CategorizedQueueManager) GetFilter() interfaces.TransactionFilter {
	cqm.mutex.RLock()
	defer cqm.mutex.RUnlock()
	return cqm.filter
}

// EvictOldTransactions removes old transactions from all queues
func (cqm *CategorizedQueueManager) EvictOldTransactions(maxAge time.Duration) error {
	cqm.mutex.Lock()
	defer cqm.mutex.Unlock()
	
	cutoffTime := time.Now().Add(-maxAge)
	
	for txType, queue := range cqm.queues {
		if pqImpl, ok := queue.(*PriorityQueueImpl); ok {
			pqImpl.mutex.Lock()
			
			// Collect transactions to evict
			var toEvict []string
			for _, tx := range *pqImpl.heap {
				if tx.Timestamp.Before(cutoffTime) {
					toEvict = append(toEvict, tx.Hash)
				}
			}
			
			// Remove old transactions
			evictedCount := 0
			for _, hash := range toEvict {
				if pqImpl.removeByHashUnsafe(hash) {
					evictedCount++
				}
			}
			
			pqImpl.mutex.Unlock()
			
			// Update stats
			if evictedCount > 0 {
				stats := cqm.stats[txType]
				stats.EvictedCount += int64(evictedCount)
				stats.LastEviction = time.Now()
				stats.CurrentSize = queue.Size()
				cqm.stats[txType] = stats
			}
		}
	}
	
	return nil
}