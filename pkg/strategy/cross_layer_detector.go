package strategy

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

// CrossLayerDetectorImpl implements the CrossLayerArbitrageDetector interface
type CrossLayerDetectorImpl struct {
	config         *interfaces.CrossLayerConfig
	eventParser    interfaces.EventParser
	priceOracle    PriceOracle
	bridgeContract common.Address
}

// PriceOracle interface for getting token prices on different layers
type PriceOracle interface {
	GetL1Price(ctx context.Context, token string) (*big.Int, error)
	GetL2Price(ctx context.Context, token string) (*big.Int, error)
}

// NewCrossLayerDetector creates a new cross-layer arbitrage detector
func NewCrossLayerDetector(config *interfaces.CrossLayerConfig, eventParser interfaces.EventParser, priceOracle PriceOracle, bridgeContract common.Address) *CrossLayerDetectorImpl {
	return &CrossLayerDetectorImpl{
		config:         config,
		eventParser:    eventParser,
		priceOracle:    priceOracle,
		bridgeContract: bridgeContract,
	}
}

// DetectOpportunity detects cross-layer arbitrage opportunities from bridge events
func (d *CrossLayerDetectorImpl) DetectOpportunity(ctx context.Context, bridgeEvent *interfaces.BridgeEvent, l1Price, l2Price *big.Int) (*interfaces.CrossLayerOpportunity, error) {
	if bridgeEvent == nil {
		return nil, fmt.Errorf("bridge event is nil")
	}

	if l1Price == nil || l2Price == nil {
		return nil, fmt.Errorf("prices cannot be nil")
	}

	// Check if the amount meets minimum threshold
	if bridgeEvent.Amount.Cmp(d.config.MinAmount) < 0 {
		return nil, fmt.Errorf("bridge amount %s below minimum threshold %s", bridgeEvent.Amount.String(), d.config.MinAmount.String())
	}

	// Check if the amount is within maximum threshold
	if bridgeEvent.Amount.Cmp(d.config.MaxAmount) > 0 {
		return nil, fmt.Errorf("bridge amount %s exceeds maximum threshold %s", bridgeEvent.Amount.String(), d.config.MaxAmount.String())
	}

	// Calculate price gap
	priceGap := new(big.Int).Sub(l1Price, l2Price)
	if priceGap.Sign() < 0 {
		priceGap = priceGap.Neg(priceGap)
	}

	// Check if price gap meets minimum threshold
	if priceGap.Cmp(d.config.MinPriceGap) < 0 {
		return nil, fmt.Errorf("price gap %s below minimum threshold %s", priceGap.String(), d.config.MinPriceGap.String())
	}

	// Determine arbitrage direction
	var direction interfaces.ArbitrageDirection
	if l1Price.Cmp(l2Price) > 0 {
		direction = interfaces.DirectionL1ToL2
	} else {
		direction = interfaces.DirectionL2ToL1
	}

	// Calculate expected profit
	expectedProfit := d.calculateExpectedProfit(bridgeEvent.Amount, priceGap, direction)

	// Check if profit meets minimum threshold
	if expectedProfit.Cmp(d.config.MinProfitThreshold) < 0 {
		return nil, fmt.Errorf("expected profit %s below minimum threshold %s", expectedProfit.String(), d.config.MinProfitThreshold.String())
	}

	// Get token address from bridge event
	tokenAddress := bridgeEvent.Token.Hex()

	opportunity := &interfaces.CrossLayerOpportunity{
		BridgeEvent:    bridgeEvent,
		Token:          tokenAddress,
		L1Price:        new(big.Int).Set(l1Price),
		L2Price:        new(big.Int).Set(l2Price),
		PriceGap:       priceGap,
		Amount:         new(big.Int).Set(bridgeEvent.Amount),
		ExpectedProfit: expectedProfit,
		Direction:      direction,
	}

	return opportunity, nil
}

// ComparePrices compares token prices between L1 and L2
func (d *CrossLayerDetectorImpl) ComparePrices(ctx context.Context, token string) (*interfaces.PriceComparison, error) {
	if token == "" {
		return nil, fmt.Errorf("token address cannot be empty")
	}

	// Check if token is supported
	if !d.isTokenSupported(token) {
		return nil, fmt.Errorf("token %s is not supported", token)
	}

	// Get L1 price
	l1Price, err := d.priceOracle.GetL1Price(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get L1 price for token %s: %w", token, err)
	}

	// Get L2 price
	l2Price, err := d.priceOracle.GetL2Price(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get L2 price for token %s: %w", token, err)
	}

	// Calculate price gap
	priceGap := new(big.Int).Sub(l1Price, l2Price)
	if priceGap.Sign() < 0 {
		priceGap = priceGap.Neg(priceGap)
	}

	comparison := &interfaces.PriceComparison{
		Token:     token,
		L1Price:   l1Price,
		L2Price:   l2Price,
		PriceGap:  priceGap,
		Timestamp: time.Now(),
	}

	return comparison, nil
}

// ConstructBridgeTransaction constructs a bridge transaction for the arbitrage opportunity
func (d *CrossLayerDetectorImpl) ConstructBridgeTransaction(ctx context.Context, opportunity *interfaces.CrossLayerOpportunity) (*types.Transaction, error) {
	if opportunity == nil {
		return nil, fmt.Errorf("opportunity is nil")
	}

	// Construct transaction data based on direction
	var txData []byte
	var to common.Address
	var value *big.Int

	switch opportunity.Direction {
	case interfaces.DirectionL1ToL2:
		// Construct deposit transaction
		txData = d.constructDepositData(opportunity)
		to = d.bridgeContract
		value = opportunity.Amount
	case interfaces.DirectionL2ToL1:
		// Construct withdrawal transaction
		txData = d.constructWithdrawalData(opportunity)
		to = d.bridgeContract
		value = big.NewInt(0) // Withdrawals typically don't send ETH
	default:
		return nil, fmt.Errorf("unknown arbitrage direction: %s", opportunity.Direction)
	}

	// Estimate gas limit (simplified)
	gasLimit := uint64(200000) // Base bridge operations typically use around 150-200k gas

	// Calculate gas price with premium for faster execution
	gasPrice := big.NewInt(20000000000) // 20 gwei base
	gasPremium := new(big.Int).Div(gasPrice, big.NewInt(10)) // 10% premium
	gasPrice = gasPrice.Add(gasPrice, gasPremium)

	tx := &types.Transaction{
		To:       &to,
		Value:    value,
		GasLimit: gasLimit,
		GasPrice: gasPrice,
		Data:     txData,
		Nonce:    0, // Will be set by transaction manager
	}

	opportunity.BridgeTx = tx

	return tx, nil
}

// GetConfiguration returns the current configuration
func (d *CrossLayerDetectorImpl) GetConfiguration() *interfaces.CrossLayerConfig {
	return d.config
}

// calculateExpectedProfit calculates the expected profit from the arbitrage
func (d *CrossLayerDetectorImpl) calculateExpectedProfit(amount, priceGap *big.Int, direction interfaces.ArbitrageDirection) *big.Int {
	// Calculate gross profit: (amount * priceGap) / 1000 (simplified calculation)
	// In a real implementation, this would be more sophisticated
	grossProfit := new(big.Int).Mul(amount, priceGap)
	grossProfit = grossProfit.Div(grossProfit, big.NewInt(1000)) // Simplified normalization

	// Subtract bridge fees
	netProfit := new(big.Int).Sub(grossProfit, d.config.BridgeFee)

	// Ensure profit is not negative
	if netProfit.Sign() < 0 {
		return big.NewInt(0)
	}

	return netProfit
}

// isTokenSupported checks if a token is supported for cross-layer arbitrage
func (d *CrossLayerDetectorImpl) isTokenSupported(token string) bool {
	for _, supportedToken := range d.config.SupportedTokens {
		if supportedToken == token {
			return true
		}
	}
	return false
}

// constructDepositData constructs transaction data for a deposit (L1 to L2)
func (d *CrossLayerDetectorImpl) constructDepositData(opportunity *interfaces.CrossLayerOpportunity) []byte {
	// TODO: Implement proper ABI encoding for bridge deposit
	// This would typically encode a function call like:
	// depositETH(address _to, uint32 _minGasLimit, bytes calldata _extraData)
	// or
	// depositERC20(address _l1Token, address _l2Token, address _to, uint256 _amount, uint32 _minGasLimit, bytes calldata _extraData)
	
	// For now, return placeholder data that represents a deposit call
	// In production, this would use the go-ethereum ABI package to encode the function call
	return []byte("deposit_placeholder")
}

// constructWithdrawalData constructs transaction data for a withdrawal (L2 to L1)
func (d *CrossLayerDetectorImpl) constructWithdrawalData(opportunity *interfaces.CrossLayerOpportunity) []byte {
	// TODO: Implement proper ABI encoding for bridge withdrawal
	// This would typically encode a function call like:
	// withdraw(address _token, uint256 _amount, uint32 _minGasLimit, bytes calldata _extraData)
	// or for ETH:
	// withdrawETH(uint256 _amount, uint32 _minGasLimit, bytes calldata _extraData)
	
	// For now, return placeholder data that represents a withdrawal call
	// In production, this would use the go-ethereum ABI package to encode the function call
	return []byte("withdraw_placeholder")
}

// ValidateOpportunity validates a cross-layer arbitrage opportunity
func (d *CrossLayerDetectorImpl) ValidateOpportunity(ctx context.Context, opportunity *interfaces.CrossLayerOpportunity) error {
	if opportunity == nil {
		return fmt.Errorf("opportunity is nil")
	}

	// Validate price gap is still profitable
	currentComparison, err := d.ComparePrices(ctx, opportunity.Token)
	if err != nil {
		return fmt.Errorf("failed to get current prices: %w", err)
	}

	// Check if price gap has changed significantly (more than 5%)
	priceDiff := new(big.Int).Sub(currentComparison.PriceGap, opportunity.PriceGap)
	if priceDiff.Sign() < 0 {
		priceDiff = priceDiff.Neg(priceDiff)
	}

	threshold := new(big.Int).Div(opportunity.PriceGap, big.NewInt(20)) // 5% threshold
	if priceDiff.Cmp(threshold) > 0 {
		return fmt.Errorf("price gap has changed significantly: original %s, current %s", 
			opportunity.PriceGap.String(), currentComparison.PriceGap.String())
	}

	// Recalculate expected profit with current prices
	currentProfit := d.calculateExpectedProfit(opportunity.Amount, currentComparison.PriceGap, opportunity.Direction)
	if currentProfit.Cmp(d.config.MinProfitThreshold) < 0 {
		return fmt.Errorf("opportunity no longer profitable: current profit %s below threshold %s",
			currentProfit.String(), d.config.MinProfitThreshold.String())
	}

	return nil
}

// AnalyzeBridgeEvents analyzes multiple bridge events and detects arbitrage opportunities
func (d *CrossLayerDetectorImpl) AnalyzeBridgeEvents(ctx context.Context, bridgeEvents []*interfaces.BridgeEvent) ([]*interfaces.CrossLayerOpportunity, error) {
	if len(bridgeEvents) == 0 {
		return []*interfaces.CrossLayerOpportunity{}, nil
	}

	var opportunities []*interfaces.CrossLayerOpportunity

	for _, bridgeEvent := range bridgeEvents {
		if bridgeEvent == nil {
			continue
		}

		// Get token address as string
		tokenAddress := bridgeEvent.Token.Hex()

		// Skip if token is not supported
		if !d.isTokenSupported(tokenAddress) {
			continue
		}

		// Get current price comparison
		priceComparison, err := d.ComparePrices(ctx, tokenAddress)
		if err != nil {
			// Log error but continue with other events
			continue
		}

		// Detect opportunity using the bridge event and current prices
		opportunity, err := d.DetectOpportunity(ctx, bridgeEvent, priceComparison.L1Price, priceComparison.L2Price)
		if err != nil {
			// Not profitable or doesn't meet criteria, continue
			continue
		}

		opportunities = append(opportunities, opportunity)
	}

	return opportunities, nil
}

// FilterProfitableBridgeEvents filters bridge events that could lead to profitable arbitrage
func (d *CrossLayerDetectorImpl) FilterProfitableBridgeEvents(ctx context.Context, bridgeEvents []*interfaces.BridgeEvent) ([]*interfaces.BridgeEvent, error) {
	if len(bridgeEvents) == 0 {
		return []*interfaces.BridgeEvent{}, nil
	}

	var profitableEvents []*interfaces.BridgeEvent

	for _, bridgeEvent := range bridgeEvents {
		if bridgeEvent == nil {
			continue
		}

		// Check if amount meets minimum threshold
		if bridgeEvent.Amount.Cmp(d.config.MinAmount) < 0 {
			continue
		}

		// Check if amount is within maximum threshold
		if bridgeEvent.Amount.Cmp(d.config.MaxAmount) > 0 {
			continue
		}

		// Get token address as string
		tokenAddress := bridgeEvent.Token.Hex()

		// Skip if token is not supported
		if !d.isTokenSupported(tokenAddress) {
			continue
		}

		// Get current price comparison to check if there's a profitable gap
		priceComparison, err := d.ComparePrices(ctx, tokenAddress)
		if err != nil {
			continue
		}

		// Check if price gap meets minimum threshold
		if priceComparison.PriceGap.Cmp(d.config.MinPriceGap) < 0 {
			continue
		}

		profitableEvents = append(profitableEvents, bridgeEvent)
	}

	return profitableEvents, nil
}

// EstimateArbitrageProfit estimates the potential profit from a bridge event without creating a full opportunity
func (d *CrossLayerDetectorImpl) EstimateArbitrageProfit(ctx context.Context, bridgeEvent *interfaces.BridgeEvent) (*big.Int, error) {
	if bridgeEvent == nil {
		return nil, fmt.Errorf("bridge event is nil")
	}

	tokenAddress := bridgeEvent.Token.Hex()

	// Get current price comparison
	priceComparison, err := d.ComparePrices(ctx, tokenAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get price comparison: %w", err)
	}

	// Determine direction based on price difference
	var direction interfaces.ArbitrageDirection
	if priceComparison.L1Price.Cmp(priceComparison.L2Price) > 0 {
		direction = interfaces.DirectionL1ToL2
	} else {
		direction = interfaces.DirectionL2ToL1
	}

	// Calculate expected profit
	expectedProfit := d.calculateExpectedProfit(bridgeEvent.Amount, priceComparison.PriceGap, direction)

	return expectedProfit, nil
}