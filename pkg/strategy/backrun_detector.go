package strategy

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

// backrunDetector implements the BackrunDetector interface
type backrunDetector struct {
	config *interfaces.BackrunConfig
}

// NewBackrunDetector creates a new backrun detector with the given configuration
func NewBackrunDetector(config *interfaces.BackrunConfig) interfaces.BackrunDetector {
	if config == nil {
		config = &interfaces.BackrunConfig{
			MinPriceGap:        big.NewInt(50),   // 0.5% minimum price gap in basis points
			MaxTradeSize:       big.NewInt(1000000), // 1M wei maximum trade size
			MinProfitThreshold: big.NewInt(100),  // $100 minimum profit
			SupportedPools: []string{
				"0x4200000000000000000000000000000000000006", // Base WETH
				"0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913", // Base USDC
			},
		}
	}
	return &backrunDetector{
		config: config,
	}
}

// DetectOpportunity analyzes a transaction to identify backrun arbitrage opportunities
func (b *backrunDetector) DetectOpportunity(ctx context.Context, tx *types.Transaction, simResult *interfaces.SimulationResult) (*interfaces.BackrunOpportunity, error) {
	// Only analyze swap transactions
	if tx.GetTransactionType() != types.TxTypeSwap {
		return nil, nil
	}

	// Check if simulation was successful
	if simResult == nil || !simResult.Success {
		return nil, nil
	}

	// Extract price impact from simulation
	priceImpact, err := b.extractPriceImpact(simResult)
	if err != nil {
		return nil, fmt.Errorf("failed to extract price impact: %w", err)
	}

	// Check if price impact creates arbitrage opportunity
	arbitrageOpportunity, err := b.findArbitrageOpportunity(tx, priceImpact)
	if err != nil {
		return nil, fmt.Errorf("failed to find arbitrage opportunity: %w", err)
	}

	if arbitrageOpportunity == nil {
		return nil, nil // No arbitrage opportunity found
	}

	return arbitrageOpportunity, nil
}

// CalculateOptimalTradeSize uses binary search to find the optimal trade size
func (b *backrunDetector) CalculateOptimalTradeSize(ctx context.Context, opportunity *interfaces.BackrunOpportunity) (*big.Int, error) {
	if opportunity == nil {
		return nil, errors.New("opportunity cannot be nil")
	}

	// Binary search parameters
	minSize := big.NewInt(1000)  // Minimum trade size (0.001 ETH)
	maxSize := b.config.MaxTradeSize
	tolerance := big.NewInt(100) // Tolerance for convergence

	// Binary search for optimal trade size
	optimalSize, err := b.binarySearchOptimalSize(ctx, opportunity, minSize, maxSize, tolerance)
	if err != nil {
		return nil, fmt.Errorf("binary search failed: %w", err)
	}

	return optimalSize, nil
}

// ValidateArbitrage validates that an arbitrage opportunity is still profitable
func (b *backrunDetector) ValidateArbitrage(ctx context.Context, opportunity *interfaces.BackrunOpportunity) error {
	if opportunity == nil {
		return errors.New("opportunity cannot be nil")
	}

	// Check if price gap meets minimum threshold
	if opportunity.PriceGap.Cmp(b.config.MinPriceGap) < 0 {
		return errors.New("price gap below minimum threshold")
	}

	// Check if expected profit meets minimum threshold
	if opportunity.ExpectedProfit.Cmp(b.config.MinProfitThreshold) < 0 {
		return errors.New("expected profit below minimum threshold")
	}

	// Validate that we have all required components
	if opportunity.TargetTx == nil {
		return errors.New("target transaction is required")
	}

	if opportunity.Pool1 == "" || opportunity.Pool2 == "" {
		return errors.New("both pools are required for arbitrage")
	}

	if opportunity.Token == "" {
		return errors.New("token address is required")
	}

	return nil
}

// GetConfiguration returns the current backrun detector configuration
func (b *backrunDetector) GetConfiguration() *interfaces.BackrunConfig {
	return b.config
}

// extractPriceImpact extracts price impact information from simulation results
func (b *backrunDetector) extractPriceImpact(simResult *interfaces.SimulationResult) (*interfaces.PriceImpact, error) {
	// Analyze logs to find Swap events
	for _, log := range simResult.Logs {
		// Check for Uniswap V2/V3 Swap event signature
		if len(log.Topics) > 0 {
			swapEventSig := common.HexToHash("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822") // Swap(address,uint256,uint256,uint256,uint256,address)
			if log.Topics[0] == swapEventSig {
				return b.parseSwapEvent(log)
			}
		}
	}

	return nil, errors.New("no swap events found in simulation result")
}

// parseSwapEvent parses a Swap event log to extract price impact
func (b *backrunDetector) parseSwapEvent(log *ethtypes.Log) (*interfaces.PriceImpact, error) {
	// This is a simplified implementation
	// In practice, you'd use the ABI decoder to properly parse the event
	
	if len(log.Data) < 128 { // 4 * 32 bytes for amounts
		return nil, errors.New("invalid swap event data")
	}

	// Extract amounts from event data (simplified)
	// In reality, you'd decode the actual event parameters
	amount0In := new(big.Int).SetBytes(log.Data[0:32])
	amount1In := new(big.Int).SetBytes(log.Data[32:64])
	amount0Out := new(big.Int).SetBytes(log.Data[64:96])
	amount1Out := new(big.Int).SetBytes(log.Data[96:128])

	// Calculate price impact (simplified calculation)
	var impactBps int64
	if amount0In.Sign() > 0 && amount1Out.Sign() > 0 {
		// Calculate price impact as percentage of input amount
		ratio := new(big.Int).Div(new(big.Int).Mul(amount1Out, big.NewInt(10000)), amount0In)
		impactBps = ratio.Int64()
	} else if amount1In.Sign() > 0 && amount0Out.Sign() > 0 {
		ratio := new(big.Int).Div(new(big.Int).Mul(amount0Out, big.NewInt(10000)), amount1In)
		impactBps = ratio.Int64()
	}

	return &interfaces.PriceImpact{
		Token:         common.BytesToAddress(log.Topics[1].Bytes()), // Assuming token address in first topic
		Pool:          log.Address,
		ImpactBps:     impactBps,
		ImpactPercent: float64(impactBps) / 100.0,
		SlippageBps:   impactBps, // Simplified: assume slippage equals impact
	}, nil
}

// findArbitrageOpportunity identifies arbitrage opportunities from price impact
func (b *backrunDetector) findArbitrageOpportunity(tx *types.Transaction, priceImpact *interfaces.PriceImpact) (*interfaces.BackrunOpportunity, error) {
	// Check if price impact is significant enough for arbitrage
	if priceImpact.ImpactBps < b.config.MinPriceGap.Int64() {
		return nil, nil // Price impact too small
	}

	// Find alternative pools for the same token
	alternativePool, err := b.findAlternativePool(priceImpact.Token, priceImpact.Pool)
	if err != nil {
		return nil, fmt.Errorf("failed to find alternative pool: %w", err)
	}

	if alternativePool == "" {
		return nil, nil // No alternative pool found
	}

	// Calculate initial trade size estimate
	initialTradeSize := b.estimateInitialTradeSize(tx, priceImpact)

	// Estimate profit from arbitrage
	expectedProfit := b.estimateArbitrageProfit(priceImpact, initialTradeSize)

	// Create arbitrage transaction
	arbitrageTx, err := b.constructArbitrageTransaction(tx, priceImpact.Token, alternativePool, initialTradeSize)
	if err != nil {
		return nil, fmt.Errorf("failed to construct arbitrage transaction: %w", err)
	}

	opportunity := &interfaces.BackrunOpportunity{
		TargetTx:       tx,
		ArbitrageTx:    arbitrageTx,
		Pool1:          priceImpact.Pool.Hex(),
		Pool2:          alternativePool,
		Token:          priceImpact.Token.Hex(),
		PriceGap:       big.NewInt(priceImpact.ImpactBps),
		OptimalAmount:  initialTradeSize,
		ExpectedProfit: expectedProfit,
	}

	return opportunity, nil
}

// findAlternativePool finds an alternative pool for arbitrage
func (b *backrunDetector) findAlternativePool(token common.Address, excludePool common.Address) (string, error) {
	// This is a simplified implementation
	// In practice, you'd query multiple DEX protocols to find the best alternative pool
	
	// Mock alternative pools for demonstration
	alternativePools := map[string][]string{
		"0x4200000000000000000000000000000000000006": { // WETH
			"0x1111111111111111111111111111111111111111", // Mock Uniswap V3 WETH/USDC
			"0x2222222222222222222222222222222222222222", // Mock Aerodrome WETH/USDC
		},
		"0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913": { // USDC
			"0x3333333333333333333333333333333333333333", // Mock Uniswap V3 USDC/WETH
			"0x4444444444444444444444444444444444444444", // Mock Aerodrome USDC/WETH
		},
	}

	tokenHex := token.Hex()
	pools, exists := alternativePools[tokenHex]
	if !exists {
		return "", errors.New("no alternative pools found for token")
	}

	// Return first pool that's not the excluded pool
	for _, pool := range pools {
		if pool != excludePool.Hex() {
			return pool, nil
		}
	}

	return "", errors.New("no suitable alternative pool found")
}

// estimateInitialTradeSize provides an initial estimate for trade size
func (b *backrunDetector) estimateInitialTradeSize(tx *types.Transaction, priceImpact *interfaces.PriceImpact) *big.Int {
	// Start with a fraction of the original transaction value
	// This will be optimized later using binary search
	initialSize := new(big.Int).Div(tx.Value, big.NewInt(10)) // 10% of original transaction
	
	// Ensure it's within bounds
	if initialSize.Cmp(b.config.MaxTradeSize) > 0 {
		return new(big.Int).Set(b.config.MaxTradeSize)
	}
	
	minSize := big.NewInt(1000) // Minimum 0.001 ETH
	if initialSize.Cmp(minSize) < 0 {
		return minSize
	}
	
	return initialSize
}

// estimateArbitrageProfit estimates the profit from an arbitrage opportunity
func (b *backrunDetector) estimateArbitrageProfit(priceImpact *interfaces.PriceImpact, tradeSize *big.Int) *big.Int {
	// Simplified profit calculation
	// Profit = (price_gap * trade_size) - gas_costs - slippage
	
	// Calculate gross profit from price gap
	grossProfit := new(big.Int).Mul(tradeSize, big.NewInt(priceImpact.ImpactBps))
	grossProfit = grossProfit.Div(grossProfit, big.NewInt(10000)) // Convert from basis points
	
	// Subtract estimated gas costs
	gasCost := big.NewInt(300000) // ~0.0003 ETH estimated gas cost
	
	// Subtract slippage costs (simplified)
	slippageCost := new(big.Int).Mul(tradeSize, big.NewInt(priceImpact.SlippageBps))
	slippageCost = slippageCost.Div(slippageCost, big.NewInt(20000)) // Half of slippage as cost
	
	// Calculate net profit
	netProfit := new(big.Int).Sub(grossProfit, gasCost)
	netProfit = netProfit.Sub(netProfit, slippageCost)
	
	// Ensure profit is not negative
	if netProfit.Sign() < 0 {
		return big.NewInt(0)
	}
	
	return netProfit
}

// constructArbitrageTransaction creates the arbitrage transaction
func (b *backrunDetector) constructArbitrageTransaction(targetTx *types.Transaction, token common.Address, alternativePool string, tradeSize *big.Int) (*types.Transaction, error) {
	// Create arbitrage transaction with same gas price as target (to be included after)
	toAddr := common.HexToAddress(alternativePool)
	arbitrageTx := &types.Transaction{
		Hash:     "", // Will be set when transaction is created
		From:     common.HexToAddress("0x2222222222222222222222222222222222222222"), // Mock arbitrageur address
		To:       &toAddr,
		Value:    tradeSize,
		GasPrice: targetTx.GasPrice,
		GasLimit: 200000, // Estimated gas limit for arbitrage
		Nonce:    0,      // Would need to be set based on account state
		Data:     b.constructArbitrageData(token, tradeSize),
		ChainID:  targetTx.ChainID,
	}
	
	return arbitrageTx, nil
}

// constructArbitrageData creates the transaction data for arbitrage
func (b *backrunDetector) constructArbitrageData(token common.Address, amount *big.Int) []byte {
	// This is a simplified implementation
	// In practice, you'd use the ABI encoder to create proper transaction data
	
	// Mock transaction data for arbitrage swap
	mockData := make([]byte, 68) // 4 bytes method sig + 64 bytes parameters
	
	// Method signature for swapExactTokensForTokens
	copy(mockData[:4], common.Hex2Bytes("38ed1739"))
	
	// In a real implementation, you'd encode the actual parameters:
	// - amountIn
	// - amountOutMin
	// - path (token addresses for arbitrage route)
	// - to (recipient address)
	// - deadline
	
	return mockData
}

// binarySearchOptimalSize uses binary search to find optimal trade size
func (b *backrunDetector) binarySearchOptimalSize(ctx context.Context, opportunity *interfaces.BackrunOpportunity, minSize, maxSize, tolerance *big.Int) (*big.Int, error) {
	left := new(big.Int).Set(minSize)
	right := new(big.Int).Set(maxSize)
	bestSize := new(big.Int).Set(minSize)
	bestProfit := big.NewInt(0)
	
	// Binary search iterations
	maxIterations := 20 // Prevent infinite loops
	iteration := 0
	
	for left.Cmp(right) <= 0 && iteration < maxIterations {
		iteration++
		
		// Calculate midpoint
		mid := new(big.Int).Add(left, right)
		mid = mid.Div(mid, big.NewInt(2))
		
		// Calculate profit at midpoint
		profit := b.calculateProfitAtSize(opportunity, mid)
		
		// Update best if this is better
		if profit.Cmp(bestProfit) > 0 {
			bestProfit = profit
			bestSize = new(big.Int).Set(mid)
		}
		
		// Check convergence
		diff := new(big.Int).Sub(right, left)
		if diff.Cmp(tolerance) <= 0 {
			break
		}
		
		// Calculate profit at mid+1 to determine direction
		midPlusOne := new(big.Int).Add(mid, big.NewInt(1))
		profitPlusOne := b.calculateProfitAtSize(opportunity, midPlusOne)
		
		// Move search window based on profit gradient
		if profitPlusOne.Cmp(profit) > 0 {
			left = mid
		} else {
			right = mid
		}
	}
	
	return bestSize, nil
}

// calculateProfitAtSize calculates expected profit for a given trade size
func (b *backrunDetector) calculateProfitAtSize(opportunity *interfaces.BackrunOpportunity, size *big.Int) *big.Int {
	// Simplified profit calculation with diminishing returns
	// In practice, you'd simulate the actual trade to get accurate profit
	
	// Base profit from price gap
	baseProfit := new(big.Int).Mul(size, opportunity.PriceGap)
	baseProfit = baseProfit.Div(baseProfit, big.NewInt(10000))
	
	// Apply diminishing returns (larger trades have lower efficiency)
	// This simulates the effect of slippage increasing with trade size
	efficiency := b.calculateTradeEfficiency(size)
	adjustedProfit := new(big.Int).Mul(baseProfit, big.NewInt(int64(efficiency*100)))
	adjustedProfit = adjustedProfit.Div(adjustedProfit, big.NewInt(100))
	
	// Subtract gas costs
	gasCost := big.NewInt(300000) // Fixed gas cost
	netProfit := new(big.Int).Sub(adjustedProfit, gasCost)
	
	// Ensure profit is not negative
	if netProfit.Sign() < 0 {
		return big.NewInt(0)
	}
	
	return netProfit
}

// calculateTradeEfficiency calculates efficiency based on trade size (accounts for slippage)
func (b *backrunDetector) calculateTradeEfficiency(size *big.Int) float64 {
	// Simplified efficiency calculation
	// Larger trades have lower efficiency due to slippage
	
	sizeFloat := new(big.Float).SetInt(size)
	maxSizeFloat := new(big.Float).SetInt(b.config.MaxTradeSize)
	
	// Calculate ratio (0 to 1)
	ratio, _ := new(big.Float).Quo(sizeFloat, maxSizeFloat).Float64()
	
	// Efficiency decreases as size increases (quadratic decay)
	efficiency := 1.0 - (ratio * ratio * 0.5)
	
	// Ensure efficiency is between 0.1 and 1.0
	if efficiency < 0.1 {
		efficiency = 0.1
	}
	
	return efficiency
}