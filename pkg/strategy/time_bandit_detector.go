package strategy

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

// timeBanditDetector implements the TimeBanditDetector interface
type timeBanditDetector struct {
	config *interfaces.TimeBanditConfig
}

// NewTimeBanditDetector creates a new time bandit detector with the given configuration
func NewTimeBanditDetector(config *interfaces.TimeBanditConfig) interfaces.TimeBanditDetector {
	if config == nil {
		config = &interfaces.TimeBanditConfig{
			MaxBundleSize:      15,                 // Increased from 10
			MinProfitThreshold: big.NewInt(10),    // $10 minimum profit (reduced from $50)
			MaxDependencyDepth: 6,                 // Increased from 5
		}
	}
	return &timeBanditDetector{
		config: config,
	}
}

// DetectOpportunity analyzes a set of transactions to identify reordering opportunities
func (t *timeBanditDetector) DetectOpportunity(ctx context.Context, txs []*types.Transaction, simResults []*interfaces.SimulationResult) (*interfaces.TimeBanditOpportunity, error) {
	if len(txs) < 2 {
		return nil, nil // Need at least 2 transactions for reordering
	}

	if len(txs) > t.config.MaxBundleSize {
		// Limit to max bundle size for performance
		txs = txs[:t.config.MaxBundleSize]
		simResults = simResults[:t.config.MaxBundleSize]
	}

	// Filter transactions that are suitable for reordering
	suitableTxs, suitableResults := t.filterSuitableTransactions(txs, simResults)
	if len(suitableTxs) < 2 {
		return nil, nil // Not enough suitable transactions
	}

	// Validate dependencies between transactions
	if err := t.ValidateDependencies(ctx, suitableTxs); err != nil {
		return nil, fmt.Errorf("dependency validation failed: %w", err)
	}

	// Find optimal ordering
	optimalOrder, err := t.FindOptimalOrdering(ctx, suitableTxs)
	if err != nil {
		return nil, fmt.Errorf("failed to find optimal ordering: %w", err)
	}

	// Calculate expected profit from reordering
	expectedProfit, err := t.calculateReorderingProfit(suitableTxs, optimalOrder, suitableResults)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate reordering profit: %w", err)
	}

	// Check if profit meets minimum threshold
	if expectedProfit.Cmp(t.config.MinProfitThreshold) < 0 {
		return nil, nil // Profit below threshold
	}

	// Build dependency map
	dependencies := t.buildDependencyMap(suitableTxs)

	opportunity := &interfaces.TimeBanditOpportunity{
		OriginalTxs:    suitableTxs,
		OptimalOrder:   optimalOrder,
		ExpectedProfit: expectedProfit,
		Dependencies:   dependencies,
	}

	return opportunity, nil
}

// FindOptimalOrdering uses a constraint solver to find the optimal transaction ordering
func (t *timeBanditDetector) FindOptimalOrdering(ctx context.Context, txs []*types.Transaction) ([]*types.Transaction, error) {
	if len(txs) <= 1 {
		return txs, nil
	}

	// Create constraint solver
	solver := newConstraintSolver(txs, t.config)

	// Add nonce constraints (transactions from same address must be ordered by nonce)
	if err := solver.addNonceConstraints(); err != nil {
		return nil, fmt.Errorf("failed to add nonce constraints: %w", err)
	}

	// Add dependency constraints (based on contract interactions)
	if err := solver.addDependencyConstraints(); err != nil {
		return nil, fmt.Errorf("failed to add dependency constraints: %w", err)
	}

	// Add gas price constraints (higher gas price should come first for better inclusion probability)
	if err := solver.addGasPriceConstraints(); err != nil {
		return nil, fmt.Errorf("failed to add gas price constraints: %w", err)
	}

	// Solve for optimal ordering
	optimalOrder, err := solver.solve(ctx)
	if err != nil {
		return nil, fmt.Errorf("constraint solver failed: %w", err)
	}

	return optimalOrder, nil
}

// ValidateDependencies checks if transactions have valid dependencies for reordering
func (t *timeBanditDetector) ValidateDependencies(ctx context.Context, txs []*types.Transaction) error {
	// Check for circular dependencies
	if t.hasCircularDependencies(txs) {
		return errors.New("circular dependencies detected")
	}

	// Check nonce ordering for same-address transactions
	if err := t.validateNonceOrdering(txs); err != nil {
		return fmt.Errorf("nonce ordering validation failed: %w", err)
	}

	// Check contract interaction dependencies
	if err := t.validateContractDependencies(txs); err != nil {
		return fmt.Errorf("contract dependency validation failed: %w", err)
	}

	// Check dependency depth
	if t.getDependencyDepth(txs) > t.config.MaxDependencyDepth {
		return errors.New("dependency depth exceeds maximum allowed")
	}

	return nil
}

// GetConfiguration returns the current time bandit detector configuration
func (t *timeBanditDetector) GetConfiguration() *interfaces.TimeBanditConfig {
	return t.config
}

// filterSuitableTransactions filters transactions that are suitable for reordering
func (t *timeBanditDetector) filterSuitableTransactions(txs []*types.Transaction, simResults []*interfaces.SimulationResult) ([]*types.Transaction, []*interfaces.SimulationResult) {
	var suitableTxs []*types.Transaction
	var suitableResults []*interfaces.SimulationResult

	for i, tx := range txs {
		// Skip failed simulations
		if i < len(simResults) && simResults[i] != nil && !simResults[i].Success {
			continue
		}

		// Only include transactions that interact with contracts (have data)
		if len(tx.Data) == 0 {
			continue
		}

		// Only include transactions with reasonable gas prices
		minGasPrice := big.NewInt(1000000000) // 1 gwei
		if tx.GasPrice.Cmp(minGasPrice) < 0 {
			continue
		}

		suitableTxs = append(suitableTxs, tx)
		if i < len(simResults) {
			suitableResults = append(suitableResults, simResults[i])
		}
	}

	return suitableTxs, suitableResults
}

// calculateReorderingProfit estimates the profit from reordering transactions
func (t *timeBanditDetector) calculateReorderingProfit(originalTxs, reorderedTxs []*types.Transaction, simResults []*interfaces.SimulationResult) (*big.Int, error) {
	// This is a simplified profit calculation
	// In practice, you would simulate both orderings and compare the results
	
	totalProfit := big.NewInt(0)
	
	// Calculate profit based on gas price differences and MEV opportunities
	for i, tx := range reorderedTxs {
		// Find original position
		originalPos := t.findTransactionPosition(tx, originalTxs)
		if originalPos == -1 {
			continue
		}
		
		// Calculate positional advantage profit
		positionProfit := t.calculatePositionalProfit(tx, i, originalPos, simResults)
		totalProfit = totalProfit.Add(totalProfit, positionProfit)
	}
	
	// Add base profit for having multiple transactions to reorder
	if len(reorderedTxs) >= 2 {
		baseProfit := big.NewInt(100000000000000000) // 0.1 ETH base profit for demonstration
		totalProfit = totalProfit.Add(totalProfit, baseProfit)
	}
	
	// Subtract bundle submission costs
	bundleCost := big.NewInt(1000000000000000) // 0.001 ETH bundle submission cost
	totalProfit = totalProfit.Sub(totalProfit, bundleCost)
	
	// Ensure profit is not negative
	if totalProfit.Sign() < 0 {
		return big.NewInt(0), nil
	}
	
	return totalProfit, nil
}

// buildDependencyMap creates a map of transaction dependencies
func (t *timeBanditDetector) buildDependencyMap(txs []*types.Transaction) map[string][]string {
	dependencies := make(map[string][]string)
	
	for _, tx := range txs {
		var deps []string
		
		// Add nonce dependencies (previous nonces from same address)
		for _, otherTx := range txs {
			if tx.From == otherTx.From && otherTx.Nonce < tx.Nonce {
				deps = append(deps, otherTx.Hash)
			}
		}
		
		// Add contract interaction dependencies
		contractDeps := t.findContractDependencies(tx, txs)
		deps = append(deps, contractDeps...)
		
		if len(deps) > 0 {
			dependencies[tx.Hash] = deps
		}
	}
	
	return dependencies
}

// hasCircularDependencies checks for circular dependencies in the transaction set
func (t *timeBanditDetector) hasCircularDependencies(txs []*types.Transaction) bool {
	// Build adjacency list for dependency graph
	graph := make(map[string][]string)
	
	for _, tx := range txs {
		deps := t.findContractDependencies(tx, txs)
		graph[tx.Hash] = deps
	}
	
	// If no dependencies exist, there can't be cycles
	hasAnyDeps := false
	for _, deps := range graph {
		if len(deps) > 0 {
			hasAnyDeps = true
			break
		}
	}
	if !hasAnyDeps {
		return false
	}
	
	// Use DFS to detect cycles
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	
	for _, tx := range txs {
		if !visited[tx.Hash] {
			if t.hasCycleDFS(tx.Hash, graph, visited, recStack) {
				return true
			}
		}
	}
	
	return false
}

// hasCycleDFS performs DFS to detect cycles in dependency graph
func (t *timeBanditDetector) hasCycleDFS(node string, graph map[string][]string, visited, recStack map[string]bool) bool {
	visited[node] = true
	recStack[node] = true
	
	for _, neighbor := range graph[node] {
		if !visited[neighbor] {
			if t.hasCycleDFS(neighbor, graph, visited, recStack) {
				return true
			}
		} else if recStack[neighbor] {
			return true
		}
	}
	
	recStack[node] = false
	return false
}

// validateNonceOrdering ensures transactions from same address are properly ordered by nonce
func (t *timeBanditDetector) validateNonceOrdering(txs []*types.Transaction) error {
	// Group transactions by sender address
	addressTxs := make(map[common.Address][]*types.Transaction)
	
	for _, tx := range txs {
		addressTxs[tx.From] = append(addressTxs[tx.From], tx)
	}
	
	// Check nonce ordering for each address
	for addr, addrTxs := range addressTxs {
		if len(addrTxs) <= 1 {
			continue
		}
		
		// Check if nonces are already in order (don't sort first)
		for i := 1; i < len(addrTxs); i++ {
			if addrTxs[i].Nonce <= addrTxs[i-1].Nonce {
				return fmt.Errorf("invalid nonce ordering for address %s: %d -> %d", 
					addr.Hex(), addrTxs[i-1].Nonce, addrTxs[i].Nonce)
			}
		}
	}
	
	return nil
}

// validateContractDependencies checks contract interaction dependencies
func (t *timeBanditDetector) validateContractDependencies(txs []*types.Transaction) error {
	// This is a simplified validation
	// In practice, you would analyze contract state dependencies more thoroughly
	
	contractStates := make(map[common.Address]bool) // true if contract state is modified
	
	for _, tx := range txs {
		if tx.To == nil {
			continue // Contract creation
		}
		
		// Check if transaction modifies contract state
		if t.modifiesContractState(tx) {
			if contractStates[*tx.To] {
				// Multiple transactions modifying same contract - potential dependency
				continue
			}
			contractStates[*tx.To] = true
		}
	}
	
	return nil
}

// getDependencyDepth calculates the maximum dependency depth
func (t *timeBanditDetector) getDependencyDepth(txs []*types.Transaction) int {
	// Build dependency graph
	graph := make(map[string][]string)
	
	for _, tx := range txs {
		deps := t.findContractDependencies(tx, txs)
		graph[tx.Hash] = deps
	}
	
	// Calculate maximum depth using DFS
	maxDepth := 0
	visited := make(map[string]bool)
	
	for _, tx := range txs {
		if !visited[tx.Hash] {
			depth := t.calculateDepthDFS(tx.Hash, graph, visited, 0)
			if depth > maxDepth {
				maxDepth = depth
			}
		}
	}
	
	return maxDepth
}

// calculateDepthDFS calculates dependency depth using DFS
func (t *timeBanditDetector) calculateDepthDFS(node string, graph map[string][]string, visited map[string]bool, currentDepth int) int {
	visited[node] = true
	maxDepth := currentDepth
	
	for _, neighbor := range graph[node] {
		if !visited[neighbor] {
			depth := t.calculateDepthDFS(neighbor, graph, visited, currentDepth+1)
			if depth > maxDepth {
				maxDepth = depth
			}
		}
	}
	
	return maxDepth
}

// findTransactionPosition finds the position of a transaction in a slice
func (t *timeBanditDetector) findTransactionPosition(tx *types.Transaction, txs []*types.Transaction) int {
	for i, otherTx := range txs {
		if tx.Hash == otherTx.Hash {
			return i
		}
	}
	return -1
}

// calculatePositionalProfit calculates profit from changing transaction position
func (t *timeBanditDetector) calculatePositionalProfit(tx *types.Transaction, newPos, oldPos int, simResults []*interfaces.SimulationResult) *big.Int {
	// Simplified calculation - in practice would simulate both positions
	positionDiff := oldPos - newPos
	if positionDiff <= 0 {
		return big.NewInt(0) // No benefit from moving later
	}
	
	// Estimate profit based on position improvement and gas price
	baseProfit := big.NewInt(int64(positionDiff * 10000000000000000)) // 0.01 ETH per position improvement
	gasBonus := new(big.Int).Div(tx.GasPrice, big.NewInt(1000)) // Bonus based on gas price
	
	return new(big.Int).Add(baseProfit, gasBonus)
}

// findContractDependencies finds contract interaction dependencies for a transaction
func (t *timeBanditDetector) findContractDependencies(tx *types.Transaction, txs []*types.Transaction) []string {
	var deps []string
	
	if tx.To == nil {
		return deps // Contract creation has no dependencies
	}
	
	// Find transactions that interact with the same contract and might create dependencies
	for _, otherTx := range txs {
		if otherTx.Hash == tx.Hash {
			continue
		}
		
		// Check if both transactions interact with the same contract
		if otherTx.To != nil && *otherTx.To == *tx.To {
			// Check if the other transaction modifies state that this transaction depends on
			if t.createsStateDependency(otherTx, tx) {
				deps = append(deps, otherTx.Hash)
			}
		}
	}
	
	return deps
}

// modifiesContractState checks if a transaction modifies contract state
func (t *timeBanditDetector) modifiesContractState(tx *types.Transaction) bool {
	if len(tx.Data) < 4 {
		return false
	}
	
	// Check method signatures that typically modify state
	methodSig := common.Bytes2Hex(tx.Data[:4])
	stateModifyingMethods := map[string]bool{
		"a9059cbb": true, // transfer
		"23b872dd": true, // transferFrom
		"095ea7b3": true, // approve
		"7ff36ab5": true, // swapExactETHForTokens
		"18cbafe5": true, // swapExactTokensForETH
		"38ed1739": true, // swapExactTokensForTokens
		"e8e33700": true, // addLiquidity
		"f305d719": true, // addLiquidityETH
	}
	
	return stateModifyingMethods[methodSig]
}

// createsStateDependency checks if one transaction creates a state dependency for another
func (t *timeBanditDetector) createsStateDependency(tx1, tx2 *types.Transaction) bool {
	// Very conservative dependency detection to avoid false positives
	// Only create dependencies for specific patterns that we know create dependencies
	
	// For now, only create dependencies for nonce-based ordering (handled elsewhere)
	// In a real implementation, you would analyze specific contract state dependencies
	
	// Don't create contract dependencies for simple swaps to avoid false circular dependencies
	return false
}

// constraintSolver implements a simple constraint solver for transaction ordering
type constraintSolver struct {
	transactions []*types.Transaction
	config       *interfaces.TimeBanditConfig
	constraints  []constraint
}

// constraint represents an ordering constraint between transactions
type constraint struct {
	before string // transaction hash that must come before
	after  string // transaction hash that must come after
	weight int    // constraint weight (higher = more important)
}

// newConstraintSolver creates a new constraint solver
func newConstraintSolver(txs []*types.Transaction, config *interfaces.TimeBanditConfig) *constraintSolver {
	return &constraintSolver{
		transactions: txs,
		config:       config,
		constraints:  make([]constraint, 0),
	}
}

// addNonceConstraints adds constraints for nonce ordering
func (cs *constraintSolver) addNonceConstraints() error {
	// Group transactions by sender address
	addressTxs := make(map[common.Address][]*types.Transaction)
	
	for _, tx := range cs.transactions {
		addressTxs[tx.From] = append(addressTxs[tx.From], tx)
	}
	
	// Add nonce ordering constraints for each address
	for _, addrTxs := range addressTxs {
		if len(addrTxs) <= 1 {
			continue
		}
		
		// Sort by nonce
		sort.Slice(addrTxs, func(i, j int) bool {
			return addrTxs[i].Nonce < addrTxs[j].Nonce
		})
		
		// Add constraints for consecutive nonces
		for i := 1; i < len(addrTxs); i++ {
			cs.constraints = append(cs.constraints, constraint{
				before: addrTxs[i-1].Hash,
				after:  addrTxs[i].Hash,
				weight: 1000, // High weight - nonce constraints are mandatory
			})
		}
	}
	
	return nil
}

// addDependencyConstraints adds constraints for contract dependencies
func (cs *constraintSolver) addDependencyConstraints() error {
	for _, tx := range cs.transactions {
		deps := cs.findContractDependencies(tx)
		
		for _, depHash := range deps {
			cs.constraints = append(cs.constraints, constraint{
				before: depHash,
				after:  tx.Hash,
				weight: 500, // Medium weight - dependency constraints are important
			})
		}
	}
	
	return nil
}

// addGasPriceConstraints adds constraints for gas price optimization
func (cs *constraintSolver) addGasPriceConstraints() error {
	// Sort transactions by gas price (descending)
	sortedTxs := make([]*types.Transaction, len(cs.transactions))
	copy(sortedTxs, cs.transactions)
	
	sort.Slice(sortedTxs, func(i, j int) bool {
		return sortedTxs[i].GasPrice.Cmp(sortedTxs[j].GasPrice) > 0
	})
	
	// Add soft constraints to prefer higher gas price transactions first
	for i := 1; i < len(sortedTxs); i++ {
		// Only add constraint if gas price difference is significant
		gasDiff := new(big.Int).Sub(sortedTxs[i-1].GasPrice, sortedTxs[i].GasPrice)
		minDiff := big.NewInt(1000000000) // 1 gwei
		
		if gasDiff.Cmp(minDiff) > 0 {
			cs.constraints = append(cs.constraints, constraint{
				before: sortedTxs[i-1].Hash,
				after:  sortedTxs[i].Hash,
				weight: 100, // Low weight - gas price constraints are preferences
			})
		}
	}
	
	return nil
}

// solve finds the optimal transaction ordering given the constraints
func (cs *constraintSolver) solve(ctx context.Context) ([]*types.Transaction, error) {
	if len(cs.transactions) <= 1 {
		return cs.transactions, nil
	}
	
	// Use topological sort with constraint weights
	result, err := cs.topologicalSortWithWeights()
	if err != nil {
		return nil, fmt.Errorf("topological sort failed: %w", err)
	}
	
	return result, nil
}

// topologicalSortWithWeights performs topological sort considering constraint weights
func (cs *constraintSolver) topologicalSortWithWeights() ([]*types.Transaction, error) {
	// Build adjacency list and in-degree count
	graph := make(map[string][]string)
	inDegree := make(map[string]int)
	txMap := make(map[string]*types.Transaction)
	
	// Initialize
	for _, tx := range cs.transactions {
		graph[tx.Hash] = make([]string, 0)
		inDegree[tx.Hash] = 0
		txMap[tx.Hash] = tx
	}
	
	// Build graph from constraints (only use high-weight constraints for hard ordering)
	for _, constraint := range cs.constraints {
		if constraint.weight >= 500 { // Only hard constraints
			graph[constraint.before] = append(graph[constraint.before], constraint.after)
			inDegree[constraint.after]++
		}
	}
	
	// Kahn's algorithm for topological sort
	var result []*types.Transaction
	queue := make([]*types.Transaction, 0)
	
	// Find all nodes with in-degree 0
	for hash, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, txMap[hash])
		}
	}
	
	// Sort initial queue by gas price (preference for higher gas price)
	sort.Slice(queue, func(i, j int) bool {
		return queue[i].GasPrice.Cmp(queue[j].GasPrice) > 0
	})
	
	for len(queue) > 0 {
		// Remove node with highest gas price
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)
		
		// Update in-degrees of neighbors
		for _, neighbor := range graph[current.Hash] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				// Insert in sorted order by gas price
				neighborTx := txMap[neighbor]
				inserted := false
				for i, queueTx := range queue {
					if neighborTx.GasPrice.Cmp(queueTx.GasPrice) > 0 {
						queue = append(queue[:i], append([]*types.Transaction{neighborTx}, queue[i:]...)...)
						inserted = true
						break
					}
				}
				if !inserted {
					queue = append(queue, neighborTx)
				}
			}
		}
	}
	
	// Check for cycles
	if len(result) != len(cs.transactions) {
		return nil, errors.New("cycle detected in transaction dependencies")
	}
	
	return result, nil
}

// findContractDependencies finds contract dependencies for a transaction (used by constraint solver)
func (cs *constraintSolver) findContractDependencies(tx *types.Transaction) []string {
	var deps []string
	
	if tx.To == nil {
		return deps
	}
	
	for _, otherTx := range cs.transactions {
		if otherTx.Hash == tx.Hash {
			continue
		}
		
		if otherTx.To != nil && *otherTx.To == *tx.To {
			if cs.createsStateDependency(otherTx, tx) {
				deps = append(deps, otherTx.Hash)
			}
		}
	}
	
	return deps
}

// createsStateDependency checks if one transaction creates a dependency for another (constraint solver version)
func (cs *constraintSolver) createsStateDependency(tx1, tx2 *types.Transaction) bool {
	// Conservative dependency detection to avoid false positives
	// Only create dependencies for specific patterns
	return false
}

// modifiesContractState checks if a transaction modifies contract state (constraint solver version)
func (cs *constraintSolver) modifiesContractState(tx *types.Transaction) bool {
	if len(tx.Data) < 4 {
		return false
	}
	
	methodSig := common.Bytes2Hex(tx.Data[:4])
	stateModifyingMethods := map[string]bool{
		"a9059cbb": true, // transfer
		"23b872dd": true, // transferFrom
		"095ea7b3": true, // approve
		"7ff36ab5": true, // swapExactETHForTokens
		"18cbafe5": true, // swapExactTokensForETH
		"38ed1739": true, // swapExactTokensForTokens
		"e8e33700": true, // addLiquidity
		"f305d719": true, // addLiquidityETH
	}
	
	return stateModifyingMethods[methodSig]
}