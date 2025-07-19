package queue

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTransactionFilter(t *testing.T) {
	filter := NewTransactionFilter()
	require.NotNil(t, filter)
	
	criteria := filter.GetFilterCriteria()
	assert.Equal(t, big.NewInt(1000000000), criteria.MinGasPrice)
	assert.Equal(t, big.NewInt(500000000000), criteria.MaxGasPrice)
	assert.Equal(t, big.NewInt(0), criteria.MinValue)
	assert.Empty(t, criteria.ContractFilters)
	assert.Empty(t, criteria.MethodFilters)
	assert.Empty(t, criteria.ExcludeAddresses)
}

func TestNewTransactionFilterWithCriteria(t *testing.T) {
	criteria := interfaces.FilterCriteria{
		MinGasPrice:      big.NewInt(2000000000),
		MaxGasPrice:      big.NewInt(100000000000),
		MinValue:         big.NewInt(1000000000000000000),
		ContractFilters:  []string{"0x1234567890123456789012345678901234567890"},
		MethodFilters:    []string{"7ff36ab5"},
		ExcludeAddresses: []string{"0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"},
	}
	
	filter := NewTransactionFilterWithCriteria(criteria)
	require.NotNil(t, filter)
	
	retrievedCriteria := filter.GetFilterCriteria()
	assert.Equal(t, criteria.MinGasPrice, retrievedCriteria.MinGasPrice)
	assert.Equal(t, criteria.MaxGasPrice, retrievedCriteria.MaxGasPrice)
	assert.Equal(t, criteria.MinValue, retrievedCriteria.MinValue)
	assert.Equal(t, criteria.ContractFilters, retrievedCriteria.ContractFilters)
	assert.Equal(t, criteria.MethodFilters, retrievedCriteria.MethodFilters)
	assert.Equal(t, criteria.ExcludeAddresses, retrievedCriteria.ExcludeAddresses)
}

func TestTransactionFilter_ShouldProcess_GasPrice(t *testing.T) {
	filter := NewTransactionFilter()
	
	tests := []struct {
		name     string
		gasPrice *big.Int
		expected bool
	}{
		{
			name:     "gas price too low",
			gasPrice: big.NewInt(500000000), // 0.5 gwei
			expected: false,
		},
		{
			name:     "gas price at minimum",
			gasPrice: big.NewInt(1000000000), // 1 gwei
			expected: true,
		},
		{
			name:     "gas price normal",
			gasPrice: big.NewInt(20000000000), // 20 gwei
			expected: true,
		},
		{
			name:     "gas price at maximum",
			gasPrice: big.NewInt(500000000000), // 500 gwei
			expected: true,
		},
		{
			name:     "gas price too high",
			gasPrice: big.NewInt(600000000000), // 600 gwei
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := createTestTransaction("0x123", tt.gasPrice.Int64(), 1, time.Now())
			tx.GasPrice = tt.gasPrice
			
			result := filter.ShouldProcess(tx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTransactionFilter_ShouldProcess_Value(t *testing.T) {
	criteria := interfaces.FilterCriteria{
		MinGasPrice: big.NewInt(1000000000),
		MaxGasPrice: big.NewInt(500000000000),
		MinValue:    big.NewInt(1000000000000000000), // 1 ETH minimum
	}
	filter := NewTransactionFilterWithCriteria(criteria)
	
	tests := []struct {
		name     string
		value    *big.Int
		expected bool
	}{
		{
			name:     "value too low",
			value:    big.NewInt(500000000000000000), // 0.5 ETH
			expected: false,
		},
		{
			name:     "value at minimum",
			value:    big.NewInt(1000000000000000000), // 1 ETH
			expected: true,
		},
		{
			name:     "value above minimum",
			value:    big.NewInt(5000000000000000000), // 5 ETH
			expected: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := createTestTransaction("0x123", 20000000000, 1, time.Now())
			tx.Value = tt.value
			
			result := filter.ShouldProcess(tx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTransactionFilter_ShouldProcess_ExcludeAddresses(t *testing.T) {
	excludeAddr := "0x1234567890123456789012345678901234567890"
	criteria := interfaces.FilterCriteria{
		MinGasPrice:      big.NewInt(1000000000),
		MaxGasPrice:      big.NewInt(500000000000),
		MinValue:         big.NewInt(0),
		ExcludeAddresses: []string{excludeAddr},
	}
	filter := NewTransactionFilterWithCriteria(criteria)
	
	tests := []struct {
		name     string
		from     string
		to       *string
		expected bool
	}{
		{
			name:     "from address excluded",
			from:     excludeAddr,
			to:       stringPtr("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"),
			expected: false,
		},
		{
			name:     "to address excluded",
			from:     "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
			to:       &excludeAddr,
			expected: false,
		},
		{
			name:     "neither address excluded",
			from:     "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
			to:       stringPtr("0xfedcbafedcbafedcbafedcbafedcbafedcbafedcba"),
			expected: true,
		},
		{
			name:     "case insensitive exclusion",
			from:     "0x1234567890123456789012345678901234567890", // lowercase
			to:       stringPtr("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"),
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := createTestTransaction("0x123", 20000000000, 1, time.Now())
			tx.From = common.HexToAddress(tt.from)
			if tt.to != nil {
				addr := common.HexToAddress(*tt.to)
				tx.To = &addr
			} else {
				tx.To = nil
			}
			
			result := filter.ShouldProcess(tx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTransactionFilter_ShouldProcess_ContractFilters(t *testing.T) {
	contractAddr := "0x1234567890123456789012345678901234567890"
	criteria := interfaces.FilterCriteria{
		MinGasPrice:     big.NewInt(1000000000),
		MaxGasPrice:     big.NewInt(500000000000),
		MinValue:        big.NewInt(0),
		ContractFilters: []string{contractAddr},
	}
	filter := NewTransactionFilterWithCriteria(criteria)
	
	tests := []struct {
		name     string
		to       *string
		expected bool
	}{
		{
			name:     "contract address matches",
			to:       &contractAddr,
			expected: true,
		},
		{
			name:     "contract address doesn't match",
			to:       stringPtr("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"),
			expected: false,
		},
		{
			name:     "contract creation (no to address)",
			to:       nil,
			expected: false,
		},
		{
			name:     "case insensitive match",
			to:       stringPtr("0x1234567890123456789012345678901234567890"), // lowercase
			expected: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := createTestTransaction("0x123", 20000000000, 1, time.Now())
			if tt.to != nil {
				addr := common.HexToAddress(*tt.to)
				tx.To = &addr
			} else {
				tx.To = nil
			}
			
			result := filter.ShouldProcess(tx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTransactionFilter_ShouldProcess_MethodFilters(t *testing.T) {
	swapMethodSig := "7ff36ab5" // swapExactETHForTokens
	criteria := interfaces.FilterCriteria{
		MinGasPrice:   big.NewInt(1000000000),
		MaxGasPrice:   big.NewInt(500000000000),
		MinValue:      big.NewInt(0),
		MethodFilters: []string{swapMethodSig},
	}
	filter := NewTransactionFilterWithCriteria(criteria)
	
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{
			name:     "method signature matches",
			data:     common.Hex2Bytes("7ff36ab5000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000010000000000000000000000001234567890123456789012345678901234567890"),
			expected: true,
		},
		{
			name:     "method signature doesn't match",
			data:     common.Hex2Bytes("18cbafe5000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000010000000000000000000000001234567890123456789012345678901234567890"),
			expected: false,
		},
		{
			name:     "no data (transfer)",
			data:     []byte{},
			expected: false,
		},
		{
			name:     "data too short",
			data:     []byte{0x7f, 0xf3},
			expected: false,
		},
		{
			name:     "case insensitive match",
			data:     common.Hex2Bytes("7FF36AB5000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000010000000000000000000000001234567890123456789012345678901234567890"),
			expected: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := createTestTransaction("0x123", 20000000000, 1, time.Now())
			tx.Data = tt.data
			
			result := filter.ShouldProcess(tx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTransactionFilter_UpdateCriteria(t *testing.T) {
	filter := NewTransactionFilter()
	
	newCriteria := interfaces.FilterCriteria{
		MinGasPrice:      big.NewInt(5000000000),
		MaxGasPrice:      big.NewInt(200000000000),
		MinValue:         big.NewInt(500000000000000000),
		ContractFilters:  []string{"0x1111111111111111111111111111111111111111"},
		MethodFilters:    []string{"38ed1739"},
		ExcludeAddresses: []string{"0x2222222222222222222222222222222222222222"},
	}
	
	err := filter.UpdateCriteria(newCriteria)
	require.NoError(t, err)
	
	retrievedCriteria := filter.GetFilterCriteria()
	assert.Equal(t, newCriteria.MinGasPrice, retrievedCriteria.MinGasPrice)
	assert.Equal(t, newCriteria.MaxGasPrice, retrievedCriteria.MaxGasPrice)
	assert.Equal(t, newCriteria.MinValue, retrievedCriteria.MinValue)
	assert.Equal(t, newCriteria.ContractFilters, retrievedCriteria.ContractFilters)
	assert.Equal(t, newCriteria.MethodFilters, retrievedCriteria.MethodFilters)
	assert.Equal(t, newCriteria.ExcludeAddresses, retrievedCriteria.ExcludeAddresses)
}

func TestTransactionFilter_IsRelevantForMEV(t *testing.T) {
	filter := NewTransactionFilter()
	
	tests := []struct {
		name     string
		txType   types.TransactionType
		value    *big.Int
		expected bool
	}{
		{
			name:     "swap transaction",
			txType:   types.TxTypeSwap,
			value:    big.NewInt(100000000000000000), // 0.1 ETH
			expected: true,
		},
		{
			name:     "liquidity transaction",
			txType:   types.TxTypeLiquidity,
			value:    big.NewInt(100000000000000000), // 0.1 ETH
			expected: true,
		},
		{
			name:     "bridge transaction",
			txType:   types.TxTypeBridge,
			value:    big.NewInt(100000000000000000), // 0.1 ETH
			expected: true,
		},
		{
			name:     "high value transfer",
			txType:   types.TxTypeTransfer,
			value:    big.NewInt(2000000000000000000), // 2 ETH
			expected: true,
		},
		{
			name:     "low value transfer",
			txType:   types.TxTypeTransfer,
			value:    big.NewInt(100000000000000000), // 0.1 ETH
			expected: false,
		},
		{
			name:     "contract interaction with data",
			txType:   types.TxTypeContract,
			value:    big.NewInt(0),
			expected: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := createTestTransaction("0x123", 20000000000, 1, time.Now())
			tx.Value = tt.value
			
			// Set transaction data based on type to ensure GetTransactionType returns expected type
			switch tt.txType {
			case types.TxTypeSwap:
				tx.Data = common.Hex2Bytes("7ff36ab5000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000010000000000000000000000001234567890123456789012345678901234567890")
			case types.TxTypeLiquidity:
				tx.Data = common.Hex2Bytes("e8e33700000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000010000000000000000000000001234567890123456789012345678901234567890")
			case types.TxTypeBridge:
				// For this test, we'll mock bridge detection in the transaction type
				tx.Data = []byte{0x01, 0x02, 0x03, 0x04} // Some data to make it contract type
			case types.TxTypeTransfer:
				tx.Data = []byte{} // No data for transfer
			case types.TxTypeContract:
				tx.Data = []byte{0x01, 0x02, 0x03, 0x04} // Some data
			}
			
			result := filter.(*TransactionFilterImpl).IsRelevantForMEV(tx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}