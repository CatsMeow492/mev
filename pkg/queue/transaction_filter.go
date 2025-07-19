package queue

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

// TransactionFilterImpl implements the TransactionFilter interface
type TransactionFilterImpl struct {
	criteria interfaces.FilterCriteria
}

// NewTransactionFilter creates a new transaction filter with default criteria
func NewTransactionFilter() interfaces.TransactionFilter {
	return &TransactionFilterImpl{
		criteria: interfaces.FilterCriteria{
			MinGasPrice:      big.NewInt(1000000000), // 1 gwei minimum
			MaxGasPrice:      big.NewInt(500000000000), // 500 gwei maximum
			MinValue:         big.NewInt(0),
			ContractFilters:  []string{},
			MethodFilters:    []string{},
			ExcludeAddresses: []string{},
		},
	}
}

// NewTransactionFilterWithCriteria creates a new transaction filter with custom criteria
func NewTransactionFilterWithCriteria(criteria interfaces.FilterCriteria) interfaces.TransactionFilter {
	return &TransactionFilterImpl{
		criteria: criteria,
	}
}

// ShouldProcess determines if a transaction should be processed based on filter criteria
func (tf *TransactionFilterImpl) ShouldProcess(tx *types.Transaction) bool {
	// Check gas price bounds
	if tf.criteria.MinGasPrice != nil && tx.GasPrice.Cmp(tf.criteria.MinGasPrice) < 0 {
		return false
	}
	
	if tf.criteria.MaxGasPrice != nil && tx.GasPrice.Cmp(tf.criteria.MaxGasPrice) > 0 {
		return false
	}
	
	// Check minimum value
	if tf.criteria.MinValue != nil && tx.Value.Cmp(tf.criteria.MinValue) < 0 {
		return false
	}
	
	// Check excluded addresses
	for _, excludeAddr := range tf.criteria.ExcludeAddresses {
		if strings.EqualFold(tx.From.Hex(), excludeAddr) {
			return false
		}
		if tx.To != nil && strings.EqualFold(tx.To.Hex(), excludeAddr) {
			return false
		}
	}
	
	// Check contract filters (if specified, transaction must interact with one of these contracts)
	if len(tf.criteria.ContractFilters) > 0 {
		if tx.To == nil {
			return false // Contract creation, not interaction
		}
		
		found := false
		for _, contractAddr := range tf.criteria.ContractFilters {
			if strings.EqualFold(tx.To.Hex(), contractAddr) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check method filters (if specified, transaction must call one of these methods)
	if len(tf.criteria.MethodFilters) > 0 {
		if len(tx.Data) < 4 {
			return false // Not enough data for method signature
		}
		
		methodSig := common.Bytes2Hex(tx.Data[:4])
		found := false
		for _, method := range tf.criteria.MethodFilters {
			if strings.EqualFold(methodSig, method) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	return true
}

// GetFilterCriteria returns the current filter criteria
func (tf *TransactionFilterImpl) GetFilterCriteria() interfaces.FilterCriteria {
	return tf.criteria
}

// UpdateCriteria updates the filter criteria
func (tf *TransactionFilterImpl) UpdateCriteria(criteria interfaces.FilterCriteria) error {
	tf.criteria = criteria
	return nil
}

// IsRelevantForMEV determines if a transaction is relevant for MEV strategies
func (tf *TransactionFilterImpl) IsRelevantForMEV(tx *types.Transaction) bool {
	txType := tx.GetTransactionType()
	
	// MEV-relevant transaction types
	switch txType {
	case types.TxTypeSwap, types.TxTypeLiquidity, types.TxTypeBridge:
		return true
	case types.TxTypeTransfer:
		// High-value transfers might be relevant for frontrunning
		threshold := big.NewInt(1000000000000000000) // 1 ETH
		return tx.IsHighValue(threshold)
	case types.TxTypeContract:
		// Contract interactions might be relevant depending on the contract
		return len(tx.Data) > 0
	default:
		return false
	}
}