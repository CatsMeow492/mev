package profit

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

func TestNewGasEstimator(t *testing.T) {
	estimator := NewGasEstimator()
	
	if estimator == nil {
		t.Fatal("NewGasEstimator returned nil")
	}
	
	if estimator.baseGasPrice.Cmp(big.NewInt(1e9)) != 0 {
		t.Errorf("Expected base gas price to be 1e9, got %s", estimator.baseGasPrice.String())
	}
	
	// Check strategy gas usage initialization
	expectedUsage := map[interfaces.StrategyType]uint64{
		interfaces.StrategySandwich:   300000,
		interfaces.StrategyBackrun:    150000,
		interfaces.StrategyFrontrun:   100000,
		interfaces.StrategyTimeBandit: 250000,
	}
	
	for strategy, expected := range expectedUsage {
		actual := estimator.GetStrategyGasUsage(strategy)
		if actual != expected {
			t.Errorf("Expected gas usage for %s to be %d, got %d", strategy, expected, actual)
		}
	}
}

func TestEstimateGas(t *testing.T) {
	estimator := NewGasEstimator()
	ctx := context.Background()
	
	tests := []struct {
		name        string
		tx          *types.Transaction
		expectedMin uint64
		expectedMax uint64
		shouldError bool
	}{
		{
			name:        "nil transaction",
			tx:          nil,
			shouldError: true,
		},
		{
			name: "simple transfer",
			tx: &types.Transaction{
				Hash:     "0x123",
				From:     common.HexToAddress("0xabc"),
				To:       func() *common.Address { addr := common.HexToAddress("0xdef"); return &addr }(),
				Value:    big.NewInt(1e18),
				GasPrice: big.NewInt(1e9),
				GasLimit: 21000,
				Data:     []byte{},
			},
			expectedMin: 21000,
			expectedMax: 25000,
		},
		{
			name: "contract interaction with data",
			tx: &types.Transaction{
				Hash:     "0x456",
				From:     common.HexToAddress("0xabc"),
				To:       func() *common.Address { addr := common.HexToAddress("0xdef"); return &addr }(),
				Value:    big.NewInt(0),
				GasPrice: big.NewInt(1e9),
				GasLimit: 100000,
				Data:     make([]byte, 200), // 200 bytes of data
			},
			expectedMin: 70000,
			expectedMax: 85000,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gasUsage, err := estimator.EstimateGas(ctx, tt.tx)
			
			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if gasUsage < tt.expectedMin || gasUsage > tt.expectedMax {
				t.Errorf("Gas usage %d not in expected range [%d, %d]", gasUsage, tt.expectedMin, tt.expectedMax)
			}
		})
	}
}

func TestEstimateBatchGas(t *testing.T) {
	estimator := NewGasEstimator()
	ctx := context.Background()
	
	// Test empty batch
	gasUsage, err := estimator.EstimateBatchGas(ctx, []*types.Transaction{})
	if err != nil {
		t.Errorf("Unexpected error for empty batch: %v", err)
	}
	if gasUsage != 0 {
		t.Errorf("Expected 0 gas for empty batch, got %d", gasUsage)
	}
	
	// Test batch with multiple transactions
	txs := []*types.Transaction{
		{
			Hash:     "0x123",
			From:     common.HexToAddress("0xabc"),
			To:       func() *common.Address { addr := common.HexToAddress("0xdef"); return &addr }(),
			Value:    big.NewInt(1e18),
			GasPrice: big.NewInt(1e9),
			GasLimit: 21000,
			Data:     []byte{},
		},
		{
			Hash:     "0x456",
			From:     common.HexToAddress("0xabc"),
			To:       func() *common.Address { addr := common.HexToAddress("0xdef"); return &addr }(),
			Value:    big.NewInt(0),
			GasPrice: big.NewInt(1e9),
			GasLimit: 100000,
			Data:     make([]byte, 100),
		},
	}
	
	batchGas, err := estimator.EstimateBatchGas(ctx, txs)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	// Calculate expected gas (individual estimates + 5% batch overhead)
	var expectedTotal uint64
	for _, tx := range txs {
		individual, _ := estimator.EstimateGas(ctx, tx)
		expectedTotal += individual
	}
	expectedTotal += expectedTotal / 20 // 5% overhead
	
	if batchGas != expectedTotal {
		t.Errorf("Expected batch gas %d, got %d", expectedTotal, batchGas)
	}
}

func TestGetCurrentGasPrice(t *testing.T) {
	estimator := NewGasEstimator()
	ctx := context.Background()
	
	// Test initial gas price
	gasPrice, err := estimator.GetCurrentGasPrice(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	if gasPrice.Cmp(big.NewInt(1e9)) != 0 {
		t.Errorf("Expected initial gas price to be 1e9, got %s", gasPrice.String())
	}
	
	// Update gas price and test
	newPrice := big.NewInt(2e9)
	estimator.UpdateGasPrice(newPrice, interfaces.GasPriorityMedium)
	
	updatedPrice, err := estimator.GetCurrentGasPrice(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	// Should return the updated price
	if updatedPrice.Cmp(newPrice) != 0 {
		t.Errorf("Expected updated gas price to be %s, got %s", newPrice.String(), updatedPrice.String())
	}
}

func TestPredictGasPrice(t *testing.T) {
	estimator := NewGasEstimator()
	ctx := context.Background()
	
	basePrice := big.NewInt(1e9)
	
	tests := []struct {
		priority         interfaces.GasPriority
		expectedMultiplier float64
	}{
		{interfaces.GasPriorityLow, 0.9},
		{interfaces.GasPriorityMedium, 1.1},
		{interfaces.GasPriorityHigh, 1.3},
		{interfaces.GasPriorityUrgent, 1.5},
	}
	
	for _, tt := range tests {
		t.Run(string(tt.priority), func(t *testing.T) {
			predictedPrice, err := estimator.PredictGasPrice(ctx, tt.priority)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			expectedPrice := new(big.Int).Mul(basePrice, big.NewInt(int64(tt.expectedMultiplier*100)))
			expectedPrice.Div(expectedPrice, big.NewInt(100))
			
			if predictedPrice.Cmp(expectedPrice) != 0 {
				t.Errorf("Expected predicted price %s, got %s", expectedPrice.String(), predictedPrice.String())
			}
		})
	}
}

func TestUpdateGasPrice(t *testing.T) {
	estimator := NewGasEstimator()
	
	// Test updating gas price
	prices := []*big.Int{
		big.NewInt(1e9),
		big.NewInt(2e9),
		big.NewInt(1.5e9),
		big.NewInt(1.8e9),
		big.NewInt(1.2e9),
	}
	
	for i, price := range prices {
		estimator.UpdateGasPrice(price, interfaces.GasPriorityMedium)
		
		// Check that history is updated
		if len(estimator.gasHistory) != i+1 {
			t.Errorf("Expected history length %d, got %d", i+1, len(estimator.gasHistory))
		}
		
		// Check that the latest entry matches
		latest := estimator.gasHistory[len(estimator.gasHistory)-1]
		if latest.GasPrice.Cmp(price) != 0 {
			t.Errorf("Expected latest price %s, got %s", price.String(), latest.GasPrice.String())
		}
	}
	
	// Test history limit
	for i := 0; i < 1000; i++ {
		estimator.UpdateGasPrice(big.NewInt(1e9), interfaces.GasPriorityMedium)
	}
	
	if len(estimator.gasHistory) > 1000 {
		t.Errorf("Expected history length to be capped at 1000, got %d", len(estimator.gasHistory))
	}
}

func TestUpdateStrategyGasUsage(t *testing.T) {
	estimator := NewGasEstimator()
	
	strategy := interfaces.StrategySandwich
	initialUsage := estimator.GetStrategyGasUsage(strategy)
	
	// Update with actual usage
	actualUsage := uint64(350000)
	estimator.UpdateStrategyGasUsage(strategy, actualUsage)
	
	updatedUsage := estimator.GetStrategyGasUsage(strategy)
	
	// Should be between initial and actual (exponential moving average)
	if updatedUsage <= initialUsage || updatedUsage >= actualUsage {
		t.Errorf("Expected updated usage to be between %d and %d, got %d", initialUsage, actualUsage, updatedUsage)
	}
}

func TestGetGasHistory(t *testing.T) {
	estimator := NewGasEstimator()
	
	// Add some historical data
	now := time.Now()
	prices := []struct {
		price     *big.Int
		timestamp time.Time
	}{
		{big.NewInt(1e9), now.Add(-2 * time.Hour)},
		{big.NewInt(1.5e9), now.Add(-1 * time.Hour)},
		{big.NewInt(2e9), now.Add(-30 * time.Minute)},
		{big.NewInt(1.8e9), now.Add(-10 * time.Minute)},
	}
	
	for _, p := range prices {
		estimator.UpdateGasPrice(p.price, interfaces.GasPriorityMedium)
		// Manually set timestamp for testing
		if len(estimator.gasHistory) > 0 {
			estimator.gasHistory[len(estimator.gasHistory)-1].Timestamp = p.timestamp
		}
	}
	
	// Test getting history for last hour
	history := estimator.GetGasHistory(1 * time.Hour)
	
	// Should return last 3 entries (within 1 hour) - but timing might be off by a few seconds
	expectedCount := 3
	if len(history) < expectedCount-1 || len(history) > expectedCount {
		t.Errorf("Expected around %d entries in last hour, got %d", expectedCount, len(history))
	}
	
	// Test getting all history
	allHistory := estimator.GetGasHistory(24 * time.Hour)
	if len(allHistory) != len(prices) {
		t.Errorf("Expected %d entries in last 24 hours, got %d", len(prices), len(allHistory))
	}
}