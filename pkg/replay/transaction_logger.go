package replay

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// TransactionLoggerImpl implements the TransactionLogger interface
type TransactionLoggerImpl struct {
	db *sql.DB
}

// NewTransactionLogger creates a new transaction logger
func NewTransactionLogger(db *sql.DB) interfaces.TransactionLogger {
	return &TransactionLoggerImpl{
		db: db,
	}
}

// LogOpportunity logs an MEV opportunity to the database
func (tl *TransactionLoggerImpl) LogOpportunity(ctx context.Context, opportunity *interfaces.MEVOpportunity, tradeResult *interfaces.TradeResult) error {
	if opportunity == nil {
		return fmt.Errorf("opportunity cannot be nil")
	}

	// Serialize metadata
	metadataJSON, err := json.Marshal(opportunity.Metadata)
	if err != nil {
		return fmt.Errorf("failed to serialize metadata: %w", err)
	}

	// Serialize execution transactions
	execTxsJSON, err := json.Marshal(opportunity.ExecutionTxs)
	if err != nil {
		return fmt.Errorf("failed to serialize execution transactions: %w", err)
	}

	// Insert into database
	query := `
		INSERT INTO historical_transaction_logs (
			id, opportunity_id, strategy, created_at, block_number,
			target_transaction, execution_txs, expected_profit, estimated_gas_cost,
			estimated_slippage, confidence, metadata, actual_trade_result, executed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	var executedAt *time.Time
	var tradeResultJSON []byte
	if tradeResult != nil {
		executedAt = &tradeResult.ExecutedAt
		tradeResultJSON, err = json.Marshal(tradeResult)
		if err != nil {
			return fmt.Errorf("failed to serialize trade result: %w", err)
		}
	}

	_, err = tl.db.ExecContext(ctx, query,
		generateLogID(),
		opportunity.ID,
		string(opportunity.Strategy),
		time.Now(),
		0,  // Block number would need to be extracted from context
		"", // Target transaction JSON would be serialized here
		execTxsJSON,
		opportunity.ExpectedProfit.String(),
		opportunity.GasCost.String(),
		"0", // Estimated slippage placeholder
		opportunity.Confidence,
		metadataJSON,
		tradeResultJSON,
		executedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert historical log: %w", err)
	}

	return nil
}

// GetLogsByStrategy retrieves logs for a specific strategy
func (tl *TransactionLoggerImpl) GetLogsByStrategy(ctx context.Context, strategy interfaces.StrategyType, limit int) ([]*interfaces.HistoricalTransactionLog, error) {
	query := `
		SELECT id, opportunity_id, strategy, created_at, block_number,
			   expected_profit, estimated_gas_cost, estimated_slippage, confidence,
			   metadata, executed_at
		FROM historical_transaction_logs 
		WHERE strategy = $1 AND archived = false
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := tl.db.QueryContext(ctx, query, string(strategy), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs by strategy: %w", err)
	}
	defer rows.Close()

	var logs []*interfaces.HistoricalTransactionLog
	for rows.Next() {
		log, err := tl.scanHistoricalLog(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// GetLogsByTimeRange retrieves logs within a time range
func (tl *TransactionLoggerImpl) GetLogsByTimeRange(ctx context.Context, start, end time.Time) ([]*interfaces.HistoricalTransactionLog, error) {
	query := `
		SELECT id, opportunity_id, strategy, created_at, block_number,
			   expected_profit, estimated_gas_cost, estimated_slippage, confidence,
			   metadata, executed_at
		FROM historical_transaction_logs 
		WHERE created_at >= $1 AND created_at <= $2 AND archived = false
		ORDER BY created_at DESC
	`

	rows, err := tl.db.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs by time range: %w", err)
	}
	defer rows.Close()

	var logs []*interfaces.HistoricalTransactionLog
	for rows.Next() {
		log, err := tl.scanHistoricalLog(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// UpdateLogWithActualResult updates a log with actual execution results
func (tl *TransactionLoggerImpl) UpdateLogWithActualResult(ctx context.Context, logID string, actualResult *interfaces.TradeResult) error {
	if actualResult == nil {
		return fmt.Errorf("actual result cannot be nil")
	}

	tradeResultJSON, err := json.Marshal(actualResult)
	if err != nil {
		return fmt.Errorf("failed to serialize trade result: %w", err)
	}

	query := `
		UPDATE historical_transaction_logs 
		SET actual_trade_result = $1, executed_at = $2
		WHERE id = $3
	`

	_, err = tl.db.ExecContext(ctx, query, tradeResultJSON, actualResult.ExecutedAt, logID)
	if err != nil {
		return fmt.Errorf("failed to update log with actual result: %w", err)
	}

	return nil
}

// scanHistoricalLog scans a database row into a HistoricalTransactionLog
func (tl *TransactionLoggerImpl) scanHistoricalLog(rows *sql.Rows) (*interfaces.HistoricalTransactionLog, error) {
	var log interfaces.HistoricalTransactionLog
	var strategyStr string
	var expectedProfitStr, gasCostStr, slippageStr string
	var metadataJSON []byte
	var executedAt *time.Time

	err := rows.Scan(
		&log.ID,
		&log.OpportunityID,
		&strategyStr,
		&log.CreatedAt,
		&log.BlockNumber,
		&expectedProfitStr,
		&gasCostStr,
		&slippageStr,
		&log.Confidence,
		&metadataJSON,
		&executedAt,
	)
	if err != nil {
		return nil, err
	}

	// Parse strategy type
	log.Strategy = interfaces.StrategyType(strategyStr)

	// Parse big integers
	log.ExpectedProfit, _ = parseBigInt(expectedProfitStr)
	log.EstimatedGasCost, _ = parseBigInt(gasCostStr)
	log.EstimatedSlippage, _ = parseBigInt(slippageStr)

	// Parse metadata
	if len(metadataJSON) > 0 {
		err = json.Unmarshal(metadataJSON, &log.Metadata)
		if err != nil {
			log.Metadata = make(map[string]interface{})
		}
	}

	log.ExecutedAt = executedAt

	return &log, nil
}

// Helper function to parse big.Int from string
func parseBigInt(s string) (*big.Int, error) {
	result := new(big.Int)
	_, ok := result.SetString(s, 10)
	if !ok {
		return big.NewInt(0), fmt.Errorf("invalid big int string: %s", s)
	}
	return result, nil
}
