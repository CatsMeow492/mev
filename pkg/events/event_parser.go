package events

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// EventParserImpl implements the EventParser interface
type EventParserImpl struct {
	abiManager interfaces.ABIManager
}

// TokenInfo holds token information extracted from pool contracts
type TokenInfo struct {
	Token0 common.Address
	Token1 common.Address
	Fee    *big.Int // For V3 pools
}

// NewEventParser creates a new event parser instance
func NewEventParser(abiManager interfaces.ABIManager) *EventParserImpl {
	return &EventParserImpl{
		abiManager: abiManager,
	}
}

// ParseEventLogs parses multiple event logs and returns parsed events
func (p *EventParserImpl) ParseEventLogs(ctx context.Context, logs []*ethtypes.Log) ([]*interfaces.ParsedEvent, error) {
	var parsedEvents []*interfaces.ParsedEvent
	
	for _, log := range logs {
		if len(log.Topics) == 0 {
			continue // Skip logs without topics
		}
		
		parsedEvent, err := p.parseEventLog(ctx, log)
		if err != nil {
			// Log error but continue processing other events
			continue
		}
		
		if parsedEvent != nil {
			parsedEvents = append(parsedEvents, parsedEvent)
		}
	}
	
	return parsedEvents, nil
}

// DecodeSwapEvent decodes a swap event from a log
func (p *EventParserImpl) DecodeSwapEvent(ctx context.Context, log *ethtypes.Log) (*interfaces.SwapEvent, error) {
	if len(log.Topics) == 0 {
		return nil, fmt.Errorf("log has no topics")
	}
	
	eventSignature := log.Topics[0]
	
	// Try to decode as different protocol swap events
	protocols := []interfaces.Protocol{
		interfaces.ProtocolUniswapV2,
		interfaces.ProtocolUniswapV3,
		interfaces.ProtocolAerodrome,
	}
	
	for _, protocol := range protocols {
		swapEvent, err := p.decodeSwapEventForProtocol(ctx, log, protocol, eventSignature)
		if err == nil && swapEvent != nil {
			return swapEvent, nil
		}
	}
	
	return nil, fmt.Errorf("unable to decode swap event with signature %s", eventSignature.Hex())
}

// DecodeBridgeEvent decodes a bridge event from a log
func (p *EventParserImpl) DecodeBridgeEvent(ctx context.Context, log *ethtypes.Log) (*interfaces.BridgeEvent, error) {
	if len(log.Topics) == 0 {
		return nil, fmt.Errorf("log has no topics")
	}
	
	eventSignature := log.Topics[0]
	
	// Try to decode as Base bridge event
	bridgeEvent, err := p.decodeBridgeEventForProtocol(ctx, log, interfaces.ProtocolBaseBridge, eventSignature)
	if err != nil {
		return nil, fmt.Errorf("unable to decode bridge event: %w", err)
	}
	
	return bridgeEvent, nil
}

// GetSupportedProtocols returns the list of supported protocols
func (p *EventParserImpl) GetSupportedProtocols() []interfaces.Protocol {
	return []interfaces.Protocol{
		interfaces.ProtocolUniswapV2,
		interfaces.ProtocolUniswapV3,
		interfaces.ProtocolAerodrome,
		interfaces.ProtocolBaseBridge,
	}
}

// parseEventLog parses a single event log
func (p *EventParserImpl) parseEventLog(ctx context.Context, log *ethtypes.Log) (*interfaces.ParsedEvent, error) {
	if len(log.Topics) == 0 {
		return nil, fmt.Errorf("log has no topics")
	}
	
	eventSignature := log.Topics[0]
	
	// Try to parse as swap event first
	swapEvent, err := p.DecodeSwapEvent(ctx, log)
	if err == nil && swapEvent != nil {
		return &interfaces.ParsedEvent{
			Protocol:    swapEvent.Protocol,
			EventType:   interfaces.EventTypeSwap,
			Address:     log.Address,
			TxHash:      log.TxHash,
			BlockNumber: log.BlockNumber,
			LogIndex:    log.Index,
			SwapEvent:   swapEvent,
			RawLog:      log,
		}, nil
	}
	
	// Try to parse as bridge event
	bridgeEvent, err := p.DecodeBridgeEvent(ctx, log)
	if err == nil && bridgeEvent != nil {
		eventType := interfaces.EventTypeDeposit
		if bridgeEvent.EventType == interfaces.EventTypeWithdraw {
			eventType = interfaces.EventTypeWithdraw
		}
		
		return &interfaces.ParsedEvent{
			Protocol:    interfaces.ProtocolBaseBridge,
			EventType:   eventType,
			Address:     log.Address,
			TxHash:      log.TxHash,
			BlockNumber: log.BlockNumber,
			LogIndex:    log.Index,
			BridgeEvent: bridgeEvent,
			RawLog:      log,
		}, nil
	}
	
	return nil, fmt.Errorf("unable to parse event with signature %s", eventSignature.Hex())
}

// decodeSwapEventForProtocol decodes a swap event for a specific protocol
func (p *EventParserImpl) decodeSwapEventForProtocol(ctx context.Context, log *ethtypes.Log, protocol interfaces.Protocol, eventSignature common.Hash) (*interfaces.SwapEvent, error) {
	var contractType interfaces.ContractType
	
	switch protocol {
	case interfaces.ProtocolUniswapV2, interfaces.ProtocolAerodrome:
		contractType = interfaces.ContractTypePair
	case interfaces.ProtocolUniswapV3:
		contractType = interfaces.ContractTypePool
	default:
		return nil, fmt.Errorf("unsupported protocol for swap events: %s", protocol.String())
	}
	
	abiBytes, err := p.abiManager.GetABI(protocol, contractType)
	if err != nil {
		return nil, fmt.Errorf("failed to get ABI for %s: %w", protocol.String(), err)
	}
	
	var parsedABI abi.ABI
	if err := parsedABI.UnmarshalJSON(abiBytes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ABI: %w", err)
	}
	
	// Find the Swap event
	swapEvent, exists := parsedABI.Events["Swap"]
	if !exists {
		return nil, fmt.Errorf("Swap event not found in ABI for %s", protocol.String())
	}
	
	if swapEvent.ID != eventSignature {
		return nil, fmt.Errorf("event signature mismatch for %s Swap event", protocol.String())
	}
	
	// Decode the event
	decoded := make(map[string]interface{})
	if err := parsedABI.UnpackIntoMap(decoded, "Swap", log.Data); err != nil {
		return nil, fmt.Errorf("failed to unpack Swap event data: %w", err)
	}
	
	// Parse indexed topics
	if err := abi.ParseTopicsIntoMap(decoded, swapEvent.Inputs, log.Topics[1:]); err != nil {
		return nil, fmt.Errorf("failed to parse indexed topics: %w", err)
	}
	
	builtSwapEvent, err := p.buildSwapEvent(protocol, log.Address, decoded)
	if err != nil {
		return nil, err
	}
	
	// Enrich with token information
	if err := p.enrichSwapEventWithTokenInfo(ctx, builtSwapEvent); err != nil {
		// Log warning but don't fail - token info is supplementary
		// In production, you might want to log this error
	}
	
	return builtSwapEvent, nil
}

// buildSwapEvent builds a SwapEvent from decoded data
func (p *EventParserImpl) buildSwapEvent(protocol interfaces.Protocol, poolAddress common.Address, decoded map[string]interface{}) (*interfaces.SwapEvent, error) {
	swapEvent := &interfaces.SwapEvent{
		Protocol: protocol,
		Pool:     poolAddress,
	}
	
	switch protocol {
	case interfaces.ProtocolUniswapV2, interfaces.ProtocolAerodrome:
		return p.buildUniswapV2StyleSwapEvent(swapEvent, decoded)
	case interfaces.ProtocolUniswapV3:
		return p.buildUniswapV3SwapEvent(swapEvent, decoded)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol.String())
	}
}

// buildUniswapV2StyleSwapEvent builds a swap event for Uniswap V2 style protocols
func (p *EventParserImpl) buildUniswapV2StyleSwapEvent(swapEvent *interfaces.SwapEvent, decoded map[string]interface{}) (*interfaces.SwapEvent, error) {
	// Extract sender
	if sender, ok := decoded["sender"].(common.Address); ok {
		swapEvent.Sender = sender
	}
	
	// Extract recipient
	if to, ok := decoded["to"].(common.Address); ok {
		swapEvent.Recipient = to
	}
	
	// Extract amounts
	amount0In, _ := decoded["amount0In"].(*big.Int)
	amount1In, _ := decoded["amount1In"].(*big.Int)
	amount0Out, _ := decoded["amount0Out"].(*big.Int)
	amount1Out, _ := decoded["amount1Out"].(*big.Int)
	
	// Validate amounts
	if amount0In == nil {
		amount0In = big.NewInt(0)
	}
	if amount1In == nil {
		amount1In = big.NewInt(0)
	}
	if amount0Out == nil {
		amount0Out = big.NewInt(0)
	}
	if amount1Out == nil {
		amount1Out = big.NewInt(0)
	}
	
	// Determine which token is input and which is output
	if amount0In.Cmp(big.NewInt(0)) > 0 {
		swapEvent.AmountIn = amount0In
		swapEvent.AmountOut = amount1Out
		// Token0 is input, Token1 is output - we'll need to fetch these from the pool contract
	} else if amount1In.Cmp(big.NewInt(0)) > 0 {
		swapEvent.AmountIn = amount1In
		swapEvent.AmountOut = amount0Out
		// Token1 is input, Token0 is output - we'll need to fetch these from the pool contract
	} else {
		return nil, fmt.Errorf("invalid swap event: no input amount found")
	}
	
	// Validate that we have both input and output amounts
	if swapEvent.AmountIn == nil || swapEvent.AmountOut == nil || 
		swapEvent.AmountIn.Cmp(big.NewInt(0)) <= 0 || swapEvent.AmountOut.Cmp(big.NewInt(0)) <= 0 {
		return nil, fmt.Errorf("invalid swap amounts: in=%v, out=%v", swapEvent.AmountIn, swapEvent.AmountOut)
	}
	
	return swapEvent, nil
}

// buildUniswapV3SwapEvent builds a swap event for Uniswap V3
func (p *EventParserImpl) buildUniswapV3SwapEvent(swapEvent *interfaces.SwapEvent, decoded map[string]interface{}) (*interfaces.SwapEvent, error) {
	// Extract sender and recipient
	if sender, ok := decoded["sender"].(common.Address); ok {
		swapEvent.Sender = sender
	}
	if recipient, ok := decoded["recipient"].(common.Address); ok {
		swapEvent.Recipient = recipient
	}
	
	// Extract amounts (can be negative in V3)
	amount0, _ := decoded["amount0"].(*big.Int)
	amount1, _ := decoded["amount1"].(*big.Int)
	
	// Validate amounts exist
	if amount0 == nil || amount1 == nil {
		return nil, fmt.Errorf("missing amount data in V3 swap event")
	}
	
	// Convert to positive amounts and determine direction
	if amount0.Cmp(big.NewInt(0)) > 0 {
		swapEvent.AmountIn = amount0
		swapEvent.AmountOut = new(big.Int).Abs(amount1)
		// Token0 is input, Token1 is output
	} else if amount1.Cmp(big.NewInt(0)) > 0 {
		swapEvent.AmountIn = amount1
		swapEvent.AmountOut = new(big.Int).Abs(amount0)
		// Token1 is input, Token0 is output
	} else {
		return nil, fmt.Errorf("invalid V3 swap event: no positive input amount found")
	}
	
	// Validate that we have valid amounts
	if swapEvent.AmountIn == nil || swapEvent.AmountOut == nil || 
		swapEvent.AmountIn.Cmp(big.NewInt(0)) <= 0 || swapEvent.AmountOut.Cmp(big.NewInt(0)) <= 0 {
		return nil, fmt.Errorf("invalid V3 swap amounts: in=%v, out=%v", swapEvent.AmountIn, swapEvent.AmountOut)
	}
	
	// Extract V3-specific fields
	if sqrtPriceX96, ok := decoded["sqrtPriceX96"].(*big.Int); ok {
		swapEvent.SqrtPriceX96 = sqrtPriceX96
	}
	if liquidity, ok := decoded["liquidity"].(*big.Int); ok {
		swapEvent.Liquidity = liquidity
	}
	if tick, ok := decoded["tick"].(*big.Int); ok {
		swapEvent.Tick = tick
	}
	
	return swapEvent, nil
}

// decodeBridgeEventForProtocol decodes a bridge event for a specific protocol
func (p *EventParserImpl) decodeBridgeEventForProtocol(ctx context.Context, log *ethtypes.Log, protocol interfaces.Protocol, eventSignature common.Hash) (*interfaces.BridgeEvent, error) {
	abiBytes, err := p.abiManager.GetABI(protocol, interfaces.ContractTypeBridge)
	if err != nil {
		return nil, fmt.Errorf("failed to get bridge ABI: %w", err)
	}
	
	var parsedABI abi.ABI
	if err := parsedABI.UnmarshalJSON(abiBytes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bridge ABI: %w", err)
	}
	
	// Try to find matching event
	var eventName string
	var eventType interfaces.EventType
	
	for name, event := range parsedABI.Events {
		if event.ID == eventSignature {
			eventName = name
			if name == "DepositInitiated" || name == "DepositFinalized" {
				eventType = interfaces.EventTypeDeposit
			} else if name == "WithdrawalInitiated" || name == "WithdrawalFinalized" {
				eventType = interfaces.EventTypeWithdraw
			}
			break
		}
	}
	
	if eventName == "" {
		return nil, fmt.Errorf("bridge event not found for signature %s", eventSignature.Hex())
	}
	
	// Decode the event
	decoded := make(map[string]interface{})
	if err := parsedABI.UnpackIntoMap(decoded, eventName, log.Data); err != nil {
		return nil, fmt.Errorf("failed to unpack bridge event data: %w", err)
	}
	
	// Parse indexed topics
	event := parsedABI.Events[eventName]
	if err := abi.ParseTopicsIntoMap(decoded, event.Inputs, log.Topics[1:]); err != nil {
		return nil, fmt.Errorf("failed to parse indexed topics: %w", err)
	}
	
	bridgeEvent := &interfaces.BridgeEvent{
		EventType: eventType,
	}
	
	// Extract common fields
	if from, ok := decoded["from"].(common.Address); ok {
		bridgeEvent.From = from
	}
	if to, ok := decoded["to"].(common.Address); ok {
		bridgeEvent.To = to
	}
	if amount, ok := decoded["amount"].(*big.Int); ok {
		bridgeEvent.Amount = amount
	}
	
	// Set transaction hashes
	bridgeEvent.L2TxHash = log.TxHash
	
	return bridgeEvent, nil
}

// extractTokenInfo extracts token addresses from a pool/pair contract
// Note: In a real implementation, this would make RPC calls to get token0() and token1()
// For now, we'll return placeholder values and document the need for RPC integration
func (p *EventParserImpl) extractTokenInfo(ctx context.Context, poolAddress common.Address, protocol interfaces.Protocol) (*TokenInfo, error) {
	// TODO: Implement RPC calls to extract token0() and token1() from the pool contract
	// This would require an ethereum client connection
	
	// For now, return a placeholder structure
	// In a real implementation, you would:
	// 1. Get the appropriate ABI for the pool contract
	// 2. Create contract instance with ethereum client
	// 3. Call token0() and token1() methods
	// 4. For V3, also call fee() method
	
	tokenInfo := &TokenInfo{
		Token0: common.Address{}, // Would be populated from RPC call
		Token1: common.Address{}, // Would be populated from RPC call
	}
	
	if protocol == interfaces.ProtocolUniswapV3 {
		tokenInfo.Fee = big.NewInt(0) // Would be populated from fee() call
	}
	
	return tokenInfo, nil
}

// enrichSwapEventWithTokenInfo adds token address information to swap events
func (p *EventParserImpl) enrichSwapEventWithTokenInfo(ctx context.Context, swapEvent *interfaces.SwapEvent) error {
	tokenInfo, err := p.extractTokenInfo(ctx, swapEvent.Pool, swapEvent.Protocol)
	if err != nil {
		return fmt.Errorf("failed to extract token info: %w", err)
	}
	
	// Set token addresses based on swap direction
	// This is a simplified approach - in reality, you'd need to determine
	// the actual swap direction from the amounts
	swapEvent.TokenIn = tokenInfo.Token0
	swapEvent.TokenOut = tokenInfo.Token1
	
	if swapEvent.Protocol == interfaces.ProtocolUniswapV3 {
		swapEvent.Fee = tokenInfo.Fee
	}
	
	return nil
}