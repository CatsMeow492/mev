package types

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestTransaction_GetTransactionType(t *testing.T) {
	tests := []struct {
		name     string
		tx       *Transaction
		expected TransactionType
	}{
		{
			name: "Transfer transaction",
			tx: &Transaction{
				Hash:  "0x123",
				From:  common.HexToAddress("0x1"),
				To:    &common.Address{},
				Value: big.NewInt(1000),
				Data:  []byte{},
			},
			expected: TxTypeTransfer,
		},
		{
			name: "Swap transaction",
			tx: &Transaction{
				Hash:  "0x456",
				From:  common.HexToAddress("0x1"),
				To:    &common.Address{},
				Value: big.NewInt(0),
				Data:  common.FromHex("0x7ff36ab5"), // swapExactETHForTokens
			},
			expected: TxTypeSwap,
		},
		{
			name: "Contract transaction",
			tx: &Transaction{
				Hash:  "0x789",
				From:  common.HexToAddress("0x1"),
				To:    &common.Address{},
				Value: big.NewInt(0),
				Data:  common.FromHex("0x12345678"), // unknown method
			},
			expected: TxTypeContract,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.tx.GetTransactionType()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTransaction_IsHighValue(t *testing.T) {
	value, _ := new(big.Int).SetString("5000000000000000000", 10) // 5 ETH
	tx := &Transaction{
		Value: value,
	}

	threshold1, _ := new(big.Int).SetString("1000000000000000000", 10) // 1 ETH
	assert.True(t, tx.IsHighValue(threshold1))

	threshold2, _ := new(big.Int).SetString("10000000000000000000", 10) // 10 ETH
	assert.False(t, tx.IsHighValue(threshold2))
}

func TestTransaction_GetPriority(t *testing.T) {
	tx := &Transaction{
		GasPrice: big.NewInt(20000000000), // 20 gwei
		GasLimit: 21000,
	}

	expected := new(big.Int).Mul(big.NewInt(20000000000), big.NewInt(21000))
	result := tx.GetPriority()
	
	assert.Equal(t, expected, result)
}