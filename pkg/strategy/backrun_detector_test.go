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

func TestNewBackrunDetector(t *testing.T) {
	tests := []struct {
		name   string
		config *interfaces.BackrunConfig
		want   *interfaces.BackrunConfig
	}{
		{
			name:   "with nil config uses defaults",
			config: nil,
			want: &interfaces.BackrunConfig{
				MinPriceGap:        big.NewInt(50),
				MaxTradeSize:       big.NewInt(1000000),
				MinProfitThreshold: big.NewInt(100),
				SupportedPools: []string{
					"0x4200000000000000000000000000000000000006",
					"0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
				},
			},
		},
		{
			name: "with custom config",
			config: &interfaces.BackrunConfig{
				MinPriceGap:        big.NewInt(100),
				MaxTradeSize:       big.NewInt(2000000),
				MinProfitThreshold: big.NewInt(200),
				SupportedPools:     []string{"0x1234"},
			},
			want: &interfaces.BackrunConfig{
				MinPriceGap:        big.NewInt(100),
				MaxTradeSize:       big.NewInt(2000000),
				MinProfitThreshold: big.NewInt(200),
				SupportedPools:     []string{"0x1234"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := NewBackrunDetector(tt.config)
			config := detector.GetConfiguration()
			
			assert.Equal(t, tt.want.MinPriceGap, config.MinPriceGap)
			assert.Equal(t, tt.want.MaxTradeSize, config.MaxTradeSize)
			assert.Equal(t, tt.want.MinProfitThreshold, config.MinProfitThreshold)
			assert.Equal(t, tt.want.SupportedPools, config.SupportedPools)
		})
	}
}

func TestBackrunDetector_DetectOpportunity(t *testing.T) {
	detector := NewBackrunDetector(nil)
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
			tx: func() *types.Transaction {
				toAddr := common.HexToAddress("0x2")
				return &types.Transaction{
					Hash:     "0x123",
					From:     common.HexToAddress("0x1"),
					To:       &toAddr,
					Value:    big.NewInt(1000),
					GasPrice: big.NewInt(20000000000),
					GasLimit: 21000,
					Data:     []byte{}, // Empty data = transfer
					ChainID:  big.NewInt(8453),
				}
			}(),
			simResult: &interfaces.SimulationResult{Success: true},
			wantNil:   true,
			wantErr:   false,
		},
		{
			name: "failed simulation returns nil",
			tx: createMockSwapTransaction(),
			simResult: &interfaces.SimulationResult{
				Success: false,
				Error:   assert.AnError,
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name:      "nil simulation returns nil",
			tx:        createMockSwapTransaction(),
			simResult: nil,
			wantNil:   true,
			wantErr:   false,
		},
		{
			name:      "successful swap with price impact creates opportunity",
			tx:        createMockSwapTransaction(),
			simResult: createMockSimulationResult(),
			wantNil:   false,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opportunity, err := detector.DetectOpportunity(ctx, tt.tx, tt.simResult)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			
			if tt.wantNil {
				assert.Nil(t, opportunity)
			} else {
				assert.NotNil(t, opportunity)
				assert.Equal(t, tt.tx, opportunity.TargetTx)
				assert.NotEmpty(t, opportunity.Pool1)
				assert.NotEmpty(t, opportunity.Pool2)
				assert.NotEmpty(t, opportunity.Token)
				assert.True(t, opportunity.PriceGap.Sign() > 0)
				assert.True(t, opportunity.OptimalAmount.Sign() > 0)
			}
		})
	}
}

func TestBackrunDetector_CalculateOptimalTradeSize(t *testing.T) {
	detector := NewBackrunDetector(nil)
	ctx := context.Background()

	tests := []struct {
		name        string
		opportunity *interfaces.BackrunOpportunity
		wantErr     bool
		checkResult bool
	}{
		{
			name:        "nil opportunity returns error",
			opportunity: nil,
			wantErr:     true,
		},
		{
			name:        "valid opportunity calculates optimal size",
			opportunity: createMockBackrunOpportunity(),
			wantErr:     false,
			checkResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			optimalSize, err := detector.CalculateOptimalTradeSize(ctx, tt.opportunity)
			
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, optimalSize)
			} else {
				assert.NoError(t, err)
				if tt.checkResult {
					assert.NotNil(t, optimalSize)
					assert.True(t, optimalSize.Sign() > 0)
					// Optimal size should be within reasonable bounds
					assert.True(t, optimalSize.Cmp(big.NewInt(1000)) >= 0) // At least min size
					assert.True(t, optimalSize.Cmp(big.NewInt(1000000)) <= 0) // At most max size
				}
			}
		})
	}
}

func TestBackrunDetector_ValidateArbitrage(t *testing.T) {
	config := &interfaces.BackrunConfig{
		MinPriceGap:        big.NewInt(50),
		MinProfitThreshold: big.NewInt(100),
	}
	detector := NewBackrunDetector(config)
	ctx := context.Background()

	tests := []struct {
		name        string
		opportunity *interfaces.BackrunOpportunity
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
			name: "price gap below threshold",
			opportunity: &interfaces.BackrunOpportunity{
				TargetTx:       createMockSwapTransaction(),
				PriceGap:       big.NewInt(25), // Below 50 threshold
				ExpectedProfit: big.NewInt(200),
				Pool1:          "0x1111",
				Pool2:          "0x2222",
				Token:          "0x3333",
			},
			wantErr:     true,
			errContains: "price gap below minimum threshold",
		},
		{
			name: "profit below threshold",
			opportunity: &interfaces.BackrunOpportunity{
				TargetTx:       createMockSwapTransaction(),
				PriceGap:       big.NewInt(100),
				ExpectedProfit: big.NewInt(50), // Below 100 threshold
				Pool1:          "0x1111",
				Pool2:          "0x2222",
				Token:          "0x3333",
			},
			wantErr:     true,
			errContains: "expected profit below minimum threshold",
		},
		{
			name: "missing target transaction",
			opportunity: &interfaces.BackrunOpportunity{
				TargetTx:       nil,
				PriceGap:       big.NewInt(100),
				ExpectedProfit: big.NewInt(200),
				Pool1:          "0x1111",
				Pool2:          "0x2222",
				Token:          "0x3333",
			},
			wantErr:     true,
			errContains: "target transaction is required",
		},
		{
			name: "missing pool1",
			opportunity: &interfaces.BackrunOpportunity{
				TargetTx:       createMockSwapTransaction(),
				PriceGap:       big.NewInt(100),
				ExpectedProfit: big.NewInt(200),
				Pool1:          "",
				Pool2:          "0x2222",
				Token:          "0x3333",
			},
			wantErr:     true,
			errContains: "both pools are required",
		},
		{
			name: "missing token",
			opportunity: &interfaces.BackrunOpportunity{
				TargetTx:       createMockSwapTransaction(),
				PriceGap:       big.NewInt(100),
				ExpectedProfit: big.NewInt(200),
				Pool1:          "0x1111",
				Pool2:          "0x2222",
				Token:          "",
			},
			wantErr:     true,
			errContains: "token address is required",
		},
		{
			name: "valid opportunity",
			opportunity: &interfaces.BackrunOpportunity{
				TargetTx:       createMockSwapTransaction(),
				PriceGap:       big.NewInt(100),
				ExpectedProfit: big.NewInt(200),
				Pool1:          "0x1111",
				Pool2:          "0x2222",
				Token:          "0x3333",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := detector.ValidateArbitrage(ctx, tt.opportunity)
			
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

func TestBackrunDetector_BinarySearchOptimalSize(t *testing.T) {
	detector := NewBackrunDetector(nil).(*backrunDetector)
	ctx := context.Background()
	opportunity := createMockBackrunOpportunity()

	minSize := big.NewInt(1000)
	maxSize := big.NewInt(100000)
	tolerance := big.NewInt(100)

	optimalSize, err := detector.binarySearchOptimalSize(ctx, opportunity, minSize, maxSize, tolerance)
	
	require.NoError(t, err)
	assert.NotNil(t, optimalSize)
	assert.True(t, optimalSize.Cmp(minSize) >= 0)
	assert.True(t, optimalSize.Cmp(maxSize) <= 0)
}

func TestBackrunDetector_CalculateProfitAtSize(t *testing.T) {
	detector := NewBackrunDetector(nil).(*backrunDetector)
	opportunity := createMockBackrunOpportunity()

	tests := []struct {
		name string
		size *big.Int
	}{
		{
			name: "small trade size",
			size: big.NewInt(1000),
		},
		{
			name: "medium trade size",
			size: big.NewInt(50000),
		},
		{
			name: "large trade size",
			size: big.NewInt(500000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profit := detector.calculateProfitAtSize(opportunity, tt.size)
			assert.NotNil(t, profit)
			assert.True(t, profit.Sign() >= 0) // Profit should be non-negative
		})
	}
}

func TestBackrunDetector_CalculateTradeEfficiency(t *testing.T) {
	config := &interfaces.BackrunConfig{
		MaxTradeSize: big.NewInt(1000000),
	}
	detector := NewBackrunDetector(config).(*backrunDetector)

	tests := []struct {
		name         string
		size         *big.Int
		wantMin      float64
		wantMax      float64
	}{
		{
			name:    "small trade has high efficiency",
			size:    big.NewInt(10000),
			wantMin: 0.9,
			wantMax: 1.0,
		},
		{
			name:    "medium trade has medium efficiency",
			size:    big.NewInt(500000),
			wantMin: 0.5,
			wantMax: 0.9,
		},
		{
			name:    "large trade has lower efficiency",
			size:    big.NewInt(1000000),
			wantMin: 0.1,
			wantMax: 0.6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			efficiency := detector.calculateTradeEfficiency(tt.size)
			assert.True(t, efficiency >= tt.wantMin, "efficiency %f should be >= %f", efficiency, tt.wantMin)
			assert.True(t, efficiency <= tt.wantMax, "efficiency %f should be <= %f", efficiency, tt.wantMax)
			assert.True(t, efficiency >= 0.1, "efficiency should be at least 0.1")
			assert.True(t, efficiency <= 1.0, "efficiency should be at most 1.0")
		})
	}
}

func TestBackrunDetector_EstimateArbitrageProfit(t *testing.T) {
	detector := NewBackrunDetector(nil).(*backrunDetector)
	
	priceImpact := &interfaces.PriceImpact{
		ImpactBps:   200, // 2% price impact
		SlippageBps: 100, // 1% slippage
	}
	
	tests := []struct {
		name      string
		tradeSize *big.Int
		wantMin   *big.Int
	}{
		{
			name:      "small trade size",
			tradeSize: big.NewInt(10000),
			wantMin:   big.NewInt(0), // Should be non-negative
		},
		{
			name:      "medium trade size",
			tradeSize: big.NewInt(100000),
			wantMin:   big.NewInt(0),
		},
		{
			name:      "large trade size",
			tradeSize: big.NewInt(1000000),
			wantMin:   big.NewInt(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profit := detector.estimateArbitrageProfit(priceImpact, tt.tradeSize)
			assert.NotNil(t, profit)
			assert.True(t, profit.Cmp(tt.wantMin) >= 0)
		})
	}
}

func TestBackrunDetector_FindAlternativePool(t *testing.T) {
	detector := NewBackrunDetector(nil).(*backrunDetector)
	
	tests := []struct {
		name        string
		token       common.Address
		excludePool common.Address
		wantErr     bool
	}{
		{
			name:        "WETH token finds alternative pool",
			token:       common.HexToAddress("0x4200000000000000000000000000000000000006"),
			excludePool: common.HexToAddress("0x0000000000000000000000000000000000000000"),
			wantErr:     false,
		},
		{
			name:        "USDC token finds alternative pool",
			token:       common.HexToAddress("0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"),
			excludePool: common.HexToAddress("0x0000000000000000000000000000000000000000"),
			wantErr:     false,
		},
		{
			name:        "unknown token returns error",
			token:       common.HexToAddress("0x9999999999999999999999999999999999999999"),
			excludePool: common.HexToAddress("0x0000000000000000000000000000000000000000"),
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := detector.findAlternativePool(tt.token, tt.excludePool)
			
			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, pool)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, pool)
				assert.NotEqual(t, tt.excludePool.Hex(), pool)
			}
		})
	}
}

// Helper functions for creating mock data

func createMockSwapTransaction() *types.Transaction {
	// Create transaction with swap method signature
	swapData := make([]byte, 68)
	copy(swapData[:4], common.Hex2Bytes("38ed1739")) // swapExactTokensForTokens

	toAddr := common.HexToAddress("0x2222222222222222222222222222222222222222")
	return &types.Transaction{
		Hash:     "0x1234567890abcdef",
		From:     common.HexToAddress("0x1111111111111111111111111111111111111111"),
		To:       &toAddr,
		Value:    big.NewInt(100000), // 0.1 ETH
		GasPrice: big.NewInt(20000000000), // 20 gwei
		GasLimit: 200000,
		Nonce:    1,
		Data:     swapData,
		ChainID:  big.NewInt(8453), // Base chain ID
	}
}

func createMockSimulationResult() *interfaces.SimulationResult {
	// Create mock swap event log
	swapEventSig := common.HexToHash("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822")
	tokenAddress := common.HexToHash("0x4200000000000000000000000000000000000006")
	
	// Mock event data (4 * 32 bytes for amounts)
	eventData := make([]byte, 128)
	// amount0In = 100000
	copy(eventData[0:32], big.NewInt(100000).Bytes())
	// amount1In = 0
	copy(eventData[32:64], big.NewInt(0).Bytes())
	// amount0Out = 0
	copy(eventData[64:96], big.NewInt(0).Bytes())
	// amount1Out = 95000 (5% slippage)
	copy(eventData[96:128], big.NewInt(95000).Bytes())

	log := &ethtypes.Log{
		Address: common.HexToAddress("0x1111111111111111111111111111111111111111"),
		Topics: []common.Hash{
			swapEventSig,
			tokenAddress,
		},
		Data: eventData,
	}

	return &interfaces.SimulationResult{
		Success:       true,
		GasUsed:       150000,
		GasPrice:      big.NewInt(20000000000),
		Logs:          []*ethtypes.Log{log},
		StateChanges:  make(map[common.Address]*interfaces.AccountState),
		ExecutionTime: 50 * time.Millisecond,
	}
}

func createMockBackrunOpportunity() *interfaces.BackrunOpportunity {
	return &interfaces.BackrunOpportunity{
		TargetTx:       createMockSwapTransaction(),
		Pool1:          "0x1111111111111111111111111111111111111111",
		Pool2:          "0x2222222222222222222222222222222222222222",
		Token:          "0x4200000000000000000000000000000000000006",
		PriceGap:       big.NewInt(200), // 2% price gap
		OptimalAmount:  big.NewInt(50000),
		ExpectedProfit: big.NewInt(500),
	}
}