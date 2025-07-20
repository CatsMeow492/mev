-- Historical Transaction Replay System Migration

-- Create historical_transaction_logs table
CREATE TABLE IF NOT EXISTS historical_transaction_logs (
    id VARCHAR(66) PRIMARY KEY,
    opportunity_id VARCHAR(66) NOT NULL,
    strategy VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    block_number BIGINT NOT NULL DEFAULT 0,
    
    -- Original opportunity data
    target_transaction JSONB,
    execution_txs JSONB,
    expected_profit DECIMAL(78, 0) NOT NULL,
    estimated_gas_cost DECIMAL(78, 0) NOT NULL DEFAULT 0,
    estimated_slippage DECIMAL(78, 0) NOT NULL DEFAULT 0,
    confidence DECIMAL(5, 4) NOT NULL DEFAULT 0,
    
    -- Simulation and market data
    original_sim_results JSONB,
    market_conditions JSONB,
    
    -- Actual execution results
    actual_trade_result JSONB,
    executed_at TIMESTAMP WITH TIME ZONE,
    
    -- Metadata and archival
    metadata JSONB,
    archived BOOLEAN DEFAULT FALSE,
    
    -- Timestamps
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_historical_logs_strategy ON historical_transaction_logs(strategy);
CREATE INDEX IF NOT EXISTS idx_historical_logs_created_at ON historical_transaction_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_historical_logs_opportunity_id ON historical_transaction_logs(opportunity_id);
CREATE INDEX IF NOT EXISTS idx_historical_logs_block_number ON historical_transaction_logs(block_number);
CREATE INDEX IF NOT EXISTS idx_historical_logs_executed_at ON historical_transaction_logs(executed_at);
CREATE INDEX IF NOT EXISTS idx_historical_logs_archived ON historical_transaction_logs(archived);

-- Composite indexes for common queries
CREATE INDEX IF NOT EXISTS idx_historical_logs_strategy_created_at ON historical_transaction_logs(strategy, created_at);
CREATE INDEX IF NOT EXISTS idx_historical_logs_strategy_archived ON historical_transaction_logs(strategy, archived);

-- Create regression_test_results table for storing test outcomes
CREATE TABLE IF NOT EXISTS regression_test_results (
    id VARCHAR(66) PRIMARY KEY,
    test_id VARCHAR(100) NOT NULL,
    started_at TIMESTAMP WITH TIME ZONE NOT NULL,
    completed_at TIMESTAMP WITH TIME ZONE,
    
    -- Test configuration
    config JSONB NOT NULL,
    
    -- Overall results
    total_tests INTEGER NOT NULL DEFAULT 0,
    passed_tests INTEGER NOT NULL DEFAULT 0,
    failed_tests INTEGER NOT NULL DEFAULT 0,
    overall_success BOOLEAN DEFAULT FALSE,
    
    -- Performance metrics
    average_replay_latency INTERVAL,
    total_replay_time INTERVAL,
    accuracy_stats JSONB,
    
    -- Strategy-specific results
    strategy_results JSONB,
    
    -- Issues and recommendations
    critical_issues JSONB,
    warnings JSONB,
    recommendations JSONB,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for regression test results
CREATE INDEX IF NOT EXISTS idx_regression_test_results_test_id ON regression_test_results(test_id);
CREATE INDEX IF NOT EXISTS idx_regression_test_results_started_at ON regression_test_results(started_at);
CREATE INDEX IF NOT EXISTS idx_regression_test_results_overall_success ON regression_test_results(overall_success);

-- Create strategy_validation_results table for storing validation outcomes
CREATE TABLE IF NOT EXISTS strategy_validation_results (
    id VARCHAR(66) PRIMARY KEY,
    strategy VARCHAR(50) NOT NULL,
    validation_period INTERVAL NOT NULL,
    validated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Validation metrics
    total_opportunities INTEGER NOT NULL DEFAULT 0,
    successful_replays INTEGER NOT NULL DEFAULT 0,
    failed_replays INTEGER NOT NULL DEFAULT 0,
    
    -- Accuracy metrics
    average_profit_accuracy DECIMAL(5, 4) DEFAULT 0,
    average_gas_cost_accuracy DECIMAL(5, 4) DEFAULT 0,
    average_slippage_accuracy DECIMAL(5, 4) DEFAULT 0,
    overall_accuracy DECIMAL(5, 4) DEFAULT 0,
    
    -- Trends and status
    profitability_trend VARCHAR(20),
    accuracy_trend VARCHAR(20),
    strategy_status VARCHAR(20),
    
    -- Recommendations
    threshold_adjustments JSONB,
    model_recalibration BOOLEAN DEFAULT FALSE,
    
    -- Detailed data
    accuracy_distribution JSONB,
    time_series_data JSONB,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for strategy validation results
CREATE INDEX IF NOT EXISTS idx_strategy_validation_strategy ON strategy_validation_results(strategy);
CREATE INDEX IF NOT EXISTS idx_strategy_validation_validated_at ON strategy_validation_results(validated_at);
CREATE INDEX IF NOT EXISTS idx_strategy_validation_status ON strategy_validation_results(strategy_status);

-- Create performance_reports table for storing comprehensive performance analysis
CREATE TABLE IF NOT EXISTS performance_reports (
    id VARCHAR(66) PRIMARY KEY,
    strategy VARCHAR(50) NOT NULL,
    report_period INTERVAL NOT NULL,
    generated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Summary metrics
    total_opportunities INTEGER NOT NULL DEFAULT 0,
    executed_opportunities INTEGER NOT NULL DEFAULT 0,
    profitable_executions INTEGER NOT NULL DEFAULT 0,
    
    -- Financial metrics
    total_expected_profit DECIMAL(78, 0) DEFAULT 0,
    total_actual_profit DECIMAL(78, 0) DEFAULT 0,
    total_losses DECIMAL(78, 0) DEFAULT 0,
    net_profitability DECIMAL(78, 0) DEFAULT 0,
    roi DECIMAL(10, 6) DEFAULT 0,
    
    -- Analysis results
    prediction_accuracy JSONB,
    model_performance JSONB,
    trends JSONB,
    insights JSONB,
    action_items JSONB,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for performance reports
CREATE INDEX IF NOT EXISTS idx_performance_reports_strategy ON performance_reports(strategy);
CREATE INDEX IF NOT EXISTS idx_performance_reports_generated_at ON performance_reports(generated_at);

-- Create replay_accuracy_metrics table for storing accuracy measurements
CREATE TABLE IF NOT EXISTS replay_accuracy_metrics (
    id VARCHAR(66) PRIMARY KEY,
    log_id VARCHAR(66) NOT NULL REFERENCES historical_transaction_logs(id),
    replay_id VARCHAR(66) NOT NULL,
    measured_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Accuracy metrics
    profit_accuracy DECIMAL(5, 4) DEFAULT 0,
    gas_cost_accuracy DECIMAL(5, 4) DEFAULT 0,
    slippage_accuracy DECIMAL(5, 4) DEFAULT 0,
    timing_accuracy DECIMAL(5, 4) DEFAULT 0,
    overall_score DECIMAL(5, 4) DEFAULT 0,
    confidence_interval DECIMAL(5, 4) DEFAULT 0,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for replay accuracy metrics
CREATE INDEX IF NOT EXISTS idx_replay_accuracy_log_id ON replay_accuracy_metrics(log_id);
CREATE INDEX IF NOT EXISTS idx_replay_accuracy_replay_id ON replay_accuracy_metrics(replay_id);
CREATE INDEX IF NOT EXISTS idx_replay_accuracy_measured_at ON replay_accuracy_metrics(measured_at);

-- Add triggers to update timestamps
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger for historical_transaction_logs
DROP TRIGGER IF EXISTS update_historical_logs_updated_at ON historical_transaction_logs;
CREATE TRIGGER update_historical_logs_updated_at
    BEFORE UPDATE ON historical_transaction_logs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Add constraints for data integrity
ALTER TABLE historical_transaction_logs 
ADD CONSTRAINT chk_confidence_range 
CHECK (confidence >= 0 AND confidence <= 1);

ALTER TABLE strategy_validation_results 
ADD CONSTRAINT chk_accuracy_ranges 
CHECK (
    average_profit_accuracy >= 0 AND average_profit_accuracy <= 1 AND
    average_gas_cost_accuracy >= 0 AND average_gas_cost_accuracy <= 1 AND
    average_slippage_accuracy >= 0 AND average_slippage_accuracy <= 1 AND
    overall_accuracy >= 0 AND overall_accuracy <= 1
);

-- Add comments for documentation
COMMENT ON TABLE historical_transaction_logs IS 'Stores historical MEV opportunity data for replay testing and validation';
COMMENT ON TABLE regression_test_results IS 'Stores results from regression test runs';
COMMENT ON TABLE strategy_validation_results IS 'Stores strategy performance validation results';
COMMENT ON TABLE performance_reports IS 'Stores comprehensive performance analysis reports';
COMMENT ON TABLE replay_accuracy_metrics IS 'Stores detailed accuracy metrics from replay validations'; 