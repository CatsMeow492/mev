package simulation

import (
	"context"
	"fmt"
	"math/big"
	"os/exec"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	mevtypes "github.com/mev-engine/l2-mev-strategy-engine/pkg/types"
)

// anvilFork implements the Fork interface for Anvil instances
type anvilFork struct {
	id      string
	port    int
	rpcURL  string
	client  *ethclient.Client
	cmd     *exec.Cmd
	healthy bool
	mu      sync.RWMutex
}

// GetID returns the unique identifier for this fork
func (f *anvilFork) GetID() string {
	return f.id
}

// ExecuteTransaction executes a transaction on the fork and returns simulation results
func (f *anvilFork) ExecuteTransaction(ctx context.Context, tx *mevtypes.Transaction) (*interfaces.SimulationResult, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if !f.healthy {
		return nil, fmt.Errorf("fork %s is not healthy", f.id)
	}

	startTime := time.Now()

	// Convert our transaction type to go-ethereum transaction
	ethTx, err := f.convertTransaction(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to convert transaction: %w", err)
	}

	// Get pre-execution state
	addresses := []common.Address{tx.From}
	if tx.To != nil {
		addresses = append(addresses, *tx.To)
	}
	preState, err := f.captureState(ctx, addresses)
	if err != nil {
		return nil, fmt.Errorf("failed to capture pre-state: %w", err)
	}

	// Send the transaction
	err = f.client.SendTransaction(ctx, ethTx)
	if err != nil {
		return &interfaces.SimulationResult{
			Success:       false,
			Error:         err,
			ExecutionTime: time.Since(startTime),
		}, nil
	}

	// Wait for transaction to be mined (Anvil auto-mines)
	receipt, err := f.waitForReceipt(ctx, ethTx.Hash())
	if err != nil {
		return &interfaces.SimulationResult{
			Success:       false,
			Error:         err,
			ExecutionTime: time.Since(startTime),
		}, nil
	}

	// Get post-execution state
	postState, err := f.captureState(ctx, addresses)
	if err != nil {
		return nil, fmt.Errorf("failed to capture post-state: %w", err)
	}

	// Calculate state changes
	stateChanges := f.calculateStateChanges(preState, postState)

	result := &interfaces.SimulationResult{
		Success:       receipt.Status == types.ReceiptStatusSuccessful,
		GasUsed:       receipt.GasUsed,
		GasPrice:      ethTx.GasPrice(),
		Receipt:       receipt,
		Logs:          receipt.Logs,
		StateChanges:  stateChanges,
		ExecutionTime: time.Since(startTime),
	}

	return result, nil
}

// GetBlockNumber returns the current block number of the fork
func (f *anvilFork) GetBlockNumber() (*big.Int, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if !f.healthy {
		return nil, fmt.Errorf("fork %s is not healthy", f.id)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	blockNumber, err := f.client.BlockNumber(ctx)
	if err != nil {
		f.markUnhealthy()
		return nil, fmt.Errorf("failed to get block number: %w", err)
	}

	return big.NewInt(int64(blockNumber)), nil
}

// GetBalance returns the balance of an address on the fork
func (f *anvilFork) GetBalance(address common.Address) (*big.Int, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if !f.healthy {
		return nil, fmt.Errorf("fork %s is not healthy", f.id)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	balance, err := f.client.BalanceAt(ctx, address, nil)
	if err != nil {
		f.markUnhealthy()
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	return balance, nil
}

// Reset resets the fork to its initial state
func (f *anvilFork) Reset() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.healthy {
		return fmt.Errorf("fork %s is not healthy", f.id)
	}

	// Use anvil_reset RPC method to reset the fork state
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result interface{}
	err := f.client.Client().CallContext(ctx, &result, "anvil_reset")
	if err != nil {
		f.markUnhealthy()
		return fmt.Errorf("failed to reset fork: %w", err)
	}

	return nil
}

// Close shuts down the fork instance
func (f *anvilFork) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.healthy = false

	if f.client != nil {
		f.client.Close()
	}

	if f.cmd != nil && f.cmd.Process != nil {
		if err := f.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill anvil process: %w", err)
		}
		f.cmd.Wait() // Wait for process to exit
	}

	return nil
}

// IsHealthy returns whether the fork is healthy and operational
func (f *anvilFork) IsHealthy() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if !f.healthy {
		return false
	}

	// Perform a quick health check
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := f.client.BlockNumber(ctx)
	if err != nil {
		f.markUnhealthy()
		return false
	}

	return true
}

// markUnhealthy marks the fork as unhealthy (internal method)
func (f *anvilFork) markUnhealthy() {
	f.healthy = false
}

// convertTransaction converts our transaction type to go-ethereum transaction
func (f *anvilFork) convertTransaction(tx *mevtypes.Transaction) (*types.Transaction, error) {
	// Create a legacy transaction for simplicity
	// In a production system, you might want to support different transaction types
	var to common.Address
	if tx.To != nil {
		to = *tx.To
	}
	
	ethTx := types.NewTransaction(
		tx.Nonce,
		to,
		tx.Value,
		tx.GasLimit,
		tx.GasPrice,
		tx.Data,
	)

	return ethTx, nil
}

// waitForReceipt waits for a transaction receipt
func (f *anvilFork) waitForReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for transaction receipt")
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			receipt, err := f.client.TransactionReceipt(ctx, txHash)
			if err != nil {
				continue // Transaction not yet mined
			}
			return receipt, nil
		}
	}
}

// captureState captures the current state of specified addresses
func (f *anvilFork) captureState(ctx context.Context, addresses []common.Address) (map[common.Address]*interfaces.AccountState, error) {
	state := make(map[common.Address]*interfaces.AccountState)

	for _, addr := range addresses {
		balance, err := f.client.BalanceAt(ctx, addr, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get balance for %s: %w", addr.Hex(), err)
		}

		nonce, err := f.client.NonceAt(ctx, addr, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get nonce for %s: %w", addr.Hex(), err)
		}

		code, err := f.client.CodeAt(ctx, addr, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get code for %s: %w", addr.Hex(), err)
		}

		state[addr] = &interfaces.AccountState{
			Balance: balance,
			Nonce:   nonce,
			Code:    code,
			Storage: make(map[common.Hash]common.Hash), // Storage would need more complex implementation
		}
	}

	return state, nil
}

// calculateStateChanges calculates the differences between pre and post states
func (f *anvilFork) calculateStateChanges(preState, postState map[common.Address]*interfaces.AccountState) map[common.Address]*interfaces.AccountState {
	changes := make(map[common.Address]*interfaces.AccountState)

	for addr, postAccount := range postState {
		preAccount, exists := preState[addr]
		if !exists {
			// New account
			changes[addr] = postAccount
			continue
		}

		// Check for changes
		if preAccount.Balance.Cmp(postAccount.Balance) != 0 ||
			preAccount.Nonce != postAccount.Nonce {
			changes[addr] = &interfaces.AccountState{
				Balance: new(big.Int).Sub(postAccount.Balance, preAccount.Balance),
				Nonce:   postAccount.Nonce - preAccount.Nonce,
				Code:    postAccount.Code,
				Storage: make(map[common.Hash]common.Hash),
			}
		}
	}

	return changes
}