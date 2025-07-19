package simulation

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

func TestAnvilForkGetID(t *testing.T) {
	fork := &anvilFork{
		id: "test-fork-123",
	}

	assert.Equal(t, "test-fork-123", fork.GetID())
}

func TestAnvilForkIsHealthyWhenUnhealthy(t *testing.T) {
	fork := &anvilFork{
		id:      "test-fork",
		healthy: false,
	}

	assert.False(t, fork.IsHealthy())
}

func TestAnvilForkMarkUnhealthy(t *testing.T) {
	fork := &anvilFork{
		id:      "test-fork",
		healthy: true,
	}

	fork.markUnhealthy()
	assert.False(t, fork.healthy)
}

func TestAnvilForkExecuteTransactionUnhealthy(t *testing.T) {
	fork := &anvilFork{
		id:      "test-fork",
		healthy: false,
	}

	ctx := context.Background()
	toAddr := common.HexToAddress("0x0987654321098765432109876543210987654321")
	tx := &types.Transaction{
		Hash:     "0x123",
		From:     common.HexToAddress("0x1234567890123456789012345678901234567890"),
		To:       &toAddr,
		Value:    big.NewInt(1000),
		GasPrice: big.NewInt(20000000000),
		GasLimit: 21000,
		Nonce:    1,
		Data:     []byte{},
	}

	result, err := fork.ExecuteTransaction(ctx, tx)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not healthy")
}

func TestAnvilForkGetBlockNumberUnhealthy(t *testing.T) {
	fork := &anvilFork{
		id:      "test-fork",
		healthy: false,
	}

	blockNumber, err := fork.GetBlockNumber()
	assert.Error(t, err)
	assert.Nil(t, blockNumber)
	assert.Contains(t, err.Error(), "not healthy")
}

func TestAnvilForkGetBalanceUnhealthy(t *testing.T) {
	fork := &anvilFork{
		id:      "test-fork",
		healthy: false,
	}

	address := common.HexToAddress("0x1234567890123456789012345678901234567890")
	balance, err := fork.GetBalance(address)
	assert.Error(t, err)
	assert.Nil(t, balance)
	assert.Contains(t, err.Error(), "not healthy")
}

func TestAnvilForkResetUnhealthy(t *testing.T) {
	fork := &anvilFork{
		id:      "test-fork",
		healthy: false,
	}

	err := fork.Reset()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not healthy")
}

func TestAnvilForkConvertTransaction(t *testing.T) {
	fork := &anvilFork{}

	toAddr := common.HexToAddress("0x0987654321098765432109876543210987654321")
	tx := &types.Transaction{
		Hash:     "0x123",
		From:     common.HexToAddress("0x1234567890123456789012345678901234567890"),
		To:       &toAddr,
		Value:    big.NewInt(1000),
		GasPrice: big.NewInt(20000000000),
		GasLimit: 21000,
		Nonce:    1,
		Data:     []byte{0x01, 0x02},
	}

	ethTx, err := fork.convertTransaction(tx)
	require.NoError(t, err)
	require.NotNil(t, ethTx)

	assert.Equal(t, tx.Nonce, ethTx.Nonce())
	assert.Equal(t, *tx.To, *ethTx.To())
	assert.Equal(t, tx.Value, ethTx.Value())
	assert.Equal(t, tx.GasLimit, ethTx.Gas())
	assert.Equal(t, tx.GasPrice, ethTx.GasPrice())
	assert.Equal(t, tx.Data, ethTx.Data())
}

func TestAnvilForkCalculateStateChanges(t *testing.T) {
	fork := &anvilFork{}

	addr1 := common.HexToAddress("0x1234567890123456789012345678901234567890")
	addr2 := common.HexToAddress("0x0987654321098765432109876543210987654321")

	preState := map[common.Address]*interfaces.AccountState{
		addr1: {
			Balance: big.NewInt(1000),
			Nonce:   1,
			Code:    []byte{},
			Storage: make(map[common.Hash]common.Hash),
		},
		addr2: {
			Balance: big.NewInt(2000),
			Nonce:   2,
			Code:    []byte{},
			Storage: make(map[common.Hash]common.Hash),
		},
	}

	postState := map[common.Address]*interfaces.AccountState{
		addr1: {
			Balance: big.NewInt(900), // Decreased by 100
			Nonce:   2,              // Increased by 1
			Code:    []byte{},
			Storage: make(map[common.Hash]common.Hash),
		},
		addr2: {
			Balance: big.NewInt(2100), // Increased by 100
			Nonce:   2,               // No change
			Code:    []byte{},
			Storage: make(map[common.Hash]common.Hash),
		},
	}

	changes := fork.calculateStateChanges(preState, postState)

	require.Len(t, changes, 2)

	// Check addr1 changes
	change1, exists := changes[addr1]
	require.True(t, exists)
	assert.Equal(t, big.NewInt(-100), change1.Balance) // Lost 100
	assert.Equal(t, uint64(1), change1.Nonce)          // Nonce increased by 1

	// Check addr2 changes
	change2, exists := changes[addr2]
	require.True(t, exists)
	assert.Equal(t, big.NewInt(100), change2.Balance) // Gained 100
	assert.Equal(t, uint64(0), change2.Nonce)         // No nonce change
}

func TestAnvilForkCalculateStateChangesNewAccount(t *testing.T) {
	fork := &anvilFork{}

	addr1 := common.HexToAddress("0x1234567890123456789012345678901234567890")
	addr2 := common.HexToAddress("0x0987654321098765432109876543210987654321")

	preState := map[common.Address]*interfaces.AccountState{
		addr1: {
			Balance: big.NewInt(1000),
			Nonce:   1,
			Code:    []byte{},
			Storage: make(map[common.Hash]common.Hash),
		},
	}

	postState := map[common.Address]*interfaces.AccountState{
		addr1: {
			Balance: big.NewInt(1000),
			Nonce:   1,
			Code:    []byte{},
			Storage: make(map[common.Hash]common.Hash),
		},
		addr2: {
			Balance: big.NewInt(500), // New account
			Nonce:   0,
			Code:    []byte{},
			Storage: make(map[common.Hash]common.Hash),
		},
	}

	changes := fork.calculateStateChanges(preState, postState)

	require.Len(t, changes, 1)

	// Check new account
	change2, exists := changes[addr2]
	require.True(t, exists)
	assert.Equal(t, big.NewInt(500), change2.Balance)
	assert.Equal(t, uint64(0), change2.Nonce)
}

func TestAnvilForkCalculateStateChangesNoChanges(t *testing.T) {
	fork := &anvilFork{}

	addr1 := common.HexToAddress("0x1234567890123456789012345678901234567890")

	preState := map[common.Address]*interfaces.AccountState{
		addr1: {
			Balance: big.NewInt(1000),
			Nonce:   1,
			Code:    []byte{},
			Storage: make(map[common.Hash]common.Hash),
		},
	}

	postState := map[common.Address]*interfaces.AccountState{
		addr1: {
			Balance: big.NewInt(1000), // Same balance
			Nonce:   1,               // Same nonce
			Code:    []byte{},
			Storage: make(map[common.Hash]common.Hash),
		},
	}

	changes := fork.calculateStateChanges(preState, postState)

	assert.Len(t, changes, 0) // No changes detected
}

func TestAnvilForkClose(t *testing.T) {
	fork := &anvilFork{
		id:      "test-fork",
		healthy: true,
	}

	err := fork.Close()
	assert.NoError(t, err)
	assert.False(t, fork.healthy)
}

func TestAnvilForkWaitForReceiptTimeout(t *testing.T) {
	// Skip this test since it requires a real client
	t.Skip("Skipping test that requires real eth client")
}

func TestAnvilForkWaitForReceiptContextCancelled(t *testing.T) {
	// Skip this test since it requires a real client
	t.Skip("Skipping test that requires real eth client")
}