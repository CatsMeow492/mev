package interfaces

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

// ForkManager manages Anvil fork instances
type ForkManager interface {
	CreateFork(ctx context.Context, forkURL string) (Fork, error)
	GetAvailableFork(ctx context.Context) (Fork, error)
	ReleaseFork(fork Fork) error
	CleanupForks() error
	GetForkPoolStats() ForkPoolStats
}

// Fork represents a single Anvil fork instance
type Fork interface {
	GetID() string
	ExecuteTransaction(ctx context.Context, tx *types.Transaction) (*SimulationResult, error)
	GetBlockNumber() (*big.Int, error)
	GetBalance(address common.Address) (*big.Int, error)
	Reset() error
	Close() error
	IsHealthy() bool
}

// TransactionReplayer executes transactions on fork environments
type TransactionReplayer interface {
	ReplayTransaction(ctx context.Context, fork Fork, tx *types.Transaction) (*SimulationResult, error)
	BatchReplayTransactions(ctx context.Context, fork Fork, txs []*types.Transaction) ([]*SimulationResult, error)
	CapturePreState(ctx context.Context, fork Fork, addresses []common.Address) (*StateSnapshot, error)
	CapturePostState(ctx context.Context, fork Fork, addresses []common.Address) (*StateSnapshot, error)
}

// StateAnalyzer measures transaction effects and state changes
type StateAnalyzer interface {
	AnalyzeStateChanges(preState, postState *StateSnapshot) (*StateAnalysis, error)
	CalculateGasUsage(result *SimulationResult) (*GasAnalysis, error)
	ExtractEventLogs(result *SimulationResult) ([]*EventLog, error)
	MeasurePriceImpact(result *SimulationResult) (*PriceImpact, error)
}

// SimulationResult contains the results of transaction simulation
type SimulationResult struct {
	Success      bool
	GasUsed      uint64
	GasPrice     *big.Int
	Receipt      *ethtypes.Receipt
	Logs         []*ethtypes.Log
	StateChanges map[common.Address]*AccountState
	Error        error
	ExecutionTime time.Duration
}

// StateSnapshot captures blockchain state at a point in time
type StateSnapshot struct {
	BlockNumber *big.Int
	Timestamp   time.Time
	Accounts    map[common.Address]*AccountState
	TokenPrices map[common.Address]*big.Int
}

// AccountState represents the state of an account
type AccountState struct {
	Balance *big.Int
	Nonce   uint64
	Code    []byte
	Storage map[common.Hash]common.Hash
}

// StateAnalysis contains analysis of state changes
type StateAnalysis struct {
	BalanceChanges map[common.Address]*big.Int
	TokenTransfers []*TokenTransfer
	PriceChanges   map[common.Address]*PriceChange
	GasConsumed    uint64
	NetValue       *big.Int
}

// TokenTransfer represents a token transfer event
type TokenTransfer struct {
	Token  common.Address
	From   common.Address
	To     common.Address
	Amount *big.Int
}

// PriceChange represents a price change for a token
type PriceChange struct {
	Token     common.Address
	OldPrice  *big.Int
	NewPrice  *big.Int
	Change    *big.Int
	ChangePercent float64
}

// GasAnalysis contains gas usage analysis
type GasAnalysis struct {
	GasUsed     uint64
	GasPrice    *big.Int
	TotalCost   *big.Int
	Efficiency  float64
}

// EventLog represents a decoded event log
type EventLog struct {
	Address common.Address
	Topics  []common.Hash
	Data    []byte
	Decoded map[string]interface{}
}

// PriceImpact measures the price impact of a transaction
type PriceImpact struct {
	Token       common.Address
	Pool        common.Address
	ImpactBps   int64  // basis points
	ImpactPercent float64
	SlippageBps int64
}

// ForkPoolStats provides statistics about the fork pool
type ForkPoolStats struct {
	TotalForks     int
	AvailableForks int
	BusyForks      int
	FailedForks    int
	AverageLatency time.Duration
}