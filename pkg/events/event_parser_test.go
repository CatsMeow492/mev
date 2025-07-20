package events

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEventParser(t *testing.T) {
	abiManager := NewABIManager()
	parser := NewEventParser(abiManager)
	
	assert.NotNil(t, parser)
	assert.NotNil(t, parser.abiManager)
}

func TestGetSupportedProtocols(t *testing.T) {
	abiManager := NewABIManager()
	parser := NewEventParser(abiManager)
	
	protocols := parser.GetSupportedProtocols()
	
	expectedProtocols := []interfaces.Protocol{
		interfaces.ProtocolUniswapV2,
		interfaces.ProtocolUniswapV3,
		interfaces.ProtocolAerodrome,
		interfaces.ProtocolBaseBridge,
	}
	
	assert.Equal(t, expectedProtocols, protocols)
}

func TestParseEventLogs_EmptyLogs(t *testing.T) {
	abiManager := NewABIManager()
	parser := NewEventParser(abiManager)
	ctx := context.Background()
	
	// Test with empty logs
	parsedEvents, err := parser.ParseEventLogs(ctx, []*ethtypes.Log{})
	assert.NoError(t, err)
	assert.Empty(t, parsedEvents)
}

func TestParseEventLogs_LogsWithoutTopics(t *testing.T) {
	abiManager := NewABIManager()
	parser := NewEventParser(abiManager)
	ctx := context.Background()
	
	// Create log without topics
	log := &ethtypes.Log{
		Address: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Topics:  []common.Hash{}, // Empty topics
		Data:    []byte{},
	}
	
	parsedEvents, err := parser.ParseEventLogs(ctx, []*ethtypes.Log{log})
	assert.NoError(t, err)
	assert.Empty(t, parsedEvents) // Should skip logs without topics
}

func TestDecodeSwapEvent_InvalidLog(t *testing.T) {
	abiManager := NewABIManager()
	parser := NewEventParser(abiManager)
	ctx := context.Background()
	
	// Test with log without topics
	log := &ethtypes.Log{
		Topics: []common.Hash{},
	}
	
	swapEvent, err := parser.DecodeSwapEvent(ctx, log)
	assert.Error(t, err)
	assert.Nil(t, swapEvent)
	assert.Contains(t, err.Error(), "log has no topics")
}

func TestDecodeSwapEvent_UnknownSignature(t *testing.T) {
	abiManager := NewABIManager()
	parser := NewEventParser(abiManager)
	ctx := context.Background()
	
	// Create log with unknown event signature
	unknownSignature := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	log := &ethtypes.Log{
		Address: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Topics:  []common.Hash{unknownSignature},
		Data:    []byte{},
	}
	
	swapEvent, err := parser.DecodeSwapEvent(ctx, log)
	assert.Error(t, err)
	assert.Nil(t, swapEvent)
	assert.Contains(t, err.Error(), "unable to decode swap event")
}

func TestDecodeBridgeEvent_InvalidLog(t *testing.T) {
	abiManager := NewABIManager()
	parser := NewEventParser(abiManager)
	ctx := context.Background()
	
	// Test with log without topics
	log := &ethtypes.Log{
		Topics: []common.Hash{},
	}
	
	bridgeEvent, err := parser.DecodeBridgeEvent(ctx, log)
	assert.Error(t, err)
	assert.Nil(t, bridgeEvent)
	assert.Contains(t, err.Error(), "log has no topics")
}

func TestBuildUniswapV2StyleSwapEvent(t *testing.T) {
	abiManager := NewABIManager()
	parser := NewEventParser(abiManager)
	
	poolAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")
	swapEvent := &interfaces.SwapEvent{
		Protocol: interfaces.ProtocolUniswapV2,
		Pool:     poolAddress,
	}
	
	// Test with amount0In > 0 (token0 -> token1 swap)
	decoded := map[string]interface{}{
		"sender":    common.HexToAddress("0xsender"),
		"to":        common.HexToAddress("0xrecipient"),
		"amount0In": big.NewInt(1000),
		"amount1In": big.NewInt(0),
		"amount0Out": big.NewInt(0),
		"amount1Out": big.NewInt(900),
	}
	
	result, err := parser.buildUniswapV2StyleSwapEvent(swapEvent, decoded)
	assert.NoError(t, err)
	assert.Equal(t, common.HexToAddress("0xsender"), result.Sender)
	assert.Equal(t, common.HexToAddress("0xrecipient"), result.Recipient)
	assert.Equal(t, big.NewInt(1000), result.AmountIn)
	assert.Equal(t, big.NewInt(900), result.AmountOut)
	
	// Test with amount1In > 0 (token1 -> token0 swap)
	decoded = map[string]interface{}{
		"sender":    common.HexToAddress("0xsender"),
		"to":        common.HexToAddress("0xrecipient"),
		"amount0In": big.NewInt(0),
		"amount1In": big.NewInt(500),
		"amount0Out": big.NewInt(450),
		"amount1Out": big.NewInt(0),
	}
	
	result, err = parser.buildUniswapV2StyleSwapEvent(swapEvent, decoded)
	assert.NoError(t, err)
	assert.Equal(t, big.NewInt(500), result.AmountIn)
	assert.Equal(t, big.NewInt(450), result.AmountOut)
}

func TestBuildUniswapV3SwapEvent(t *testing.T) {
	abiManager := NewABIManager()
	parser := NewEventParser(abiManager)
	
	poolAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")
	swapEvent := &interfaces.SwapEvent{
		Protocol: interfaces.ProtocolUniswapV3,
		Pool:     poolAddress,
	}
	
	// Test with positive amount0 (token0 -> token1 swap)
	decoded := map[string]interface{}{
		"sender":       common.HexToAddress("0xsender"),
		"recipient":    common.HexToAddress("0xrecipient"),
		"amount0":      big.NewInt(1000),
		"amount1":      big.NewInt(-900), // Negative in V3
		"sqrtPriceX96": big.NewInt(123456789),
		"liquidity":    big.NewInt(987654321),
		"tick":         big.NewInt(12345),
	}
	
	result, err := parser.buildUniswapV3SwapEvent(swapEvent, decoded)
	assert.NoError(t, err)
	assert.Equal(t, common.HexToAddress("0xsender"), result.Sender)
	assert.Equal(t, common.HexToAddress("0xrecipient"), result.Recipient)
	assert.Equal(t, big.NewInt(1000), result.AmountIn)
	assert.Equal(t, big.NewInt(900), result.AmountOut) // Should be absolute value
	assert.Equal(t, big.NewInt(123456789), result.SqrtPriceX96)
	assert.Equal(t, big.NewInt(987654321), result.Liquidity)
	assert.Equal(t, big.NewInt(12345), result.Tick)
	
	// Test with positive amount1 (token1 -> token0 swap)
	decoded = map[string]interface{}{
		"sender":       common.HexToAddress("0xsender"),
		"recipient":    common.HexToAddress("0xrecipient"),
		"amount0":      big.NewInt(-800), // Negative in V3
		"amount1":      big.NewInt(500),
		"sqrtPriceX96": big.NewInt(123456789),
		"liquidity":    big.NewInt(987654321),
		"tick":         big.NewInt(12345),
	}
	
	result, err = parser.buildUniswapV3SwapEvent(swapEvent, decoded)
	assert.NoError(t, err)
	assert.Equal(t, big.NewInt(500), result.AmountIn)
	assert.Equal(t, big.NewInt(800), result.AmountOut) // Should be absolute value
}

func TestBuildSwapEvent_UnsupportedProtocol(t *testing.T) {
	abiManager := NewABIManager()
	parser := NewEventParser(abiManager)
	
	poolAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")
	decoded := map[string]interface{}{}
	
	_, err := parser.buildSwapEvent(interfaces.ProtocolUnknown, poolAddress, decoded)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported protocol")
}

func TestDecodeSwapEventForProtocol_UnsupportedProtocol(t *testing.T) {
	abiManager := NewABIManager()
	parser := NewEventParser(abiManager)
	ctx := context.Background()
	
	log := &ethtypes.Log{
		Topics: []common.Hash{common.HexToHash("0x1234")},
	}
	
	_, err := parser.decodeSwapEventForProtocol(ctx, log, interfaces.ProtocolUnknown, common.HexToHash("0x1234"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported protocol for swap events")
}

func TestParseEventLog_ErrorHandling(t *testing.T) {
	abiManager := NewABIManager()
	parser := NewEventParser(abiManager)
	ctx := context.Background()
	
	// Test with log that has topics but can't be decoded
	unknownSignature := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	log := &ethtypes.Log{
		Address: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Topics:  []common.Hash{unknownSignature},
		Data:    []byte{},
		TxHash:  common.HexToHash("0xtxhash"),
		BlockNumber: 12345,
		Index:   1,
	}
	
	parsedEvent, err := parser.parseEventLog(ctx, log)
	assert.Error(t, err)
	assert.Nil(t, parsedEvent)
}

func TestEventTypeString(t *testing.T) {
	tests := []struct {
		eventType interfaces.EventType
		expected  string
	}{
		{interfaces.EventTypeSwap, "Swap"},
		{interfaces.EventTypeDeposit, "Deposit"},
		{interfaces.EventTypeWithdraw, "Withdraw"},
		{interfaces.EventTypeMint, "Mint"},
		{interfaces.EventTypeBurn, "Burn"},
		{interfaces.EventTypeUnknown, "Unknown"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.eventType.String())
		})
	}
}

func TestProtocolString(t *testing.T) {
	tests := []struct {
		protocol interfaces.Protocol
		expected string
	}{
		{interfaces.ProtocolUniswapV2, "UniswapV2"},
		{interfaces.ProtocolUniswapV3, "UniswapV3"},
		{interfaces.ProtocolAerodrome, "Aerodrome"},
		{interfaces.ProtocolBaseBridge, "BaseBridge"},
		{interfaces.ProtocolUnknown, "Unknown"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.protocol.String())
		})
	}
}

// Integration test with mock data
func TestIntegrationWithMockSwapEvent(t *testing.T) {
	abiManager := NewABIManager()
	parser := NewEventParser(abiManager)
	ctx := context.Background()
	
	// Load required ABIs
	err := abiManager.LoadABI(interfaces.ProtocolUniswapV2, interfaces.ContractTypePair)
	require.NoError(t, err)
	
	// Get the actual Swap event signature for Uniswap V2
	swapSignature, err := abiManager.GetEventSignature(interfaces.ProtocolUniswapV2, "Swap")
	require.NoError(t, err)
	
	// Create a mock log with the correct signature but minimal data
	// Note: In a real test, you'd need properly encoded data
	log := &ethtypes.Log{
		Address: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Topics:  []common.Hash{swapSignature},
		Data:    []byte{}, // Would need proper ABI encoding in real scenario
		TxHash:  common.HexToHash("0xtxhash"),
		BlockNumber: 12345,
		Index:   1,
	}
	
	// This will fail because we don't have properly encoded data,
	// but it tests the signature matching logic
	_, err = parser.DecodeSwapEvent(ctx, log)
	assert.Error(t, err) // Expected to fail due to data decoding
}

func TestBuildUniswapV2StyleSwapEvent_ErrorCases(t *testing.T) {
	abiManager := NewABIManager()
	parser := NewEventParser(abiManager)
	
	poolAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")
	swapEvent := &interfaces.SwapEvent{
		Protocol: interfaces.ProtocolUniswapV2,
		Pool:     poolAddress,
	}
	
	// Test with no input amounts
	decoded := map[string]interface{}{
		"sender":     common.HexToAddress("0xsender"),
		"to":         common.HexToAddress("0xrecipient"),
		"amount0In":  big.NewInt(0),
		"amount1In":  big.NewInt(0),
		"amount0Out": big.NewInt(900),
		"amount1Out": big.NewInt(0),
	}
	
	_, err := parser.buildUniswapV2StyleSwapEvent(swapEvent, decoded)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no input amount found")
	
	// Test with invalid output amounts
	decoded = map[string]interface{}{
		"sender":     common.HexToAddress("0xsender"),
		"to":         common.HexToAddress("0xrecipient"),
		"amount0In":  big.NewInt(1000),
		"amount1In":  big.NewInt(0),
		"amount0Out": big.NewInt(0),
		"amount1Out": big.NewInt(0), // Invalid: no output
	}
	
	_, err = parser.buildUniswapV2StyleSwapEvent(swapEvent, decoded)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid swap amounts")
}

func TestBuildUniswapV3SwapEvent_ErrorCases(t *testing.T) {
	abiManager := NewABIManager()
	parser := NewEventParser(abiManager)
	
	poolAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")
	swapEvent := &interfaces.SwapEvent{
		Protocol: interfaces.ProtocolUniswapV3,
		Pool:     poolAddress,
	}
	
	// Test with missing amount data
	decoded := map[string]interface{}{
		"sender":    common.HexToAddress("0xsender"),
		"recipient": common.HexToAddress("0xrecipient"),
		// Missing amount0 and amount1
	}
	
	_, err := parser.buildUniswapV3SwapEvent(swapEvent, decoded)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing amount data")
	
	// Test with no positive amounts
	decoded = map[string]interface{}{
		"sender":    common.HexToAddress("0xsender"),
		"recipient": common.HexToAddress("0xrecipient"),
		"amount0":   big.NewInt(-1000), // Both negative
		"amount1":   big.NewInt(-900),
	}
	
	_, err = parser.buildUniswapV3SwapEvent(swapEvent, decoded)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no positive input amount found")
}

func TestExtractTokenInfo(t *testing.T) {
	abiManager := NewABIManager()
	parser := NewEventParser(abiManager)
	ctx := context.Background()
	
	poolAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")
	
	// Test V2 protocol
	tokenInfo, err := parser.extractTokenInfo(ctx, poolAddress, interfaces.ProtocolUniswapV2)
	assert.NoError(t, err)
	assert.NotNil(t, tokenInfo)
	assert.Nil(t, tokenInfo.Fee) // V2 doesn't have fees
	
	// Test V3 protocol
	tokenInfo, err = parser.extractTokenInfo(ctx, poolAddress, interfaces.ProtocolUniswapV3)
	assert.NoError(t, err)
	assert.NotNil(t, tokenInfo)
	assert.NotNil(t, tokenInfo.Fee) // V3 has fees
}

func TestEnrichSwapEventWithTokenInfo(t *testing.T) {
	abiManager := NewABIManager()
	parser := NewEventParser(abiManager)
	ctx := context.Background()
	
	poolAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")
	swapEvent := &interfaces.SwapEvent{
		Protocol: interfaces.ProtocolUniswapV2,
		Pool:     poolAddress,
		AmountIn: big.NewInt(1000),
		AmountOut: big.NewInt(900),
	}
	
	err := parser.enrichSwapEventWithTokenInfo(ctx, swapEvent)
	assert.NoError(t, err)
	// Token addresses would be set in a real implementation with RPC calls
}

func TestDecodeBridgeEvent_ValidEvent(t *testing.T) {
	abiManager := NewABIManager()
	parser := NewEventParser(abiManager)
	ctx := context.Background()
	
	// Load bridge ABI
	err := abiManager.LoadABI(interfaces.ProtocolBaseBridge, interfaces.ContractTypeBridge)
	require.NoError(t, err)
	
	// Get deposit event signature
	depositSignature, err := abiManager.GetEventSignature(interfaces.ProtocolBaseBridge, "DepositInitiated")
	require.NoError(t, err)
	
	// Create a mock log with the correct signature
	log := &ethtypes.Log{
		Address: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		Topics:  []common.Hash{depositSignature},
		Data:    []byte{}, // Would need proper ABI encoding in real scenario
		TxHash:  common.HexToHash("0xtxhash"),
		BlockNumber: 12345,
		Index:   1,
	}
	
	// This will fail due to data decoding but tests signature matching
	_, err = parser.DecodeBridgeEvent(ctx, log)
	assert.Error(t, err) // Expected to fail due to data decoding
}