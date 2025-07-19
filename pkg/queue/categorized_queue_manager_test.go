package queue

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCategorizedQueueManager(t *testing.T) {
	filter := NewTransactionFilter()
	cqm := NewCategorizedQueueManager(filter)
	
	require.NotNil(t, cqm)
	assert.Equal(t, filter, cqm.GetFilter())
	
	// Check that all transaction type queues are initialized
	expectedTypes := []types.TransactionType{
		types.TxTypeSwap,
		types.TxTypeTransfer,
		types.TxTypeLiquidity,
		types.TxTypeBridge,
		types.TxTypeContract,
		types.TxTypeUnknown,
	}
	
	for _, txType := range expectedTypes {
		size := cqm.GetQueueSize(txType)
		assert.Equal(t, 0, size, "Queue for %s should be empty initially", txType)
		
		stats, err := cqm.GetQueueStats(txType)
		require.NoError(t, err)
		assert.Equal(t, 0, stats.CurrentSize)
		assert.Equal(t, DefaultMaxCapacity, stats.MaxSize)
	}
}

func TestCategorizedQueueManager_AddTransaction(t *testing.T) {
	filter := NewTransactionFilter()
	cqm := NewCategorizedQueueManager(filter)
	
	tests := []struct {
		name           string
		tx             *types.Transaction
		expectedType   types.TransactionType
		shouldSucceed  bool
	}{
		{
			name:          "swap transaction",
			tx:            createSwapTransaction(),
			expectedType:  types.TxTypeSwap,
			shouldSucceed: true,
		},
		{
			name:          "transfer transaction",
			tx:            createTransferTransaction(),
			expectedType:  types.TxTypeTransfer,
			shouldSucceed: true,
		},
		{
			name:          "liquidity transaction",
			tx:            createLiquidityTransaction(),
			expectedType:  types.TxTypeLiquidity,
			shouldSucceed: true,
		},
		{
			name:          "contract transaction",
			tx:            createContractTransaction(),
			expectedType:  types.TxTypeContract,
			shouldSucceed: true,
		},
		{
			name:          "filtered out transaction (low gas price)",
			tx:            createLowGasPriceTransaction(),
			expectedType:  types.TxTypeTransfer,
			shouldSucceed: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialSize := cqm.GetQueueSize(tt.expectedType)
			
			err := cqm.AddTransaction(tt.tx)
			
			if tt.shouldSucceed {
				require.NoError(t, err)
				assert.Equal(t, initialSize+1, cqm.GetQueueSize(tt.expectedType))
				
				// Check stats
				stats, err := cqm.GetQueueStats(tt.expectedType)
				require.NoError(t, err)
				assert.Equal(t, initialSize+1, stats.CurrentSize)
				assert.Equal(t, int64(1), stats.TotalProcessed)
			} else {
				require.Error(t, err)
				assert.Equal(t, initialSize, cqm.GetQueueSize(tt.expectedType))
			}
		})
	}
}

func TestCategorizedQueueManager_GetTransaction(t *testing.T) {
	filter := NewTransactionFilter()
	cqm := NewCategorizedQueueManager(filter)
	
	// Add transactions to different queues
	swapTx := createSwapTransaction()
	transferTx := createTransferTransaction()
	
	err := cqm.AddTransaction(swapTx)
	require.NoError(t, err)
	err = cqm.AddTransaction(transferTx)
	require.NoError(t, err)
	
	// Get transaction from swap queue
	retrievedTx, err := cqm.GetTransaction(types.TxTypeSwap)
	require.NoError(t, err)
	assert.Equal(t, swapTx.Hash, retrievedTx.Hash)
	assert.Equal(t, 0, cqm.GetQueueSize(types.TxTypeSwap))
	
	// Get transaction from transfer queue
	retrievedTx, err = cqm.GetTransaction(types.TxTypeTransfer)
	require.NoError(t, err)
	assert.Equal(t, transferTx.Hash, retrievedTx.Hash)
	assert.Equal(t, 0, cqm.GetQueueSize(types.TxTypeTransfer))
	
	// Try to get from empty queue
	_, err = cqm.GetTransaction(types.TxTypeSwap)
	require.Error(t, err)
}

func TestCategorizedQueueManager_GetNextTransaction(t *testing.T) {
	filter := NewTransactionFilter()
	cqm := NewCategorizedQueueManager(filter)
	
	// Add transactions in reverse priority order
	transferTx := createTransferTransaction()
	contractTx := createContractTransaction()
	liquidityTx := createLiquidityTransaction()
	swapTx := createSwapTransaction()
	
	err := cqm.AddTransaction(transferTx)
	require.NoError(t, err)
	err = cqm.AddTransaction(contractTx)
	require.NoError(t, err)
	err = cqm.AddTransaction(liquidityTx)
	require.NoError(t, err)
	err = cqm.AddTransaction(swapTx)
	require.NoError(t, err)
	
	// Should get swap transaction first (highest priority)
	retrievedTx, err := cqm.GetNextTransaction()
	require.NoError(t, err)
	assert.Equal(t, swapTx.Hash, retrievedTx.Hash)
	
	// Should get liquidity transaction next
	retrievedTx, err = cqm.GetNextTransaction()
	require.NoError(t, err)
	assert.Equal(t, liquidityTx.Hash, retrievedTx.Hash)
	
	// Should get transfer transaction next
	retrievedTx, err = cqm.GetNextTransaction()
	require.NoError(t, err)
	assert.Equal(t, transferTx.Hash, retrievedTx.Hash)
	
	// Should get contract transaction last
	retrievedTx, err = cqm.GetNextTransaction()
	require.NoError(t, err)
	assert.Equal(t, contractTx.Hash, retrievedTx.Hash)
	
	// Should return error when all queues are empty
	_, err = cqm.GetNextTransaction()
	require.Error(t, err)
}

func TestCategorizedQueueManager_PeekTransaction(t *testing.T) {
	filter := NewTransactionFilter()
	cqm := NewCategorizedQueueManager(filter)
	
	swapTx := createSwapTransaction()
	err := cqm.AddTransaction(swapTx)
	require.NoError(t, err)
	
	// Peek should return transaction without removing it
	peekedTx, err := cqm.PeekTransaction(types.TxTypeSwap)
	require.NoError(t, err)
	assert.Equal(t, swapTx.Hash, peekedTx.Hash)
	assert.Equal(t, 1, cqm.GetQueueSize(types.TxTypeSwap))
	
	// Peek again should return same transaction
	peekedTx2, err := cqm.PeekTransaction(types.TxTypeSwap)
	require.NoError(t, err)
	assert.Equal(t, swapTx.Hash, peekedTx2.Hash)
	assert.Equal(t, 1, cqm.GetQueueSize(types.TxTypeSwap))
	
	// Peek empty queue should return error
	_, err = cqm.PeekTransaction(types.TxTypeTransfer)
	require.Error(t, err)
}

func TestCategorizedQueueManager_GetTotalSize(t *testing.T) {
	filter := NewTransactionFilter()
	cqm := NewCategorizedQueueManager(filter)
	
	assert.Equal(t, 0, cqm.GetTotalSize())
	
	// Add transactions to different queues
	err := cqm.AddTransaction(createSwapTransaction())
	require.NoError(t, err)
	assert.Equal(t, 1, cqm.GetTotalSize())
	
	err = cqm.AddTransaction(createTransferTransaction())
	require.NoError(t, err)
	assert.Equal(t, 2, cqm.GetTotalSize())
	
	err = cqm.AddTransaction(createLiquidityTransaction())
	require.NoError(t, err)
	assert.Equal(t, 3, cqm.GetTotalSize())
}

func TestCategorizedQueueManager_GetAllQueueStats(t *testing.T) {
	filter := NewTransactionFilter()
	cqm := NewCategorizedQueueManager(filter)
	
	// Add some transactions
	err := cqm.AddTransaction(createSwapTransaction())
	require.NoError(t, err)
	err = cqm.AddTransaction(createTransferTransaction())
	require.NoError(t, err)
	
	allStats := cqm.GetAllQueueStats()
	
	// Check that all transaction types have stats
	expectedTypes := []types.TransactionType{
		types.TxTypeSwap,
		types.TxTypeTransfer,
		types.TxTypeLiquidity,
		types.TxTypeBridge,
		types.TxTypeContract,
		types.TxTypeUnknown,
	}
	
	for _, txType := range expectedTypes {
		stats, exists := allStats[txType]
		assert.True(t, exists, "Stats should exist for %s", txType)
		
		if txType == types.TxTypeSwap || txType == types.TxTypeTransfer {
			assert.Equal(t, 1, stats.CurrentSize)
			assert.Equal(t, int64(1), stats.TotalProcessed)
		} else {
			assert.Equal(t, 0, stats.CurrentSize)
			assert.Equal(t, int64(0), stats.TotalProcessed)
		}
	}
}

func TestCategorizedQueueManager_ClearQueue(t *testing.T) {
	filter := NewTransactionFilter()
	cqm := NewCategorizedQueueManager(filter)
	
	// Add transactions
	err := cqm.AddTransaction(createSwapTransaction())
	require.NoError(t, err)
	err = cqm.AddTransaction(createTransferTransaction())
	require.NoError(t, err)
	
	assert.Equal(t, 1, cqm.GetQueueSize(types.TxTypeSwap))
	assert.Equal(t, 1, cqm.GetQueueSize(types.TxTypeTransfer))
	
	// Clear swap queue
	err = cqm.ClearQueue(types.TxTypeSwap)
	require.NoError(t, err)
	
	assert.Equal(t, 0, cqm.GetQueueSize(types.TxTypeSwap))
	assert.Equal(t, 1, cqm.GetQueueSize(types.TxTypeTransfer)) // Should remain unchanged
	
	// Check stats
	stats, err := cqm.GetQueueStats(types.TxTypeSwap)
	require.NoError(t, err)
	assert.Equal(t, 0, stats.CurrentSize)
}

func TestCategorizedQueueManager_ClearAllQueues(t *testing.T) {
	filter := NewTransactionFilter()
	cqm := NewCategorizedQueueManager(filter)
	
	// Add transactions to multiple queues
	err := cqm.AddTransaction(createSwapTransaction())
	require.NoError(t, err)
	err = cqm.AddTransaction(createTransferTransaction())
	require.NoError(t, err)
	err = cqm.AddTransaction(createLiquidityTransaction())
	require.NoError(t, err)
	
	assert.Equal(t, 3, cqm.GetTotalSize())
	
	// Clear all queues
	err = cqm.ClearAllQueues()
	require.NoError(t, err)
	
	assert.Equal(t, 0, cqm.GetTotalSize())
	
	// Check individual queue sizes
	expectedTypes := []types.TransactionType{
		types.TxTypeSwap,
		types.TxTypeTransfer,
		types.TxTypeLiquidity,
		types.TxTypeBridge,
		types.TxTypeContract,
		types.TxTypeUnknown,
	}
	
	for _, txType := range expectedTypes {
		assert.Equal(t, 0, cqm.GetQueueSize(txType))
	}
}

func TestCategorizedQueueManager_UpdateFilter(t *testing.T) {
	filter := NewTransactionFilter()
	cqm := NewCategorizedQueueManager(filter)
	
	newFilter := NewTransactionFilterWithCriteria(interfaces.FilterCriteria{
		MinGasPrice: big.NewInt(5000000000), // 5 gwei
	})
	
	cqm.UpdateFilter(newFilter)
	assert.Equal(t, newFilter, cqm.GetFilter())
	
	// Test that new filter is applied
	lowGasTx := createLowGasPriceTransaction() // 0.5 gwei
	err := cqm.AddTransaction(lowGasTx)
	require.Error(t, err) // Should be filtered out
}

func TestCategorizedQueueManager_EvictOldTransactions(t *testing.T) {
	filter := NewTransactionFilter()
	cqm := NewCategorizedQueueManager(filter)
	
	// Create old and new transactions
	oldTx := createSwapTransaction()
	oldTx.Timestamp = time.Now().Add(-2 * time.Hour)
	
	newTx := createTransferTransaction()
	newTx.Timestamp = time.Now()
	
	err := cqm.AddTransaction(oldTx)
	require.NoError(t, err)
	err = cqm.AddTransaction(newTx)
	require.NoError(t, err)
	
	assert.Equal(t, 2, cqm.GetTotalSize())
	
	// Evict transactions older than 1 hour
	err = cqm.EvictOldTransactions(1 * time.Hour)
	require.NoError(t, err)
	
	// Only new transaction should remain
	assert.Equal(t, 1, cqm.GetTotalSize())
	assert.Equal(t, 0, cqm.GetQueueSize(types.TxTypeSwap))    // Old transaction removed
	assert.Equal(t, 1, cqm.GetQueueSize(types.TxTypeTransfer)) // New transaction remains
}

// Helper functions for creating test transactions
func createSwapTransaction() *types.Transaction {
	tx := createTestTransaction("0xswap1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcd", 20000000000, 1, time.Now())
	tx.Data = common.Hex2Bytes("7ff36ab5000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000010000000000000000000000001234567890123456789012345678901234567890")
	return tx
}

func createTransferTransaction() *types.Transaction {
	tx := createTestTransaction("0xtransfer1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab", 20000000000, 1, time.Now())
	tx.Data = []byte{} // No data for transfer
	return tx
}

func createLiquidityTransaction() *types.Transaction {
	tx := createTestTransaction("0xliquidity1234567890abcdef1234567890abcdef1234567890abcdef1234567890", 20000000000, 1, time.Now())
	tx.Data = common.Hex2Bytes("e8e33700000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000010000000000000000000000001234567890123456789012345678901234567890")
	return tx
}

func createContractTransaction() *types.Transaction {
	tx := createTestTransaction("0xcontract1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab", 20000000000, 1, time.Now())
	tx.Data = []byte{0x01, 0x02, 0x03, 0x04} // Some contract data
	return tx
}

func createLowGasPriceTransaction() *types.Transaction {
	tx := createTestTransaction("0xlowgas1234567890abcdef1234567890abcdef1234567890abcdef1234567890abc", 500000000, 1, time.Now())
	tx.GasPrice = big.NewInt(500000000) // 0.5 gwei - below minimum
	tx.Data = []byte{}                  // Transfer transaction
	return tx
}