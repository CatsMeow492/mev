package queue

import (
	"fmt"
	"sync"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// QueueManagerImpl implements the QueueManager interface
type QueueManagerImpl struct {
	maxCapacity int
	maxAge      time.Duration
	stats       interfaces.QueueStats
	mutex       sync.RWMutex
}

// NewQueueManager creates a new queue manager with default settings
func NewQueueManager() interfaces.QueueManager {
	return &QueueManagerImpl{
		maxCapacity: DefaultMaxCapacity,
		maxAge:      DefaultMaxAge,
		stats: interfaces.QueueStats{
			MaxSize: DefaultMaxCapacity,
		},
	}
}

// NewQueueManagerWithConfig creates a new queue manager with custom settings
func NewQueueManagerWithConfig(maxCapacity int, maxAge time.Duration) interfaces.QueueManager {
	return &QueueManagerImpl{
		maxCapacity: maxCapacity,
		maxAge:      maxAge,
		stats: interfaces.QueueStats{
			MaxSize: maxCapacity,
		},
	}
}

// ManageCapacity ensures the queue doesn't exceed maximum capacity
func (qm *QueueManagerImpl) ManageCapacity(queue interfaces.PriorityQueue) error {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()

	currentSize := queue.Size()
	
	// Update current size in stats
	qm.stats.CurrentSize = currentSize

	// The priority queue already handles capacity management internally
	// This method is mainly for monitoring and updating stats
	if pqImpl, ok := queue.(*PriorityQueueImpl); ok {
		stats := pqImpl.GetStats()
		qm.stats.EvictedCount = stats.EvictedCount
		qm.stats.LastEviction = stats.LastEviction
	}

	return nil
}

// EvictOldTransactions removes transactions older than maxAge
func (qm *QueueManagerImpl) EvictOldTransactions(queue interfaces.PriorityQueue, maxAge time.Duration) error {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()

	if maxAge == 0 {
		maxAge = qm.maxAge
	}

	cutoffTime := time.Now().Add(-maxAge)
	evictedCount := 0

	// For this implementation, we need to access the internal heap
	// This is a limitation of the current interface design
	if pqImpl, ok := queue.(*PriorityQueueImpl); ok {
		pqImpl.mutex.Lock()
		defer pqImpl.mutex.Unlock()

		// Collect transactions to evict
		var toEvict []string
		for _, tx := range *pqImpl.heap {
			if tx.Timestamp.Before(cutoffTime) {
				toEvict = append(toEvict, tx.Hash)
			}
		}

		// Remove old transactions
		for _, hash := range toEvict {
			if pqImpl.removeByHashUnsafe(hash) {
				evictedCount++
			}
		}

		if evictedCount > 0 {
			qm.stats.EvictedCount += int64(evictedCount)
			qm.stats.LastEviction = time.Now()
		}
	} else {
		return fmt.Errorf("queue does not support age-based eviction")
	}

	return nil
}

// GetQueueStats returns current queue statistics
func (qm *QueueManagerImpl) GetQueueStats() interfaces.QueueStats {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()
	return qm.stats
}

// SetMaxCapacity updates the maximum capacity
func (qm *QueueManagerImpl) SetMaxCapacity(capacity int) {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()
	qm.maxCapacity = capacity
	qm.stats.MaxSize = capacity
}

// SetMaxAge updates the maximum age for transactions
func (qm *QueueManagerImpl) SetMaxAge(maxAge time.Duration) {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()
	qm.maxAge = maxAge
}

