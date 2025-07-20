package strategy

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSandwichDetector(t *testing.T) {
	tests := []struct {
		name   string
		config *interfaces.SandwichConfig
		want   *interfaces.SandwichConfig
	}{
		{
			name:   "with nil config uses defaults",
			config: nil,
			want: &interfaces.SandwichConfig{
				MinSwapAmount:     big.NewInt(10000),
				MaxSlippage:       0.02,
				GasPremiumPercent: 0.10,
				MinProfitThreshold: big.NewInt(100),
			},
		},
		{
			name: "with custom config",
			config: &interfaces.SandwichConfig{
				MinSwapAmount:     big.NewInt(50000),
				MaxSlippage:       0.01,
				GasPremiumPercent: 0.15,
				MinProfitThreshold: big.NewInt(200),
			},
			want: &interfaces.SandwichConfig{
				MinSwapAmount:     big.NewInt(50000),
				MaxSlippage:       0.01,
				GasPremiumPercent: 0.15,
				MinProfitThreshold: big.NewInt(200),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := NewSandwichDetector(tt.config)
			config := detector.GetConfiguration()
			
			assert.Equal(t, tt.want.MinSwapAmount, config.MinSwapAmount)
			assert.Equal(t, tt.want.MaxSlippage, config.MaxSlippage)
			assert.Equal(t, tt.want.GasPremiumPercent, config.GasPremiumPercent)
			assert.Equal(t, tt.want.MinProfitThreshold, config.MinProfitThreshold)
		})
	}
}

func TestSandwichDetector_DetectOpportunity(t *testing.T) {
	detector := NewSandwichDetector(nil)
	ctx := context.Background()

	tests := []struct {
		name      string
		tx        *types.Transaction
		simResult *interfaces.SimulationResult
		wantNil   bool
		wantErr   bool
	}{
		{
			name: "non-swap transaction returns nil",
			tx: &types.Transaction{
				Hash:     "0x123",
				Value:    big.NewInt(20000),
				Data:     []byte{}, // Empty data = transfer
				GasPrice: big.NewInt(1000000000),
				GasLimit: 21000,
				ChainID:  big.NewInt(8453),
			},
			simResult: &interfaces.SimulationResult{Success: true},
			wantNil:   true,
			wantErr:   false,
		},
		{
			name: "small swap returns nil",
			tx: &types.Transaction{
				Hash:     "0x123",
				Value:    big.NewInt(5000), // Below minimum threshold
				Data:     common.Hex2Bytes("38ed1739"), // swapExactTokensForTokens
				GasPrice: big.NewInt(1000000000),
				GasLimit: 200000,
				ChainID:  big.NewInt(8453),
			},
			simResult: &interfaces.SimulationResult{Success: true},
			wantNil:   true,
			wantErr:   false,
		},
		{
			name: "large swap with acceptable slippage",
			tx: &types.Transaction{
				Hash:     "0x123",
				Value:    big.NewInt(50000), // Above minimum threshold
				Data:     common.Hex2Bytes("38ed1739"), // swapExactTokensForTokens
				GasPrice: big.NewInt(1000000000),
				GasLimit: 200000,
				ChainID:  big.NewInt(8453),
			},
			simResult: &interfaces.SimulationResult{
				Success: true,
				Logs:    []*ethtypes.Log{}, // Would contain swap logs in real scenario
			},
			wantNil: false, // Should detect opportunity with 2% slippage
			wantErr: false,
		},
		{
			name: "valid large swap opportunity",
			tx: &types.Transaction{
				Hash:     "0x123",
				Value:    big.NewInt(50000), // Above minimum threshold
				Data:     common.Hex2Bytes("7ff36ab5"), // swapExactETHForTokens
				GasPrice: big.NewInt(1000000000),
				GasLimit: 200000,
				ChainID:  big.NewInt(8453),
			},
			simResult: &interfaces.SimulationResult{
				Success: true,
				Logs:    []*ethtypes.Log{},
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
				assert.NotNil(t, opportunity)
				assert.Equal(t, tt.tx, opportunity.TargetTx)
				assert.NotNil(t, opportunity.ExpectedProfit)
				assert.True(t, opportunity.SlippageTolerance >= 0)
			}
		})
	}
}

func TestSandwichDetector_ValidateOpportunity(t *testing.T) {
	config := &interfaces.SandwichConfig{
		MinSwapAmount:     big.NewInt(10000),
		MaxSlippage:       0.02,
		GasPremiumPercent: 0.10,
		MinProfitThreshold: big.NewInt(100),
	}
	detector := NewSandwichDetector(config)
	ctx := context.Background()

	tests := []struct {
		name        string
		opportunity *interfaces.SandwichOpportunity
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
			name: "profit below threshold",
			opportunity: &interfaces.SandwichOpportunity{
				TargetTx:          &types.Transaction{Hash: "0x123"},
				ExpectedProfit:    big.NewInt(50), // Below threshold of 100
				SlippageTolerance: 0.01,
			},
			wantErr:     true,
			errContains: "expected profit below minimum threshold",
		},
		{
			name: "slippage too high",
			opportunity: &interfaces.SandwichOpportunity{
				TargetTx:          &types.Transaction{Hash: "0x123"},
				ExpectedProfit:    big.NewInt(200),
				SlippageTolerance: 0.05, // Above max of 0.02
			},
			wantErr:     true,
			errContains: "slippage tolerance exceeds maximum allowed",
		},
		{
			name: "missing target transaction",
			opportunity: &interfaces.SandwichOpportunity{
				TargetTx:          nil,
				ExpectedProfit:    big.NewInt(200),
				SlippageTolerance: 0.01,
			},
			wantErr:     true,
			errContains: "target transaction is required",
		},
		{
			name: "valid opportunity",
			opportunity: &interfaces.SandwichOpportunity{
				TargetTx:          &types.Transaction{Hash: "0x123"},
				ExpectedProfit:    big.NewInt(200),
				SlippageTolerance: 0.01,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := detector.ValidateOpportunity(ctx, tt.opportunity)
			
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

func TestSandwichDetector_ConstructTransactions(t *testing.T) {
	config := &interfaces.SandwichConfig{
		MinSwapAmount:     big.NewInt(10000),
		MaxSlippage:       0.02,
		GasPremiumPercent: 0.10,
		MinProfitThreshold: big.NewInt(100),
	}
	detector := NewSandwichDetector(config)
	ctx := context.Background()

	targetTx := &types.Transaction{
		Hash:     "0x123",
		From:     common.HexToAddress("0x456"),
		To:       &common.Address{},
		Value:    big.NewInt(50000),
		GasPrice: big.NewInt(1000000000),
		GasLimit: 200000,
		Data:     common.Hex2Bytes("38ed1739"),
		ChainID:  big.NewInt(8453),
	}

	tests := []struct {
		name        string
		opportunity *interfaces.SandwichOpportunity
		wantErr     bool
		errContains string
	}{
		{
			name: "invalid opportunity",
			opportunity: &interfaces.SandwichOpportunity{
				TargetTx:          nil, // Invalid
				ExpectedProfit:    big.NewInt(200),
				SlippageTolerance: 0.01,
			},
			wantErr:     true,
			errContains: "invalid opportunity",
		},
		{
			name: "valid opportunity",
			opportunity: &interfaces.SandwichOpportunity{
				TargetTx:          targetTx,
				ExpectedProfit:    big.NewInt(200),
				SlippageTolerance: 0.01,
				Pool:              "0x1234567890123456789012345678901234567890",
				Token0:            "0xA0b86a33E6441b8435b662f0E2d0B5B0B5B5B5B5",
				Token1:            "0xB1c97a44F7552c9A6B8B8B8B8B8B8B8B8B8B8B8B",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txs, err := detector.ConstructTransactions(ctx, tt.opportunity)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			
			require.NoError(t, err)
			require.Len(t, txs, 2)
			
			// Verify frontrun transaction
			frontrunTx := txs[0]
			assert.NotNil(t, frontrunTx)
			assert.Equal(t, tt.opportunity.TargetTx.To, frontrunTx.To)
			assert.Equal(t, tt.opportunity.TargetTx.ChainID, frontrunTx.ChainID)
			
			// Verify gas price premium for frontrun
			expectedGasPremium := new(big.Int).Mul(targetTx.GasPrice, big.NewInt(10))
			expectedGasPremium = expectedGasPremium.Div(expectedGasPremium, big.NewInt(100))
			expectedFrontrunGasPrice := new(big.Int).Add(targetTx.GasPrice, expectedGasPremium)
			assert.Equal(t, expectedFrontrunGasPrice, frontrunTx.GasPrice)
			
			// Verify backrun transaction
			backrunTx := txs[1]
			assert.NotNil(t, backrunTx)
			assert.Equal(t, tt.opportunity.TargetTx.To, backrunTx.To)
			assert.Equal(t, tt.opportunity.TargetTx.GasPrice, backrunTx.GasPrice)
			
			// Verify opportunity is updated with constructed transactions
			assert.Equal(t, frontrunTx, tt.opportunity.FrontrunTx)
			assert.Equal(t, backrunTx, tt.opportunity.BackrunTx)
		})
	}
}

func TestSandwichDetector_IsLargeSwap(t *testing.T) {
	config := &interfaces.SandwichConfig{
		MinSwapAmount: big.NewInt(10000),
	}
	detector := NewSandwichDetector(config).(*sandwichDetector)

	tests := []struct {
		name  string
		value *big.Int
		want  bool
	}{
		{
			name:  "below threshold",
			value: big.NewInt(5000),
			want:  false,
		},
		{
			name:  "at threshold",
			value: big.NewInt(10000),
			want:  true,
		},
		{
			name:  "above threshold",
			value: big.NewInt(50000),
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := &types.Transaction{Value: tt.value}
			result := detector.isLargeSwap(tx)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestSandwichDetector_ExtractSwapDetails(t *testing.T) {
	detector := NewSandwichDetector(nil).(*sandwichDetector)

	tests := []struct {
		name      string
		tx        *types.Transaction
		simResult *interfaces.SimulationResult
		wantErr   bool
	}{
		{
			name: "invalid transaction data",
			tx: &types.Transaction{
				Data: []byte{0x01, 0x02}, // Less than 4 bytes
			},
			wantErr: true,
		},
		{
			name: "valid swap transaction",
			tx: &types.Transaction{
				Data:  common.Hex2Bytes("38ed1739"), // swapExactTokensForTokens
				Value: big.NewInt(50000),
			},
			simResult: &interfaces.SimulationResult{Success: true},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			details, err := detector.extractSwapDetails(tt.tx, tt.simResult)
			
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, details)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, details)
				assert.NotEmpty(t, details.Pool)
				assert.NotEmpty(t, details.Token0)
				assert.NotEmpty(t, details.Token1)
				assert.NotNil(t, details.AmountIn)
				assert.NotNil(t, details.AmountOut)
				assert.True(t, details.SlippageTolerance >= 0)
			}
		})
	}
}

func TestSandwichDetector_CalculatePriceImpact(t *testing.T) {
	detector := NewSandwichDetector(nil).(*sandwichDetector)

	tests := []struct {
		name      string
		simResult *interfaces.SimulationResult
		wantErr   bool
	}{
		{
			name:      "nil simulation result",
			simResult: nil,
			wantErr:   true,
		},
		{
			name: "failed simulation",
			simResult: &interfaces.SimulationResult{
				Success: false,
			},
			wantErr: true,
		},
		{
			name: "successful simulation",
			simResult: &interfaces.SimulationResult{
				Success: true,
				Logs:    []*ethtypes.Log{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			impact, err := detector.calculatePriceImpact(tt.simResult)
			
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, impact)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, impact)
				assert.True(t, impact.Sign() >= 0)
			}
		})
	}
}

func TestSandwichDetector_EstimateProfit(t *testing.T) {
	detector := NewSandwichDetector(nil).(*sandwichDetector)

	details := &swapDetails{
		AmountIn: big.NewInt(100000),
	}
	priceImpact := big.NewInt(200) // 2% in basis points

	profit := detector.estimateProfit(details, priceImpact)
	
	assert.NotNil(t, profit)
	assert.True(t, profit.Sign() >= 0) // Profit should not be negative
}

func TestSandwichDetector_ConstructSwapData(t *testing.T) {
	detector := NewSandwichDetector(nil).(*sandwichDetector)
	
	opportunity := &interfaces.SandwichOpportunity{
		Pool:   "0x1234567890123456789012345678901234567890",
		Token0: "0xA0b86a33E6441b8435b662f0E2d0B5B0B5B5B5B5",
		Token1: "0xB1c97a44F7552c9A6B8B8B8B8B8B8B8B8B8B8B8B",
	}

	tests := []struct {
		name       string
		isFrontrun bool
	}{
		{
			name:       "frontrun transaction data",
			isFrontrun: true,
		},
		{
			name:       "backrun transaction data",
			isFrontrun: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := detector.constructSwapData(opportunity, tt.isFrontrun)
			
			assert.NotNil(t, data)
			assert.Equal(t, 68, len(data)) // 4 bytes method sig + 64 bytes parameters
			
			// Verify method signature
			methodSig := common.Bytes2Hex(data[:4])
			assert.Equal(t, "38ed1739", methodSig) // swapExactTokensForTokens
		})
	}
}

// Benchmark tests
func BenchmarkSandwichDetector_DetectOpportunity(b *testing.B) {
	detector := NewSandwichDetector(nil)
	ctx := context.Background()
	
	tx := &types.Transaction{
		Hash:     "0x123",
		Value:    big.NewInt(50000),
		Data:     common.Hex2Bytes("38ed1739"),
		GasPrice: big.NewInt(1000000000),
		GasLimit: 200000,
		ChainID:  big.NewInt(8453),
	}
	
	simResult := &interfaces.SimulationResult{
		Success: true,
		Logs:    []*ethtypes.Log{},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = detector.DetectOpportunity(ctx, tx, simResult)
	}
}

func BenchmarkSandwichDetector_ConstructTransactions(b *testing.B) {
	detector := NewSandwichDetector(nil)
	ctx := context.Background()
	
	opportunity := &interfaces.SandwichOpportunity{
		TargetTx: &types.Transaction{
			Hash:     "0x123",
			Value:    big.NewInt(50000),
			GasPrice: big.NewInt(1000000000),
			GasLimit: 200000,
			ChainID:  big.NewInt(8453),
		},
		ExpectedProfit:    big.NewInt(200),
		SlippageTolerance: 0.01,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = detector.ConstructTransactions(ctx, opportunity)
	}
}