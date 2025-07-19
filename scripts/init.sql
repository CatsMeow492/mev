-- Initialize MEV Engine database schema

-- Create tables for storing MEV opportunities
CREATE TABLE IF NOT EXISTS mev_opportunities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    strategy VARCHAR(50) NOT NULL,
    target_tx VARCHAR(66) NOT NULL,
    expected_profit DECIMAL(78, 0) NOT NULL,
    gas_cost DECIMAL(78, 0) NOT NULL,
    net_profit DECIMAL(78, 0) NOT NULL,
    confidence DECIMAL(5, 4) NOT NULL,
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB
);

-- Create index for efficient querying
CREATE INDEX IF NOT EXISTS idx_mev_opportunities_strategy ON mev_opportunities(strategy);
CREATE INDEX IF NOT EXISTS idx_mev_opportunities_status ON mev_opportunities(status);
CREATE INDEX IF NOT EXISTS idx_mev_opportunities_created_at ON mev_opportunities(created_at);

-- Create table for performance metrics
CREATE TABLE IF NOT EXISTS performance_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    window_size INTEGER NOT NULL,
    total_trades INTEGER NOT NULL,
    profitable_trades INTEGER NOT NULL,
    loss_rate DECIMAL(5, 4) NOT NULL,
    avg_latency_ms INTEGER NOT NULL,
    total_profit DECIMAL(78, 0) NOT NULL,
    recorded_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create index for performance metrics
CREATE INDEX IF NOT EXISTS idx_performance_metrics_recorded_at ON performance_metrics(recorded_at);

-- Create table for transaction logs
CREATE TABLE IF NOT EXISTS transaction_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tx_hash VARCHAR(66) NOT NULL,
    from_address VARCHAR(42) NOT NULL,
    to_address VARCHAR(42),
    value DECIMAL(78, 0) NOT NULL,
    gas_price DECIMAL(78, 0) NOT NULL,
    gas_limit BIGINT NOT NULL,
    nonce BIGINT NOT NULL,
    tx_data TEXT,
    block_number BIGINT,
    tx_index INTEGER,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    tx_type VARCHAR(20) NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for transaction logs
CREATE INDEX IF NOT EXISTS idx_transaction_logs_tx_hash ON transaction_logs(tx_hash);
CREATE INDEX IF NOT EXISTS idx_transaction_logs_timestamp ON transaction_logs(timestamp);
CREATE INDEX IF NOT EXISTS idx_transaction_logs_tx_type ON transaction_logs(tx_type);

-- Create table for strategy configurations
CREATE TABLE IF NOT EXISTS strategy_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    strategy_name VARCHAR(50) NOT NULL UNIQUE,
    config JSONB NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Insert default strategy configurations
INSERT INTO strategy_configs (strategy_name, config, enabled) VALUES
('sandwich', '{
    "min_swap_amount": "10000000000000000000",
    "max_slippage": 0.02,
    "gas_premium_percent": 0.1,
    "min_profit_threshold": "100000000000000000"
}', true),
('backrun', '{
    "min_price_gap": "1000000000000000000",
    "max_trade_size": "100000000000000000000",
    "min_profit_threshold": "50000000000000000",
    "supported_pools": ["uniswap_v2", "uniswap_v3", "aerodrome"]
}', true),
('frontrun', '{
    "min_tx_value": "5000000000000000000",
    "max_gas_premium": "10000000000000000000",
    "min_success_probability": 0.7,
    "min_profit_threshold": "100000000000000000"
}', true),
('time_bandit', '{
    "max_bundle_size": 5,
    "min_profit_threshold": "200000000000000000",
    "max_dependency_depth": 3
}', false)
ON CONFLICT (strategy_name) DO NOTHING;