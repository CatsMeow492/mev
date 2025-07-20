package profit

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockGasEstimator is a mock implementation of GasEstimator
type MockGasEstimator struct {
	mock.Mock
}

func (m *MockGasEstimator) EstimateGas(ctx context.Context, tx *types.Transaction) (uint64, error) {
	args := m.Called(ctx, tx)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockGasEstimator) EstimateBatchGas(ctx context.Context, txs []*types.Transaction) (uint64, error) {
	args := m.Called(ctx, txs)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockGasEstimator) GetCurrentGasPrice(ctx context.Context) (*big.Int, error) {
	args := m.Called(ctx)
	return args.Get(0).(*big.Int), args.Error(1)
}

func (m *MockGasEstimator) PredictGasPrice(ctx context.Context, priority interfaces.GasPriority) (*big.Int, error) {
	args := m.Called(ctx, priority)
	return args.Get(0).(*big.Int), args.Error(1)
}

// MockSlippageCalculator is a mock implementation of SlippageCalculator
type MockSlippageCalculator struct {
	mock.Mock
}

func (m *MockSlippageCalculator) CalculateSlippage(ctx context.Context, pool string, token string, amount *big.Int) (*interfaces.SlippageEstimate, error) {
	args := m.Called(ctx, pool, token, amount)
	return args.Get(0).(*interfaces.SlippageEstimate), args.Error(1)
}

func (m *MockSlippageCalculator) GetHistoricalSlippage(pool string, token string, timeWindow time.Duration) ([]*interfaces.SlippageData, error) {
	args := m.Called(pool, token, timeWindow)
	return args.Get(0).([]*interfaces.SlippageData), args.Error(1)
}

func (m *MockSlippageCalculator) UpdateSlippageModel(pool string, token string, actualSlippage *big.Int) error {
	args := m.Called(pool, token, actualSlippage)
	return args.Error(0)
}

func TestNewCalculator(t *testing.T) {
	gasEstimator := &MockGasEstimator{}
	slippageCalculator := &MockSlippageCalculator{}

	calc := NewCalculator(gasEstimator, slippageCalculator)

	assert.NotNil(t, calc)
	assert.Equal(t, gasEstimator, calc.gasEstimator)
	assert.Equal(t, slippageCalculator, calc.slippageCalculator)
	assert.Len(t, calc.thresholds, 4) // Should have thresholds for all 4 strategies
}

func TestCalculateProfit_Success(t *testing.T) {
	gasEstimator := &MockGasEstimator{}
	slippageCalculator := &MockSlippageCalculator{}
	calc := NewCalculator(gasEstimator, slippageCalculator)

	ctx := context.Background()
	
	// Create test transaction
	tx := &types.Transaction{
		Hash:     "0x123",
		From:     common.HexToAddress("0x1"),
		To:       &common.Address{},
		Value:    big.NewInt(1e18),
		GasPrice: big.NewInt(20e9),
		GasLimit: 21000,
		Nonce:    1,
		Data:     []byte{},
		ChainID:  big.NewInt(8453),
	}

	// Create test opportunity
	opportunity := &interfaces.MEVOpportunity{
		ID:             "test-1",
		Strategy:       interfaces.StrategySandwich,
		TargetTx:       "0x123",
		ExpectedProfit: big.NewInt(5e16), // 0.05 ETH
		ExecutionTxs:   []*types.Transaction{tx},
		Metadata: map[string]interface{}{
			"pool":   "0xpool",
			"token":  "0xtoken",
			"amount": big.NewInt(1e18),
		},
	}

	// Mock gas estimation
	gasEstimator.On("GetCurrentGasPrice", ctx).Return(big.NewInt(20e9), nil)
	gasEstimator.On("EstimateGas", ctx, tx).Return(uint64(21000), nil)

	// Mock slippage calculation
	slippageEst := &interfaces.SlippageEstimate{
		ExpectedSlippage: big.NewInt(1e15), // 0.001 ETH
		MaxSlippage:      big.NewInt(2e15),
		PriceImpact:      0.01,
		Confidence:       0.9,
	}
	slippageCalculator.On("CalculateSlippage", ctx, "0xpool", "0xtoken", big.NewInt(1e18)).Return(slippageEst, nil)

	// Calculate profit
	estimate, err := calc.CalculateProfit(ctx, opportunity)

	require.NoError(t, err)
	assert.NotNil(t, estimate)
	assert.Equal(t, big.NewInt(5e16), estimate.GrossProfit)
	assert.Equal(t, big.NewInt(42e13), estimate.GasCosts) // 20e9 * 21000
	assert.Equal(t, big.NewInt(1e15), estimate.SlippageCosts)
	
	expectedNetProfit := big.NewInt(5e16)
	expectedNetProfit.Sub(expectedNetProfit, big.NewInt(42e13))
	expectedNetProfit.Sub(expectedNetProfit, big.NewInt(1e15))
	assert.Equal(t, expectedNetProfit, estimate.NetProfit)

	gasEstimator.AssertExpectations(t)
	slippageCalculator.AssertExpectations(t)
}

func TestCalculateProfit_NilOpportunity(t *testing.T) {
	gasEstimator := &MockGasEstimator{}
	slippageCalculator := &MockSlippageCalculator{}
	calc := NewCalculator(gasEstimator, slippageCalculator)

	ctx := context.Background()
	
	_, err := calc.CalculateProfit(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "opportunity cannot be nil")
}

func TestCalculateGasCosts_EmptyTransactions(t *testing.T) {
	gasEstimator := &MockGasEstimator{}
	slippageCalculator := &MockSlippageCalculator{}
	calc := NewCalculator(gasEstimator, slippageCalculator)

	ctx := context.Background()
	
	gasCosts, err := calc.CalculateGasCosts(ctx, []*types.Transaction{})
	
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(0), gasCosts)
}

func TestCalculateGasCosts_MultipleTransactions(t *testing.T) {
	gasEstimator := &MockGasEstimator{}
	slippageCalculator := &MockSlippageCalculator{}
	calc := NewCalculator(gasEstimator, slippageCalculator)

	ctx := context.Background()
	
	tx1 := &types.Transaction{Hash: "0x1", GasLimit: 21000}
	tx2 := &types.Transaction{Hash: "0x2", GasLimit: 50000}
	txs := []*types.Transaction{tx1, tx2}

	gasEstimator.On("GetCurrentGasPrice", ctx).Return(big.NewInt(20e9), nil)
	gasEstimator.On("EstimateGas", ctx, tx1).Return(uint64(21000), nil)
	gasEstimator.On("EstimateGas", ctx, tx2).Return(uint64(50000), nil)

	gasCosts, err := calc.CalculateGasCosts(ctx, txs)
	
	require.NoError(t, err)
	expectedCosts := big.NewInt(0)
	expectedCosts.Add(expectedCosts, big.NewInt(20e9*21000)) // tx1 cost
	expectedCosts.Add(expectedCosts, big.NewInt(20e9*50000)) // tx2 cost
	assert.Equal(t, expectedCosts, gasCosts)

	gasEstimator.AssertExpectations(t)
}

func TestCalculateSlippage_NoMetadata(t *testing.T) {
	gasEstimator := &MockGasEstimator{}
	slippageCalculator := &MockSlippageCalculator{}
	calc := NewCalculator(gasEstimator, slippageCalculator)

	ctx := context.Background()
	opportunity := &interfaces.MEVOpportunity{
		ID:       "test-1",
		Strategy: interfaces.StrategySandwich,
		Metadata: nil,
	}

	slippage, err := calc.CalculateSlippage(ctx, opportunity)
	
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(0), slippage)
}

func TestValidateProfitability_Success(t *testing.T) {
	gasEstimator := &MockGasEstimator{}
	slippageCalculator := &MockSlippageCalculator{}
	calc := NewCalculator(gasEstimator, slippageCalculator)

	ctx := context.Background()
	
	// Create profitable opportunity
	tx := &types.Transaction{
		Hash:     "0x123",
		GasLimit: 21000,
	}

	opportunity := &interfaces.MEVOpportunity{
		ID:             "test-1",
		Strategy:       interfaces.StrategySandwich,
		ExpectedProfit: big.NewInt(5e16), // 0.05 ETH - above threshold
		ExecutionTxs:   []*types.Transaction{tx},
		Metadata: map[string]interface{}{
			"pool":   "0xpool",
			"token":  "0xtoken",
			"amount": big.NewInt(1e18),
		},
	}

	// Mock gas estimation (low cost)
	gasEstimator.On("GetCurrentGasPrice", ctx).Return(big.NewInt(1e9), nil)
	gasEstimator.On("EstimateGas", ctx, tx).Return(uint64(21000), nil)

	// Mock slippage calculation (low slippage)
	slippageEst := &interfaces.SlippageEstimate{
		ExpectedSlippage: big.NewInt(1e14), // Very low slippage
		MaxSlippage:      big.NewInt(2e14),
		PriceImpact:      0.001,
		Confidence:       0.95,
	}
	slippageCalculator.On("CalculateSlippage", ctx, "0xpool", "0xtoken", big.NewInt(1e18)).Return(slippageEst, nil)

	isProfitable, err := calc.ValidateProfitability(ctx, opportunity)
	
	require.NoError(t, err)
	assert.True(t, isProfitable)

	gasEstimator.AssertExpectations(t)
	slippageCalculator.AssertExpectations(t)
}

func TestValidateProfitability_BelowThreshold(t *testing.T) {
	gasEstimator := &MockGasEstimator{}
	slippageCalculator := &MockSlippageCalculator{}
	calc := NewCalculator(gasEstimator, slippageCalculator)

	ctx := context.Background()
	
	tx := &types.Transaction{
		Hash:     "0x123",
		GasLimit: 21000,
	}

	// Create opportunity with profit below threshold
	opportunity := &interfaces.MEVOpportunity{
		ID:             "test-1",
		Strategy:       interfaces.StrategySandwich,
		ExpectedProfit: big.NewInt(5e15), // 0.005 ETH - below sandwich threshold
		ExecutionTxs:   []*types.Transaction{tx},
		Metadata: map[string]interface{}{
			"pool":   "0xpool",
			"token":  "0xtoken",
			"amount": big.NewInt(1e18),
		},
	}

	gasEstimator.On("GetCurrentGasPrice", ctx).Return(big.NewInt(1e9), nil)
	gasEstimator.On("EstimateGas", ctx, tx).Return(uint64(21000), nil)

	slippageEst := &interfaces.SlippageEstimate{
		ExpectedSlippage: big.NewInt(1e14),
		MaxSlippage:      big.NewInt(2e14),
		PriceImpact:      0.001,
		Confidence:       0.95,
	}
	slippageCalculator.On("CalculateSlippage", ctx, "0xpool", "0xtoken", big.NewInt(1e18)).Return(slippageEst, nil)

	isProfitable, err := calc.ValidateProfitability(ctx, opportunity)
	
	require.NoError(t, err)
	assert.False(t, isProfitable)

	gasEstimator.AssertExpectations(t)
	slippageCalculator.AssertExpectations(t)
}

func TestValidateProfitability_UnknownStrategy(t *testing.T) {
	gasEstimator := &MockGasEstimator{}
	slippageCalculator := &MockSlippageCalculator{}
	calc := NewCalculator(gasEstimator, slippageCalculator)

	ctx := context.Background()
	
	tx := &types.Transaction{
		Hash:     "0x123",
		GasLimit: 21000,
	}

	opportunity := &interfaces.MEVOpportunity{
		ID:             "test-1",
		Strategy:       interfaces.StrategyType("unknown"),
		ExpectedProfit: big.NewInt(1e16),
		ExecutionTxs:   []*types.Transaction{tx},
		Metadata: map[string]interface{}{
			"pool":   "0xpool",
			"token":  "0xtoken",
			"amount": big.NewInt(1e18),
		},
	}

	gasEstimator.On("GetCurrentGasPrice", ctx).Return(big.NewInt(20e9), nil)
	gasEstimator.On("EstimateGas", ctx, tx).Return(uint64(21000), nil)

	slippageEst := &interfaces.SlippageEstimate{
		ExpectedSlippage: big.NewInt(1e15),
		MaxSlippage:      big.NewInt(2e15),
		PriceImpact:      0.01,
		Confidence:       0.9,
	}
	slippageCalculator.On("CalculateSlippage", ctx, "0xpool", "0xtoken", big.NewInt(1e18)).Return(slippageEst, nil)

	_, err := calc.ValidateProfitability(ctx, opportunity)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no threshold defined for strategy")

	gasEstimator.AssertExpectations(t)
	slippageCalculator.AssertExpectations(t)
}

func TestSetAndGetThreshold(t *testing.T) {
	gasEstimator := &MockGasEstimator{}
	slippageCalculator := &MockSlippageCalculator{}
	calc := NewCalculator(gasEstimator, slippageCalculator)

	customThreshold := &ProfitThreshold{
		MinNetProfit:          big.NewInt(1e17),
		MinProfitMargin:       0.05,
		MinSuccessProbability: 0.9,
		MaxRiskScore:          0.1,
	}

	// Set custom threshold
	calc.SetThreshold(interfaces.StrategySandwich, customThreshold)

	// Get threshold
	retrieved, exists := calc.GetThreshold(interfaces.StrategySandwich)
	
	assert.True(t, exists)
	assert.Equal(t, customThreshold, retrieved)
}

func TestGetThreshold_NonExistent(t *testing.T) {
	gasEstimator := &MockGasEstimator{}
	slippageCalculator := &MockSlippageCalculator{}
	calc := NewCalculator(gasEstimator, slippageCalculator)

	_, exists := calc.GetThreshold(interfaces.StrategyType("nonexistent"))
	assert.False(t, exists)
}

func TestMonteCarloSimulation_Integration(t *testing.T) {
	gasEstimator := &MockGasEstimator{}
	slippageCalculator := &MockSlippageCalculator{}
	calc := NewCalculator(gasEstimator, slippageCalculator)

	ctx := context.Background()
	
	tx := &types.Transaction{
		Hash:     "0x123",
		GasLimit: 21000,
	}

	opportunity := &interfaces.MEVOpportunity{
		ID:             "test-1",
		Strategy:       interfaces.StrategySandwich,
		ExpectedProfit: big.NewInt(5e16),
		ExecutionTxs:   []*types.Transaction{tx},
		Metadata: map[string]interface{}{
			"pool":   "0xpool",
			"token":  "0xtoken",
			"amount": big.NewInt(1e18),
		},
	}

	gasEstimator.On("GetCurrentGasPrice", ctx).Return(big.NewInt(20e9), nil)
	gasEstimator.On("EstimateGas", ctx, tx).Return(uint64(21000), nil)

	slippageEst := &interfaces.SlippageEstimate{
		ExpectedSlippage: big.NewInt(1e15),
		MaxSlippage:      big.NewInt(2e15),
		PriceImpact:      0.01,
		Confidence:       0.9,
	}
	slippageCalculator.On("CalculateSlippage", ctx, "0xpool", "0xtoken", big.NewInt(1e18)).Return(slippageEst, nil)

	estimate, err := calc.CalculateProfit(ctx, opportunity)
	
	require.NoError(t, err)
	assert.NotNil(t, estimate)
	
	// Monte Carlo results should be reasonable
	assert.True(t, estimate.SuccessProbability >= 0 && estimate.SuccessProbability <= 1)
	assert.True(t, estimate.RiskScore >= 0 && estimate.RiskScore <= 1)
	assert.True(t, estimate.Confidence >= 0 && estimate.Confidence <= 1)

	gasEstimator.AssertExpectations(t)
	slippageCalculator.AssertExpectations(t)
}

func TestCalculateVariance(t *testing.T) {
	gasEstimator := &MockGasEstimator{}
	slippageCalculator := &MockSlippageCalculator{}
	calc := NewCalculator(gasEstimator, slippageCalculator)

	// Test with known values
	values := []float64{1.0, 2.0, 3.0, 4.0, 5.0}
	variance := calc.calculateVariance(values)
	
	// Expected variance for this set is 2.0
	assert.InDelta(t, 2.0, variance, 0.001)

	// Test with empty slice
	emptyVariance := calc.calculateVariance([]float64{})
	assert.Equal(t, 0.0, emptyVariance)
}

func TestDefaultThresholds(t *testing.T) {
	gasEstimator := &MockGasEstimator{}
	slippageCalculator := &MockSlippageCalculator{}
	calc := NewCalculator(gasEstimator, slippageCalculator)

	// Test that all strategies have default thresholds
	strategies := []interfaces.StrategyType{
		interfaces.StrategySandwich,
		interfaces.StrategyBackrun,
		interfaces.StrategyFrontrun,
		interfaces.StrategyTimeBandit,
	}

	for _, strategy := range strategies {
		threshold, exists := calc.GetThreshold(strategy)
		assert.True(t, exists, "Strategy %s should have a default threshold", strategy)
		assert.NotNil(t, threshold)
		assert.True(t, threshold.MinNetProfit.Cmp(big.NewInt(0)) > 0)
		assert.True(t, threshold.MinProfitMargin > 0)
		assert.True(t, threshold.MinSuccessProbability > 0 && threshold.MinSuccessProbability <= 1)
		assert.True(t, threshold.MaxRiskScore >= 0 && threshold.MaxRiskScore <= 1)
	}
}

// Benchmark tests
func BenchmarkCalculateProfit(b *testing.B) {
	gasEstimator := &MockGasEstimator{}
	slippageCalculator := &MockSlippageCalculator{}
	calc := NewCalculator(gasEstimator, slippageCalculator)

	ctx := context.Background()
	tx := &types.Transaction{Hash: "0x123", GasLimit: 21000}
	opportunity := &interfaces.MEVOpportunity{
		ID:             "bench-1",
		Strategy:       interfaces.StrategySandwich,
		ExpectedProfit: big.NewInt(5e16),
		ExecutionTxs:   []*types.Transaction{tx},
		Metadata: map[string]interface{}{
			"pool":   "0xpool",
			"token":  "0xtoken",
			"amount": big.NewInt(1e18),
		},
	}

	gasEstimator.On("GetCurrentGasPrice", ctx).Return(big.NewInt(20e9), nil)
	gasEstimator.On("EstimateGas", ctx, tx).Return(uint64(21000), nil)

	slippageEst := &interfaces.SlippageEstimate{
		ExpectedSlippage: big.NewInt(1e15),
		MaxSlippage:      big.NewInt(2e15),
		PriceImpact:      0.01,
		Confidence:       0.9,
	}
	slippageCalculator.On("CalculateSlippage", ctx, "0xpool", "0xtoken", big.NewInt(1e18)).Return(slippageEst, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := calc.CalculateProfit(ctx, opportunity)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Additional edge case tests

func TestCalculateProfit_GasEstimationError(t *testing.T) {
	gasEstimator := &MockGasEstimator{}
	slippageCalculator := &MockSlippageCalculator{}
	calc := NewCalculator(gasEstimator, slippageCalculator)

	ctx := context.Background()
	
	tx := &types.Transaction{
		Hash:     "0x123",
		GasLimit: 21000,
	}

	opportunity := &interfaces.MEVOpportunity{
		ID:             "test-1",
		Strategy:       interfaces.StrategySandwich,
		ExpectedProfit: big.NewInt(5e16),
		ExecutionTxs:   []*types.Transaction{tx},
	}

	// Mock gas estimation to return error
	gasEstimator.On("GetCurrentGasPrice", ctx).Return((*big.Int)(nil), fmt.Errorf("gas price error"))

	_, err := calc.CalculateProfit(ctx, opportunity)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to calculate gas costs")

	gasEstimator.AssertExpectations(t)
}

func TestCalculateProfit_SlippageCalculationError(t *testing.T) {
	gasEstimator := &MockGasEstimator{}
	slippageCalculator := &MockSlippageCalculator{}
	calc := NewCalculator(gasEstimator, slippageCalculator)

	ctx := context.Background()
	
	tx := &types.Transaction{
		Hash:     "0x123",
		GasLimit: 21000,
	}

	opportunity := &interfaces.MEVOpportunity{
		ID:             "test-1",
		Strategy:       interfaces.StrategySandwich,
		ExpectedProfit: big.NewInt(5e16),
		ExecutionTxs:   []*types.Transaction{tx},
		Metadata: map[string]interface{}{
			"pool":   "0xpool",
			"token":  "0xtoken",
			"amount": big.NewInt(1e18),
		},
	}

	gasEstimator.On("GetCurrentGasPrice", ctx).Return(big.NewInt(20e9), nil)
	gasEstimator.On("EstimateGas", ctx, tx).Return(uint64(21000), nil)

	// Mock slippage calculation to return error
	slippageCalculator.On("CalculateSlippage", ctx, "0xpool", "0xtoken", big.NewInt(1e18)).Return((*interfaces.SlippageEstimate)(nil), fmt.Errorf("slippage error"))

	_, err := calc.CalculateProfit(ctx, opportunity)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to calculate slippage")

	gasEstimator.AssertExpectations(t)
	slippageCalculator.AssertExpectations(t)
}

func TestCalculateProfit_ZeroExpectedProfit(t *testing.T) {
	gasEstimator := &MockGasEstimator{}
	slippageCalculator := &MockSlippageCalculator{}
	calc := NewCalculator(gasEstimator, slippageCalculator)

	ctx := context.Background()
	
	tx := &types.Transaction{
		Hash:     "0x123",
		GasLimit: 21000,
	}

	opportunity := &interfaces.MEVOpportunity{
		ID:             "test-1",
		Strategy:       interfaces.StrategySandwich,
		ExpectedProfit: big.NewInt(0), // Zero expected profit
		ExecutionTxs:   []*types.Transaction{tx},
		Metadata: map[string]interface{}{
			"pool":   "0xpool",
			"token":  "0xtoken",
			"amount": big.NewInt(1e18),
		},
	}

	gasEstimator.On("GetCurrentGasPrice", ctx).Return(big.NewInt(20e9), nil)
	gasEstimator.On("EstimateGas", ctx, tx).Return(uint64(21000), nil)

	slippageEst := &interfaces.SlippageEstimate{
		ExpectedSlippage: big.NewInt(1e15),
		MaxSlippage:      big.NewInt(2e15),
		PriceImpact:      0.01,
		Confidence:       0.9,
	}
	slippageCalculator.On("CalculateSlippage", ctx, "0xpool", "0xtoken", big.NewInt(1e18)).Return(slippageEst, nil)

	estimate, err := calc.CalculateProfit(ctx, opportunity)
	
	require.NoError(t, err)
	assert.NotNil(t, estimate)
	assert.Equal(t, big.NewInt(0), estimate.GrossProfit)
	assert.Equal(t, 0.0, estimate.ProfitMargin) // Should be 0 when expected profit is 0
	assert.True(t, estimate.NetProfit.Cmp(big.NewInt(0)) < 0) // Should be negative due to costs

	gasEstimator.AssertExpectations(t)
	slippageCalculator.AssertExpectations(t)
}

func TestCalculateProfit_HighGasCosts(t *testing.T) {
	gasEstimator := &MockGasEstimator{}
	slippageCalculator := &MockSlippageCalculator{}
	calc := NewCalculator(gasEstimator, slippageCalculator)

	ctx := context.Background()
	
	tx := &types.Transaction{
		Hash:     "0x123",
		GasLimit: 1000000, // Very high gas limit
	}

	opportunity := &interfaces.MEVOpportunity{
		ID:             "test-1",
		Strategy:       interfaces.StrategySandwich,
		ExpectedProfit: big.NewInt(1e16), // 0.01 ETH
		ExecutionTxs:   []*types.Transaction{tx},
		Metadata: map[string]interface{}{
			"pool":   "0xpool",
			"token":  "0xtoken",
			"amount": big.NewInt(1e18),
		},
	}

	// Very high gas price
	gasEstimator.On("GetCurrentGasPrice", ctx).Return(big.NewInt(100e9), nil)
	gasEstimator.On("EstimateGas", ctx, tx).Return(uint64(1000000), nil)

	slippageEst := &interfaces.SlippageEstimate{
		ExpectedSlippage: big.NewInt(1e15),
		MaxSlippage:      big.NewInt(2e15),
		PriceImpact:      0.01,
		Confidence:       0.9,
	}
	slippageCalculator.On("CalculateSlippage", ctx, "0xpool", "0xtoken", big.NewInt(1e18)).Return(slippageEst, nil)

	estimate, err := calc.CalculateProfit(ctx, opportunity)
	
	require.NoError(t, err)
	assert.NotNil(t, estimate)
	
	// Gas costs should be very high (100e9 * 1000000 = 1e17)
	expectedGasCosts := big.NewInt(1e17)
	assert.Equal(t, expectedGasCosts, estimate.GasCosts)
	
	// Net profit should be negative due to high gas costs
	assert.True(t, estimate.NetProfit.Cmp(big.NewInt(0)) < 0)

	gasEstimator.AssertExpectations(t)
	slippageCalculator.AssertExpectations(t)
}

func TestMonteCarloSimulation_EdgeCases(t *testing.T) {
	gasEstimator := &MockGasEstimator{}
	slippageCalculator := &MockSlippageCalculator{}
	calc := NewCalculator(gasEstimator, slippageCalculator)

	ctx := context.Background()
	
	// Test with very small expected profit
	tx := &types.Transaction{
		Hash:     "0x123",
		GasLimit: 21000,
	}

	opportunity := &interfaces.MEVOpportunity{
		ID:             "test-1",
		Strategy:       interfaces.StrategySandwich,
		ExpectedProfit: big.NewInt(1e12), // Very small profit (0.000001 ETH)
		ExecutionTxs:   []*types.Transaction{tx},
		Metadata: map[string]interface{}{
			"pool":   "0xpool",
			"token":  "0xtoken",
			"amount": big.NewInt(1e18),
		},
	}

	gasEstimator.On("GetCurrentGasPrice", ctx).Return(big.NewInt(1e9), nil)
	gasEstimator.On("EstimateGas", ctx, tx).Return(uint64(21000), nil)

	slippageEst := &interfaces.SlippageEstimate{
		ExpectedSlippage: big.NewInt(1e12),
		MaxSlippage:      big.NewInt(2e12),
		PriceImpact:      0.001,
		Confidence:       0.95,
	}
	slippageCalculator.On("CalculateSlippage", ctx, "0xpool", "0xtoken", big.NewInt(1e18)).Return(slippageEst, nil)

	estimate, err := calc.CalculateProfit(ctx, opportunity)
	
	require.NoError(t, err)
	assert.NotNil(t, estimate)
	
	// With very small profits, success probability should be low due to variance
	assert.True(t, estimate.SuccessProbability >= 0 && estimate.SuccessProbability <= 1)
	assert.True(t, estimate.RiskScore >= 0 && estimate.RiskScore <= 1)

	gasEstimator.AssertExpectations(t)
	slippageCalculator.AssertExpectations(t)
}

func TestProfitThreshold_AllStrategies(t *testing.T) {
	gasEstimator := &MockGasEstimator{}
	slippageCalculator := &MockSlippageCalculator{}
	calc := NewCalculator(gasEstimator, slippageCalculator)

	// Test that each strategy type has different thresholds
	strategies := []interfaces.StrategyType{
		interfaces.StrategySandwich,
		interfaces.StrategyBackrun,
		interfaces.StrategyFrontrun,
		interfaces.StrategyTimeBandit,
	}

	thresholds := make(map[interfaces.StrategyType]*ProfitThreshold)
	
	for _, strategy := range strategies {
		threshold, exists := calc.GetThreshold(strategy)
		require.True(t, exists, "Strategy %s should have a threshold", strategy)
		thresholds[strategy] = threshold
	}

	// Verify that thresholds are different for different strategies
	sandwichThreshold := thresholds[interfaces.StrategySandwich]
	backrunThreshold := thresholds[interfaces.StrategyBackrun]
	
	// Sandwich should have higher minimum profit than backrun
	assert.True(t, sandwichThreshold.MinNetProfit.Cmp(backrunThreshold.MinNetProfit) > 0)
	
	// Backrun should have higher success probability requirement
	assert.True(t, backrunThreshold.MinSuccessProbability > sandwichThreshold.MinSuccessProbability)
}

func TestConcurrentAccess(t *testing.T) {
	gasEstimator := &MockGasEstimator{}
	slippageCalculator := &MockSlippageCalculator{}
	calc := NewCalculator(gasEstimator, slippageCalculator)

	// Test concurrent access to thresholds
	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2) // readers and writers

	// Start readers
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_, exists := calc.GetThreshold(interfaces.StrategySandwich)
				assert.True(t, exists)
			}
		}()
	}

	// Start writers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				threshold := &ProfitThreshold{
					MinNetProfit:          big.NewInt(int64(id + j)),
					MinProfitMargin:       0.01,
					MinSuccessProbability: 0.5,
					MaxRiskScore:          0.5,
				}
				calc.SetThreshold(interfaces.StrategySandwich, threshold)
			}
		}(i)
	}

	wg.Wait()
	
	// Verify final state is consistent
	threshold, exists := calc.GetThreshold(interfaces.StrategySandwich)
	assert.True(t, exists)
	assert.NotNil(t, threshold)
}