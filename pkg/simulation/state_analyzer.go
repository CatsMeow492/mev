package simulation

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// stateAnalyzer implements the StateAnalyzer interface
type stateAnalyzer struct {
	// Common contract ABIs for event parsing
	erc20ABI    abi.ABI
	uniswapV2ABI abi.ABI
	uniswapV3ABI abi.ABI
}

// NewStateAnalyzer creates a new state analyzer instance
func NewStateAnalyzer() (interfaces.StateAnalyzer, error) {
	analyzer := &stateAnalyzer{}
	
	// Initialize common ABIs for event parsing
	if err := analyzer.initializeABIs(); err != nil {
		return nil, fmt.Errorf("failed to initialize ABIs: %w", err)
	}
	
	return analyzer, nil
}

// AnalyzeStateChanges compares pre and post state snapshots to identify changes
func (sa *stateAnalyzer) AnalyzeStateChanges(preState, postState *interfaces.StateSnapshot) (*interfaces.StateAnalysis, error) {
	if preState == nil || postState == nil {
		return nil, fmt.Errorf("pre and post state snapshots cannot be nil")
	}

	analysis := &interfaces.StateAnalysis{
		BalanceChanges: make(map[common.Address]*big.Int),
		TokenTransfers: make([]*interfaces.TokenTransfer, 0),
		PriceChanges:   make(map[common.Address]*interfaces.PriceChange, 0),
		NetValue:       big.NewInt(0),
	}

	// Analyze balance changes
	for addr, postAccount := range postState.Accounts {
		preAccount, exists := preState.Accounts[addr]
		if !exists {
			// New account created
			analysis.BalanceChanges[addr] = new(big.Int).Set(postAccount.Balance)
		} else {
			// Calculate balance difference
			diff := new(big.Int).Sub(postAccount.Balance, preAccount.Balance)
			if diff.Sign() != 0 {
				analysis.BalanceChanges[addr] = diff
			}
		}
	}

	// Check for accounts that existed in pre-state but not in post-state
	for addr, preAccount := range preState.Accounts {
		if _, exists := postState.Accounts[addr]; !exists {
			// Account removed or balance went to zero
			analysis.BalanceChanges[addr] = new(big.Int).Neg(preAccount.Balance)
		}
	}

	// Analyze price changes
	for token, postPrice := range postState.TokenPrices {
		prePrice, exists := preState.TokenPrices[token]
		if exists && postPrice.Cmp(prePrice) != 0 {
			change := new(big.Int).Sub(postPrice, prePrice)
			changePercent := 0.0
			if prePrice.Sign() > 0 {
				changePercent = float64(change.Int64()) / float64(prePrice.Int64()) * 100
			}
			
			analysis.PriceChanges[token] = &interfaces.PriceChange{
				Token:         token,
				OldPrice:      prePrice,
				NewPrice:      postPrice,
				Change:        change,
				ChangePercent: changePercent,
			}
		}
	}

	// Calculate net value change
	netValue := big.NewInt(0)
	for _, balanceChange := range analysis.BalanceChanges {
		netValue.Add(netValue, balanceChange)
	}
	analysis.NetValue = netValue

	return analysis, nil
}

// CalculateGasUsage analyzes gas usage from simulation results
func (sa *stateAnalyzer) CalculateGasUsage(result *interfaces.SimulationResult) (*interfaces.GasAnalysis, error) {
	if result == nil {
		return nil, fmt.Errorf("simulation result cannot be nil")
	}

	totalCost := new(big.Int).Mul(big.NewInt(int64(result.GasUsed)), result.GasPrice)
	
	// Calculate efficiency as a ratio of gas used vs gas limit
	// This would need the original transaction's gas limit for accurate calculation
	efficiency := 1.0 // Placeholder - would need more context for accurate calculation

	analysis := &interfaces.GasAnalysis{
		GasUsed:    result.GasUsed,
		GasPrice:   result.GasPrice,
		TotalCost:  totalCost,
		Efficiency: efficiency,
	}

	return analysis, nil
}

// ExtractEventLogs parses and decodes event logs from simulation results
func (sa *stateAnalyzer) ExtractEventLogs(result *interfaces.SimulationResult) ([]*interfaces.EventLog, error) {
	if result == nil {
		return nil, fmt.Errorf("simulation result cannot be nil")
	}

	eventLogs := make([]*interfaces.EventLog, 0, len(result.Logs))

	for _, log := range result.Logs {
		eventLog := &interfaces.EventLog{
			Address: log.Address,
			Topics:  log.Topics,
			Data:    log.Data,
			Decoded: make(map[string]interface{}),
		}

		// Attempt to decode the log based on known contract types
		decoded, err := sa.decodeEventLog(log)
		if err == nil {
			eventLog.Decoded = decoded
		}

		eventLogs = append(eventLogs, eventLog)
	}

	return eventLogs, nil
}

// MeasurePriceImpact calculates the price impact of a transaction
func (sa *stateAnalyzer) MeasurePriceImpact(result *interfaces.SimulationResult) (*interfaces.PriceImpact, error) {
	if result == nil {
		return nil, fmt.Errorf("simulation result cannot be nil")
	}

	// This is a simplified implementation
	// In practice, you'd need to analyze swap events and pool state changes
	impact := &interfaces.PriceImpact{
		Token:         common.Address{}, // Would be extracted from swap events
		Pool:          common.Address{}, // Would be extracted from swap events
		ImpactBps:     0,                // Would be calculated from price changes
		ImpactPercent: 0.0,              // Would be calculated from price changes
		SlippageBps:   0,                // Would be calculated from expected vs actual output
	}

	// Parse swap events to calculate actual price impact
	for _, log := range result.Logs {
		if sa.isSwapEvent(log) {
			// Extract swap details and calculate price impact
			// This would involve parsing the swap event data
			// and comparing input/output amounts to calculate slippage
		}
	}

	return impact, nil
}

// initializeABIs loads common contract ABIs for event parsing
func (sa *stateAnalyzer) initializeABIs() error {
	// ERC-20 Transfer event ABI
	erc20JSON := `[{"anonymous":false,"inputs":[{"indexed":true,"name":"from","type":"address"},{"indexed":true,"name":"to","type":"address"},{"indexed":false,"name":"value","type":"uint256"}],"name":"Transfer","type":"event"}]`
	
	var err error
	sa.erc20ABI, err = abi.JSON(strings.NewReader(erc20JSON))
	if err != nil {
		return fmt.Errorf("failed to parse ERC-20 ABI: %w", err)
	}

	// Uniswap V2 Swap event ABI
	uniswapV2JSON := `[{"anonymous":false,"inputs":[{"indexed":true,"name":"sender","type":"address"},{"indexed":false,"name":"amount0In","type":"uint256"},{"indexed":false,"name":"amount1In","type":"uint256"},{"indexed":false,"name":"amount0Out","type":"uint256"},{"indexed":false,"name":"amount1Out","type":"uint256"},{"indexed":true,"name":"to","type":"address"}],"name":"Swap","type":"event"}]`
	
	sa.uniswapV2ABI, err = abi.JSON(strings.NewReader(uniswapV2JSON))
	if err != nil {
		return fmt.Errorf("failed to parse Uniswap V2 ABI: %w", err)
	}

	// Uniswap V3 Swap event ABI
	uniswapV3JSON := `[{"anonymous":false,"inputs":[{"indexed":true,"name":"sender","type":"address"},{"indexed":true,"name":"recipient","type":"address"},{"indexed":false,"name":"amount0","type":"int256"},{"indexed":false,"name":"amount1","type":"int256"},{"indexed":false,"name":"sqrtPriceX96","type":"uint160"},{"indexed":false,"name":"liquidity","type":"uint128"},{"indexed":false,"name":"tick","type":"int24"}],"name":"Swap","type":"event"}]`
	
	sa.uniswapV3ABI, err = abi.JSON(strings.NewReader(uniswapV3JSON))
	if err != nil {
		return fmt.Errorf("failed to parse Uniswap V3 ABI: %w", err)
	}

	return nil
}

// decodeEventLog attempts to decode an event log using known ABIs
func (sa *stateAnalyzer) decodeEventLog(log *ethtypes.Log) (map[string]interface{}, error) {
	if len(log.Topics) == 0 {
		return nil, fmt.Errorf("log has no topics")
	}

	eventSignature := log.Topics[0]
	
	// Try to decode as ERC-20 Transfer event
	if transferEvent, exists := sa.erc20ABI.Events["Transfer"]; exists {
		if eventSignature == transferEvent.ID {
			return sa.decodeTransferEvent(log)
		}
	}

	// Try to decode as Uniswap V2 Swap event
	if swapEvent, exists := sa.uniswapV2ABI.Events["Swap"]; exists {
		if eventSignature == swapEvent.ID {
			return sa.decodeUniswapV2SwapEvent(log)
		}
	}

	// Try to decode as Uniswap V3 Swap event
	if swapEvent, exists := sa.uniswapV3ABI.Events["Swap"]; exists {
		if eventSignature == swapEvent.ID {
			return sa.decodeUniswapV3SwapEvent(log)
		}
	}

	return nil, fmt.Errorf("unknown event signature: %s", eventSignature.Hex())
}

// decodeTransferEvent decodes ERC-20 Transfer events
func (sa *stateAnalyzer) decodeTransferEvent(log *ethtypes.Log) (map[string]interface{}, error) {
	if len(log.Topics) < 3 {
		return nil, fmt.Errorf("insufficient topics for Transfer event")
	}

	decoded := make(map[string]interface{})
	decoded["event"] = "Transfer"
	decoded["from"] = common.HexToAddress(log.Topics[1].Hex())
	decoded["to"] = common.HexToAddress(log.Topics[2].Hex())
	
	// Decode value from data
	if len(log.Data) >= 32 {
		value := new(big.Int).SetBytes(log.Data[:32])
		decoded["value"] = value
	}

	return decoded, nil
}

// decodeUniswapV2SwapEvent decodes Uniswap V2 Swap events
func (sa *stateAnalyzer) decodeUniswapV2SwapEvent(log *ethtypes.Log) (map[string]interface{}, error) {
	if len(log.Topics) < 3 {
		return nil, fmt.Errorf("insufficient topics for Uniswap V2 Swap event")
	}

	decoded := make(map[string]interface{})
	decoded["event"] = "UniswapV2Swap"
	decoded["sender"] = common.HexToAddress(log.Topics[1].Hex())
	decoded["to"] = common.HexToAddress(log.Topics[2].Hex())

	// Decode amounts from data (4 uint256 values)
	if len(log.Data) >= 128 {
		decoded["amount0In"] = new(big.Int).SetBytes(log.Data[0:32])
		decoded["amount1In"] = new(big.Int).SetBytes(log.Data[32:64])
		decoded["amount0Out"] = new(big.Int).SetBytes(log.Data[64:96])
		decoded["amount1Out"] = new(big.Int).SetBytes(log.Data[96:128])
	}

	return decoded, nil
}

// decodeUniswapV3SwapEvent decodes Uniswap V3 Swap events
func (sa *stateAnalyzer) decodeUniswapV3SwapEvent(log *ethtypes.Log) (map[string]interface{}, error) {
	if len(log.Topics) < 3 {
		return nil, fmt.Errorf("insufficient topics for Uniswap V3 Swap event")
	}

	decoded := make(map[string]interface{})
	decoded["event"] = "UniswapV3Swap"
	decoded["sender"] = common.HexToAddress(log.Topics[1].Hex())
	decoded["recipient"] = common.HexToAddress(log.Topics[2].Hex())

	// Decode amounts and price data from log data
	if len(log.Data) >= 160 {
		// amount0 (int256)
		amount0 := new(big.Int).SetBytes(log.Data[0:32])
		if log.Data[0]&0x80 != 0 { // Check sign bit
			amount0.Sub(amount0, new(big.Int).Lsh(big.NewInt(1), 256))
		}
		decoded["amount0"] = amount0

		// amount1 (int256)
		amount1 := new(big.Int).SetBytes(log.Data[32:64])
		if log.Data[32]&0x80 != 0 { // Check sign bit
			amount1.Sub(amount1, new(big.Int).Lsh(big.NewInt(1), 256))
		}
		decoded["amount1"] = amount1

		decoded["sqrtPriceX96"] = new(big.Int).SetBytes(log.Data[64:96])
		decoded["liquidity"] = new(big.Int).SetBytes(log.Data[96:128])
		
		// tick (int24) - only use last 3 bytes
		tick := new(big.Int).SetBytes(log.Data[157:160])
		if log.Data[157]&0x80 != 0 { // Check sign bit for 24-bit signed integer
			tick.Sub(tick, new(big.Int).Lsh(big.NewInt(1), 24))
		}
		decoded["tick"] = tick
	}

	return decoded, nil
}

// isSwapEvent checks if a log represents a swap event
func (sa *stateAnalyzer) isSwapEvent(log *ethtypes.Log) bool {
	if len(log.Topics) == 0 {
		return false
	}

	eventSignature := log.Topics[0]
	
	// Check against known swap event signatures
	if swapEvent, exists := sa.uniswapV2ABI.Events["Swap"]; exists {
		if eventSignature == swapEvent.ID {
			return true
		}
	}

	if swapEvent, exists := sa.uniswapV3ABI.Events["Swap"]; exists {
		if eventSignature == swapEvent.ID {
			return true
		}
	}

	return false
}