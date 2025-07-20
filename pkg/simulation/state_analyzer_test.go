package simulation

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStateAnalyzer(t *testing.T) {
	analyzer, err := NewStateAnalyzer()
	require.NoError(t, err)
	require.NotNil(t, analyzer)
}

func TestAnalyzeStateChanges(t *testing.T) {
	analyzer, err := NewStateAnalyzer()
	require.NoError(t, err)

	tests := []struct {
		name        string
		preState    *interfaces.StateSnapshot
		postState   *interfaces.StateSnapshot
		expectError bool
		validate    func(t *testing.T, analysis *interfaces.StateAnalysis)
	}{
		{
			name:        "nil pre-state",
			preState:    nil,
			postState:   &interfaces.StateSnapshot{},
			expectError: true,
		},
		{
			name:        "nil post-state",
			preState:    &interfaces.StateSnapshot{},
			postState:   nil,
			expectError: true,
		},
		{
			name: "no balance changes",
			preState: &interfaces.StateSnapshot{
				BlockNumber: big.NewInt(100),
				Timestamp:   time.Now(),
				Accounts: map[common.Address]*interfaces.AccountState{
					common.HexToAddress("0x1"): {Balance: big.NewInt(1000)},
				},
				TokenPrices: make(map[common.Address]*big.Int),
			},
			postState: &interfaces.StateSnapshot{
				BlockNumber: big.NewInt(101),
				Timestamp:   time.Now(),
				Accounts: map[common.Address]*interfaces.AccountState{
					common.HexToAddress("0x1"): {Balance: big.NewInt(1000)},
				},
				TokenPrices: make(map[common.Address]*big.Int),
			},
			expectError: false,
			validate: func(t *testing.T, analysis *interfaces.StateAnalysis) {
				assert.Empty(t, analysis.BalanceChanges)
				assert.Equal(t, big.NewInt(0), analysis.NetValue)
			},
		},
		{
			name: "positive balance change",
			preState: &interfaces.StateSnapshot{
				BlockNumber: big.NewInt(100),
				Timestamp:   time.Now(),
				Accounts: map[common.Address]*interfaces.AccountState{
					common.HexToAddress("0x1"): {Balance: big.NewInt(1000)},
				},
				TokenPrices: make(map[common.Address]*big.Int),
			},
			postState: &interfaces.StateSnapshot{
				BlockNumber: big.NewInt(101),
				Timestamp:   time.Now(),
				Accounts: map[common.Address]*interfaces.AccountState{
					common.HexToAddress("0x1"): {Balance: big.NewInt(1500)},
				},
				TokenPrices: make(map[common.Address]*big.Int),
			},
			expectError: false,
			validate: func(t *testing.T, analysis *interfaces.StateAnalysis) {
				addr := common.HexToAddress("0x1")
				require.Contains(t, analysis.BalanceChanges, addr)
				assert.Equal(t, big.NewInt(500), analysis.BalanceChanges[addr])
				assert.Equal(t, big.NewInt(500), analysis.NetValue)
			},
		},
		{
			name: "negative balance change",
			preState: &interfaces.StateSnapshot{
				BlockNumber: big.NewInt(100),
				Timestamp:   time.Now(),
				Accounts: map[common.Address]*interfaces.AccountState{
					common.HexToAddress("0x1"): {Balance: big.NewInt(1000)},
				},
				TokenPrices: make(map[common.Address]*big.Int),
			},
			postState: &interfaces.StateSnapshot{
				BlockNumber: big.NewInt(101),
				Timestamp:   time.Now(),
				Accounts: map[common.Address]*interfaces.AccountState{
					common.HexToAddress("0x1"): {Balance: big.NewInt(800)},
				},
				TokenPrices: make(map[common.Address]*big.Int),
			},
			expectError: false,
			validate: func(t *testing.T, analysis *interfaces.StateAnalysis) {
				addr := common.HexToAddress("0x1")
				require.Contains(t, analysis.BalanceChanges, addr)
				assert.Equal(t, big.NewInt(-200), analysis.BalanceChanges[addr])
				assert.Equal(t, big.NewInt(-200), analysis.NetValue)
			},
		},
		{
			name: "new account created",
			preState: &interfaces.StateSnapshot{
				BlockNumber: big.NewInt(100),
				Timestamp:   time.Now(),
				Accounts:    make(map[common.Address]*interfaces.AccountState),
				TokenPrices: make(map[common.Address]*big.Int),
			},
			postState: &interfaces.StateSnapshot{
				BlockNumber: big.NewInt(101),
				Timestamp:   time.Now(),
				Accounts: map[common.Address]*interfaces.AccountState{
					common.HexToAddress("0x1"): {Balance: big.NewInt(1000)},
				},
				TokenPrices: make(map[common.Address]*big.Int),
			},
			expectError: false,
			validate: func(t *testing.T, analysis *interfaces.StateAnalysis) {
				addr := common.HexToAddress("0x1")
				require.Contains(t, analysis.BalanceChanges, addr)
				assert.Equal(t, big.NewInt(1000), analysis.BalanceChanges[addr])
				assert.Equal(t, big.NewInt(1000), analysis.NetValue)
			},
		},
		{
			name: "price changes",
			preState: &interfaces.StateSnapshot{
				BlockNumber: big.NewInt(100),
				Timestamp:   time.Now(),
				Accounts:    make(map[common.Address]*interfaces.AccountState),
				TokenPrices: map[common.Address]*big.Int{
					common.HexToAddress("0x1"): big.NewInt(1000),
				},
			},
			postState: &interfaces.StateSnapshot{
				BlockNumber: big.NewInt(101),
				Timestamp:   time.Now(),
				Accounts:    make(map[common.Address]*interfaces.AccountState),
				TokenPrices: map[common.Address]*big.Int{
					common.HexToAddress("0x1"): big.NewInt(1100),
				},
			},
			expectError: false,
			validate: func(t *testing.T, analysis *interfaces.StateAnalysis) {
				addr := common.HexToAddress("0x1")
				require.Contains(t, analysis.PriceChanges, addr)
				priceChange := analysis.PriceChanges[addr]
				assert.Equal(t, big.NewInt(1000), priceChange.OldPrice)
				assert.Equal(t, big.NewInt(1100), priceChange.NewPrice)
				assert.Equal(t, big.NewInt(100), priceChange.Change)
				assert.Equal(t, 10.0, priceChange.ChangePercent)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis, err := analyzer.AnalyzeStateChanges(tt.preState, tt.postState)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, analysis)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, analysis)
				if tt.validate != nil {
					tt.validate(t, analysis)
				}
			}
		})
	}
}

func TestCalculateGasUsage(t *testing.T) {
	analyzer, err := NewStateAnalyzer()
	require.NoError(t, err)

	tests := []struct {
		name        string
		result      *interfaces.SimulationResult
		expectError bool
		validate    func(t *testing.T, analysis *interfaces.GasAnalysis)
	}{
		{
			name:        "nil result",
			result:      nil,
			expectError: true,
		},
		{
			name: "valid gas calculation",
			result: &interfaces.SimulationResult{
				Success:  true,
				GasUsed:  21000,
				GasPrice: big.NewInt(20000000000), // 20 gwei
			},
			expectError: false,
			validate: func(t *testing.T, analysis *interfaces.GasAnalysis) {
				assert.Equal(t, uint64(21000), analysis.GasUsed)
				assert.Equal(t, big.NewInt(20000000000), analysis.GasPrice)
				expectedCost := big.NewInt(420000000000000) // 21000 * 20 gwei
				assert.Equal(t, expectedCost, analysis.TotalCost)
				assert.Equal(t, 1.0, analysis.Efficiency) // Placeholder value
			},
		},
		{
			name: "high gas usage",
			result: &interfaces.SimulationResult{
				Success:  true,
				GasUsed:  500000,
				GasPrice: big.NewInt(50000000000), // 50 gwei
			},
			expectError: false,
			validate: func(t *testing.T, analysis *interfaces.GasAnalysis) {
				assert.Equal(t, uint64(500000), analysis.GasUsed)
				assert.Equal(t, big.NewInt(50000000000), analysis.GasPrice)
				expectedCost := big.NewInt(25000000000000000) // 500000 * 50 gwei
				assert.Equal(t, expectedCost, analysis.TotalCost)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis, err := analyzer.CalculateGasUsage(tt.result)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, analysis)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, analysis)
				if tt.validate != nil {
					tt.validate(t, analysis)
				}
			}
		})
	}
}

func TestExtractEventLogs(t *testing.T) {
	analyzer, err := NewStateAnalyzer()
	require.NoError(t, err)

	tests := []struct {
		name        string
		result      *interfaces.SimulationResult
		expectError bool
		validate    func(t *testing.T, logs []*interfaces.EventLog)
	}{
		{
			name:        "nil result",
			result:      nil,
			expectError: true,
		},
		{
			name: "no logs",
			result: &interfaces.SimulationResult{
				Success: true,
				Logs:    []*ethtypes.Log{},
			},
			expectError: false,
			validate: func(t *testing.T, logs []*interfaces.EventLog) {
				assert.Empty(t, logs)
			},
		},
		{
			name: "ERC-20 Transfer event",
			result: &interfaces.SimulationResult{
				Success: true,
				Logs: []*ethtypes.Log{
					{
						Address: common.HexToAddress("0x1"),
						Topics: []common.Hash{
							common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"), // Transfer event signature
							common.HexToHash("0x000000000000000000000000" + "1111111111111111111111111111111111111111"), // from
							common.HexToHash("0x000000000000000000000000" + "2222222222222222222222222222222222222222"), // to
						},
						Data: common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000064"), // value = 100
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, logs []*interfaces.EventLog) {
				require.Len(t, logs, 1)
				log := logs[0]
				assert.Equal(t, common.HexToAddress("0x1"), log.Address)
				assert.Len(t, log.Topics, 3)
				
				// Check decoded data
				require.Contains(t, log.Decoded, "event")
				assert.Equal(t, "Transfer", log.Decoded["event"])
				require.Contains(t, log.Decoded, "value")
				assert.Equal(t, big.NewInt(100), log.Decoded["value"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs, err := analyzer.ExtractEventLogs(tt.result)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, logs)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, logs)
				if tt.validate != nil {
					tt.validate(t, logs)
				}
			}
		})
	}
}

func TestMeasurePriceImpact(t *testing.T) {
	analyzer, err := NewStateAnalyzer()
	require.NoError(t, err)

	tests := []struct {
		name        string
		result      *interfaces.SimulationResult
		expectError bool
		validate    func(t *testing.T, impact *interfaces.PriceImpact)
	}{
		{
			name:        "nil result",
			result:      nil,
			expectError: true,
		},
		{
			name: "no swap events",
			result: &interfaces.SimulationResult{
				Success: true,
				Logs:    []*ethtypes.Log{},
			},
			expectError: false,
			validate: func(t *testing.T, impact *interfaces.PriceImpact) {
				assert.Equal(t, common.Address{}, impact.Token)
				assert.Equal(t, common.Address{}, impact.Pool)
				assert.Equal(t, int64(0), impact.ImpactBps)
				assert.Equal(t, 0.0, impact.ImpactPercent)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			impact, err := analyzer.MeasurePriceImpact(tt.result)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, impact)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, impact)
				if tt.validate != nil {
					tt.validate(t, impact)
				}
			}
		})
	}
}

func TestDecodeTransferEvent(t *testing.T) {
	analyzer, err := NewStateAnalyzer()
	require.NoError(t, err)
	sa := analyzer.(*stateAnalyzer)

	tests := []struct {
		name        string
		log         *ethtypes.Log
		expectError bool
		validate    func(t *testing.T, decoded map[string]interface{})
	}{
		{
			name: "insufficient topics",
			log: &ethtypes.Log{
				Topics: []common.Hash{
					common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"),
				},
			},
			expectError: true,
		},
		{
			name: "valid transfer event",
			log: &ethtypes.Log{
				Topics: []common.Hash{
					common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"), // Transfer signature
					common.HexToHash("0x000000000000000000000000" + "1111111111111111111111111111111111111111"), // from
					common.HexToHash("0x000000000000000000000000" + "2222222222222222222222222222222222222222"), // to
				},
				Data: common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000064"), // value = 100
			},
			expectError: false,
			validate: func(t *testing.T, decoded map[string]interface{}) {
				assert.Equal(t, "Transfer", decoded["event"])
				assert.Equal(t, common.HexToAddress("0x1111111111111111111111111111111111111111"), decoded["from"])
				assert.Equal(t, common.HexToAddress("0x2222222222222222222222222222222222222222"), decoded["to"])
				assert.Equal(t, big.NewInt(100), decoded["value"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := sa.decodeTransferEvent(tt.log)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, decoded)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, decoded)
				if tt.validate != nil {
					tt.validate(t, decoded)
				}
			}
		})
	}
}

func TestDecodeUniswapV2SwapEvent(t *testing.T) {
	analyzer, err := NewStateAnalyzer()
	require.NoError(t, err)
	sa := analyzer.(*stateAnalyzer)

	tests := []struct {
		name        string
		log         *ethtypes.Log
		expectError bool
		validate    func(t *testing.T, decoded map[string]interface{})
	}{
		{
			name: "insufficient topics",
			log: &ethtypes.Log{
				Topics: []common.Hash{
					common.HexToHash("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822"),
				},
			},
			expectError: true,
		},
		{
			name: "valid uniswap v2 swap event",
			log: &ethtypes.Log{
				Topics: []common.Hash{
					common.HexToHash("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822"), // Swap signature
					common.HexToHash("0x000000000000000000000000" + "1111111111111111111111111111111111111111"), // sender
					common.HexToHash("0x000000000000000000000000" + "2222222222222222222222222222222222222222"), // to
				},
				Data: func() []byte {
					// 4 uint256 values: amount0In, amount1In, amount0Out, amount1Out
					data := make([]byte, 128)
					copy(data[28:32], []byte{0, 0, 0, 100})    // amount0In = 100
					copy(data[60:64], []byte{0, 0, 0, 0})      // amount1In = 0
					copy(data[92:96], []byte{0, 0, 0, 0})      // amount0Out = 0
					copy(data[124:128], []byte{0, 0, 0, 200})  // amount1Out = 200
					return data
				}(),
			},
			expectError: false,
			validate: func(t *testing.T, decoded map[string]interface{}) {
				assert.Equal(t, "UniswapV2Swap", decoded["event"])
				assert.Equal(t, common.HexToAddress("0x1111111111111111111111111111111111111111"), decoded["sender"])
				assert.Equal(t, common.HexToAddress("0x2222222222222222222222222222222222222222"), decoded["to"])
				assert.Equal(t, 0, big.NewInt(100).Cmp(decoded["amount0In"].(*big.Int)))
				assert.Equal(t, 0, big.NewInt(0).Cmp(decoded["amount1In"].(*big.Int)))
				assert.Equal(t, 0, big.NewInt(0).Cmp(decoded["amount0Out"].(*big.Int)))
				assert.Equal(t, 0, big.NewInt(200).Cmp(decoded["amount1Out"].(*big.Int)))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := sa.decodeUniswapV2SwapEvent(tt.log)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, decoded)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, decoded)
				if tt.validate != nil {
					tt.validate(t, decoded)
				}
			}
		})
	}
}

func TestIsSwapEvent(t *testing.T) {
	analyzer, err := NewStateAnalyzer()
	require.NoError(t, err)
	sa := analyzer.(*stateAnalyzer)

	tests := []struct {
		name     string
		log      *ethtypes.Log
		expected bool
	}{
		{
			name: "no topics",
			log: &ethtypes.Log{
				Topics: []common.Hash{},
			},
			expected: false,
		},
		{
			name: "uniswap v2 swap event",
			log: &ethtypes.Log{
				Topics: []common.Hash{
					common.HexToHash("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822"), // Uniswap V2 Swap signature
				},
			},
			expected: true,
		},
		{
			name: "uniswap v3 swap event",
			log: &ethtypes.Log{
				Topics: []common.Hash{
					common.HexToHash("0xc42079f94a6350d7e6235f29174924f928cc2ac818eb64fed8004e115fbcca67"), // Uniswap V3 Swap signature
				},
			},
			expected: true,
		},
		{
			name: "transfer event (not swap)",
			log: &ethtypes.Log{
				Topics: []common.Hash{
					common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"), // Transfer signature
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sa.isSwapEvent(tt.log)
			assert.Equal(t, tt.expected, result)
		})
	}
}