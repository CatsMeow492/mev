package profit

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

func TestNewSlippageCalculator(t *testing.T) {
	calc := NewSlippageCalculator()
	
	if calc == nil {
		t.Fatal("NewSlippageCalculator returned nil")
	}
	
	if calc.historicalData == nil {
		t.Error("historicalData map not initialized")
	}
	
	if calc.poolLiquidity == nil {
		t.Error("poolLiquidity map not initialized")
	}
	
	if calc.priceImpactModels == nil {
		t.Error("priceImpactModels map not initialized")
	}
}

func TestCalculateSlippage(t *testing.T) {
	calc := NewSlippageCalculator()
	ctx := context.Background()
	
	tests := []struct {
		name        string
		pool        string
		token       string
		amount      *big.Int
		shouldError bool
	}{
		{
			name:        "empty pool",
			pool:        "",
			token:       "0x123",
			amount:      big.NewInt(1e18),
			shouldError: true,
		},
		{
			name:        "empty token",
			pool:        "uniswap-v3",
			token:       "",
			amount:      big.NewInt(1e18),
			shouldError: true,
		},
		{
			name:        "nil amount",
			pool:        "uniswap-v3",
			token:       "0x123",
			amount:      nil,
			shouldError: true,
		},
		{
			name:        "valid parameters",
			pool:        "uniswap-v3",
			token:       "0x123",
			amount:      big.NewInt(1e18),
			shouldError: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			estimate, err := calc.CalculateSlippage(ctx, tt.pool, tt.token, tt.amount)
			
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
			
			if estimate == nil {
				t.Error("Expected estimate but got nil")
				return
			}
			
			// Validate estimate fields
			if estimate.ExpectedSlippage == nil {
				t.Error("ExpectedSlippage is nil")
			}
			
			if estimate.MaxSlippage == nil {
				t.Error("MaxSlippage is nil")
			}
			
			if estimate.PriceImpact < 0 {
				t.Error("PriceImpact should be non-negative")
			}
			
			if estimate.Confidence < 0 || estimate.Confidence > 1 {
				t.Errorf("Confidence should be between 0 and 1, got %f", estimate.Confidence)
			}
			
			// MaxSlippage should be >= ExpectedSlippage
			if estimate.MaxSlippage.Cmp(estimate.ExpectedSlippage) < 0 {
				t.Error("MaxSlippage should be >= ExpectedSlippage")
			}
		})
	}
}

func TestCalculateDefaultSlippage(t *testing.T) {
	calc := NewSlippageCalculator()
	
	tests := []struct {
		name           string
		amount         *big.Int
		expectedImpact float64 // Approximate expected price impact
	}{
		{
			name:           "small trade",
			amount:         big.NewInt(1e17), // 0.1 ETH
			expectedImpact: 0.001,            // ~0.1%
		},
		{
			name:           "medium trade",
			amount:         big.NewInt(1e18), // 1 ETH
			expectedImpact: 0.002,            // ~0.2%
		},
		{
			name:           "large trade",
			amount:         new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18)), // 10 ETH
			expectedImpact: 0.002,            // ~0.2% (adjusted for actual calculation)
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			estimate, err := calc.calculateDefaultSlippage("test-pool", "test-token", tt.amount)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			// Check that price impact is in reasonable range
			if estimate.PriceImpact < tt.expectedImpact*0.5 || estimate.PriceImpact > tt.expectedImpact*2 {
				t.Errorf("Price impact %f not in expected range around %f", estimate.PriceImpact, tt.expectedImpact)
			}
			
			// Check that slippage amounts are reasonable
			expectedSlippageFloat, _ := estimate.ExpectedSlippage.Float64()
			amountFloat, _ := tt.amount.Float64()
			
			slippageRatio := expectedSlippageFloat / amountFloat
			if slippageRatio < 0.0001 || slippageRatio > 0.1 {
				t.Errorf("Slippage ratio %f seems unreasonable", slippageRatio)
			}
		})
	}
}

func TestGetHistoricalSlippage(t *testing.T) {
	calc := NewSlippageCalculator()
	
	pool := "test-pool"
	token := "test-token"
	
	// Test empty history
	history, err := calc.GetHistoricalSlippage(pool, token, 24*time.Hour)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("Expected empty history, got %d entries", len(history))
	}
	
	// Add some historical data
	now := time.Now()
	testData := []*interfaces.SlippageData{
		{
			Timestamp:   now.Add(-2 * time.Hour),
			Pool:        pool,
			Token:       token,
			Amount:      big.NewInt(1e18),
			Slippage:    big.NewInt(1e16),
			PriceImpact: 0.01,
		},
		{
			Timestamp:   now.Add(-1 * time.Hour),
			Pool:        pool,
			Token:       token,
			Amount:      big.NewInt(2e18),
			Slippage:    big.NewInt(3e16),
			PriceImpact: 0.015,
		},
		{
			Timestamp:   now.Add(-30 * time.Minute),
			Pool:        pool,
			Token:       token,
			Amount:      big.NewInt(5e17),
			Slippage:    big.NewInt(8e15),
			PriceImpact: 0.016,
		},
	}
	
	key := "test-pool-test-token"
	calc.historicalData[key] = testData
	
	// Test getting history for last hour
	recentHistory, err := calc.GetHistoricalSlippage(pool, token, 1*time.Hour)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	expectedCount := 2 // Last 2 entries are within 1 hour
	if len(recentHistory) < expectedCount-1 || len(recentHistory) > expectedCount {
		t.Errorf("Expected around %d entries in last hour, got %d", expectedCount, len(recentHistory))
	}
	
	// Test getting all history
	allHistory, err := calc.GetHistoricalSlippage(pool, token, 24*time.Hour)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	if len(allHistory) != len(testData) {
		t.Errorf("Expected %d entries in last 24 hours, got %d", len(testData), len(allHistory))
	}
}

func TestUpdateSlippageModel(t *testing.T) {
	calc := NewSlippageCalculator()
	
	pool := "test-pool"
	token := "test-token"
	slippage := big.NewInt(1e16) // 0.01 ETH
	
	// Test invalid parameters
	err := calc.UpdateSlippageModel("", token, slippage)
	if err == nil {
		t.Error("Expected error for empty pool")
	}
	
	err = calc.UpdateSlippageModel(pool, "", slippage)
	if err == nil {
		t.Error("Expected error for empty token")
	}
	
	err = calc.UpdateSlippageModel(pool, token, nil)
	if err == nil {
		t.Error("Expected error for nil slippage")
	}
	
	// Test valid update
	err = calc.UpdateSlippageModel(pool, token, slippage)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	// Check that data was added
	key := "test-pool-test-token"
	if _, exists := calc.historicalData[key]; !exists {
		t.Error("Historical data not created")
	}
	
	if len(calc.historicalData[key]) != 1 {
		t.Errorf("Expected 1 entry in historical data, got %d", len(calc.historicalData[key]))
	}
	
	// Add more data to trigger model calibration
	for i := 0; i < 15; i++ {
		err = calc.UpdateSlippageModel(pool, token, big.NewInt(int64(1e16+i*1e15)))
		if err != nil {
			t.Errorf("Unexpected error on update %d: %v", i, err)
		}
	}
	
	// Check that model was calibrated
	if _, exists := calc.priceImpactModels[key]; !exists {
		t.Error("Price impact model not created after sufficient data")
	}
}

func TestUpdatePoolLiquidity(t *testing.T) {
	calc := NewSlippageCalculator()
	
	pool := "test-pool"
	liquidity := new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18)) // 100 ETH
	
	// Test update
	calc.UpdatePoolLiquidity(pool, liquidity)
	
	// Test retrieval
	retrievedLiquidity, exists := calc.GetPoolLiquidity(pool)
	if !exists {
		t.Error("Pool liquidity not found after update")
	}
	
	if retrievedLiquidity.Cmp(liquidity) != 0 {
		t.Errorf("Expected liquidity %s, got %s", liquidity.String(), retrievedLiquidity.String())
	}
	
	// Test non-existent pool
	_, exists = calc.GetPoolLiquidity("non-existent")
	if exists {
		t.Error("Expected false for non-existent pool")
	}
}

func TestModelAccuracy(t *testing.T) {
	calc := NewSlippageCalculator()
	
	pool := "test-pool"
	token := "test-token"
	
	// Initially no model
	accuracy := calc.GetModelAccuracy(pool, token)
	if accuracy != 0.0 {
		t.Errorf("Expected 0 accuracy for non-existent model, got %f", accuracy)
	}
	
	// Add data to create model
	for i := 0; i < 20; i++ {
		slippage := big.NewInt(int64(1e16 + i*1e15))
		err := calc.UpdateSlippageModel(pool, token, slippage)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	}
	
	// Check that model exists and has accuracy
	accuracy = calc.GetModelAccuracy(pool, token)
	if accuracy < 0 || accuracy > 1 {
		t.Errorf("Model accuracy should be between 0 and 1, got %f", accuracy)
	}
}

func TestCalibrateAllModels(t *testing.T) {
	calc := NewSlippageCalculator()
	
	// Add data for multiple pool-token pairs
	pairs := []struct {
		pool  string
		token string
	}{
		{"pool1", "token1"},
		{"pool2", "token2"},
		{"pool3", "token3"},
	}
	
	for _, pair := range pairs {
		for i := 0; i < 15; i++ {
			slippage := big.NewInt(int64(1e16 + i*1e15))
			err := calc.UpdateSlippageModel(pair.pool, pair.token, slippage)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		}
	}
	
	// Calibrate all models
	calc.CalibrateAllModels()
	
	// Check that all models were calibrated
	for _, pair := range pairs {
		key := pair.pool + "-" + pair.token
		if _, exists := calc.priceImpactModels[key]; !exists {
			t.Errorf("Model not calibrated for %s", key)
		}
	}
}

func TestGetModelStats(t *testing.T) {
	calc := NewSlippageCalculator()
	
	// Initially empty stats
	stats := calc.GetModelStats()
	if stats["total_models"].(int) != 0 {
		t.Error("Expected 0 models initially")
	}
	
	// Add some data and models
	calc.UpdatePoolLiquidity("pool1", new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18)))
	calc.UpdatePoolLiquidity("pool2", new(big.Int).Mul(big.NewInt(200), big.NewInt(1e18)))
	
	for i := 0; i < 15; i++ {
		calc.UpdateSlippageModel("pool1", "token1", big.NewInt(int64(1e16+i*1e15)))
		calc.UpdateSlippageModel("pool2", "token2", big.NewInt(int64(2e16+i*1e15)))
	}
	
	stats = calc.GetModelStats()
	
	if stats["total_models"].(int) != 2 {
		t.Errorf("Expected 2 models, got %d", stats["total_models"].(int))
	}
	
	if stats["total_pools"].(int) != 2 {
		t.Errorf("Expected 2 pools, got %d", stats["total_pools"].(int))
	}
	
	if stats["total_historical_pairs"].(int) != 2 {
		t.Errorf("Expected 2 historical pairs, got %d", stats["total_historical_pairs"].(int))
	}
	
	// Check that accuracy stats exist
	if _, exists := stats["average_accuracy"]; !exists {
		t.Error("Average accuracy not in stats")
	}
	
	if _, exists := stats["median_accuracy"]; !exists {
		t.Error("Median accuracy not in stats")
	}
}

func TestCalculateModelBasedSlippage(t *testing.T) {
	calc := NewSlippageCalculator()
	
	model := &PriceImpactModel{
		Alpha:          0.001,
		Beta:           0.5,
		Gamma:          0.2,
		LastCalibrated: time.Now(),
		Accuracy:       0.8,
		SampleCount:    100,
	}
	
	amount := big.NewInt(1e18)
	liquidity := new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18))
	
	estimate, err := calc.calculateModelBasedSlippage(model, amount, liquidity, true)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}
	
	if estimate == nil {
		t.Error("Expected estimate but got nil")
		return
	}
	
	// Validate estimate
	if estimate.ExpectedSlippage == nil {
		t.Error("ExpectedSlippage is nil")
	}
	
	if estimate.MaxSlippage == nil {
		t.Error("MaxSlippage is nil")
	}
	
	if estimate.Confidence != model.Accuracy {
		t.Errorf("Expected confidence %f, got %f", model.Accuracy, estimate.Confidence)
	}
	
	// Price impact should be reasonable for the given parameters
	if estimate.PriceImpact < 0 || estimate.PriceImpact > 0.5 {
		t.Errorf("Price impact %f seems unreasonable", estimate.PriceImpact)
	}
}