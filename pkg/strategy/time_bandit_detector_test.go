package strategy

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTimeBanditDetector(t *testing.T) {
	tests := []struct {
		name   string
		config *interfaces.TimeBanditConfig
		want   *interfaces.TimeBanditConfig
	}{
		{
			name:   "with nil config",
			config: nil,
			want: &interfaces.TimeBanditConfig{
				MaxBundleSize:      10,
				MinProfitThreshold: big.NewInt(50),
				MaxDependencyDepth: 5,
			},
		},
		{
			name: "with custom config",
			config: &interfaces.TimeBanditConfig{
				MaxBundleSize:      5,
				MinProfitThreshold: big.NewInt(100),
				MaxDependencyDepth: 3,
			},
			want: &interfaces.TimeBanditConfig{
				MaxBundleSize:      5,
				MinProfitThreshold: big.NewInt(100),
				MaxDependencyDepth: 3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := NewTimeBanditDetector(tt.config)
			config := detector.GetConfiguration()
			
			assert.Equal(t, tt.want.MaxBundleSize, config.MaxBundleSize)
			assert.Equal(t, tt.want.MinProfitThreshold.String(), config.MinProfitThreshold.String())
			assert.Equal(t, tt.want.MaxDependencyDepth, config.MaxDependencyDepth)
		})
	}
}

func TestTimeBanditDetector_DetectOpportunity(t *testing.T) {
	detector := NewTimeBanditDetector(nil)
	ctx := context.Background()

	tests := []struct {
		name        string
		txs         []*types.Transaction
		simResults  []*interfaces.SimulationResult
		wantNil     bool
		wantErr     bool
		description string
	}{
		{
			name:        "empty transactions",
			txs:         []*types.Transaction{},
			simResults:  []*interfaces.SimulationResult{},
			wantNil:     true,
			wantErr:     false,
			description: "should return nil for empty transaction list",
		},
		{
			name:        "single transaction",
			txs:         []*types.Transaction{createMockTransaction("0x1", 1, big.NewInt(1000000000))},
			simResults:  []*interfaces.SimulationResult{createMockSimResult(true)},
			wantNil:     true,
			wantErr:     false,
			description: "should return nil for single transaction",
		},
		{
			name: "two suitable transactions",
			txs: []*types.Transaction{
				createMockTimeBanditSwapTransactionWithNonce("0x1", common.HexToAddress("0xaaa"), 1, big.NewInt(2000000000)),
				createMockTimeBanditSwapTransactionWithNonce("0x2", common.HexToAddress("0xbbb"), 1, big.NewInt(1500000000)),
			},
			simResults: []*interfaces.SimulationResult{
				createMockSimResult(true),
				createMockSimResult(true),
			},
			wantNil:     false,
			wantErr:     false,
			description: "should detect opportunity with two suitable transactions",
		},
		{
			name: "transactions with nonce dependency",
			txs: []*types.Transaction{
				createMockTimeBanditSwapTransactionWithNonce("0x1", common.HexToAddress("0xaaa"), 1, big.NewInt(2000000000)),
				createMockTimeBanditSwapTransactionWithNonce("0x2", common.HexToAddress("0xaaa"), 2, big.NewInt(1500000000)),
			},
			simResults: []*interfaces.SimulationResult{
				createMockSimResult(true),
				createMockSimResult(true),
			},
			wantNil:     false,
			wantErr:     false,
			description: "should handle nonce dependencies correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opportunity, err := detector.DetectOpportunity(ctx, tt.txs, tt.simResults)
			
			if tt.wantErr {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
			
			if tt.wantNil {
				assert.Nil(t, opportunity, tt.description)
			} else {
				assert.NotNil(t, opportunity, tt.description)
				if opportunity != nil {
					assert.NotNil(t, opportunity.OriginalTxs)
					assert.NotNil(t, opportunity.OptimalOrder)
					assert.NotNil(t, opportunity.ExpectedProfit)
					assert.NotNil(t, opportunity.Dependencies)
				}
			}
		})
	}
}

func TestTimeBanditDetector_FindOptimalOrdering(t *testing.T) {
	detector := NewTimeBanditDetector(nil)
	ctx := context.Background()

	tests := []struct {
		name        string
		txs         []*types.Transaction
		wantErr     bool
		description string
	}{
		{
			name:        "empty transactions",
			txs:         []*types.Transaction{},
			wantErr:     false,
			description: "should handle empty transaction list",
		},
		{
			name:        "single transaction",
			txs:         []*types.Transaction{createMockTransaction("0x1", 1, big.NewInt(1000000000))},
			wantErr:     false,
			description: "should handle single transaction",
		},
		{
			name: "transactions with different gas prices",
			txs: []*types.Transaction{
				createMockTransactionWithNonce("0x1", common.HexToAddress("0xaaa"), 1, big.NewInt(1000000000)), // 1 gwei
				createMockTransactionWithNonce("0x2", common.HexToAddress("0xbbb"), 1, big.NewInt(2000000000)), // 2 gwei
				createMockTransactionWithNonce("0x3", common.HexToAddress("0xccc"), 1, big.NewInt(1500000000)), // 1.5 gwei
			},
			wantErr:     false,
			description: "should order by gas price preference",
		},
		{
			name: "transactions with nonce dependencies",
			txs: []*types.Transaction{
				createMockTransactionWithNonce("0x1", common.HexToAddress("0xaaa"), 2, big.NewInt(1000000000)),
				createMockTransactionWithNonce("0x2", common.HexToAddress("0xaaa"), 1, big.NewInt(2000000000)),
			},
			wantErr:     false,
			description: "should respect nonce ordering",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := detector.FindOptimalOrdering(ctx, tt.txs)
			
			if tt.wantErr {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
				assert.Equal(t, len(tt.txs), len(result), "result should have same length as input")
				
				// Verify nonce ordering is preserved for same address
				if len(result) > 1 {
					addressNonces := make(map[common.Address]uint64)
					for _, tx := range result {
						if lastNonce, exists := addressNonces[tx.From]; exists {
							assert.True(t, tx.Nonce > lastNonce, "nonce ordering should be preserved for same address")
						}
						addressNonces[tx.From] = tx.Nonce
					}
				}
			}
		})
	}
}

func TestTimeBanditDetector_ValidateDependencies(t *testing.T) {
	detector := NewTimeBanditDetector(nil)
	ctx := context.Background()

	tests := []struct {
		name        string
		txs         []*types.Transaction
		wantErr     bool
		description string
	}{
		{
			name:        "empty transactions",
			txs:         []*types.Transaction{},
			wantErr:     false,
			description: "should handle empty transaction list",
		},
		{
			name: "valid nonce ordering",
			txs: []*types.Transaction{
				createMockTransactionWithNonce("0x1", common.HexToAddress("0xaaa"), 1, big.NewInt(1000000000)),
				createMockTransactionWithNonce("0x2", common.HexToAddress("0xaaa"), 2, big.NewInt(1000000000)),
			},
			wantErr:     false,
			description: "should pass with valid nonce ordering",
		},
		{
			name: "invalid nonce ordering",
			txs: []*types.Transaction{
				createMockTransactionWithNonce("0x1", common.HexToAddress("0xaaa"), 2, big.NewInt(1000000000)),
				createMockTransactionWithNonce("0x2", common.HexToAddress("0xaaa"), 1, big.NewInt(1000000000)), // 2 -> 1 is invalid
			},
			wantErr:     true,
			description: "should fail with invalid nonce ordering",
		},
		{
			name: "different addresses",
			txs: []*types.Transaction{
				createMockTransactionWithNonce("0x1", common.HexToAddress("0xaaa"), 1, big.NewInt(1000000000)),
				createMockTransactionWithNonce("0x2", common.HexToAddress("0xbbb"), 1, big.NewInt(1000000000)),
			},
			wantErr:     false,
			description: "should handle different addresses correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := detector.ValidateDependencies(ctx, tt.txs)
			
			if tt.wantErr {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

func TestConstraintSolver_AddNonceConstraints(t *testing.T) {
	txs := []*types.Transaction{
		createMockTransactionWithNonce("0x1", common.HexToAddress("0xaaa"), 1, big.NewInt(1000000000)),
		createMockTransactionWithNonce("0x2", common.HexToAddress("0xaaa"), 2, big.NewInt(1000000000)),
		createMockTransactionWithNonce("0x3", common.HexToAddress("0xbbb"), 1, big.NewInt(1000000000)),
	}

	config := &interfaces.TimeBanditConfig{
		MaxBundleSize:      10,
		MinProfitThreshold: big.NewInt(50),
		MaxDependencyDepth: 5,
	}

	solver := newConstraintSolver(txs, config)
	err := solver.addNonceConstraints()
	
	require.NoError(t, err)
	
	// Should have one constraint: tx1 -> tx2 (same address, consecutive nonces)
	assert.Len(t, solver.constraints, 1)
	assert.Equal(t, "0x1", solver.constraints[0].before)
	assert.Equal(t, "0x2", solver.constraints[0].after)
	assert.Equal(t, 1000, solver.constraints[0].weight) // High weight for nonce constraints
}

func TestConstraintSolver_TopologicalSort(t *testing.T) {
	txs := []*types.Transaction{
		createMockTransactionWithNonce("0x1", common.HexToAddress("0xaaa"), 2, big.NewInt(1000000000)),
		createMockTransactionWithNonce("0x2", common.HexToAddress("0xaaa"), 1, big.NewInt(2000000000)),
		createMockTransactionWithNonce("0x3", common.HexToAddress("0xbbb"), 1, big.NewInt(1500000000)),
	}

	config := &interfaces.TimeBanditConfig{
		MaxBundleSize:      10,
		MinProfitThreshold: big.NewInt(50),
		MaxDependencyDepth: 5,
	}

	solver := newConstraintSolver(txs, config)
	err := solver.addNonceConstraints()
	require.NoError(t, err)

	result, err := solver.solve(context.Background())
	require.NoError(t, err)
	require.Len(t, result, 3)

	// Find positions of transactions in result
	var pos1, pos2, pos3 int
	for i, tx := range result {
		switch tx.Hash {
		case "0x1":
			pos1 = i
		case "0x2":
			pos2 = i
		case "0x3":
			pos3 = i
		}
	}

	// tx2 should come before tx1 (nonce constraint)
	assert.True(t, pos2 < pos1, "tx2 (nonce 1) should come before tx1 (nonce 2)")
	
	// tx3 can be anywhere since it's from different address
	assert.True(t, pos3 >= 0 && pos3 < 3, "tx3 should be in valid position")
}

func TestTimeBanditDetector_CircularDependencyDetection(t *testing.T) {
	detector := &timeBanditDetector{
		config: &interfaces.TimeBanditConfig{
			MaxBundleSize:      10,
			MinProfitThreshold: big.NewInt(50),
			MaxDependencyDepth: 5,
		},
	}

	// Create transactions that would create a circular dependency
	// This is a simplified test - in practice, circular dependencies would be more complex
	txs := []*types.Transaction{
		createMockTimeBanditSwapTransaction("0x1", 1, big.NewInt(1000000000)),
		createMockTimeBanditSwapTransaction("0x2", 1, big.NewInt(1000000000)),
	}

	// The current implementation doesn't create circular dependencies easily,
	// so this test mainly ensures the detection logic doesn't crash
	hasCircular := detector.hasCircularDependencies(txs)
	assert.False(t, hasCircular, "should not detect circular dependencies in simple case")
}

func TestTimeBanditDetector_DependencyDepthCalculation(t *testing.T) {
	detector := &timeBanditDetector{
		config: &interfaces.TimeBanditConfig{
			MaxBundleSize:      10,
			MinProfitThreshold: big.NewInt(50),
			MaxDependencyDepth: 5,
		},
	}

	txs := []*types.Transaction{
		createMockTimeBanditSwapTransaction("0x1", 1, big.NewInt(1000000000)),
		createMockTimeBanditSwapTransaction("0x2", 1, big.NewInt(1000000000)),
		createMockTimeBanditSwapTransaction("0x3", 1, big.NewInt(1000000000)),
	}

	depth := detector.getDependencyDepth(txs)
	assert.True(t, depth >= 0, "dependency depth should be non-negative")
	assert.True(t, depth <= len(txs), "dependency depth should not exceed transaction count")
}

// Helper functions for creating mock data

func createMockTransaction(hash string, nonce uint64, gasPrice *big.Int) *types.Transaction {
	return createMockTransactionWithNonce(hash, common.HexToAddress("0x1234567890123456789012345678901234567890"), nonce, gasPrice)
}

func createMockTransactionWithNonce(hash string, from common.Address, nonce uint64, gasPrice *big.Int) *types.Transaction {
	to := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd")
	return &types.Transaction{
		Hash:     hash,
		From:     from,
		To:       &to,
		Value:    big.NewInt(1000000000000000000), // 1 ETH
		GasPrice: gasPrice,
		GasLimit: 21000,
		Nonce:    nonce,
		Data:     []byte{0x01, 0x02, 0x03, 0x04}, // Some data to make it a contract call
		ChainID:  big.NewInt(8453), // Base chain ID
	}
}

func createMockTimeBanditSwapTransaction(hash string, nonce uint64, gasPrice *big.Int) *types.Transaction {
	from := common.HexToAddress("0x1234567890123456789012345678901234567890")
	to := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd")
	
	// Create transaction data that looks like a swap (swapExactTokensForTokens)
	data := make([]byte, 68)
	copy(data[:4], common.Hex2Bytes("38ed1739")) // swapExactTokensForTokens method signature
	
	return &types.Transaction{
		Hash:     hash,
		From:     from,
		To:       &to,
		Value:    big.NewInt(1000000000000000000), // 1 ETH
		GasPrice: gasPrice,
		GasLimit: 200000, // Higher gas limit for swap
		Nonce:    nonce,
		Data:     data,
		ChainID:  big.NewInt(8453), // Base chain ID
	}
}

func createMockTimeBanditSwapTransactionWithNonce(hash string, from common.Address, nonce uint64, gasPrice *big.Int) *types.Transaction {
	to := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd")
	
	// Create transaction data that looks like a swap
	data := make([]byte, 68)
	copy(data[:4], common.Hex2Bytes("38ed1739")) // swapExactTokensForTokens method signature
	
	return &types.Transaction{
		Hash:     hash,
		From:     from,
		To:       &to,
		Value:    big.NewInt(1000000000000000000), // 1 ETH
		GasPrice: gasPrice,
		GasLimit: 200000,
		Nonce:    nonce,
		Data:     data,
		ChainID:  big.NewInt(8453), // Base chain ID
	}
}

func createMockSimResult(success bool) *interfaces.SimulationResult {
	var receipt *ethtypes.Receipt
	var logs []*ethtypes.Log
	
	if success {
		receipt = &ethtypes.Receipt{
			Status: 1,
			GasUsed: 150000,
		}
		logs = []*ethtypes.Log{}
	}
	
	return &interfaces.SimulationResult{
		Success:       success,
		GasUsed:       150000,
		GasPrice:      big.NewInt(1000000000),
		Receipt:       receipt,
		Logs:          logs,
		StateChanges:  make(map[common.Address]*interfaces.AccountState),
		Error:         nil,
		ExecutionTime: time.Millisecond * 50,
	}
}