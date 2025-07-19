package interfaces

import (
	"math/big"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

// PriorityQueue manages prioritized transaction queue
type PriorityQueue interface {
	Push(tx *types.Transaction) error
	Pop() (*types.Transaction, error)
	Peek() (*types.Transaction, error)
	Size() int
	IsEmpty() bool
	Clear() error
	GetByHash(hash string) (*types.Transaction, bool)
	RemoveByHash(hash string) bool
}

// TransactionFilter filters transactions by relevance criteria
type TransactionFilter interface {
	ShouldProcess(tx *types.Transaction) bool
	GetFilterCriteria() FilterCriteria
	UpdateCriteria(criteria FilterCriteria) error
}

// FilterCriteria defines transaction filtering parameters
type FilterCriteria struct {
	MinGasPrice      *big.Int
	MaxGasPrice      *big.Int
	MinValue         *big.Int
	ContractFilters  []string
	MethodFilters    []string
	ExcludeAddresses []string
}

// QueueManager manages queue size and eviction policies
type QueueManager interface {
	ManageCapacity(queue PriorityQueue) error
	EvictOldTransactions(queue PriorityQueue, maxAge time.Duration) error
	GetQueueStats() QueueStats
}

// QueueStats provides queue performance metrics
type QueueStats struct {
	CurrentSize    int
	MaxSize        int
	TotalProcessed int64
	EvictedCount   int64
	AverageWaitTime time.Duration
	LastEviction   time.Time
}