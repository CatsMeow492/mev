package mempool

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	mevtypes "github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// TransactionStreamImpl implements the TransactionStream interface
type TransactionStreamImpl struct {
	minGasPrice     *big.Int
	maxGasPrice     *big.Int
	minValue        *big.Int
	contractFilters []string // Contract addresses to filter for
	methodFilters   []string // Method signatures to filter for
}

// TransactionStreamConfig holds configuration for the transaction stream
type TransactionStreamConfig struct {
	MinGasPrice     *big.Int
	MaxGasPrice     *big.Int
	MinValue        *big.Int
	ContractFilters []string
	MethodFilters   []string
}

// NewTransactionStream creates a new transaction stream processor
func NewTransactionStream(config TransactionStreamConfig) interfaces.TransactionStream {
	// Set default values if not provided
	minGasPrice := config.MinGasPrice
	if minGasPrice == nil {
		minGasPrice = big.NewInt(1000000000) // 1 Gwei default
	}

	maxGasPrice := config.MaxGasPrice
	if maxGasPrice == nil {
		maxGasPrice = big.NewInt(100000000000) // 100 Gwei default
	}

	minValue := config.MinValue
	if minValue == nil {
		minValue = big.NewInt(0) // No minimum value by default
	}

	// Default method filters for common DEX operations
	methodFilters := config.MethodFilters
	if len(methodFilters) == 0 {
		methodFilters = []string{
			"7ff36ab5", // swapExactETHForTokens
			"18cbafe5", // swapExactTokensForETH
			"38ed1739", // swapExactTokensForTokens
			"e8e33700", // addLiquidity
			"f305d719", // addLiquidityETH
			"baa2abde", // removeLiquidity
			"02751cec", // removeLiquidityETH
			"ac9650d8", // multicall (common in DEX aggregators)
		}
	}

	return &TransactionStreamImpl{
		minGasPrice:     minGasPrice,
		maxGasPrice:     maxGasPrice,
		minValue:        minValue,
		contractFilters: config.ContractFilters,
		methodFilters:   methodFilters,
	}
}

// EthSubscriptionResponse represents the structure of eth_subscription responses
type EthSubscriptionResponse struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  struct {
		Subscription string      `json:"subscription"`
		Result       interface{} `json:"result"`
	} `json:"params"`
}

// RawTransaction represents the raw transaction data from eth_subscribe
type RawTransaction struct {
	Hash             string `json:"hash"`
	From             string `json:"from"`
	To               string `json:"to"`
	Value            string `json:"value"`
	GasPrice         string `json:"gasPrice"`
	Gas              string `json:"gas"`
	Nonce            string `json:"nonce"`
	Input            string `json:"input"`
	BlockNumber      string `json:"blockNumber,omitempty"`
	TransactionIndex string `json:"transactionIndex,omitempty"`
	ChainId          string `json:"chainId,omitempty"`
}

// ProcessTransaction processes raw transaction data from WebSocket eth_subscribe responses
func (ts *TransactionStreamImpl) ProcessTransaction(ctx context.Context, rawTx []byte) (*mevtypes.Transaction, error) {
	// Parse the eth_subscription response
	var subResp EthSubscriptionResponse
	if err := json.Unmarshal(rawTx, &subResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal subscription response: %w", err)
	}

	// Check if this is an eth_subscription method
	if subResp.Method != "eth_subscription" {
		return nil, fmt.Errorf("unexpected method: %s", subResp.Method)
	}

	// Extract the transaction data from the result
	resultBytes, err := json.Marshal(subResp.Params.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	var rawTxData RawTransaction
	if err := json.Unmarshal(resultBytes, &rawTxData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction data: %w", err)
	}

	// Convert raw transaction to our Transaction type
	tx, err := ts.convertRawTransaction(rawTxData)
	if err != nil {
		return nil, fmt.Errorf("failed to convert raw transaction: %w", err)
	}

	return tx, nil
}

// convertRawTransaction converts a RawTransaction to our Transaction type
func (ts *TransactionStreamImpl) convertRawTransaction(rawTx RawTransaction) (*mevtypes.Transaction, error) {
	// Parse hash
	if rawTx.Hash == "" {
		return nil, fmt.Errorf("transaction hash is empty")
	}

	// Parse from address
	if !common.IsHexAddress(rawTx.From) {
		return nil, fmt.Errorf("invalid from address: %s", rawTx.From)
	}
	fromAddr := common.HexToAddress(rawTx.From)

	// Parse to address (can be nil for contract creation)
	var toAddr *common.Address
	if rawTx.To != "" {
		if !common.IsHexAddress(rawTx.To) {
			return nil, fmt.Errorf("invalid to address: %s", rawTx.To)
		}
		addr := common.HexToAddress(rawTx.To)
		toAddr = &addr
	}

	// Parse value
	value, err := hexutil.DecodeBig(rawTx.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to decode value: %w", err)
	}

	// Parse gas price
	gasPrice, err := hexutil.DecodeBig(rawTx.GasPrice)
	if err != nil {
		return nil, fmt.Errorf("failed to decode gas price: %w", err)
	}

	// Parse gas limit
	gasLimit, err := hexutil.DecodeUint64(rawTx.Gas)
	if err != nil {
		return nil, fmt.Errorf("failed to decode gas limit: %w", err)
	}

	// Parse nonce
	nonce, err := hexutil.DecodeUint64(rawTx.Nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to decode nonce: %w", err)
	}

	// Parse input data
	var data []byte
	if rawTx.Input != "" && rawTx.Input != "0x" {
		data, err = hexutil.Decode(rawTx.Input)
		if err != nil {
			return nil, fmt.Errorf("failed to decode input data: %w", err)
		}
	}

	// Parse optional fields
	var blockNumber *big.Int
	if rawTx.BlockNumber != "" && rawTx.BlockNumber != "0x" {
		blockNumber, err = hexutil.DecodeBig(rawTx.BlockNumber)
		if err != nil {
			return nil, fmt.Errorf("failed to decode block number: %w", err)
		}
	}

	var txIndex uint
	if rawTx.TransactionIndex != "" && rawTx.TransactionIndex != "0x" {
		txIndexUint64, err := hexutil.DecodeUint64(rawTx.TransactionIndex)
		if err != nil {
			return nil, fmt.Errorf("failed to decode transaction index: %w", err)
		}
		txIndex = uint(txIndexUint64)
	}

	var chainID *big.Int
	if rawTx.ChainId != "" && rawTx.ChainId != "0x" {
		chainID, err = hexutil.DecodeBig(rawTx.ChainId)
		if err != nil {
			return nil, fmt.Errorf("failed to decode chain ID: %w", err)
		}
	} else {
		// Default to Base mainnet chain ID (8453)
		chainID = big.NewInt(8453)
	}

	return &mevtypes.Transaction{
		Hash:        rawTx.Hash,
		From:        fromAddr,
		To:          toAddr,
		Value:       value,
		GasPrice:    gasPrice,
		GasLimit:    gasLimit,
		Nonce:       nonce,
		Data:        data,
		Timestamp:   time.Now(),
		BlockNumber: blockNumber,
		TxIndex:     txIndex,
		ChainID:     chainID,
	}, nil
}

// FilterTransaction determines if a transaction should be processed based on filtering criteria
func (ts *TransactionStreamImpl) FilterTransaction(tx *mevtypes.Transaction) bool {
	// Filter by gas price range
	if tx.GasPrice.Cmp(ts.minGasPrice) < 0 {
		return false
	}
	if tx.GasPrice.Cmp(ts.maxGasPrice) > 0 {
		return false
	}

	// Filter by minimum value
	if tx.Value.Cmp(ts.minValue) < 0 {
		return false
	}

	// Filter by contract addresses if specified
	if len(ts.contractFilters) > 0 && tx.To != nil {
		found := false
		toAddrStr := strings.ToLower(tx.To.Hex())
		for _, contractAddr := range ts.contractFilters {
			if strings.ToLower(contractAddr) == toAddrStr {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by method signatures for contract interactions
	if len(ts.methodFilters) > 0 && len(tx.Data) >= 4 {
		methodSig := common.Bytes2Hex(tx.Data[:4])
		found := false
		for _, filter := range ts.methodFilters {
			if strings.ToLower(filter) == strings.ToLower(methodSig) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// If no method filters and no data, it's a simple transfer - include it
	if len(ts.methodFilters) > 0 && len(tx.Data) == 0 {
		return false
	}

	return true
}

// ValidateTransaction performs validation checks on a transaction
func (ts *TransactionStreamImpl) ValidateTransaction(tx *mevtypes.Transaction) error {
	if tx == nil {
		return fmt.Errorf("transaction is nil")
	}

	// Validate hash
	if tx.Hash == "" {
		return fmt.Errorf("transaction hash is empty")
	}
	if !strings.HasPrefix(tx.Hash, "0x") || len(tx.Hash) != 66 {
		return fmt.Errorf("invalid transaction hash format: %s", tx.Hash)
	}

	// Validate from address (cannot be zero address)
	if tx.From == (common.Address{}) {
		return fmt.Errorf("from address is zero address")
	}

	// Validate gas price (must be positive)
	if tx.GasPrice == nil || tx.GasPrice.Sign() <= 0 {
		return fmt.Errorf("gas price must be positive")
	}

	// Validate gas limit (must be positive and reasonable)
	if tx.GasLimit == 0 {
		return fmt.Errorf("gas limit must be positive")
	}
	if tx.GasLimit > 30000000 { // 30M gas limit seems reasonable for L2
		return fmt.Errorf("gas limit too high: %d", tx.GasLimit)
	}

	// Validate value (must not be nil)
	if tx.Value == nil {
		return fmt.Errorf("transaction value is nil")
	}
	if tx.Value.Sign() < 0 {
		return fmt.Errorf("transaction value cannot be negative")
	}

	// Validate chain ID
	if tx.ChainID == nil || tx.ChainID.Sign() <= 0 {
		return fmt.Errorf("invalid chain ID")
	}

	// Validate data format if present
	if len(tx.Data) > 0 {
		// Check for reasonable data size (prevent DoS)
		if len(tx.Data) > 1024*1024 { // 1MB limit
			return fmt.Errorf("transaction data too large: %d bytes", len(tx.Data))
		}
	}

	return nil
}