package interfaces

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// EventParser handles parsing and decoding of transaction event logs
type EventParser interface {
	ParseEventLogs(ctx context.Context, logs []*ethtypes.Log) ([]*ParsedEvent, error)
	DecodeSwapEvent(ctx context.Context, log *ethtypes.Log) (*SwapEvent, error)
	DecodeBridgeEvent(ctx context.Context, log *ethtypes.Log) (*BridgeEvent, error)
	GetSupportedProtocols() []Protocol
}

// ABIManager manages contract ABIs for different protocols
type ABIManager interface {
	LoadABI(protocol Protocol, contractType ContractType) error
	GetABI(protocol Protocol, contractType ContractType) ([]byte, error)
	IsEventSupported(protocol Protocol, eventSignature string) bool
	GetEventSignature(protocol Protocol, eventName string) (common.Hash, error)
}

// ParsedEvent represents a decoded event log
type ParsedEvent struct {
	Protocol    Protocol
	EventType   EventType
	Address     common.Address
	TxHash      common.Hash
	BlockNumber uint64
	LogIndex    uint
	SwapEvent   *SwapEvent
	BridgeEvent *BridgeEvent
	RawLog      *ethtypes.Log
}

// SwapEvent represents a decoded swap event from DEX protocols
type SwapEvent struct {
	Protocol     Protocol
	Pool         common.Address
	TokenIn      common.Address
	TokenOut     common.Address
	AmountIn     *big.Int
	AmountOut    *big.Int
	Sender       common.Address
	Recipient    common.Address
	Fee          *big.Int
	SqrtPriceX96 *big.Int // For Uniswap V3
	Liquidity    *big.Int // For Uniswap V3
	Tick         *big.Int // For Uniswap V3
}

// BridgeEvent represents a bridge deposit/withdrawal event
type BridgeEvent struct {
	EventType EventType // Deposit or Withdraw
	Token     common.Address
	Amount    *big.Int
	From      common.Address
	To        common.Address
	L1TxHash  common.Hash
	L2TxHash  common.Hash
}

// Protocol represents different DeFi protocols
type Protocol int

const (
	ProtocolUnknown Protocol = iota
	ProtocolUniswapV2
	ProtocolUniswapV3
	ProtocolAerodrome
	ProtocolBaseBridge
)

func (p Protocol) String() string {
	switch p {
	case ProtocolUniswapV2:
		return "UniswapV2"
	case ProtocolUniswapV3:
		return "UniswapV3"
	case ProtocolAerodrome:
		return "Aerodrome"
	case ProtocolBaseBridge:
		return "BaseBridge"
	default:
		return "Unknown"
	}
}

// ContractType represents different contract types within protocols
type ContractType int

const (
	ContractTypeUnknown ContractType = iota
	ContractTypePair    // Uniswap V2 style pairs
	ContractTypePool    // Uniswap V3 style pools
	ContractTypeRouter  // Router contracts
	ContractTypeBridge  // Bridge contracts
)

func (c ContractType) String() string {
	switch c {
	case ContractTypePair:
		return "Pair"
	case ContractTypePool:
		return "Pool"
	case ContractTypeRouter:
		return "Router"
	case ContractTypeBridge:
		return "Bridge"
	default:
		return "Unknown"
	}
}

// EventType represents different types of events
type EventType int

const (
	EventTypeUnknown EventType = iota
	EventTypeSwap
	EventTypeDeposit
	EventTypeWithdraw
	EventTypeMint
	EventTypeBurn
)

func (e EventType) String() string {
	switch e {
	case EventTypeSwap:
		return "Swap"
	case EventTypeDeposit:
		return "Deposit"
	case EventTypeWithdraw:
		return "Withdraw"
	case EventTypeMint:
		return "Mint"
	case EventTypeBurn:
		return "Burn"
	default:
		return "Unknown"
	}
}