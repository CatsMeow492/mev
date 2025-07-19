package mempool

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mevtypes "github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

func TestNewTransactionStream(t *testing.T) {
	tests := []struct {
		name     string
		config   TransactionStreamConfig
		expected *TransactionStreamImpl
	}{
		{
			name:   "default configuration",
			config: TransactionStreamConfig{},
			expected: &TransactionStreamImpl{
				minGasPrice: big.NewInt(1000000000),  // 1 Gwei
				maxGasPrice: big.NewInt(100000000000), // 100 Gwei
				minValue:    big.NewInt(0),
				methodFilters: []string{
					"7ff36ab5", "18cbafe5", "38ed1739", "e8e33700",
					"f305d719", "baa2abde", "02751cec", "ac9650d8",
				},
			},
		},
		{
			name: "custom configuration",
			config: TransactionStreamConfig{
				MinGasPrice:     big.NewInt(2000000000),
				MaxGasPrice:     big.NewInt(50000000000),
				MinValue:        big.NewInt(1000000000000000000), // 1 ETH
				ContractFilters: []string{"0x1234567890123456789012345678901234567890"},
				MethodFilters:   []string{"a9059cbb"}, // transfer
			},
			expected: &TransactionStreamImpl{
				minGasPrice:     big.NewInt(2000000000),
				maxGasPrice:     big.NewInt(50000000000),
				minValue:        big.NewInt(1000000000000000000),
				contractFilters: []string{"0x1234567890123456789012345678901234567890"},
				methodFilters:   []string{"a9059cbb"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := NewTransactionStream(tt.config).(*TransactionStreamImpl)
			
			assert.Equal(t, tt.expected.minGasPrice, stream.minGasPrice)
			assert.Equal(t, tt.expected.maxGasPrice, stream.maxGasPrice)
			assert.Equal(t, tt.expected.minValue, stream.minValue)
			assert.Equal(t, tt.expected.contractFilters, stream.contractFilters)
			assert.Equal(t, tt.expected.methodFilters, stream.methodFilters)
		})
	}
}

func TestProcessTransaction(t *testing.T) {
	stream := NewTransactionStream(TransactionStreamConfig{})

	tests := []struct {
		name        string
		rawData     []byte
		expectError bool
		validate    func(t *testing.T, tx *mevtypes.Transaction)
	}{
		{
			name: "valid swap transaction",
			rawData: createEthSubscriptionResponse(RawTransaction{
				Hash:     "0x1234567890123456789012345678901234567890123456789012345678901234",
				From:     "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
				To:       "0x1234567890123456789012345678901234567890",
				Value:    "0x0",
				GasPrice: "0x3b9aca00", // 1 Gwei
				Gas:      "0x5208",     // 21000
				Nonce:    "0x1",
				Input:    "0x7ff36ab5000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000c0",
				ChainId:  "0x2105", // Base mainnet
			}),
			expectError: false,
			validate: func(t *testing.T, tx *mevtypes.Transaction) {
				assert.Equal(t, "0x1234567890123456789012345678901234567890123456789012345678901234", tx.Hash)
				assert.Equal(t, common.HexToAddress("0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"), tx.From)
				assert.Equal(t, common.HexToAddress("0x1234567890123456789012345678901234567890"), *tx.To)
				assert.True(t, tx.Value.Cmp(big.NewInt(0)) == 0)
				assert.True(t, tx.GasPrice.Cmp(big.NewInt(1000000000)) == 0)
				assert.Equal(t, uint64(21000), tx.GasLimit)
				assert.Equal(t, uint64(1), tx.Nonce)
				assert.True(t, tx.ChainID.Cmp(big.NewInt(8453)) == 0) // Base mainnet
				assert.True(t, len(tx.Data) > 0)
			},
		},
		{
			name: "contract creation transaction",
			rawData: createEthSubscriptionResponse(RawTransaction{
				Hash:     "0x1234567890123456789012345678901234567890123456789012345678901234",
				From:     "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
				To:       "", // Contract creation
				Value:    "0x0",
				GasPrice: "0x3b9aca00",
				Gas:      "0x5208",
				Nonce:    "0x1",
				Input:    "0x608060405234801561001057600080fd5b50",
			}),
			expectError: false,
			validate: func(t *testing.T, tx *mevtypes.Transaction) {
				assert.Nil(t, tx.To)
				assert.True(t, len(tx.Data) > 0)
			},
		},
		{
			name: "simple transfer transaction",
			rawData: createEthSubscriptionResponse(RawTransaction{
				Hash:     "0x1234567890123456789012345678901234567890123456789012345678901234",
				From:     "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
				To:       "0x1234567890123456789012345678901234567890",
				Value:    "0xde0b6b3a7640000", // 1 ETH
				GasPrice: "0x3b9aca00",
				Gas:      "0x5208",
				Nonce:    "0x1",
				Input:    "0x",
			}),
			expectError: false,
			validate: func(t *testing.T, tx *mevtypes.Transaction) {
				assert.True(t, tx.Value.Cmp(big.NewInt(1000000000000000000)) == 0) // 1 ETH
				assert.Equal(t, 0, len(tx.Data))
			},
		},
		{
			name:        "invalid JSON",
			rawData:     []byte("invalid json"),
			expectError: true,
		},
		{
			name: "wrong method",
			rawData: []byte(`{
				"jsonrpc": "2.0",
				"method": "eth_newHeads",
				"params": {}
			}`),
			expectError: true,
		},
		{
			name: "invalid transaction data",
			rawData: createEthSubscriptionResponse(RawTransaction{
				Hash:     "", // Empty hash
				From:     "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
				To:       "0x1234567890123456789012345678901234567890",
				Value:    "0x0",
				GasPrice: "0x3b9aca00",
				Gas:      "0x5208",
				Nonce:    "0x1",
			}),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := stream.ProcessTransaction(context.Background(), tt.rawData)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, tx)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tx)
				if tt.validate != nil {
					tt.validate(t, tx)
				}
			}
		})
	}
}

func TestFilterTransaction(t *testing.T) {
	tests := []struct {
		name     string
		config   TransactionStreamConfig
		tx       *mevtypes.Transaction
		expected bool
	}{
		{
			name:   "valid swap transaction passes filter",
			config: TransactionStreamConfig{},
			tx: &mevtypes.Transaction{
				Hash:     "0x123",
				From:     common.HexToAddress("0xabc"),
				To:       &common.Address{},
				Value:    big.NewInt(0),
				GasPrice: big.NewInt(2000000000), // 2 Gwei
				GasLimit: 21000,
				Data:     common.Hex2Bytes("7ff36ab5"), // swapExactETHForTokens
			},
			expected: true,
		},
		{
			name: "gas price too low",
			config: TransactionStreamConfig{
				MinGasPrice: big.NewInt(5000000000), // 5 Gwei
			},
			tx: &mevtypes.Transaction{
				GasPrice: big.NewInt(1000000000), // 1 Gwei
				Value:    big.NewInt(0),
				Data:     common.Hex2Bytes("7ff36ab5"),
			},
			expected: false,
		},
		{
			name: "gas price too high",
			config: TransactionStreamConfig{
				MaxGasPrice: big.NewInt(10000000000), // 10 Gwei
			},
			tx: &mevtypes.Transaction{
				GasPrice: big.NewInt(50000000000), // 50 Gwei
				Value:    big.NewInt(0),
				Data:     common.Hex2Bytes("7ff36ab5"),
			},
			expected: false,
		},
		{
			name: "value too low",
			config: TransactionStreamConfig{
				MinValue: big.NewInt(1000000000000000000), // 1 ETH
			},
			tx: &mevtypes.Transaction{
				GasPrice: big.NewInt(2000000000),
				Value:    big.NewInt(500000000000000000), // 0.5 ETH
				Data:     common.Hex2Bytes("7ff36ab5"),
			},
			expected: false,
		},
		{
			name: "contract filter match",
			config: TransactionStreamConfig{
				ContractFilters: []string{"0x1234567890123456789012345678901234567890"},
			},
			tx: &mevtypes.Transaction{
				To:       func() *common.Address { addr := common.HexToAddress("0x1234567890123456789012345678901234567890"); return &addr }(),
				GasPrice: big.NewInt(2000000000),
				Value:    big.NewInt(0),
				Data:     common.Hex2Bytes("7ff36ab5"),
			},
			expected: true,
		},
		{
			name: "contract filter no match",
			config: TransactionStreamConfig{
				ContractFilters: []string{"0x1234567890123456789012345678901234567890"},
			},
			tx: &mevtypes.Transaction{
				To:       func() *common.Address { addr := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcdef"); return &addr }(),
				GasPrice: big.NewInt(2000000000),
				Value:    big.NewInt(0),
				Data:     common.Hex2Bytes("7ff36ab5"),
			},
			expected: false,
		},
		{
			name: "method filter match",
			config: TransactionStreamConfig{
				MethodFilters: []string{"a9059cbb"}, // transfer
			},
			tx: &mevtypes.Transaction{
				GasPrice: big.NewInt(2000000000),
				Value:    big.NewInt(0),
				Data:     common.Hex2Bytes("a9059cbb"),
			},
			expected: true,
		},
		{
			name: "method filter no match",
			config: TransactionStreamConfig{
				MethodFilters: []string{"a9059cbb"}, // transfer
			},
			tx: &mevtypes.Transaction{
				GasPrice: big.NewInt(2000000000),
				Value:    big.NewInt(0),
				Data:     common.Hex2Bytes("7ff36ab5"), // swap
			},
			expected: false,
		},
		{
			name: "simple transfer with method filters should be rejected",
			config: TransactionStreamConfig{
				MethodFilters: []string{"7ff36ab5"}, // Only swaps
			},
			tx: &mevtypes.Transaction{
				GasPrice: big.NewInt(2000000000),
				Value:    big.NewInt(1000000000000000000), // 1 ETH
				Data:     []byte{}, // No data = simple transfer
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := NewTransactionStream(tt.config)
			result := stream.FilterTransaction(tt.tx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateTransaction(t *testing.T) {
	validTx := &mevtypes.Transaction{
		Hash:     "0x1234567890123456789012345678901234567890123456789012345678901234",
		From:     common.HexToAddress("0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"),
		To:       func() *common.Address { addr := common.HexToAddress("0x1234567890123456789012345678901234567890"); return &addr }(),
		Value:    big.NewInt(1000000000000000000),
		GasPrice: big.NewInt(2000000000),
		GasLimit: 21000,
		Nonce:    1,
		Data:     []byte{},
		ChainID:  big.NewInt(8453),
	}

	tests := []struct {
		name        string
		tx          *mevtypes.Transaction
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid transaction",
			tx:          validTx,
			expectError: false,
		},
		{
			name:        "nil transaction",
			tx:          nil,
			expectError: true,
			errorMsg:    "transaction is nil",
		},
		{
			name: "empty hash",
			tx: func() *mevtypes.Transaction {
				tx := *validTx
				tx.Hash = ""
				return &tx
			}(),
			expectError: true,
			errorMsg:    "transaction hash is empty",
		},
		{
			name: "invalid hash format",
			tx: func() *mevtypes.Transaction {
				tx := *validTx
				tx.Hash = "invalid_hash"
				return &tx
			}(),
			expectError: true,
			errorMsg:    "invalid transaction hash format",
		},
		{
			name: "zero from address",
			tx: func() *mevtypes.Transaction {
				tx := *validTx
				tx.From = common.Address{}
				return &tx
			}(),
			expectError: true,
			errorMsg:    "from address is zero address",
		},
		{
			name: "nil gas price",
			tx: func() *mevtypes.Transaction {
				tx := *validTx
				tx.GasPrice = nil
				return &tx
			}(),
			expectError: true,
			errorMsg:    "gas price must be positive",
		},
		{
			name: "zero gas price",
			tx: func() *mevtypes.Transaction {
				tx := *validTx
				tx.GasPrice = big.NewInt(0)
				return &tx
			}(),
			expectError: true,
			errorMsg:    "gas price must be positive",
		},
		{
			name: "zero gas limit",
			tx: func() *mevtypes.Transaction {
				tx := *validTx
				tx.GasLimit = 0
				return &tx
			}(),
			expectError: true,
			errorMsg:    "gas limit must be positive",
		},
		{
			name: "gas limit too high",
			tx: func() *mevtypes.Transaction {
				tx := *validTx
				tx.GasLimit = 50000000 // 50M gas
				return &tx
			}(),
			expectError: true,
			errorMsg:    "gas limit too high",
		},
		{
			name: "nil value",
			tx: func() *mevtypes.Transaction {
				tx := *validTx
				tx.Value = nil
				return &tx
			}(),
			expectError: true,
			errorMsg:    "transaction value is nil",
		},
		{
			name: "negative value",
			tx: func() *mevtypes.Transaction {
				tx := *validTx
				tx.Value = big.NewInt(-1)
				return &tx
			}(),
			expectError: true,
			errorMsg:    "transaction value cannot be negative",
		},
		{
			name: "nil chain ID",
			tx: func() *mevtypes.Transaction {
				tx := *validTx
				tx.ChainID = nil
				return &tx
			}(),
			expectError: true,
			errorMsg:    "invalid chain ID",
		},
		{
			name: "zero chain ID",
			tx: func() *mevtypes.Transaction {
				tx := *validTx
				tx.ChainID = big.NewInt(0)
				return &tx
			}(),
			expectError: true,
			errorMsg:    "invalid chain ID",
		},
		{
			name: "data too large",
			tx: func() *mevtypes.Transaction {
				tx := *validTx
				tx.Data = make([]byte, 1024*1024+1) // 1MB + 1 byte
				return &tx
			}(),
			expectError: true,
			errorMsg:    "transaction data too large",
		},
	}

	stream := NewTransactionStream(TransactionStreamConfig{})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := stream.ValidateTransaction(tt.tx)
			
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConvertRawTransaction(t *testing.T) {
	stream := NewTransactionStream(TransactionStreamConfig{}).(*TransactionStreamImpl)

	tests := []struct {
		name        string
		rawTx       RawTransaction
		expectError bool
		validate    func(t *testing.T, tx *mevtypes.Transaction)
	}{
		{
			name: "complete transaction",
			rawTx: RawTransaction{
				Hash:             "0x1234567890123456789012345678901234567890123456789012345678901234",
				From:             "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
				To:               "0x1234567890123456789012345678901234567890",
				Value:            "0xde0b6b3a7640000", // 1 ETH
				GasPrice:         "0x3b9aca00",        // 1 Gwei
				Gas:              "0x5208",            // 21000
				Nonce:            "0x1",
				Input:            "0x7ff36ab5",
				BlockNumber:      "0x123456",
				TransactionIndex: "0x1",
				ChainId:          "0x2105", // Base mainnet
			},
			expectError: false,
			validate: func(t *testing.T, tx *mevtypes.Transaction) {
				assert.Equal(t, "0x1234567890123456789012345678901234567890123456789012345678901234", tx.Hash)
				assert.True(t, tx.Value.Cmp(big.NewInt(1000000000000000000)) == 0)
				assert.True(t, tx.GasPrice.Cmp(big.NewInt(1000000000)) == 0)
				assert.Equal(t, uint64(21000), tx.GasLimit)
				assert.Equal(t, uint64(1), tx.Nonce)
				assert.True(t, tx.BlockNumber.Cmp(big.NewInt(0x123456)) == 0)
				assert.Equal(t, uint(1), tx.TxIndex)
				assert.True(t, tx.ChainID.Cmp(big.NewInt(8453)) == 0)
			},
		},
		{
			name: "minimal transaction",
			rawTx: RawTransaction{
				Hash:     "0x1234567890123456789012345678901234567890123456789012345678901234",
				From:     "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
				To:       "0x1234567890123456789012345678901234567890",
				Value:    "0x0",
				GasPrice: "0x3b9aca00",
				Gas:      "0x5208",
				Nonce:    "0x0",
				Input:    "0x",
			},
			expectError: false,
			validate: func(t *testing.T, tx *mevtypes.Transaction) {
				assert.True(t, tx.ChainID.Cmp(big.NewInt(8453)) == 0) // Default Base mainnet
				assert.Nil(t, tx.BlockNumber)
				assert.Equal(t, uint(0), tx.TxIndex)
				assert.Equal(t, 0, len(tx.Data))
			},
		},
		{
			name: "empty hash",
			rawTx: RawTransaction{
				Hash: "",
			},
			expectError: true,
		},
		{
			name: "invalid from address",
			rawTx: RawTransaction{
				Hash: "0x1234567890123456789012345678901234567890123456789012345678901234",
				From: "invalid_address",
			},
			expectError: true,
		},
		{
			name: "invalid to address",
			rawTx: RawTransaction{
				Hash: "0x1234567890123456789012345678901234567890123456789012345678901234",
				From: "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
				To:   "invalid_address",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := stream.convertRawTransaction(tt.rawTx)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, tx)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tx)
				if tt.validate != nil {
					tt.validate(t, tx)
				}
			}
		})
	}
}

// Helper function to create eth_subscription response
func createEthSubscriptionResponse(rawTx RawTransaction) []byte {
	response := EthSubscriptionResponse{
		JSONRPC: "2.0",
		Method:  "eth_subscription",
		Params: struct {
			Subscription string      `json:"subscription"`
			Result       interface{} `json:"result"`
		}{
			Subscription: "0x123456789",
			Result:       rawTx,
		},
	}
	
	data, _ := json.Marshal(response)
	return data
}

func TestTransactionStreamIntegration(t *testing.T) {
	// Integration test that combines all functionality
	config := TransactionStreamConfig{
		MinGasPrice:   big.NewInt(1000000000), // 1 Gwei
		MaxGasPrice:   big.NewInt(50000000000), // 50 Gwei
		MinValue:      big.NewInt(0),
		MethodFilters: []string{"7ff36ab5"}, // Only swapExactETHForTokens
	}
	
	stream := NewTransactionStream(config)
	
	// Create a valid swap transaction
	rawData := createEthSubscriptionResponse(RawTransaction{
		Hash:     "0x1234567890123456789012345678901234567890123456789012345678901234",
		From:     "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
		To:       "0x1234567890123456789012345678901234567890",
		Value:    "0xde0b6b3a7640000", // 1 ETH
		GasPrice: "0x77359400",        // 2 Gwei
		Gas:      "0x30d40",           // 200000
		Nonce:    "0x1",
		Input:    "0x7ff36ab5000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000c0",
	})
	
	// Process the transaction
	tx, err := stream.ProcessTransaction(context.Background(), rawData)
	require.NoError(t, err)
	require.NotNil(t, tx)
	
	// Validate the transaction
	err = stream.ValidateTransaction(tx)
	assert.NoError(t, err)
	
	// Filter the transaction
	shouldProcess := stream.FilterTransaction(tx)
	assert.True(t, shouldProcess, "Valid swap transaction should pass filter")
	
	// Test with transaction that should be filtered out
	rawDataFiltered := createEthSubscriptionResponse(RawTransaction{
		Hash:     "0x1234567890123456789012345678901234567890123456789012345678901234",
		From:     "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
		To:       "0x1234567890123456789012345678901234567890",
		Value:    "0xde0b6b3a7640000",
		GasPrice: "0x77359400",
		Gas:      "0x30d40",
		Nonce:    "0x1",
		Input:    "0xa9059cbb", // transfer method, not in our filter
	})
	
	txFiltered, err := stream.ProcessTransaction(context.Background(), rawDataFiltered)
	require.NoError(t, err)
	require.NotNil(t, txFiltered)
	
	shouldProcessFiltered := stream.FilterTransaction(txFiltered)
	assert.False(t, shouldProcessFiltered, "Transfer transaction should be filtered out")
}