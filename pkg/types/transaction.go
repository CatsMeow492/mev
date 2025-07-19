package types

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// Transaction represents a blockchain transaction
type Transaction struct {
	Hash        string         `json:"hash"`
	From        common.Address `json:"from"`
	To          *common.Address `json:"to"`
	Value       *big.Int       `json:"value"`
	GasPrice    *big.Int       `json:"gasPrice"`
	GasLimit    uint64         `json:"gasLimit"`
	Nonce       uint64         `json:"nonce"`
	Data        []byte         `json:"data"`
	Timestamp   time.Time      `json:"timestamp"`
	BlockNumber *big.Int       `json:"blockNumber,omitempty"`
	TxIndex     uint           `json:"transactionIndex,omitempty"`
	ChainID     *big.Int       `json:"chainId"`
}

// TransactionType represents different types of transactions
type TransactionType string

const (
	TxTypeTransfer TransactionType = "transfer"
	TxTypeSwap     TransactionType = "swap"
	TxTypeLiquidity TransactionType = "liquidity"
	TxTypeBridge   TransactionType = "bridge"
	TxTypeContract TransactionType = "contract"
	TxTypeUnknown  TransactionType = "unknown"
)

// GetTransactionType determines the type of transaction based on its data
func (t *Transaction) GetTransactionType() TransactionType {
	if len(t.Data) == 0 {
		return TxTypeTransfer
	}
	
	// Check method signatures for common DEX operations
	if len(t.Data) >= 4 {
		methodSig := common.Bytes2Hex(t.Data[:4])
		switch methodSig {
		case "7ff36ab5": // swapExactETHForTokens
			return TxTypeSwap
		case "18cbafe5": // swapExactTokensForETH
			return TxTypeSwap
		case "38ed1739": // swapExactTokensForTokens
			return TxTypeSwap
		case "e8e33700": // addLiquidity
			return TxTypeLiquidity
		case "f305d719": // addLiquidityETH
			return TxTypeLiquidity
		case "baa2abde": // removeLiquidity
			return TxTypeLiquidity
		case "02751cec": // removeLiquidityETH
			return TxTypeLiquidity
		}
	}
	
	return TxTypeContract
}

// IsHighValue determines if the transaction is high value
func (t *Transaction) IsHighValue(threshold *big.Int) bool {
	return t.Value.Cmp(threshold) >= 0
}

// GetPriority calculates transaction priority based on gas price
func (t *Transaction) GetPriority() *big.Int {
	// Simple priority calculation: gasPrice * gasLimit
	priority := new(big.Int).Mul(t.GasPrice, big.NewInt(int64(t.GasLimit)))
	return priority
}