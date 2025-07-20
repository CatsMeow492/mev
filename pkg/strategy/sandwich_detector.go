package strategy

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

// sandwichDetector implements the SandwichDetector interface
type sandwichDetector struct {
	config *interfaces.SandwichConfig
}

// NewSandwichDetector creates a new sandwich detector with the given configuration
func NewSandwichDetector(config *interfaces.SandwichConfig) interfaces.SandwichDetector {
	if config == nil {
		config = &interfaces.SandwichConfig{
			MinSwapAmount:     big.NewInt(10000), // $10k minimum
			MaxSlippage:       0.02,              // 2% max slippage
			GasPremiumPercent: 0.10,              // 10% gas premium
			MinProfitThreshold: big.NewInt(100),  // $100 minimum profit
		}
	}
	return &sandwichDetector{
		config: config,
	}
}

// DetectOpportunity analyzes a transaction to identify sandwich attack opportunities
func (s *sandwichDetector) DetectOpportunity(ctx context.Context, tx *types.Transaction, simResult *interfaces.SimulationResult) (*interfaces.SandwichOpportunity, error) {
	// Check if transaction is a swap
	if tx.GetTransactionType() != types.TxTypeSwap {
		return nil, nil // Not a swap, no sandwich opportunity
	}

	// Check if swap amount meets minimum threshold
	if !s.isLargeSwap(tx) {
		return nil, nil // Swap too small for profitable sandwich
	}

	// Extract swap details from simulation result
	swapDetails, err := s.extractSwapDetails(tx, simResult)
	if err != nil {
		return nil, fmt.Errorf("failed to extract swap details: %w", err)
	}

	// Check slippage tolerance
	if swapDetails.SlippageTolerance > s.config.MaxSlippage {
		return nil, nil // Slippage tolerance too high, not profitable
	}

	// Calculate price impact
	priceImpact, err := s.calculatePriceImpact(simResult)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate price impact: %w", err)
	}

	// Create sandwich opportunity
	opportunity := &interfaces.SandwichOpportunity{
		TargetTx:          tx,
		ExpectedProfit:    s.estimateProfit(swapDetails, priceImpact),
		SlippageTolerance: swapDetails.SlippageTolerance,
		PriceImpact:       priceImpact,
		Pool:              swapDetails.Pool,
		Token0:            swapDetails.Token0,
		Token1:            swapDetails.Token1,
	}

	return opportunity, nil
}

// ValidateOpportunity validates that a sandwich opportunity is still profitable
func (s *sandwichDetector) ValidateOpportunity(ctx context.Context, opportunity *interfaces.SandwichOpportunity) error {
	if opportunity == nil {
		return errors.New("opportunity cannot be nil")
	}

	// Check if expected profit meets minimum threshold
	if opportunity.ExpectedProfit.Cmp(s.config.MinProfitThreshold) < 0 {
		return errors.New("expected profit below minimum threshold")
	}

	// Check slippage tolerance
	if opportunity.SlippageTolerance > s.config.MaxSlippage {
		return errors.New("slippage tolerance exceeds maximum allowed")
	}

	// Validate that we have all required transaction components
	if opportunity.TargetTx == nil {
		return errors.New("target transaction is required")
	}

	return nil
}

// ConstructTransactions creates the front-run and back-run transactions for the sandwich
func (s *sandwichDetector) ConstructTransactions(ctx context.Context, opportunity *interfaces.SandwichOpportunity) ([]*types.Transaction, error) {
	if err := s.ValidateOpportunity(ctx, opportunity); err != nil {
		return nil, fmt.Errorf("invalid opportunity: %w", err)
	}

	transactions := make([]*types.Transaction, 0, 2)

	// Construct front-run transaction
	frontrunTx, err := s.constructFrontrunTransaction(opportunity)
	if err != nil {
		return nil, fmt.Errorf("failed to construct frontrun transaction: %w", err)
	}
	opportunity.FrontrunTx = frontrunTx
	transactions = append(transactions, frontrunTx)

	// Construct back-run transaction
	backrunTx, err := s.constructBackrunTransaction(opportunity)
	if err != nil {
		return nil, fmt.Errorf("failed to construct backrun transaction: %w", err)
	}
	opportunity.BackrunTx = backrunTx
	transactions = append(transactions, backrunTx)

	return transactions, nil
}

// GetConfiguration returns the current sandwich detector configuration
func (s *sandwichDetector) GetConfiguration() *interfaces.SandwichConfig {
	return s.config
}

// isLargeSwap checks if the swap amount meets the minimum threshold
func (s *sandwichDetector) isLargeSwap(tx *types.Transaction) bool {
	// For simplicity, we'll use the transaction value as a proxy for swap amount
	// In a real implementation, we'd decode the transaction data to get the exact swap amount
	return tx.Value.Cmp(s.config.MinSwapAmount) >= 0
}

// swapDetails contains extracted information about a swap transaction
type swapDetails struct {
	Pool              string
	Token0            string
	Token1            string
	AmountIn          *big.Int
	AmountOut         *big.Int
	SlippageTolerance float64
}

// extractSwapDetails extracts swap information from transaction and simulation result
func (s *sandwichDetector) extractSwapDetails(tx *types.Transaction, simResult *interfaces.SimulationResult) (*swapDetails, error) {
	// This is a simplified implementation
	// In practice, you'd decode the transaction data and analyze the logs
	
	if len(tx.Data) < 4 {
		return nil, errors.New("invalid transaction data")
	}

	// Extract method signature
	methodSig := common.Bytes2Hex(tx.Data[:4])
	
	// For demonstration, we'll create mock swap details
	// In reality, you'd decode the actual transaction parameters
	details := &swapDetails{
		Pool:              "0x1234567890123456789012345678901234567890", // Mock pool address
		Token0:            "0xA0b86a33E6441b8435b662f0E2d0B5B0B5B5B5B5", // Mock token0
		Token1:            "0xB1c97a44F7552c9A6B8B8B8B8B8B8B8B8B8B8B8B", // Mock token1
		AmountIn:          tx.Value,
		AmountOut:         new(big.Int).Mul(tx.Value, big.NewInt(95)).Div(new(big.Int).Mul(tx.Value, big.NewInt(95)), big.NewInt(100)), // 5% slippage estimate
		SlippageTolerance: 0.05, // 5% default slippage tolerance
	}

	// Adjust slippage based on method signature
	switch methodSig {
	case "7ff36ab5", "18cbafe5", "38ed1739": // Common swap methods
		// Use simulation result to get more accurate slippage if available
		if simResult != nil && simResult.Success {
			details.SlippageTolerance = s.calculateSlippageFromLogs(simResult)
		}
	}

	return details, nil
}

// calculatePriceImpact calculates the price impact from simulation results
func (s *sandwichDetector) calculatePriceImpact(simResult *interfaces.SimulationResult) (*big.Int, error) {
	if simResult == nil || !simResult.Success {
		return nil, errors.New("invalid simulation result")
	}

	// This is a simplified calculation
	// In practice, you'd analyze the actual price changes from the logs
	
	// Mock price impact calculation (in basis points)
	// For a large swap, assume 1-3% price impact
	priceImpactBps := big.NewInt(150) // 1.5% in basis points
	
	return priceImpactBps, nil
}

// calculateSlippageFromLogs extracts slippage tolerance from transaction logs
func (s *sandwichDetector) calculateSlippageFromLogs(simResult *interfaces.SimulationResult) float64 {
	// This would analyze the actual swap logs to determine slippage
	// For now, return a default value
	return 0.02 // 2%
}

// estimateProfit estimates the potential profit from a sandwich attack
func (s *sandwichDetector) estimateProfit(details *swapDetails, priceImpact *big.Int) *big.Int {
	// Simplified profit calculation
	// Profit = (price_impact * amount_in) - gas_costs
	
	// Calculate profit from price impact (in wei)
	profit := new(big.Int).Mul(details.AmountIn, priceImpact)
	profit = profit.Div(profit, big.NewInt(10000)) // Convert from basis points
	
	// Subtract estimated gas costs (simplified)
	gasCost := big.NewInt(500000) // ~0.0005 ETH in wei (rough estimate)
	profit = profit.Sub(profit, gasCost)
	
	// Ensure profit is not negative
	if profit.Sign() < 0 {
		return big.NewInt(0)
	}
	
	return profit
}

// constructFrontrunTransaction creates the front-run transaction
func (s *sandwichDetector) constructFrontrunTransaction(opportunity *interfaces.SandwichOpportunity) (*types.Transaction, error) {
	targetTx := opportunity.TargetTx
	
	// Calculate gas price with premium
	gasPremium := new(big.Int).Mul(targetTx.GasPrice, big.NewInt(int64(s.config.GasPremiumPercent*100)))
	gasPremium = gasPremium.Div(gasPremium, big.NewInt(100))
	frontrunGasPrice := new(big.Int).Add(targetTx.GasPrice, gasPremium)
	
	// Create front-run transaction (buy before target transaction)
	frontrunTx := &types.Transaction{
		Hash:     "", // Will be set when transaction is created
		From:     common.HexToAddress("0x1111111111111111111111111111111111111111"), // Mock frontrunner address
		To:       targetTx.To,
		Value:    new(big.Int).Div(targetTx.Value, big.NewInt(10)), // Use 10% of target amount
		GasPrice: frontrunGasPrice,
		GasLimit: targetTx.GasLimit,
		Nonce:    0, // Would need to be set based on account state
		Data:     s.constructSwapData(opportunity, true), // true for frontrun
		ChainID:  targetTx.ChainID,
	}
	
	return frontrunTx, nil
}

// constructBackrunTransaction creates the back-run transaction
func (s *sandwichDetector) constructBackrunTransaction(opportunity *interfaces.SandwichOpportunity) (*types.Transaction, error) {
	targetTx := opportunity.TargetTx
	
	// Back-run transaction uses same gas price as target (will be included after)
	backrunTx := &types.Transaction{
		Hash:     "", // Will be set when transaction is created
		From:     common.HexToAddress("0x1111111111111111111111111111111111111111"), // Mock frontrunner address
		To:       targetTx.To,
		Value:    new(big.Int).Div(targetTx.Value, big.NewInt(10)), // Use 10% of target amount
		GasPrice: targetTx.GasPrice,
		GasLimit: targetTx.GasLimit,
		Nonce:    1, // Would need to be set based on account state
		Data:     s.constructSwapData(opportunity, false), // false for backrun
		ChainID:  targetTx.ChainID,
	}
	
	return backrunTx, nil
}

// constructSwapData creates the transaction data for swap transactions
func (s *sandwichDetector) constructSwapData(opportunity *interfaces.SandwichOpportunity, isFrontrun bool) []byte {
	// This is a simplified implementation
	// In practice, you'd use the ABI encoder to create proper transaction data
	
	// Mock transaction data for a swap
	// This would be the encoded function call with parameters
	mockData := make([]byte, 68) // 4 bytes method sig + 64 bytes parameters
	
	if isFrontrun {
		// Method signature for swapExactTokensForTokens (frontrun - same direction as target)
		copy(mockData[:4], common.Hex2Bytes("38ed1739"))
	} else {
		// Method signature for swapExactTokensForTokens (backrun - reverse direction)
		copy(mockData[:4], common.Hex2Bytes("38ed1739"))
	}
	
	// In a real implementation, you'd encode the actual parameters:
	// - amountIn
	// - amountOutMin
	// - path (token addresses)
	// - to (recipient address)
	// - deadline
	
	return mockData
}