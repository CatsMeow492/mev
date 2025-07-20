package metrics

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Collector implements the MetricsCollector interface
type Collector struct {
	mu sync.RWMutex
	
	// Trade data storage
	trades       []*interfaces.TradeResult
	maxTrades    int
	
	// Latency data storage
	latencies    map[string][]LatencyRecord
	maxLatencies int
	
	// Opportunities data
	opportunities []*interfaces.MEVOpportunity
	maxOpportunities int
	
	// Prometheus metrics
	prometheusMetrics *PrometheusMetrics
	
	// Configuration
	config *CollectorConfig
}

// LatencyRecord stores latency measurements
type LatencyRecord struct {
	Timestamp time.Time
	Duration  time.Duration
}

// CollectorConfig contains configuration for the metrics collector
type CollectorConfig struct {
	MaxTrades        int
	MaxLatencies     int
	MaxOpportunities int
	WindowSizes      []int
}

// PrometheusMetrics contains all Prometheus metric collectors
type PrometheusMetrics struct {
	// Trade metrics
	tradesTotal       prometheus.Counter
	profitableTrades  prometheus.Counter
	totalProfit       prometheus.Gauge
	tradeLatency      prometheus.Histogram
	
	// Strategy metrics
	opportunitiesDetected *prometheus.CounterVec
	opportunitiesExecuted *prometheus.CounterVec
	strategyProfit        *prometheus.GaugeVec
	
	// System metrics
	processingLatency     *prometheus.HistogramVec
	queueSize            prometheus.Gauge
	connectionStatus     prometheus.Gauge
	
	// Performance metrics
	successRate          *prometheus.GaugeVec
	lossRate            *prometheus.GaugeVec
	averageProfit       *prometheus.GaugeVec
}

// NewCollector creates a new metrics collector
func NewCollector(config *CollectorConfig) *Collector {
	if config == nil {
		config = &CollectorConfig{
			MaxTrades:        10000,
			MaxLatencies:     10000,
			MaxOpportunities: 10000,
			WindowSizes:      []int{50, 100, 500},
		}
	}
	
	collector := &Collector{
		trades:           make([]*interfaces.TradeResult, 0, config.MaxTrades),
		latencies:        make(map[string][]LatencyRecord),
		opportunities:    make([]*interfaces.MEVOpportunity, 0, config.MaxOpportunities),
		maxTrades:        config.MaxTrades,
		maxLatencies:     config.MaxLatencies,
		maxOpportunities: config.MaxOpportunities,
		config:           config,
	}
	
	collector.initPrometheusMetrics()
	return collector
}

// NewCollectorWithRegistry creates a new metrics collector with a custom Prometheus registry
func NewCollectorWithRegistry(config *CollectorConfig, registry *prometheus.Registry) *Collector {
	if config == nil {
		config = &CollectorConfig{
			MaxTrades:        10000,
			MaxLatencies:     10000,
			MaxOpportunities: 10000,
			WindowSizes:      []int{50, 100, 500},
		}
	}
	
	collector := &Collector{
		trades:           make([]*interfaces.TradeResult, 0, config.MaxTrades),
		latencies:        make(map[string][]LatencyRecord),
		opportunities:    make([]*interfaces.MEVOpportunity, 0, config.MaxOpportunities),
		maxTrades:        config.MaxTrades,
		maxLatencies:     config.MaxLatencies,
		maxOpportunities: config.MaxOpportunities,
		config:           config,
	}
	
	collector.initPrometheusMetricsWithRegistry(registry)
	return collector
}

// initPrometheusMetrics initializes Prometheus metrics
func (c *Collector) initPrometheusMetrics() {
	c.prometheusMetrics = &PrometheusMetrics{
		tradesTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "mev_trades_total",
			Help: "Total number of trades executed",
		}),
		profitableTrades: promauto.NewCounter(prometheus.CounterOpts{
			Name: "mev_profitable_trades_total",
			Help: "Total number of profitable trades",
		}),
		totalProfit: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "mev_total_profit_wei",
			Help: "Total profit in wei",
		}),
		tradeLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "mev_trade_execution_duration_seconds",
			Help:    "Trade execution duration in seconds",
			Buckets: prometheus.DefBuckets,
		}),
		opportunitiesDetected: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "mev_opportunities_detected_total",
			Help: "Total number of MEV opportunities detected by strategy",
		}, []string{"strategy"}),
		opportunitiesExecuted: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "mev_opportunities_executed_total",
			Help: "Total number of MEV opportunities executed by strategy",
		}, []string{"strategy"}),
		strategyProfit: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mev_strategy_profit_wei",
			Help: "Total profit by strategy in wei",
		}, []string{"strategy"}),
		processingLatency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "mev_processing_duration_seconds",
			Help:    "Processing duration by operation in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"operation"}),
		queueSize: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "mev_queue_size",
			Help: "Current transaction queue size",
		}),
		connectionStatus: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "mev_connection_status",
			Help: "Connection status (1 = connected, 0 = disconnected)",
		}),
		successRate: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mev_success_rate",
			Help: "Trade success rate by window size",
		}, []string{"window_size"}),
		lossRate: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mev_loss_rate",
			Help: "Trade loss rate by window size",
		}, []string{"window_size"}),
		averageProfit: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mev_average_profit_wei",
			Help: "Average profit by window size in wei",
		}, []string{"window_size"}),
	}
}

// initPrometheusMetricsWithRegistry initializes Prometheus metrics with a custom registry
func (c *Collector) initPrometheusMetricsWithRegistry(registry *prometheus.Registry) {
	factory := promauto.With(registry)
	
	c.prometheusMetrics = &PrometheusMetrics{
		tradesTotal: factory.NewCounter(prometheus.CounterOpts{
			Name: "mev_trades_total",
			Help: "Total number of trades executed",
		}),
		profitableTrades: factory.NewCounter(prometheus.CounterOpts{
			Name: "mev_profitable_trades_total",
			Help: "Total number of profitable trades",
		}),
		totalProfit: factory.NewGauge(prometheus.GaugeOpts{
			Name: "mev_total_profit_wei",
			Help: "Total profit in wei",
		}),
		tradeLatency: factory.NewHistogram(prometheus.HistogramOpts{
			Name:    "mev_trade_execution_duration_seconds",
			Help:    "Trade execution duration in seconds",
			Buckets: prometheus.DefBuckets,
		}),
		opportunitiesDetected: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "mev_opportunities_detected_total",
			Help: "Total number of MEV opportunities detected by strategy",
		}, []string{"strategy"}),
		opportunitiesExecuted: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "mev_opportunities_executed_total",
			Help: "Total number of MEV opportunities executed by strategy",
		}, []string{"strategy"}),
		strategyProfit: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mev_strategy_profit_wei",
			Help: "Total profit by strategy in wei",
		}, []string{"strategy"}),
		processingLatency: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "mev_processing_duration_seconds",
			Help:    "Processing duration by operation in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"operation"}),
		queueSize: factory.NewGauge(prometheus.GaugeOpts{
			Name: "mev_queue_size",
			Help: "Current transaction queue size",
		}),
		connectionStatus: factory.NewGauge(prometheus.GaugeOpts{
			Name: "mev_connection_status",
			Help: "Connection status (1 = connected, 0 = disconnected)",
		}),
		successRate: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mev_success_rate",
			Help: "Trade success rate by window size",
		}, []string{"window_size"}),
		lossRate: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mev_loss_rate",
			Help: "Trade loss rate by window size",
		}, []string{"window_size"}),
		averageProfit: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mev_average_profit_wei",
			Help: "Average profit by window size in wei",
		}, []string{"window_size"}),
	}
}

// RecordTrade records a trade result
func (c *Collector) RecordTrade(ctx context.Context, trade *interfaces.TradeResult) error {
	c.mu.Lock()
	
	// Add to trades slice
	c.trades = append(c.trades, trade)
	
	// Maintain max size using circular buffer approach
	if len(c.trades) > c.maxTrades {
		c.trades = c.trades[1:]
	}
	
	// Update Prometheus metrics
	c.prometheusMetrics.tradesTotal.Inc()
	if trade.Success && trade.NetProfit.Cmp(big.NewInt(0)) > 0 {
		c.prometheusMetrics.profitableTrades.Inc()
	}
	
	c.prometheusMetrics.tradeLatency.Observe(trade.ExecutionTime.Seconds())
	c.prometheusMetrics.opportunitiesExecuted.WithLabelValues(string(trade.Strategy)).Inc()
	
	c.mu.Unlock()
	
	// Update rolling window metrics (without holding the lock)
	c.updateRollingMetrics()
	
	return nil
}

// RecordLatency records operation latency
func (c *Collector) RecordLatency(ctx context.Context, operation string, duration time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Initialize slice if not exists
	if c.latencies[operation] == nil {
		c.latencies[operation] = make([]LatencyRecord, 0, c.maxLatencies)
	}
	
	// Add latency record
	record := LatencyRecord{
		Timestamp: time.Now(),
		Duration:  duration,
	}
	
	c.latencies[operation] = append(c.latencies[operation], record)
	
	// Maintain max size
	if len(c.latencies[operation]) > c.maxLatencies {
		c.latencies[operation] = c.latencies[operation][1:]
	}
	
	// Update Prometheus metrics
	c.prometheusMetrics.processingLatency.WithLabelValues(operation).Observe(duration.Seconds())
	
	return nil
}

// RecordOpportunity records a detected MEV opportunity
func (c *Collector) RecordOpportunity(ctx context.Context, opportunity *interfaces.MEVOpportunity) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Add to opportunities slice
	c.opportunities = append(c.opportunities, opportunity)
	
	// Maintain max size
	if len(c.opportunities) > c.maxOpportunities {
		c.opportunities = c.opportunities[1:]
	}
	
	// Update Prometheus metrics
	c.prometheusMetrics.opportunitiesDetected.WithLabelValues(string(opportunity.Strategy)).Inc()
	
	return nil
}

// GetTradeSuccessRate calculates trade success rate for a given window size
func (c *Collector) GetTradeSuccessRate(windowSize int) (float64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if len(c.trades) == 0 {
		return 0.0, nil
	}
	
	// Get the last N trades
	start := len(c.trades) - windowSize
	if start < 0 {
		start = 0
	}
	
	windowTrades := c.trades[start:]
	if len(windowTrades) == 0 {
		return 0.0, nil
	}
	
	successfulTrades := 0
	for _, trade := range windowTrades {
		if trade.Success && trade.NetProfit.Cmp(big.NewInt(0)) > 0 {
			successfulTrades++
		}
	}
	
	return float64(successfulTrades) / float64(len(windowTrades)), nil
}

// GetProfitabilityMetrics calculates profitability metrics for a given window size
func (c *Collector) GetProfitabilityMetrics(windowSize int) (*interfaces.ProfitabilityMetrics, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if len(c.trades) == 0 {
		return &interfaces.ProfitabilityMetrics{
			WindowSize:  windowSize,
			LastUpdated: time.Now(),
		}, nil
	}
	
	// Get the last N trades
	start := len(c.trades) - windowSize
	if start < 0 {
		start = 0
	}
	
	windowTrades := c.trades[start:]
	if len(windowTrades) == 0 {
		return &interfaces.ProfitabilityMetrics{
			WindowSize:  windowSize,
			LastUpdated: time.Now(),
		}, nil
	}
	
	metrics := &interfaces.ProfitabilityMetrics{
		WindowSize:    windowSize,
		TotalTrades:   len(windowTrades),
		TotalProfit:   big.NewInt(0),
		TotalLoss:     big.NewInt(0),
		NetProfit:     big.NewInt(0),
		MaxProfit:     big.NewInt(0),
		MaxLoss:       big.NewInt(0),
		LastUpdated:   time.Now(),
	}
	
	profits := make([]*big.Int, 0, len(windowTrades))
	
	for _, trade := range windowTrades {
		if trade.Success && trade.NetProfit.Cmp(big.NewInt(0)) > 0 {
			metrics.ProfitableTrades++
			metrics.TotalProfit.Add(metrics.TotalProfit, trade.NetProfit)
			
			if trade.NetProfit.Cmp(metrics.MaxProfit) > 0 {
				metrics.MaxProfit.Set(trade.NetProfit)
			}
		} else {
			metrics.LossTrades++
			loss := new(big.Int).Abs(trade.NetProfit)
			metrics.TotalLoss.Add(metrics.TotalLoss, loss)
			
			if loss.Cmp(metrics.MaxLoss) > 0 {
				metrics.MaxLoss.Set(loss)
			}
		}
		
		profits = append(profits, new(big.Int).Set(trade.NetProfit))
		metrics.NetProfit.Add(metrics.NetProfit, trade.NetProfit)
	}
	
	// Calculate rates
	if metrics.TotalTrades > 0 {
		metrics.SuccessRate = float64(metrics.ProfitableTrades) / float64(metrics.TotalTrades)
		metrics.LossRate = float64(metrics.LossTrades) / float64(metrics.TotalTrades)
	}
	
	// Calculate average profit
	if metrics.TotalTrades > 0 {
		metrics.AverageProfit = new(big.Int).Div(metrics.NetProfit, big.NewInt(int64(metrics.TotalTrades)))
	} else {
		metrics.AverageProfit = big.NewInt(0)
	}
	
	// Calculate median profit
	if len(profits) > 0 {
		sort.Slice(profits, func(i, j int) bool {
			return profits[i].Cmp(profits[j]) < 0
		})
		
		mid := len(profits) / 2
		if len(profits)%2 == 0 {
			// Even number of elements, average the two middle values
			sum := new(big.Int).Add(profits[mid-1], profits[mid])
			metrics.MedianProfit = new(big.Int).Div(sum, big.NewInt(2))
		} else {
			// Odd number of elements, take the middle value
			metrics.MedianProfit = new(big.Int).Set(profits[mid])
		}
	} else {
		metrics.MedianProfit = big.NewInt(0)
	}
	
	// Calculate profit margin
	if metrics.TotalProfit.Cmp(big.NewInt(0)) > 0 && metrics.TotalLoss.Cmp(big.NewInt(0)) > 0 {
		totalProfitFloat, _ := metrics.TotalProfit.Float64()
		totalLossFloat, _ := metrics.TotalLoss.Float64()
		metrics.ProfitMargin = (totalProfitFloat - totalLossFloat) / totalProfitFloat
	}
	
	return metrics, nil
}

// GetLatencyMetrics calculates latency metrics for a given operation and window size
func (c *Collector) GetLatencyMetrics(operation string, windowSize int) (*interfaces.LatencyMetrics, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	latencies, exists := c.latencies[operation]
	if !exists || len(latencies) == 0 {
		return &interfaces.LatencyMetrics{
			Operation:   operation,
			WindowSize:  windowSize,
			LastUpdated: time.Now(),
		}, nil
	}
	
	// Get the last N latency records
	start := len(latencies) - windowSize
	if start < 0 {
		start = 0
	}
	
	windowLatencies := latencies[start:]
	if len(windowLatencies) == 0 {
		return &interfaces.LatencyMetrics{
			Operation:   operation,
			WindowSize:  windowSize,
			LastUpdated: time.Now(),
		}, nil
	}
	
	// Extract durations and sort for percentile calculations
	durations := make([]time.Duration, len(windowLatencies))
	var totalDuration time.Duration
	minDuration := windowLatencies[0].Duration
	maxDuration := windowLatencies[0].Duration
	
	for i, record := range windowLatencies {
		durations[i] = record.Duration
		totalDuration += record.Duration
		
		if record.Duration < minDuration {
			minDuration = record.Duration
		}
		if record.Duration > maxDuration {
			maxDuration = record.Duration
		}
	}
	
	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})
	
	metrics := &interfaces.LatencyMetrics{
		Operation:      operation,
		WindowSize:     windowSize,
		SampleCount:    len(durations),
		AverageLatency: totalDuration / time.Duration(len(durations)),
		MinLatency:     minDuration,
		MaxLatency:     maxDuration,
		LastUpdated:    time.Now(),
	}
	
	// Calculate percentiles
	if len(durations) > 0 {
		medianIdx := len(durations) / 2
		if len(durations)%2 == 0 {
			metrics.MedianLatency = (durations[medianIdx-1] + durations[medianIdx]) / 2
		} else {
			metrics.MedianLatency = durations[medianIdx]
		}
		
		p95Idx := int(float64(len(durations)) * 0.95)
		if p95Idx >= len(durations) {
			p95Idx = len(durations) - 1
		}
		metrics.P95Latency = durations[p95Idx]
		
		p99Idx := int(float64(len(durations)) * 0.99)
		if p99Idx >= len(durations) {
			p99Idx = len(durations) - 1
		}
		metrics.P99Latency = durations[p99Idx]
	}
	
	return metrics, nil
}

// updateRollingMetrics updates Prometheus rolling window metrics
func (c *Collector) updateRollingMetrics() {
	for _, windowSize := range c.config.WindowSizes {
		windowSizeStr := fmt.Sprintf("%d", windowSize)
		
		// Update success rate
		successRate, _ := c.GetTradeSuccessRate(windowSize)
		c.prometheusMetrics.successRate.WithLabelValues(windowSizeStr).Set(successRate)
		
		// Update loss rate
		lossRate := 1.0 - successRate
		c.prometheusMetrics.lossRate.WithLabelValues(windowSizeStr).Set(lossRate)
		
		// Update average profit
		profitMetrics, _ := c.GetProfitabilityMetrics(windowSize)
		if profitMetrics.AverageProfit != nil {
			avgProfitFloat, _ := profitMetrics.AverageProfit.Float64()
			c.prometheusMetrics.averageProfit.WithLabelValues(windowSizeStr).Set(avgProfitFloat)
		}
	}
}

// GetPerformanceMetrics returns overall system performance metrics
func (c *Collector) GetPerformanceMetrics() (*interfaces.PerformanceMetrics, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	metrics := &interfaces.PerformanceMetrics{
		TradeMetrics:          make(map[int]*interfaces.ProfitabilityMetrics),
		LatencyMetrics:        make(map[string]*interfaces.LatencyMetrics),
		TransactionsProcessed: uint64(len(c.trades)),
		OpportunitiesDetected: uint64(len(c.opportunities)),
		LastUpdated:           time.Now(),
	}
	
	// Calculate trade metrics for each window size
	for _, windowSize := range c.config.WindowSizes {
		profitMetrics, err := c.GetProfitabilityMetrics(windowSize)
		if err != nil {
			return nil, fmt.Errorf("failed to get profitability metrics for window %d: %w", windowSize, err)
		}
		metrics.TradeMetrics[windowSize] = profitMetrics
	}
	
	// Calculate latency metrics for each operation
	for operation := range c.latencies {
		latencyMetrics, err := c.GetLatencyMetrics(operation, c.config.WindowSizes[0]) // Use first window size as default
		if err != nil {
			return nil, fmt.Errorf("failed to get latency metrics for operation %s: %w", operation, err)
		}
		metrics.LatencyMetrics[operation] = latencyMetrics
	}
	
	// Count executed opportunities
	executedCount := uint64(0)
	for _, trade := range c.trades {
		if trade.Success {
			executedCount++
		}
	}
	metrics.OpportunitiesExecuted = executedCount
	
	// Determine system health
	metrics.IsHealthy = true
	metrics.WarningMode = false
	metrics.ShutdownPending = false
	
	// Check if we should be in warning mode (loss rate > 70% over last 100 trades)
	if len(c.trades) >= 100 {
		lossRate100, _ := c.calculateLossRate(100)
		if lossRate100 > 0.70 {
			metrics.WarningMode = true
			metrics.IsHealthy = false
		}
		
		// Check if we should shutdown (loss rate > 80% over last 50 trades)
		if len(c.trades) >= 50 {
			lossRate50, _ := c.calculateLossRate(50)
			if lossRate50 > 0.80 {
				metrics.ShutdownPending = true
				metrics.IsHealthy = false
			}
		}
	}
	
	return metrics, nil
}

// GetSystemMetrics returns system resource metrics
func (c *Collector) GetSystemMetrics() (*interfaces.SystemMetrics, error) {
	// This would typically integrate with system monitoring libraries
	// For now, return basic metrics structure
	return &interfaces.SystemMetrics{
		CPUUsage:       0.0, // Would be populated by system monitoring
		MemoryUsage:    0.0, // Would be populated by system monitoring
		GoroutineCount: 0,   // Would be populated by runtime.NumGoroutine()
		HeapSize:       0,   // Would be populated by runtime.MemStats
		GCPauseTime:    0,   // Would be populated by runtime.MemStats
		NetworkLatency: 0,   // Would be populated by network monitoring
		LastUpdated:    time.Now(),
	}, nil
}

// GetPrometheusMetrics returns metrics in Prometheus format
func (c *Collector) GetPrometheusMetrics() (map[string]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	metrics := make(map[string]interface{})
	
	// Trade metrics
	metrics["trades_total"] = len(c.trades)
	
	profitableTrades := 0
	totalProfit := big.NewInt(0)
	
	for _, trade := range c.trades {
		if trade.Success && trade.NetProfit.Cmp(big.NewInt(0)) > 0 {
			profitableTrades++
			totalProfit.Add(totalProfit, trade.NetProfit)
		}
	}
	
	metrics["profitable_trades_total"] = profitableTrades
	totalProfitFloat, _ := totalProfit.Float64()
	metrics["total_profit_wei"] = totalProfitFloat
	
	// Opportunities metrics
	opportunitiesByStrategy := make(map[string]int)
	for _, opp := range c.opportunities {
		opportunitiesByStrategy[string(opp.Strategy)]++
	}
	metrics["opportunities_by_strategy"] = opportunitiesByStrategy
	
	// Rolling window metrics
	rollingMetrics := make(map[string]interface{})
	for _, windowSize := range c.config.WindowSizes {
		windowKey := fmt.Sprintf("window_%d", windowSize)
		
		successRate, _ := c.GetTradeSuccessRate(windowSize)
		profitMetrics, _ := c.GetProfitabilityMetrics(windowSize)
		
		rollingMetrics[windowKey] = map[string]interface{}{
			"success_rate": successRate,
			"loss_rate":    1.0 - successRate,
			"total_trades": profitMetrics.TotalTrades,
			"net_profit":   profitMetrics.NetProfit.String(),
		}
	}
	metrics["rolling_windows"] = rollingMetrics
	
	return metrics, nil
}

// RegisterPrometheusCollectors registers Prometheus metric collectors
func (c *Collector) RegisterPrometheusCollectors() error {
	// Prometheus metrics are already registered in initPrometheusMetrics
	// This method can be used for additional custom collectors if needed
	return nil
}

// calculateLossRate calculates the loss rate for a given window size
func (c *Collector) calculateLossRate(windowSize int) (float64, error) {
	if len(c.trades) == 0 {
		return 0.0, nil
	}
	
	start := len(c.trades) - windowSize
	if start < 0 {
		start = 0
	}
	
	windowTrades := c.trades[start:]
	if len(windowTrades) == 0 {
		return 0.0, nil
	}
	
	lossTrades := 0
	for _, trade := range windowTrades {
		if !trade.Success || trade.NetProfit.Cmp(big.NewInt(0)) <= 0 {
			lossTrades++
		}
	}
	
	return float64(lossTrades) / float64(len(windowTrades)), nil
}

// UpdateQueueSize updates the current queue size metric
func (c *Collector) UpdateQueueSize(size int) {
	c.prometheusMetrics.queueSize.Set(float64(size))
}

// UpdateConnectionStatus updates the connection status metric
func (c *Collector) UpdateConnectionStatus(connected bool) {
	if connected {
		c.prometheusMetrics.connectionStatus.Set(1)
	} else {
		c.prometheusMetrics.connectionStatus.Set(0)
	}
}

// UpdateTotalProfit updates the total profit metric
func (c *Collector) UpdateTotalProfit() {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	totalProfit := big.NewInt(0)
	for _, trade := range c.trades {
		if trade.NetProfit != nil {
			totalProfit.Add(totalProfit, trade.NetProfit)
		}
	}
	
	totalProfitFloat, _ := totalProfit.Float64()
	c.prometheusMetrics.totalProfit.Set(totalProfitFloat)
}