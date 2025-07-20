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

func TestNewFrontrunDetector(t *testing.T) {
	tests := []struct {
		name   string
		config *interfaces.FrontrunConfig
		want   *interfaces.FrontrunConfig
	}{
		{
			name:   "with nil config",
			config: nil,
			want: &interfaces.FrontrunConfig{
				MinTxValue:            big.NewInt(50000),
				MaxGasPremium:         big.NewInt(500000),
				MinSuccessProbability: 0.7,
				MinProfitThreshold:    big.NewInt(100),
			},
		},
		{
			name: "with custom config",
			config: &interfaces.FrontrunConfig{
				MinTxValue:            big.NewInt(100000),
				MaxGasPremium:         big.NewInt(1000000),
				MinSuccessProbability: 0.8,
				MinProfitThreshold:    big.NewInt(200),
			},
			want: &interfaces.FrontrunConfig{
				MinTxValue:            big.NewInt(100000),
				MaxGasPremium:         big.NewInt(1000000),
				MinSuccessProbability: 0.8,
				MinProfitThreshold:    big.NewInt(200),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := NewFrontrunDetector(tt.config)
			config := detector.GetConfiguration()
			
			assert.Equal(t, tt.want.MinTxValue, config.MinTxValue)
			assert.Equal(t, tt.want.MaxGasPremium, config.MaxGasPremium)
			assert.Equal(t, tt.want.MinSuccessProbability, config.MinSuccessProbability)
			assert.Equal(t, tt.want.MinProfitThreshold, config.MinProfitThreshold)
		})
	}
}

func TestFrontrunDetector_DetectOpportunity(t *testing.T) {
	// Use more permissive configuration for testing
	config := &interfaces.FrontrunConfig{
		MinTxValue:            big.NewInt(50000),
		MaxGasPremium:         big.NewInt(10000000000), // 10 gwei max premium
		MinSuccessProbability: 0.5,
		MinProfitThreshold:    big.NewInt(10),
	}
	detector := NewFrontrunDetector(config)
	ctx := context.Background()

	tests := []struct {
		name      string
		tx        *types.Transaction
		simResult *interfaces.SimulationResult
		wantNil   bool
		wantErr   bool
	}{
		{
			name: "low value transaction",
			tx: &types.Transaction{
				Hash:     "0x123",
				From:     common.HexToAddress("0x1"),
				To:       &common.Address{},
				Value:    big.NewInt(1000), // Below minimum threshold
				GasPrice: big.NewInt(20000000000),
				GasLimit: 21000,
				Data:     []byte{},
				ChainID:  big.NewInt(8453),
			},
			simResult: &interfaces.SimulationResult{Success: true},
			wantNil:   true,
			wantErr:   false,
		},
		{
			name: "non-frontrunnable transaction type",
			tx: &types.Transaction{
				Hash:     "0x123",
				From:     common.HexToAddress("0x1"),
				To:       &common.Address{},
				Value:    big.NewInt(100000), // Above minimum threshold
				GasPrice: big.NewInt(20000000000),
				GasLimit: 21000,
				Data:     []byte{0x12, 0x34, 0x56, 0x78}, // Unknown method signature
				ChainID:  big.NewInt(8453),
			},
			simResult: &interfaces.SimulationResult{Success: true},
			wantNil:   true,
			wantErr:   false,
		},
		{
			name: "failed simulation",
			tx: &types.Transaction{
				Hash:     "0x123",
				From:     common.HexToAddress("0x1"),
				To:       &common.Address{},
				Value:    big.NewInt(100000),
				GasPrice: big.NewInt(20000000000),
				GasLimit: 21000,
				Data:     common.Hex2Bytes("7ff36ab5"), // swapExactETHForTokens
				ChainID:  big.NewInt(8453),
			},
			simResult: &interfaces.SimulationResult{Success: false},
			wantNil:   true,
			wantErr:   false,
		},
		{
			name: "successful swap frontrun detection",
			tx: &types.Transaction{
				Hash:     "0x123",
				From:     common.HexToAddress("0x1"),
				To:       &common.Address{},
				Value:    big.NewInt(1000000), // High value
				GasPrice: big.NewInt(10000000000), // Lower gas price to allow for premium
				GasLimit: 200000,
				Data:     common.Hex2Bytes("7ff36ab5"), // swapExactETHForTokens
				ChainID:  big.NewInt(8453),
			},
			simResult: &interfaces.SimulationResult{
				Success: true,
				GasUsed: 150000,
				Logs: []*ethtypes.Log{
					{
						Address: common.HexToAddress("0x4200000000000000000000000000000000000006"),
						Topics: []common.Hash{
							common.HexToHash("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822"), // Swap event
						},
						Data: make([]byte, 128),
					},
				},
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "high value transfer frontrun detection",
			tx: &types.Transaction{
				Hash:     "0x123",
				From:     common.HexToAddress("0x1"),
				To:       &common.Address{},
				Value:    big.NewInt(5000000), // Very high value transfer
				GasPrice: big.NewInt(20000000000),
				GasLimit: 21000,
				Data:     []byte{}, // Simple transfer
				ChainID:  big.NewInt(8453),
			},
			simResult: &interfaces.SimulationResult{
				Success: true,
				GasUsed: 21000,
			},
			wantNil: false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opportunity, err := detector.DetectOpportunity(ctx, tt.tx, tt.simResult)
			
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			
			assert.NoError(t, err)
			
			if tt.wantNil {
				assert.Nil(t, opportunity)
			} else {
				if !assert.NotNil(t, opportunity, "opportunity should not be nil for test case: %s", tt.name) {
					return
				}
				assert.NotNil(t, opportunity.TargetTx)
				assert.NotNil(t, opportunity.FrontrunTx)
				assert.NotNil(t, opportunity.ExpectedProfit)
				assert.NotNil(t, opportunity.GasPremium)
				assert.Greater(t, opportunity.SuccessProbability, 0.0)
				assert.LessOrEqual(t, opportunity.SuccessProbability, 1.0)
			}
		})
	}
}

func TestFrontrunDetector_CalculateOptimalGasPrice(t *testing.T) {
	// Use permissive configuration for testing
	config := &interfaces.FrontrunConfig{
		MinTxValue:            big.NewInt(50000),
		MaxGasPremium:         big.NewInt(10000000000), // 10 gwei max premium
		MinSuccessProbability: 0.5,
		MinProfitThreshold:    big.NewInt(10),
	}
	detector := NewFrontrunDetector(config)
	ctx := context.Background()

	tests := []struct {
		name     string
		targetTx *types.Transaction
		wantErr  bool
		checkFn  func(t *testing.T, gasPrice *big.Int)
	}{
		{
			name:     "nil transaction",
			targetTx: nil,
			wantErr:  true,
		},
		{
			name: "low value swap",
			targetTx: &types.Transaction{
				Value:    big.NewInt(100000),
				GasPrice: big.NewInt(20000000000),
				Data:     common.Hex2Bytes("7ff36ab5"), // swapExactETHForTokens
			},
			wantErr: false,
			checkFn: func(t *testing.T, gasPrice *big.Int) {
				// Should be higher than original gas price
				assert.Greater(t, gasPrice.Int64(), int64(20000000000))
				// Should include premium for swap (more flexible check)
				expectedMinimum := big.NewInt(24000000000) // 20 + 20% minimum premium
				assert.GreaterOrEqual(t, gasPrice.Cmp(expectedMinimum), 0)
			},
		},
		{
			name: "high value transaction",
			targetTx: &types.Transaction{
				Value:    big.NewInt(10000000), // High value
				GasPrice: big.NewInt(20000000000),
				Data:     []byte{}, // Simple transfer
			},
			wantErr: false,
			checkFn: func(t *testing.T, gasPrice *big.Int) {
				// Should be higher than original gas price
				assert.Greater(t, gasPrice.Int64(), int64(20000000000))
				// Should include premium for high value (more flexible check)
				expectedMinimum := big.NewInt(24000000000) // 20 + 20% minimum premium
				assert.GreaterOrEqual(t, gasPrice.Cmp(expectedMinimum), 0)
			},
		},
		{
			name: "gas premium cap test",
			targetTx: &types.Transaction{
				Value:    big.NewInt(100000000), // Very high value
				GasPrice: big.NewInt(2000000000000), // Very high gas price
				Data:     common.Hex2Bytes("7ff36ab5"), // swapExactETHForTokens
			},
			wantErr: false,
			checkFn: func(t *testing.T, gasPrice *big.Int) {
				// Should be capped at max premium
				config := detector.GetConfiguration()
				maxAllowed := new(big.Int).Add(big.NewInt(2000000000000), config.MaxGasPremium)
				assert.LessOrEqual(t, gasPrice.Cmp(maxAllowed), 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gasPrice, err := detector.CalculateOptimalGasPrice(ctx, tt.targetTx)
			
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, gasPrice)
				return
			}
			
			assert.NoError(t, err)
			assert.NotNil(t, gasPrice)
			
			if tt.checkFn != nil {
				tt.checkFn(t, gasPrice)
			}
		})
	}
}

func TestFrontrunDetector_ValidateProfitability(t *testing.T) {
	detector := NewFrontrunDetector(nil)
	ctx := context.Background()

	validOpportunity := &interfaces.FrontrunOpportunity{
		TargetTx: &types.Transaction{
			Hash:     "0x123",
			GasPrice: big.NewInt(20000000000),
		},
		FrontrunTx: &types.Transaction{
			Hash:     "0x456",
			GasPrice: big.NewInt(25000000000), // Higher than target
		},
		ExpectedProfit:     big.NewInt(200), // Above minimum threshold
		GasPremium:         big.NewInt(100000), // Within limits
		SuccessProbability: 0.8, // Above minimum threshold
	}

	tests := []struct {
		name        string
		opportunity *interfaces.FrontrunOpportunity
		wantErr     bool
		errContains string
	}{
		{
			name:        "nil opportunity",
			opportunity: nil,
			wantErr:     true,
			errContains: "opportunity cannot be nil",
		},
		{
			name: "low expected profit",
			opportunity: &interfaces.FrontrunOpportunity{
				TargetTx:           validOpportunity.TargetTx,
				FrontrunTx:         validOpportunity.FrontrunTx,
				ExpectedProfit:     big.NewInt(50), // Below threshold
				GasPremium:         validOpportunity.GasPremium,
				SuccessProbability: validOpportunity.SuccessProbability,
			},
			wantErr:     true,
			errContains: "expected profit below minimum threshold",
		},
		{
			name: "excessive gas premium",
			opportunity: &interfaces.FrontrunOpportunity{
				TargetTx:           validOpportunity.TargetTx,
				FrontrunTx:         validOpportunity.FrontrunTx,
				ExpectedProfit:     validOpportunity.ExpectedProfit,
				GasPremium:         big.NewInt(1000000), // Above maximum
				SuccessProbability: validOpportunity.SuccessProbability,
			},
			wantErr:     true,
			errContains: "gas premium exceeds maximum allowed",
		},
		{
			name: "low success probability",
			opportunity: &interfaces.FrontrunOpportunity{
				TargetTx:           validOpportunity.TargetTx,
				FrontrunTx:         validOpportunity.FrontrunTx,
				ExpectedProfit:     validOpportunity.ExpectedProfit,
				GasPremium:         validOpportunity.GasPremium,
				SuccessProbability: 0.5, // Below threshold
			},
			wantErr:     true,
			errContains: "success probability below minimum threshold",
		},
		{
			name: "missing target transaction",
			opportunity: &interfaces.FrontrunOpportunity{
				TargetTx:           nil,
				FrontrunTx:         validOpportunity.FrontrunTx,
				ExpectedProfit:     validOpportunity.ExpectedProfit,
				GasPremium:         validOpportunity.GasPremium,
				SuccessProbability: validOpportunity.SuccessProbability,
			},
			wantErr:     true,
			errContains: "target transaction is required",
		},
		{
			name: "missing frontrun transaction",
			opportunity: &interfaces.FrontrunOpportunity{
				TargetTx:           validOpportunity.TargetTx,
				FrontrunTx:         nil,
				ExpectedProfit:     validOpportunity.ExpectedProfit,
				GasPremium:         validOpportunity.GasPremium,
				SuccessProbability: validOpportunity.SuccessProbability,
			},
			wantErr:     true,
			errContains: "frontrun transaction is required",
		},
		{
			name: "frontrun gas price not higher",
			opportunity: &interfaces.FrontrunOpportunity{
				TargetTx: validOpportunity.TargetTx,
				FrontrunTx: &types.Transaction{
					Hash:     "0x456",
					GasPrice: big.NewInt(15000000000), // Lower than target
				},
				ExpectedProfit:     validOpportunity.ExpectedProfit,
				GasPremium:         validOpportunity.GasPremium,
				SuccessProbability: validOpportunity.SuccessProbability,
			},
			wantErr:     true,
			errContains: "frontrun transaction must have higher gas price than target",
		},
		{
			name:        "valid opportunity",
			opportunity: validOpportunity,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := detector.ValidateProfitability(ctx, tt.opportunity)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFrontrunDetector_IsFrontrunnable(t *testing.T) {
	detector := NewFrontrunDetector(nil).(*frontrunDetector)

	tests := []struct {
		name   string
		txType types.TransactionType
		want   bool
	}{
		{
			name:   "swap transaction",
			txType: types.TxTypeSwap,
			want:   true,
		},
		{
			name:   "transfer transaction",
			txType: types.TxTypeTransfer,
			want:   true,
		},
		{
			name:   "liquidity transaction",
			txType: types.TxTypeLiquidity,
			want:   true,
		},
		{
			name:   "bridge transaction",
			txType: types.TxTypeBridge,
			want:   true,
		},
		{
			name:   "contract transaction",
			txType: types.TxTypeContract,
			want:   false,
		},
		{
			name:   "unknown transaction",
			txType: types.TxTypeUnknown,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.isFrontrunnable(tt.txType)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestFrontrunDetector_AnalyzeFrontrunPotential(t *testing.T) {
	detector := NewFrontrunDetector(nil).(*frontrunDetector)

	tests := []struct {
		name      string
		tx        *types.Transaction
		simResult *interfaces.SimulationResult
		wantErr   bool
		checkFn   func(t *testing.T, potential *frontrunPotential)
	}{
		{
			name: "swap transaction analysis",
			tx: &types.Transaction{
				Value:    big.NewInt(1000000),
				GasPrice: big.NewInt(20000000000),
				Data:     common.Hex2Bytes("7ff36ab5"), // swapExactETHForTokens
			},
			simResult: &interfaces.SimulationResult{
				Success: true,
				GasUsed: 150000,
				Logs: []*ethtypes.Log{
					{
						Topics: []common.Hash{
							common.HexToHash("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822"),
						},
						Data: make([]byte, 128),
					},
				},
			},
			wantErr: false,
			checkFn: func(t *testing.T, potential *frontrunPotential) {
				assert.NotNil(t, potential.PriceImpact)
				assert.Greater(t, potential.ExpectedSlippage, 0.0)
				assert.NotNil(t, potential.LiquidityDepth)
				assert.GreaterOrEqual(t, potential.MarketVolatility, 0.0)
				assert.Greater(t, potential.CompetitionLevel, 0)
			},
		},
		{
			name: "transfer transaction analysis",
			tx: &types.Transaction{
				Value:    big.NewInt(5000000),
				GasPrice: big.NewInt(20000000000),
				Data:     []byte{}, // Simple transfer
			},
			simResult: &interfaces.SimulationResult{
				Success: true,
				GasUsed: 21000,
			},
			wantErr: false,
			checkFn: func(t *testing.T, potential *frontrunPotential) {
				assert.Equal(t, big.NewInt(0), potential.PriceImpact)
				assert.Equal(t, 0.0, potential.ExpectedSlippage)
				assert.Equal(t, big.NewInt(5000000), potential.LiquidityDepth)
			},
		},
		{
			name: "unsupported transaction type",
			tx: &types.Transaction{
				Value:    big.NewInt(1000000),
				GasPrice: big.NewInt(20000000000),
				Data:     []byte{0x12, 0x34, 0x56, 0x78}, // Unknown method
			},
			simResult: &interfaces.SimulationResult{Success: true},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			potential, err := detector.analyzeFrontrunPotential(tt.tx, tt.simResult)
			
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			
			assert.NoError(t, err)
			assert.NotNil(t, potential)
			
			if tt.checkFn != nil {
				tt.checkFn(t, potential)
			}
		})
	}
}

func TestFrontrunDetector_CalculateGasPremiumPercent(t *testing.T) {
	detector := NewFrontrunDetector(nil).(*frontrunDetector)

	tests := []struct {
		name string
		tx   *types.Transaction
		want float64
	}{
		{
			name: "low value transfer",
			tx: &types.Transaction{
				Value: big.NewInt(10000),
				Data:  []byte{}, // Transfer
			},
			want: 0.20, // 20% base premium
		},
		{
			name: "high value transfer",
			tx: &types.Transaction{
				Value: big.NewInt(2000000), // > $10k threshold
				Data:  []byte{}, // Transfer
			},
			want: 0.30, // 30% for high value
		},
		{
			name: "low value swap",
			tx: &types.Transaction{
				Value: big.NewInt(100000),
				Data:  common.Hex2Bytes("7ff36ab5"), // swapExactETHForTokens
			},
			want: 0.3, // 20% base + 10% for swap (use exact float)
		},
		{
			name: "high value swap",
			tx: &types.Transaction{
				Value: big.NewInt(2000000), // > $10k threshold
				Data:  common.Hex2Bytes("7ff36ab5"), // swapExactETHForTokens
			},
			want: 0.40, // 30% high value + 10% for swap
		},
		{
			name: "liquidity operation",
			tx: &types.Transaction{
				Value: big.NewInt(100000),
				Data:  common.Hex2Bytes("e8e33700"), // addLiquidity
			},
			want: 0.25, // 20% base + 5% for liquidity
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.calculateGasPremiumPercent(tt.tx)
			assert.InDelta(t, tt.want, result, 0.001) // Allow small floating point differences
		})
	}
}

func TestFrontrunDetector_EstimateFrontrunProfit(t *testing.T) {
	detector := NewFrontrunDetector(nil).(*frontrunDetector)

	tx := &types.Transaction{
		Value:    big.NewInt(1000000),
		GasPrice: big.NewInt(20000000000),
		GasLimit: 200000,
	}

	potential := &frontrunPotential{
		PriceImpact:      big.NewInt(150), // 1.5% in basis points
		ExpectedSlippage: 0.01,            // 1%
		CompetitionLevel: 2,               // Medium competition
	}

	gasPremium := big.NewInt(5000000000) // 5 gwei premium

	profit := detector.estimateFrontrunProfit(tx, potential, gasPremium)

	// Should return non-negative profit
	assert.GreaterOrEqual(t, profit.Sign(), 0)

	// Test with high competition
	potential.CompetitionLevel = 5
	highCompetitionProfit := detector.estimateFrontrunProfit(tx, potential, gasPremium)
	
	// High competition should reduce profit
	assert.LessOrEqual(t, highCompetitionProfit.Cmp(profit), 0)
}

func TestFrontrunDetector_CalculateSuccessProbability(t *testing.T) {
	detector := NewFrontrunDetector(nil).(*frontrunDetector)

	tx := &types.Transaction{
		GasPrice: big.NewInt(20000000000),
	}

	potential := &frontrunPotential{
		CompetitionLevel: 2,
		MarketVolatility: 0.02,
	}

	tests := []struct {
		name     string
		gasPrice *big.Int
		want     func(float64) bool
	}{
		{
			name:     "same gas price",
			gasPrice: big.NewInt(20000000000),
			want:     func(prob float64) bool { return prob >= 0.1 && prob <= 0.95 },
		},
		{
			name:     "higher gas price",
			gasPrice: big.NewInt(30000000000), // 50% higher
			want:     func(prob float64) bool { return prob > 0.8 }, // Should be higher
		},
		{
			name:     "much higher gas price",
			gasPrice: big.NewInt(100000000000), // 5x higher
			want:     func(prob float64) bool { return prob >= 0.8 }, // More realistic expectation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prob := detector.calculateSuccessProbability(tx, tt.gasPrice, potential)
			assert.True(t, tt.want(prob), "probability %f doesn't meet expectations", prob)
			assert.GreaterOrEqual(t, prob, 0.1)
			assert.LessOrEqual(t, prob, 0.95)
		})
	}
}

func TestFrontrunDetector_ConstructFrontrunTransaction(t *testing.T) {
	detector := NewFrontrunDetector(nil).(*frontrunDetector)

	targetTx := &types.Transaction{
		Hash:     "0x123",
		From:     common.HexToAddress("0x1"),
		To:       &common.Address{},
		Value:    big.NewInt(1000000),
		GasPrice: big.NewInt(20000000000),
		GasLimit: 200000,
		Data:     common.Hex2Bytes("7ff36ab5"), // swapExactETHForTokens
		ChainID:  big.NewInt(8453),
	}

	gasPrice := big.NewInt(25000000000)
	potential := &frontrunPotential{
		PriceImpact: big.NewInt(150),
	}

	frontrunTx, err := detector.constructFrontrunTransaction(targetTx, gasPrice, potential)

	require.NoError(t, err)
	assert.NotNil(t, frontrunTx)
	assert.Equal(t, gasPrice, frontrunTx.GasPrice)
	assert.Equal(t, targetTx.To, frontrunTx.To)
	assert.Equal(t, targetTx.ChainID, frontrunTx.ChainID)
	assert.Equal(t, targetTx.GasLimit, frontrunTx.GasLimit)
	assert.NotNil(t, frontrunTx.Value)
	assert.NotNil(t, frontrunTx.Data)
}

func TestFrontrunDetector_Integration(t *testing.T) {
	// Integration test that tests the full flow
	detector := NewFrontrunDetector(&interfaces.FrontrunConfig{
		MinTxValue:            big.NewInt(100000),
		MaxGasPremium:         big.NewInt(10000000000), // 10 gwei max premium
		MinSuccessProbability: 0.5,
		MinProfitThreshold:    big.NewInt(10),
	})

	ctx := context.Background()

	// Create a high-value swap transaction
	tx := &types.Transaction{
		Hash:      "0x123456789abcdef",
		From:      common.HexToAddress("0x1111111111111111111111111111111111111111"),
		To:        &common.Address{},
		Value:     big.NewInt(2000000), // High value
		GasPrice:  big.NewInt(10000000000), // Lower gas price for integration test
		GasLimit:  200000,
		Nonce:     1,
		Data:      common.Hex2Bytes("7ff36ab5"), // swapExactETHForTokens
		Timestamp: time.Now(),
		ChainID:   big.NewInt(8453),
	}

	// Create simulation result with swap event
	simResult := &interfaces.SimulationResult{
		Success: true,
		GasUsed: 150000,
		GasPrice: big.NewInt(20000000000),
		Logs: []*ethtypes.Log{
			{
				Address: common.HexToAddress("0x4200000000000000000000000000000000000006"),
				Topics: []common.Hash{
					common.HexToHash("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822"), // Swap event
				},
				Data: make([]byte, 128),
			},
		},
		ExecutionTime: 50 * time.Millisecond,
	}

	// Test opportunity detection
	opportunity, err := detector.DetectOpportunity(ctx, tx, simResult)
	require.NoError(t, err)
	require.NotNil(t, opportunity)

	// Validate the opportunity
	err = detector.ValidateProfitability(ctx, opportunity)
	assert.NoError(t, err)

	// Test gas price calculation
	gasPrice, err := detector.CalculateOptimalGasPrice(ctx, tx)
	require.NoError(t, err)
	assert.Greater(t, gasPrice.Int64(), tx.GasPrice.Int64())

	// Verify opportunity properties
	assert.Equal(t, tx, opportunity.TargetTx)
	assert.NotNil(t, opportunity.FrontrunTx)
	assert.Greater(t, opportunity.FrontrunTx.GasPrice.Int64(), tx.GasPrice.Int64())
	assert.Greater(t, opportunity.ExpectedProfit.Int64(), int64(0))
	assert.Greater(t, opportunity.GasPremium.Int64(), int64(0))
	assert.Greater(t, opportunity.SuccessProbability, 0.0)
	assert.LessOrEqual(t, opportunity.SuccessProbability, 1.0)
}