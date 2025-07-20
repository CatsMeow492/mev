package events

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewABIManager(t *testing.T) {
	manager := NewABIManager()
	
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.abis)
	assert.NotNil(t, manager.protocols)
	
	// Check that protocol mappings are initialized
	assert.Contains(t, manager.protocols, interfaces.ProtocolUniswapV2)
	assert.Contains(t, manager.protocols, interfaces.ProtocolUniswapV3)
	assert.Contains(t, manager.protocols, interfaces.ProtocolAerodrome)
	assert.Contains(t, manager.protocols, interfaces.ProtocolBaseBridge)
}

func TestLoadABI(t *testing.T) {
	manager := NewABIManager()
	
	tests := []struct {
		name         string
		protocol     interfaces.Protocol
		contractType interfaces.ContractType
		expectError  bool
	}{
		{
			name:         "Load Uniswap V2 Pair ABI",
			protocol:     interfaces.ProtocolUniswapV2,
			contractType: interfaces.ContractTypePair,
			expectError:  false,
		},
		{
			name:         "Load Uniswap V3 Pool ABI",
			protocol:     interfaces.ProtocolUniswapV3,
			contractType: interfaces.ContractTypePool,
			expectError:  false,
		},
		{
			name:         "Load Aerodrome Pair ABI",
			protocol:     interfaces.ProtocolAerodrome,
			contractType: interfaces.ContractTypePair,
			expectError:  false,
		},
		{
			name:         "Load Base Bridge ABI",
			protocol:     interfaces.ProtocolBaseBridge,
			contractType: interfaces.ContractTypeBridge,
			expectError:  false,
		},
		{
			name:         "Load unsupported protocol",
			protocol:     interfaces.ProtocolUnknown,
			contractType: interfaces.ContractTypePair,
			expectError:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.LoadABI(tt.protocol, tt.contractType)
			
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				
				// Verify ABI is loaded
				key := manager.getABIKey(tt.protocol, tt.contractType)
				assert.Contains(t, manager.abis, key)
			}
		})
	}
}

func TestGetABI(t *testing.T) {
	manager := NewABIManager()
	
	// Load an ABI first
	err := manager.LoadABI(interfaces.ProtocolUniswapV2, interfaces.ContractTypePair)
	require.NoError(t, err)
	
	// Test getting the ABI
	abiBytes, err := manager.GetABI(interfaces.ProtocolUniswapV2, interfaces.ContractTypePair)
	assert.NoError(t, err)
	assert.NotEmpty(t, abiBytes)
	
	// Test getting non-existent ABI
	_, err = manager.GetABI(interfaces.ProtocolUnknown, interfaces.ContractTypePair)
	assert.Error(t, err)
}

func TestIsEventSupported(t *testing.T) {
	manager := NewABIManager()
	
	// Load ABIs
	err := manager.LoadABI(interfaces.ProtocolUniswapV2, interfaces.ContractTypePair)
	require.NoError(t, err)
	
	// Test supported event (we need to calculate the actual signature)
	// For now, we'll test the method exists and handles unknown signatures
	supported := manager.IsEventSupported(interfaces.ProtocolUniswapV2, "0x1234567890abcdef")
	assert.False(t, supported) // Should be false for random signature
	
	// Test unsupported protocol
	supported = manager.IsEventSupported(interfaces.ProtocolUnknown, "0x1234567890abcdef")
	assert.False(t, supported)
}

func TestGetEventSignature(t *testing.T) {
	manager := NewABIManager()
	
	// Load ABIs
	err := manager.LoadABI(interfaces.ProtocolUniswapV2, interfaces.ContractTypePair)
	require.NoError(t, err)
	
	// Test getting Swap event signature
	signature, err := manager.GetEventSignature(interfaces.ProtocolUniswapV2, "Swap")
	assert.NoError(t, err)
	assert.NotEqual(t, common.Hash{}, signature)
	
	// Test getting non-existent event
	_, err = manager.GetEventSignature(interfaces.ProtocolUniswapV2, "NonExistentEvent")
	assert.Error(t, err)
	
	// Test unsupported protocol
	_, err = manager.GetEventSignature(interfaces.ProtocolUnknown, "Swap")
	assert.Error(t, err)
}

func TestGetABIKey(t *testing.T) {
	manager := NewABIManager()
	
	key1 := manager.getABIKey(interfaces.ProtocolUniswapV2, interfaces.ContractTypePair)
	key2 := manager.getABIKey(interfaces.ProtocolUniswapV3, interfaces.ContractTypePool)
	key3 := manager.getABIKey(interfaces.ProtocolUniswapV2, interfaces.ContractTypePair)
	
	// Keys should be unique for different protocol/contract combinations
	assert.NotEqual(t, key1, key2)
	
	// Keys should be consistent for same protocol/contract combination
	assert.Equal(t, key1, key3)
	
	// Keys should contain protocol information
	assert.Contains(t, key1, "UniswapV2")
	assert.Contains(t, key2, "UniswapV3")
}

func TestGetUniswapV2ABI(t *testing.T) {
	manager := NewABIManager()
	
	// Test supported contract types
	abi, err := manager.getUniswapV2ABI(interfaces.ContractTypePair)
	assert.NoError(t, err)
	assert.NotEmpty(t, abi)
	assert.Contains(t, abi, "Swap") // Should contain Swap event
	
	abi, err = manager.getUniswapV2ABI(interfaces.ContractTypeRouter)
	assert.NoError(t, err)
	assert.NotEmpty(t, abi)
	
	// Test unsupported contract type
	_, err = manager.getUniswapV2ABI(interfaces.ContractTypeBridge)
	assert.Error(t, err)
}

func TestGetUniswapV3ABI(t *testing.T) {
	manager := NewABIManager()
	
	// Test supported contract types
	abi, err := manager.getUniswapV3ABI(interfaces.ContractTypePool)
	assert.NoError(t, err)
	assert.NotEmpty(t, abi)
	assert.Contains(t, abi, "Swap") // Should contain Swap event
	
	abi, err = manager.getUniswapV3ABI(interfaces.ContractTypeRouter)
	assert.NoError(t, err)
	assert.NotEmpty(t, abi)
	
	// Test unsupported contract type
	_, err = manager.getUniswapV3ABI(interfaces.ContractTypeBridge)
	assert.Error(t, err)
}

func TestGetAerodromeABI(t *testing.T) {
	manager := NewABIManager()
	
	// Test supported contract types
	abi, err := manager.getAerodromeABI(interfaces.ContractTypePair)
	assert.NoError(t, err)
	assert.NotEmpty(t, abi)
	assert.Contains(t, abi, "Swap") // Should contain Swap event
	
	abi, err = manager.getAerodromeABI(interfaces.ContractTypeRouter)
	assert.NoError(t, err)
	assert.NotEmpty(t, abi)
	
	// Test unsupported contract type
	_, err = manager.getAerodromeABI(interfaces.ContractTypeBridge)
	assert.Error(t, err)
}

func TestGetBaseBridgeABI(t *testing.T) {
	manager := NewABIManager()
	
	// Test supported contract type
	abi, err := manager.getBaseBridgeABI(interfaces.ContractTypeBridge)
	assert.NoError(t, err)
	assert.NotEmpty(t, abi)
	assert.Contains(t, abi, "DepositInitiated") // Should contain bridge events
	assert.Contains(t, abi, "WithdrawalInitiated")
	
	// Test unsupported contract type
	_, err = manager.getBaseBridgeABI(interfaces.ContractTypePair)
	assert.Error(t, err)
}

func TestConcurrentAccess(t *testing.T) {
	manager := NewABIManager()
	
	// Test concurrent loading and reading
	done := make(chan bool, 10)
	
	// Start multiple goroutines loading ABIs
	for i := 0; i < 5; i++ {
		go func() {
			defer func() { done <- true }()
			err := manager.LoadABI(interfaces.ProtocolUniswapV2, interfaces.ContractTypePair)
			assert.NoError(t, err)
		}()
	}
	
	// Start multiple goroutines reading ABIs
	for i := 0; i < 5; i++ {
		go func() {
			defer func() { done <- true }()
			_, err := manager.GetABI(interfaces.ProtocolUniswapV2, interfaces.ContractTypePair)
			// Error is expected if ABI hasn't been loaded yet
			_ = err
		}()
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// Verify final state
	abiBytes, err := manager.GetABI(interfaces.ProtocolUniswapV2, interfaces.ContractTypePair)
	assert.NoError(t, err)
	assert.NotEmpty(t, abiBytes)
}