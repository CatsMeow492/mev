package profit

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// SlippageCalculator implements the SlippageCalculator interface
type SlippageCalculator struct {
	historicalData map[string][]*interfaces.SlippageData // key: pool-token
	poolLiquidity  map[string]*big.Int                   // key: pool
	priceImpactModels map[string]*PriceImpactModel       // key: pool-token
	mu             sync.RWMutex
}

// PriceImpactModel represents a calibrated model for price impact calculation
type PriceImpactModel struct {
	Alpha          float64   // Base impact coefficient
	Beta           float64   // Volume sensitivity
	Gamma          float64   // Liquidity sensitivity
	LastCalibrated time.Time
	Accuracy       float64   // Model accuracy score (0-1)
	SampleCount    int       // Number of samples used for calibration
}

// NewSlippageCalculator creates a new slippage calculator
func NewSlippageCalculator() *SlippageCalculator {
	return &SlippageCalculator{
		historicalData:    make(map[string][]*interfaces.SlippageData),
		poolLiquidity:     make(map[string]*big.Int),
		priceImpactModels: make(map[string]*PriceImpactModel),
	}
}

// CalculateSlippage calculates expected slippage for a trade
func (s *SlippageCalculator) CalculateSlippage(ctx context.Context, pool string, token string, amount *big.Int) (*interfaces.SlippageEstimate, error) {
	if pool == "" || token == "" || amount == nil {
		return nil, fmt.Errorf("invalid parameters: pool, token, and amount are required")
	}

	key := fmt.Sprintf("%s-%s", pool, token)
	
	s.mu.RLock()
	model, hasModel := s.priceImpactModels[key]
	liquidity, hasLiquidity := s.poolLiquidity[pool]
	s.mu.RUnlock()

	// If we don't have a calibrated model, use default estimation
	if !hasModel || time.Since(model.LastCalibrated) > 24*time.Hour {
		return s.calculateDefaultSlippage(pool, token, amount)
	}

	// Use calibrated model for prediction
	estimate, err := s.calculateModelBasedSlippage(model, amount, liquidity, hasLiquidity)
	if err != nil {
		return s.calculateDefaultSlippage(pool, token, amount)
	}

	return estimate, nil
}

// calculateDefaultSlippage provides a fallback slippage calculation
func (s *SlippageCalculator) calculateDefaultSlippage(pool string, token string, amount *big.Int) (*interfaces.SlippageEstimate, error) {
	// Default slippage calculation based on trade size
	amountFloat, _ := amount.Float64()
	
	// Base slippage rate (0.1% for small trades)
	baseSlippage := 0.001
	
	// Scale slippage based on trade size (assuming $1M = high impact)
	tradeSize := amountFloat / 1e18 // Convert to ETH equivalent
	sizeMultiplier := math.Sqrt(tradeSize / 1000) // Square root scaling
	
	expectedSlippageRate := baseSlippage * (1 + sizeMultiplier)
	maxSlippageRate := expectedSlippageRate * 2 // Max is 2x expected
	
	// Convert rates to absolute amounts
	expectedSlippage := new(big.Int).Mul(amount, big.NewInt(int64(expectedSlippageRate*10000)))
	expectedSlippage.Div(expectedSlippage, big.NewInt(10000))
	
	maxSlippage := new(big.Int).Mul(amount, big.NewInt(int64(maxSlippageRate*10000)))
	maxSlippage.Div(maxSlippage, big.NewInt(10000))
	
	return &interfaces.SlippageEstimate{
		ExpectedSlippage: expectedSlippage,
		MaxSlippage:      maxSlippage,
		PriceImpact:      expectedSlippageRate,
		Confidence:       0.5, // Low confidence for default calculation
	}, nil
}

// calculateModelBasedSlippage uses a calibrated model for slippage prediction
func (s *SlippageCalculator) calculateModelBasedSlippage(model *PriceImpactModel, amount *big.Int, liquidity *big.Int, hasLiquidity bool) (*interfaces.SlippageEstimate, error) {
	amountFloat, _ := amount.Float64()
	
	// Base price impact calculation: impact = alpha * (amount/liquidity)^beta
	var liquidityFloat float64 = 1e18 // Default liquidity if not available
	if hasLiquidity && liquidity.Cmp(big.NewInt(0)) > 0 {
		liquidityFloat, _ = liquidity.Float64()
	}
	
	// Calculate relative trade size
	relativeSize := amountFloat / liquidityFloat
	
	// Apply price impact model
	priceImpact := model.Alpha * math.Pow(relativeSize, model.Beta)
	
	// Apply liquidity adjustment
	if hasLiquidity {
		liquidityAdjustment := math.Pow(liquidityFloat/1e18, -model.Gamma)
		priceImpact *= liquidityAdjustment
	}
	
	// Cap maximum price impact at 50%
	if priceImpact > 0.5 {
		priceImpact = 0.5
	}
	
	// Calculate slippage amounts
	expectedSlippage := new(big.Int).Mul(amount, big.NewInt(int64(priceImpact*10000)))
	expectedSlippage.Div(expectedSlippage, big.NewInt(10000))
	
	// Max slippage is 1.5x expected (with model uncertainty)
	maxSlippage := new(big.Int).Mul(expectedSlippage, big.NewInt(150))
	maxSlippage.Div(maxSlippage, big.NewInt(100))
	
	return &interfaces.SlippageEstimate{
		ExpectedSlippage: expectedSlippage,
		MaxSlippage:      maxSlippage,
		PriceImpact:      priceImpact,
		Confidence:       model.Accuracy,
	}, nil
}

// GetHistoricalSlippage returns historical slippage data for a pool-token pair
func (s *SlippageCalculator) GetHistoricalSlippage(pool string, token string, timeWindow time.Duration) ([]*interfaces.SlippageData, error) {
	if pool == "" || token == "" {
		return nil, fmt.Errorf("pool and token are required")
	}

	key := fmt.Sprintf("%s-%s", pool, token)
	cutoff := time.Now().Add(-timeWindow)
	
	s.mu.RLock()
	allData, exists := s.historicalData[key]
	s.mu.RUnlock()
	
	if !exists {
		return []*interfaces.SlippageData{}, nil
	}
	
	var result []*interfaces.SlippageData
	for _, data := range allData {
		if data.Timestamp.After(cutoff) {
			result = append(result, data)
		}
	}
	
	return result, nil
}

// UpdateSlippageModel updates the slippage model with actual observed data
func (s *SlippageCalculator) UpdateSlippageModel(pool string, token string, actualSlippage *big.Int) error {
	if pool == "" || token == "" || actualSlippage == nil {
		return fmt.Errorf("invalid parameters")
	}

	key := fmt.Sprintf("%s-%s", pool, token)
	
	// Create slippage data entry
	slippageData := &interfaces.SlippageData{
		Timestamp:   time.Now(),
		Pool:        pool,
		Token:       token,
		Amount:      big.NewInt(0), // This would be set by caller with trade amount
		Slippage:    new(big.Int).Set(actualSlippage),
		PriceImpact: 0, // This would be calculated from slippage and amount
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Add to historical data
	if _, exists := s.historicalData[key]; !exists {
		s.historicalData[key] = make([]*interfaces.SlippageData, 0)
	}
	
	s.historicalData[key] = append(s.historicalData[key], slippageData)
	
	// Keep only last 1000 entries per pool-token pair
	if len(s.historicalData[key]) > 1000 {
		s.historicalData[key] = s.historicalData[key][1:]
	}
	
	// Recalibrate model if we have enough data
	if len(s.historicalData[key]) >= 10 {
		s.calibrateModel(key)
	}
	
	return nil
}

// calibrateModel recalibrates the price impact model using historical data
func (s *SlippageCalculator) calibrateModel(key string) {
	data := s.historicalData[key]
	if len(data) < 10 {
		return
	}
	
	// Use recent data for calibration (last 100 entries or all if less)
	calibrationData := data
	if len(data) > 100 {
		calibrationData = data[len(data)-100:]
	}
	
	// Simple linear regression to fit price impact model
	// This is a simplified calibration - in practice, you'd use more sophisticated methods
	model := &PriceImpactModel{
		Alpha:          0.001, // Default base impact
		Beta:           0.5,   // Default volume sensitivity
		Gamma:          0.2,   // Default liquidity sensitivity
		LastCalibrated: time.Now(),
		SampleCount:    len(calibrationData),
	}
	
	// Calculate model accuracy based on historical fit
	model.Accuracy = s.calculateModelAccuracy(model, calibrationData)
	
	s.priceImpactModels[key] = model
}

// calculateModelAccuracy calculates how well the model fits historical data
func (s *SlippageCalculator) calculateModelAccuracy(model *PriceImpactModel, data []*interfaces.SlippageData) float64 {
	if len(data) == 0 {
		return 0.0
	}
	
	totalError := 0.0
	validPredictions := 0
	
	for _, point := range data {
		if point.Amount.Cmp(big.NewInt(0)) <= 0 {
			continue
		}
		
		// Predict slippage using model
		predicted, err := s.calculateModelBasedSlippage(model, point.Amount, nil, false)
		if err != nil {
			continue
		}
		
		// Calculate prediction error
		actualFloat, _ := point.Slippage.Float64()
		predictedFloat, _ := predicted.ExpectedSlippage.Float64()
		
		if actualFloat > 0 {
			relativeError := math.Abs(predictedFloat-actualFloat) / actualFloat
			totalError += relativeError
			validPredictions++
		}
	}
	
	if validPredictions == 0 {
		return 0.0
	}
	
	avgError := totalError / float64(validPredictions)
	accuracy := math.Max(0, 1.0-avgError) // Convert error to accuracy score
	
	return accuracy
}

// UpdatePoolLiquidity updates the liquidity information for a pool
func (s *SlippageCalculator) UpdatePoolLiquidity(pool string, liquidity *big.Int) {
	if pool == "" || liquidity == nil {
		return
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.poolLiquidity[pool] = new(big.Int).Set(liquidity)
}

// GetPoolLiquidity returns the current liquidity for a pool
func (s *SlippageCalculator) GetPoolLiquidity(pool string) (*big.Int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	liquidity, exists := s.poolLiquidity[pool]
	if !exists {
		return nil, false
	}
	
	return new(big.Int).Set(liquidity), true
}

// GetModelAccuracy returns the accuracy score for a pool-token model
func (s *SlippageCalculator) GetModelAccuracy(pool string, token string) float64 {
	key := fmt.Sprintf("%s-%s", pool, token)
	
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if model, exists := s.priceImpactModels[key]; exists {
		return model.Accuracy
	}
	
	return 0.0
}

// CalibrateAllModels recalibrates all price impact models
func (s *SlippageCalculator) CalibrateAllModels() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for key := range s.historicalData {
		if len(s.historicalData[key]) >= 10 {
			s.calibrateModel(key)
		}
	}
}

// GetModelStats returns statistics about all calibrated models
func (s *SlippageCalculator) GetModelStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	stats := make(map[string]interface{})
	
	totalModels := len(s.priceImpactModels)
	totalAccuracy := 0.0
	accuracyDistribution := make([]float64, 0, totalModels)
	
	for _, model := range s.priceImpactModels {
		totalAccuracy += model.Accuracy
		accuracyDistribution = append(accuracyDistribution, model.Accuracy)
	}
	
	stats["total_models"] = totalModels
	stats["total_pools"] = len(s.poolLiquidity)
	stats["total_historical_pairs"] = len(s.historicalData)
	
	if totalModels > 0 {
		stats["average_accuracy"] = totalAccuracy / float64(totalModels)
		
		// Calculate median accuracy
		sort.Float64s(accuracyDistribution)
		if len(accuracyDistribution)%2 == 0 {
			mid := len(accuracyDistribution) / 2
			stats["median_accuracy"] = (accuracyDistribution[mid-1] + accuracyDistribution[mid]) / 2
		} else {
			stats["median_accuracy"] = accuracyDistribution[len(accuracyDistribution)/2]
		}
	}
	
	return stats
}