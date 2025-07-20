package events

import (
	"fmt"

	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// getUniswapV2ABI returns the ABI for Uniswap V2 contracts
func (m *ABIManagerImpl) getUniswapV2ABI(contractType interfaces.ContractType) (string, error) {
	switch contractType {
	case interfaces.ContractTypePair:
		return uniswapV2PairABI, nil
	case interfaces.ContractTypeRouter:
		return uniswapV2RouterABI, nil
	default:
		return "", fmt.Errorf("unsupported Uniswap V2 contract type: %s", contractType.String())
	}
}

// getUniswapV3ABI returns the ABI for Uniswap V3 contracts
func (m *ABIManagerImpl) getUniswapV3ABI(contractType interfaces.ContractType) (string, error) {
	switch contractType {
	case interfaces.ContractTypePool:
		return uniswapV3PoolABI, nil
	case interfaces.ContractTypeRouter:
		return uniswapV3RouterABI, nil
	default:
		return "", fmt.Errorf("unsupported Uniswap V3 contract type: %s", contractType.String())
	}
}

// getAerodromeABI returns the ABI for Aerodrome contracts
func (m *ABIManagerImpl) getAerodromeABI(contractType interfaces.ContractType) (string, error) {
	switch contractType {
	case interfaces.ContractTypePair:
		return aerodromePairABI, nil
	case interfaces.ContractTypeRouter:
		return aerodromeRouterABI, nil
	default:
		return "", fmt.Errorf("unsupported Aerodrome contract type: %s", contractType.String())
	}
}

// getBaseBridgeABI returns the ABI for Base Bridge contracts
func (m *ABIManagerImpl) getBaseBridgeABI(contractType interfaces.ContractType) (string, error) {
	switch contractType {
	case interfaces.ContractTypeBridge:
		return baseBridgeABI, nil
	default:
		return "", fmt.Errorf("unsupported Base Bridge contract type: %s", contractType.String())
	}
}

// Uniswap V2 Pair ABI (focused on Swap event)
const uniswapV2PairABI = `[
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "sender", "type": "address"},
			{"indexed": false, "name": "amount0In", "type": "uint256"},
			{"indexed": false, "name": "amount1In", "type": "uint256"},
			{"indexed": false, "name": "amount0Out", "type": "uint256"},
			{"indexed": false, "name": "amount1Out", "type": "uint256"},
			{"indexed": true, "name": "to", "type": "address"}
		],
		"name": "Swap",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "sender", "type": "address"},
			{"indexed": false, "name": "amount0", "type": "uint256"},
			{"indexed": false, "name": "amount1", "type": "uint256"}
		],
		"name": "Mint",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "sender", "type": "address"},
			{"indexed": false, "name": "amount0", "type": "uint256"},
			{"indexed": false, "name": "amount1", "type": "uint256"},
			{"indexed": true, "name": "to", "type": "address"}
		],
		"name": "Burn",
		"type": "event"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "token0",
		"outputs": [{"name": "", "type": "address"}],
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "token1",
		"outputs": [{"name": "", "type": "address"}],
		"type": "function"
	}
]`

// Uniswap V2 Router ABI (minimal)
const uniswapV2RouterABI = `[
	{
		"inputs": [
			{"name": "amountIn", "type": "uint256"},
			{"name": "amountOutMin", "type": "uint256"},
			{"name": "path", "type": "address[]"},
			{"name": "to", "type": "address"},
			{"name": "deadline", "type": "uint256"}
		],
		"name": "swapExactTokensForTokens",
		"outputs": [{"name": "amounts", "type": "uint256[]"}],
		"type": "function"
	}
]`

// Uniswap V3 Pool ABI (focused on Swap event)
const uniswapV3PoolABI = `[
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "sender", "type": "address"},
			{"indexed": true, "name": "recipient", "type": "address"},
			{"indexed": false, "name": "amount0", "type": "int256"},
			{"indexed": false, "name": "amount1", "type": "int256"},
			{"indexed": false, "name": "sqrtPriceX96", "type": "uint160"},
			{"indexed": false, "name": "liquidity", "type": "uint128"},
			{"indexed": false, "name": "tick", "type": "int24"}
		],
		"name": "Swap",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "owner", "type": "address"},
			{"indexed": true, "name": "tickLower", "type": "int24"},
			{"indexed": true, "name": "tickUpper", "type": "int24"},
			{"indexed": false, "name": "amount", "type": "uint128"},
			{"indexed": false, "name": "amount0", "type": "uint256"},
			{"indexed": false, "name": "amount1", "type": "uint256"}
		],
		"name": "Mint",
		"type": "event"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "token0",
		"outputs": [{"name": "", "type": "address"}],
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "token1",
		"outputs": [{"name": "", "type": "address"}],
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "fee",
		"outputs": [{"name": "", "type": "uint24"}],
		"type": "function"
	}
]`

// Uniswap V3 Router ABI (minimal)
const uniswapV3RouterABI = `[
	{
		"inputs": [
			{
				"components": [
					{"name": "tokenIn", "type": "address"},
					{"name": "tokenOut", "type": "address"},
					{"name": "fee", "type": "uint24"},
					{"name": "recipient", "type": "address"},
					{"name": "deadline", "type": "uint256"},
					{"name": "amountIn", "type": "uint256"},
					{"name": "amountOutMinimum", "type": "uint256"},
					{"name": "sqrtPriceLimitX96", "type": "uint160"}
				],
				"name": "params",
				"type": "tuple"
			}
		],
		"name": "exactInputSingle",
		"outputs": [{"name": "amountOut", "type": "uint256"}],
		"type": "function"
	}
]`

// Aerodrome Pair ABI (similar to Uniswap V2 but with some differences)
const aerodromePairABI = `[
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "sender", "type": "address"},
			{"indexed": false, "name": "amount0In", "type": "uint256"},
			{"indexed": false, "name": "amount1In", "type": "uint256"},
			{"indexed": false, "name": "amount0Out", "type": "uint256"},
			{"indexed": false, "name": "amount1Out", "type": "uint256"},
			{"indexed": true, "name": "to", "type": "address"}
		],
		"name": "Swap",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "sender", "type": "address"},
			{"indexed": false, "name": "amount0", "type": "uint256"},
			{"indexed": false, "name": "amount1", "type": "uint256"}
		],
		"name": "Mint",
		"type": "event"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "token0",
		"outputs": [{"name": "", "type": "address"}],
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "token1",
		"outputs": [{"name": "", "type": "address"}],
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "stable",
		"outputs": [{"name": "", "type": "bool"}],
		"type": "function"
	}
]`

// Aerodrome Router ABI (minimal)
const aerodromeRouterABI = `[
	{
		"inputs": [
			{"name": "amountIn", "type": "uint256"},
			{"name": "amountOutMin", "type": "uint256"},
			{
				"components": [
					{"name": "from", "type": "address"},
					{"name": "to", "type": "address"},
					{"name": "stable", "type": "bool"}
				],
				"name": "routes",
				"type": "tuple[]"
			},
			{"name": "to", "type": "address"},
			{"name": "deadline", "type": "uint256"}
		],
		"name": "swapExactTokensForTokens",
		"outputs": [{"name": "amounts", "type": "uint256[]"}],
		"type": "function"
	}
]`

// Base Bridge ABI (simplified)
const baseBridgeABI = `[
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "from", "type": "address"},
			{"indexed": true, "name": "to", "type": "address"},
			{"indexed": false, "name": "amount", "type": "uint256"},
			{"indexed": false, "name": "extraData", "type": "bytes"}
		],
		"name": "DepositInitiated",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "from", "type": "address"},
			{"indexed": true, "name": "to", "type": "address"},
			{"indexed": false, "name": "amount", "type": "uint256"},
			{"indexed": false, "name": "extraData", "type": "bytes"}
		],
		"name": "WithdrawalInitiated",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "from", "type": "address"},
			{"indexed": true, "name": "to", "type": "address"},
			{"indexed": false, "name": "amount", "type": "uint256"},
			{"indexed": false, "name": "extraData", "type": "bytes"}
		],
		"name": "DepositFinalized",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "from", "type": "address"},
			{"indexed": true, "name": "to", "type": "address"},
			{"indexed": false, "name": "amount", "type": "uint256"},
			{"indexed": false, "name": "extraData", "type": "bytes"}
		],
		"name": "WithdrawalFinalized",
		"type": "event"
	}
]`