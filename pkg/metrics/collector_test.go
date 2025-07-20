package metrics

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestCollector creates a collector for testing with a custom registry
func newTestCollector(config *CollectorConfig) *Collector {
	registry := prometheus.NewRegistry()
	return NewCollectorWithRegistry(config, registry)
}

func TestNewCollector(t *testing.T) {
	tests := []struct {
		name   string
		config *CollectorConfig
		want   *CollectorConfig
	}{
		{
			name:   "default config",
			config: nil,
			want: &CollectorConfig{
				MaxTrades:        10000,
				MaxLatencies:     10000,
				MaxOpportunities: 10000,
				WindowSizes:      []int{50, 100, 500},
			},
		},
		{
			name: "custom config",
			config: &CollectorConfig{
				MaxTrades:        5000,
				MaxLatencies:     5000,
				MaxOpportunities: 5000,
				WindowSizes:      []int{25, 50, 100},
			},
			want: &CollectorConfig{
				MaxTrades:        5000,
				MaxLatencies:     5000,
				MaxOpportunities: 5000,
				WindowSizes:      []int{25, 50, 100},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use custom registry for tests to avoid conflicts
			registry := prometheus.NewRegistry()
			collector := NewCollectorWithRegistry(tt.config, registry)
			
			assert.NotNil(t, collector)
			assert.Equal(t, tt.want.MaxTrades, collector.config.MaxTrades)
			assert.Equal(t, tt.want.MaxLatencies, collector.config.MaxLatencies)
			assert.Equal(t, tt.want.MaxOpportunities, collector.config.MaxOpportunities)
			assert.Equal(t, tt.want.WindowSizes, collector.config.WindowSizes)
			assert.NotNil(t, collector.prometheusMetrics)
		})
	}
}

func TestCollector_RecordTrade(t *testing.T) {
	collector := newTestCollector(nil)
	ctx := context.Background()

	// Test recording a profitable trade
	profitableTrade := &interfaces.TradeResult{
		ID:              "trade-1",
		Strategy:        interfaces.StrategySandwich,
		OpportunityID:   "opp-1",
		ExecutedAt:      time.Now(),
		Success:         true,
		ActualProfit:    big.NewInt(1000000),
		ExpectedProfit:  big.NewInt(900000),
		GasCost:         big.NewInt(100000),
		NetProfit:       big.NewInt(900000),
		ExecutionTime:   50 * time.Millisecond,
		TransactionHash: "0x123",
	}

	err := collector.RecordTrade(ctx, profitableTrade)
	require.NoError(t, err)

	assert.Len(t, collector.trades, 1)
	assert.Equal(t, profitableTrade, collector.trades[0])

	// Test recording a losing trade
	losingTrade := &interfaces.TradeResult{
		ID:              "trade-2",
		Strategy:        interfaces.StrategyBackrun,
		OpportunityID:   "opp-2",
		ExecutedAt:      time.Now(),
		Success:         false,
		ActualProfit:    big.NewInt(-200000),
		ExpectedProfit:  big.NewInt(100000),
		GasCost:         big.NewInt(150000),
		NetProfit:       big.NewInt(-200000),
		ExecutionTime:   75 * time.Millisecond,
		ErrorMessage:    "execution failed",
	}

	err = collector.RecordTrade(ctx, losingTrade)
	require.NoError(t, err)

	assert.Len(t, collector.trades, 2)
	assert.Equal(t, losingTrade, collector.trades[1])
}

func TestCollector_RecordLatency(t *testing.T) {
	collector := newTestCollector(nil)
	ctx := context.Background()

	// Record latencies for different operations
	operations := []string{"simulation", "strategy_detection", "profit_calculation"}
	durations := []time.Duration{50 * time.Millisecond, 25 * time.Millisecond, 10 * time.Millisecond}

	for i, op := range operations {
		err := collector.RecordLatency(ctx, op, durations[i])
		require.NoError(t, err)
		
		assert.Contains(t, collector.latencies, op)
		assert.Len(t, collector.latencies[op], 1)
		assert.Equal(t, durations[i], collector.latencies[op][0].Duration)
	}

	// Record multiple latencies for the same operation
	for i := 0; i < 5; i++ {
		err := collector.RecordLatency(ctx, "simulation", time.Duration(i+1)*10*time.Millisecond)
		require.NoError(t, err)
	}

	assert.Len(t, collector.latencies["simulation"], 6) // 1 initial + 5 additional
}

func TestCollector_RecordOpportunity(t *testing.T) {
	collector := newTestCollector(nil)
	ctx := context.Background()

	opportunity := &interfaces.MEVOpportunity{
		ID:             "opp-1",
		Strategy:       interfaces.StrategySandwich,
		TargetTx:       "0x456",
		ExpectedProfit: big.NewInt(500000),
		GasCost:        big.NewInt(100000),
		NetProfit:      big.NewInt(400000),
		Confidence:     0.85,
		Status:         interfaces.StatusDetected,
		CreatedAt:      time.Now(),
	}

	err := collector.RecordOpportunity(ctx, opportunity)
	require.NoError(t, err)

	assert.Len(t, collector.opportunities, 1)
	assert.Equal(t, opportunity, collector.opportunities[0])
}

func TestCollector_GetTradeSuccessRate(t *testing.T) {
	collector := newTestCollector(nil)
	ctx := context.Background()

	// Test with no trades
	rate, err := collector.GetTradeSuccessRate(50)
	require.NoError(t, err)
	assert.Equal(t, 0.0, rate)

	// Add some trades
	trades := []*interfaces.TradeResult{
		{Success: true, NetProfit: big.NewInt(100000)},   // profitable
		{Success: true, NetProfit: big.NewInt(200000)},   // profitable
		{Success: false, NetProfit: big.NewInt(-50000)},  // loss
		{Success: true, NetProfit: big.NewInt(-10000)},   // unprofitable success
		{Success: true, NetProfit: big.NewInt(150000)},   // profitable
	}

	for _, trade := range trades {
		err := collector.RecordTrade(ctx, trade)
		require.NoError(t, err)
	}

	// Test success rate calculation
	rate, err = collector.GetTradeSuccessRate(5)
	require.NoError(t, err)
	assert.Equal(t, 0.6, rate) // 3 profitable out of 5 total

	// Test with window size larger than available trades
	rate, err = collector.GetTradeSuccessRate(10)
	require.NoError(t, err)
	assert.Equal(t, 0.6, rate) // Should use all available trades
}

func TestCollector_GetProfitabilityMetrics(t *testing.T) {
	collector := newTestCollector(nil)
	ctx := context.Background()

	// Test with no trades
	metrics, err := collector.GetProfitabilityMetrics(50)
	require.NoError(t, err)
	assert.Equal(t, 50, metrics.WindowSize)
	assert.Equal(t, 0, metrics.TotalTrades)

	// Add trades with known profits
	trades := []*interfaces.TradeResult{
		{Success: true, NetProfit: big.NewInt(100000)},   // +100k
		{Success: true, NetProfit: big.NewInt(200000)},   // +200k
		{Success: false, NetProfit: big.NewInt(-50000)},  // -50k
		{Success: true, NetProfit: big.NewInt(-10000)},   // -10k (unprofitable success)
		{Success: true, NetProfit: big.NewInt(150000)},   // +150k
	}

	for _, trade := range trades {
		err := collector.RecordTrade(ctx, trade)
		require.NoError(t, err)
	}

	metrics, err = collector.GetProfitabilityMetrics(5)
	require.NoError(t, err)

	assert.Equal(t, 5, metrics.TotalTrades)
	assert.Equal(t, 3, metrics.ProfitableTrades) // 3 trades with positive profit
	assert.Equal(t, 2, metrics.LossTrades)       // 2 trades with loss/no profit
	assert.Equal(t, 0.6, metrics.SuccessRate)
	assert.Equal(t, 0.4, metrics.LossRate)

	// Check total calculations
	expectedNetProfit := big.NewInt(390000) // 100k + 200k - 50k - 10k + 150k
	assert.Equal(t, expectedNetProfit, metrics.NetProfit)

	expectedTotalProfit := big.NewInt(450000) // 100k + 200k + 150k
	assert.Equal(t, expectedTotalProfit, metrics.TotalProfit)

	expectedTotalLoss := big.NewInt(60000) // 50k + 10k
	assert.Equal(t, expectedTotalLoss, metrics.TotalLoss)

	// Check max values
	assert.Equal(t, big.NewInt(200000), metrics.MaxProfit)
	assert.Equal(t, big.NewInt(50000), metrics.MaxLoss)

	// Check average profit
	expectedAverage := big.NewInt(78000) // 390k / 5
	assert.Equal(t, expectedAverage, metrics.AverageProfit)

	// Check median profit (sorted: -50k, -10k, 100k, 150k, 200k -> median = 100k)
	assert.Equal(t, big.NewInt(100000), metrics.MedianProfit)
}

func TestCollector_GetLatencyMetrics(t *testing.T) {
	collector := newTestCollector(nil)
	ctx := context.Background()

	operation := "simulation"

	// Test with no latencies
	metrics, err := collector.GetLatencyMetrics(operation, 50)
	require.NoError(t, err)
	assert.Equal(t, operation, metrics.Operation)
	assert.Equal(t, 50, metrics.WindowSize)
	assert.Equal(t, 0, metrics.SampleCount)

	// Add latency records
	latencies := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
	}

	for _, latency := range latencies {
		err := collector.RecordLatency(ctx, operation, latency)
		require.NoError(t, err)
	}

	metrics, err = collector.GetLatencyMetrics(operation, 5)
	require.NoError(t, err)

	assert.Equal(t, 5, metrics.SampleCount)
	assert.Equal(t, 30*time.Millisecond, metrics.AverageLatency) // (10+20+30+40+50)/5 = 30
	assert.Equal(t, 30*time.Millisecond, metrics.MedianLatency)  // middle value
	assert.Equal(t, 10*time.Millisecond, metrics.MinLatency)
	assert.Equal(t, 50*time.Millisecond, metrics.MaxLatency)

	// P95 should be 50ms (95% of 5 = 4.75, rounded to index 4)
	assert.Equal(t, 50*time.Millisecond, metrics.P95Latency)
	// P99 should be 50ms (99% of 5 = 4.95, rounded to index 4)
	assert.Equal(t, 50*time.Millisecond, metrics.P99Latency)
}

func TestCollector_RollingWindowCalculations(t *testing.T) {
	config := &CollectorConfig{
		MaxTrades:        1000,
		MaxLatencies:     1000,
		MaxOpportunities: 1000,
		WindowSizes:      []int{10, 20, 50},
	}
	collector := newTestCollector(config)
	ctx := context.Background()

	// Add 100 trades with varying profitability
	for i := 0; i < 100; i++ {
		var profit *big.Int
		success := true

		if i%3 == 0 {
			profit = big.NewInt(-10000) // loss
			success = false
		} else {
			profit = big.NewInt(int64((i + 1) * 1000)) // varying profits
		}

		trade := &interfaces.TradeResult{
			ID:        fmt.Sprintf("trade-%d", i),
			Strategy:  interfaces.StrategySandwich,
			Success:   success,
			NetProfit: profit,
		}

		err := collector.RecordTrade(ctx, trade)
		require.NoError(t, err)
	}

	// Test different window sizes
	for _, windowSize := range config.WindowSizes {
		t.Run(fmt.Sprintf("window_%d", windowSize), func(t *testing.T) {
			successRate, err := collector.GetTradeSuccessRate(windowSize)
			require.NoError(t, err)
			
			// Should be approximately 2/3 success rate (every 3rd trade is a loss)
			expectedRate := 2.0 / 3.0
			assert.InDelta(t, expectedRate, successRate, 0.1)

			profitMetrics, err := collector.GetProfitabilityMetrics(windowSize)
			require.NoError(t, err)
			
			assert.Equal(t, windowSize, profitMetrics.WindowSize)
			assert.Equal(t, windowSize, profitMetrics.TotalTrades)
			
			// Verify success and loss counts
			expectedProfitable := (windowSize * 2) / 3
			expectedLosses := windowSize / 3
			
			assert.InDelta(t, expectedProfitable, profitMetrics.ProfitableTrades, 1)
			assert.InDelta(t, expectedLosses, profitMetrics.LossTrades, 1)
		})
	}
}

func TestCollector_CircularBuffer(t *testing.T) {
	// Test with small max size to verify circular buffer behavior
	config := &CollectorConfig{
		MaxTrades:        5,
		MaxLatencies:     3,
		MaxOpportunities: 4,
		WindowSizes:      []int{5},
	}
	collector := newTestCollector(config)
	ctx := context.Background()

	// Add more trades than max size
	for i := 0; i < 10; i++ {
		trade := &interfaces.TradeResult{
			ID:        fmt.Sprintf("trade-%d", i),
			Success:   true,
			NetProfit: big.NewInt(int64(i * 1000)),
		}
		err := collector.RecordTrade(ctx, trade)
		require.NoError(t, err)
	}

	// Should only keep the last 5 trades
	assert.Len(t, collector.trades, 5)
	assert.Equal(t, "trade-5", collector.trades[0].ID)
	assert.Equal(t, "trade-9", collector.trades[4].ID)

	// Test latency circular buffer
	for i := 0; i < 6; i++ {
		err := collector.RecordLatency(ctx, "test", time.Duration(i)*time.Millisecond)
		require.NoError(t, err)
	}

	// Should only keep the last 3 latency records
	assert.Len(t, collector.latencies["test"], 3)
	assert.Equal(t, 3*time.Millisecond, collector.latencies["test"][0].Duration)
	assert.Equal(t, 5*time.Millisecond, collector.latencies["test"][2].Duration)
}

func TestCollector_GetPerformanceMetrics(t *testing.T) {
	collector := newTestCollector(nil)
	ctx := context.Background()

	// Add some test data
	for i := 0; i < 150; i++ {
		success := i%4 != 0 // 75% success rate
		profit := big.NewInt(int64(i * 1000))
		if !success {
			profit = big.NewInt(-10000)
		}

		trade := &interfaces.TradeResult{
			ID:        fmt.Sprintf("trade-%d", i),
			Success:   success,
			NetProfit: profit,
		}
		err := collector.RecordTrade(ctx, trade)
		require.NoError(t, err)
	}

	// Add some opportunities
	for i := 0; i < 10; i++ {
		opp := &interfaces.MEVOpportunity{
			ID:       fmt.Sprintf("opp-%d", i),
			Strategy: interfaces.StrategySandwich,
		}
		err := collector.RecordOpportunity(ctx, opp)
		require.NoError(t, err)
	}

	metrics, err := collector.GetPerformanceMetrics()
	require.NoError(t, err)

	assert.NotNil(t, metrics.TradeMetrics)
	assert.NotNil(t, metrics.LatencyMetrics)
	assert.Equal(t, uint64(150), metrics.TransactionsProcessed)
	assert.Equal(t, uint64(10), metrics.OpportunitiesDetected)

	// Check that trade metrics are calculated for each window size
	for _, windowSize := range collector.config.WindowSizes {
		assert.Contains(t, metrics.TradeMetrics, windowSize)
		tradeMetrics := metrics.TradeMetrics[windowSize]
		assert.Equal(t, windowSize, tradeMetrics.WindowSize)
	}

	// With 150 trades and 75% success rate, should not be in warning mode
	assert.True(t, metrics.IsHealthy)
	assert.False(t, metrics.WarningMode)
	assert.False(t, metrics.ShutdownPending)
}

func TestCollector_ShutdownConditions(t *testing.T) {
	collector := newTestCollector(nil)
	ctx := context.Background()

	// Add 100 trades with high loss rate (90% losses)
	for i := 0; i < 100; i++ {
		success := i%10 == 0 // Only 10% success rate
		profit := big.NewInt(-10000)
		if success {
			profit = big.NewInt(50000)
		}

		trade := &interfaces.TradeResult{
			ID:        fmt.Sprintf("trade-%d", i),
			Success:   success,
			NetProfit: profit,
		}
		err := collector.RecordTrade(ctx, trade)
		require.NoError(t, err)
	}

	metrics, err := collector.GetPerformanceMetrics()
	require.NoError(t, err)

	// Should be in warning mode and shutdown pending due to high loss rate
	assert.False(t, metrics.IsHealthy)
	assert.True(t, metrics.WarningMode)
	assert.True(t, metrics.ShutdownPending)
}

func TestCollector_PrometheusMetrics(t *testing.T) {
	collector := newTestCollector(nil)
	ctx := context.Background()

	// Add some test data
	trades := []*interfaces.TradeResult{
		{Success: true, NetProfit: big.NewInt(100000), Strategy: interfaces.StrategySandwich},
		{Success: false, NetProfit: big.NewInt(-50000), Strategy: interfaces.StrategyBackrun},
		{Success: true, NetProfit: big.NewInt(200000), Strategy: interfaces.StrategySandwich},
	}

	for _, trade := range trades {
		err := collector.RecordTrade(ctx, trade)
		require.NoError(t, err)
	}

	opportunities := []*interfaces.MEVOpportunity{
		{Strategy: interfaces.StrategySandwich},
		{Strategy: interfaces.StrategyBackrun},
		{Strategy: interfaces.StrategySandwich},
	}

	for _, opp := range opportunities {
		err := collector.RecordOpportunity(ctx, opp)
		require.NoError(t, err)
	}

	promMetrics, err := collector.GetPrometheusMetrics()
	require.NoError(t, err)

	assert.Equal(t, 3, promMetrics["trades_total"])
	assert.Equal(t, 2, promMetrics["profitable_trades_total"])
	assert.Equal(t, float64(300000), promMetrics["total_profit_wei"]) // 100k + 200k (only profitable trades)

	// Check opportunities by strategy
	oppByStrategy := promMetrics["opportunities_by_strategy"].(map[string]int)
	assert.Equal(t, 2, oppByStrategy["sandwich"])
	assert.Equal(t, 1, oppByStrategy["backrun"])

	// Check rolling window metrics
	rollingMetrics := promMetrics["rolling_windows"].(map[string]interface{})
	assert.Contains(t, rollingMetrics, "window_50")
	assert.Contains(t, rollingMetrics, "window_100")
	assert.Contains(t, rollingMetrics, "window_500")
}