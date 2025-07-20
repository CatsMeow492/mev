package simulation

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	mevtypes "github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockReplayerFork implements the Fork interface for testing transaction replayer
type mockReplayerFork struct {
	mock.Mock
}

func (m *mockReplayerFork) GetID() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockReplayerFork) ExecuteTransaction(ctx context.Context, tx *mevtypes.Transaction) (*interfaces.SimulationResult, error) {
	args := m.Called(ctx, tx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.SimulationResult), args.Error(1)
}

func (m *mockReplayerFork) GetBlockNumber() (*big.Int, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*big.Int), args.Error(1)
}

func (m *mockReplayerFork) GetBalance(address common.Address) (*big.Int, error) {
	args := m.Called(address)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*big.Int), args.Error(1)
}

func (m *mockReplayerFork) Reset() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockReplayerFork) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockReplayerFork) IsHealthy() bool {
	args := m.Called()
	return args.Bool(0)
}

func TestNewTransactionReplayer(t *testing.T) {
	replayer := NewTransactionReplayer()
	assert.NotNil(t, replayer)
}

func TestTransactionReplayer_ReplayTransaction_Success(t *testing.T) {
	replayer := NewTransactionReplayer()
	mockFork := &mockReplayerFork{}
	
	tx := &mevtypes.Transaction{
		Hash:     "0x123",
		From:     common.HexToAddress("0x1"),
		To:       &[]common.Address{common.HexToAddress("0x2")}[0],
		Value:    big.NewInt(1000),
		GasPrice: big.NewInt(20000000000),
		GasLimit: 21000,
		Nonce:    1,
	}

	result := &interfaces.SimulationResult{
		Success:       true,
		GasUsed:       21000,
		GasPrice:      big.NewInt(20000000000),
		ExecutionTime: time.Millisecond * 100,
	}
	mockFork.On("ExecuteTransaction", mock.Anything, mock.Anything).Return(result, nil)

	ctx := context.Background()
	actualResult, err := replayer.ReplayTransaction(ctx, mockFork, tx)

	assert.NoError(t, err)
	assert.NotNil(t, actualResult)
	assert.True(t, actualResult.Success)
	mockFork.AssertExpectations(t)
}

func TestTransactionReplayer_ReplayTransaction_NilFork(t *testing.T) {
	replayer := NewTransactionReplayer()
	tx := &mevtypes.Transaction{}

	ctx := context.Background()
	result, err := replayer.ReplayTransaction(ctx, nil, tx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fork cannot be nil")
	assert.Nil(t, result)
}

func TestTransactionReplayer_ReplayTransaction_NilTransaction(t *testing.T) {
	replayer := NewTransactionReplayer()
	mockFork := &mockReplayerFork{}

	ctx := context.Background()
	result, err := replayer.ReplayTransaction(ctx, mockFork, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction cannot be nil")
	assert.Nil(t, result)
}

func TestTransactionReplayer_BatchReplayTransactions_Success(t *testing.T) {
	replayer := NewTransactionReplayer()
	mockFork := &mockReplayerFork{}
	
	txs := []*mevtypes.Transaction{
		{Hash: "0x123", From: common.HexToAddress("0x1")},
		{Hash: "0x456", From: common.HexToAddress("0x2")},
	}

	result := &interfaces.SimulationResult{
		Success: true,
		GasUsed: 21000,
	}
	mockFork.On("ExecuteTransaction", mock.Anything, mock.Anything).Return(result, nil).Twice()

	ctx := context.Background()
	results, err := replayer.BatchReplayTransactions(ctx, mockFork, txs)

	assert.NoError(t, err)
	assert.Len(t, results, 2)
	mockFork.AssertExpectations(t)
}

func TestTransactionReplayer_BatchReplayTransactions_EmptyList(t *testing.T) {
	replayer := NewTransactionReplayer()
	mockFork := &mockReplayerFork{}

	ctx := context.Background()
	results, err := replayer.BatchReplayTransactions(ctx, mockFork, []*mevtypes.Transaction{})

	assert.NoError(t, err)
	assert.Len(t, results, 0)
}

func TestTransactionReplayer_CapturePreState_Success(t *testing.T) {
	replayer := NewTransactionReplayer()
	mockFork := &mockReplayerFork{}
	
	addresses := []common.Address{
		common.HexToAddress("0x1"),
		common.HexToAddress("0x2"),
	}

	mockFork.On("GetBlockNumber").Return(big.NewInt(12345), nil)
	mockFork.On("GetBalance", common.HexToAddress("0x1")).Return(big.NewInt(1000), nil)
	mockFork.On("GetBalance", common.HexToAddress("0x2")).Return(big.NewInt(2000), nil)

	ctx := context.Background()
	snapshot, err := replayer.CapturePreState(ctx, mockFork, addresses)

	assert.NoError(t, err)
	assert.NotNil(t, snapshot)
	assert.Equal(t, big.NewInt(12345), snapshot.BlockNumber)
	assert.Len(t, snapshot.Accounts, 2)
	mockFork.AssertExpectations(t)
}

func TestTransactionReplayer_CapturePostState_Success(t *testing.T) {
	replayer := NewTransactionReplayer()
	mockFork := &mockReplayerFork{}
	
	addresses := []common.Address{common.HexToAddress("0x1")}
	
	mockFork.On("GetBlockNumber").Return(big.NewInt(12346), nil)
	mockFork.On("GetBalance", common.HexToAddress("0x1")).Return(big.NewInt(1500), nil)

	ctx := context.Background()
	snapshot, err := replayer.CapturePostState(ctx, mockFork, addresses)

	assert.NoError(t, err)
	assert.NotNil(t, snapshot)
	assert.Equal(t, big.NewInt(12346), snapshot.BlockNumber)
	assert.Len(t, snapshot.Accounts, 1)
	mockFork.AssertExpectations(t)
}

func TestTransactionReplayer_isTokenContract(t *testing.T) {
	replayer := &transactionReplayer{}

	// Zero address should return false
	result := replayer.isTokenContract(common.Address{})
	assert.False(t, result)

	// Regular address should return true
	result = replayer.isTokenContract(common.HexToAddress("0x1234567890123456789012345678901234567890"))
	assert.True(t, result)
}