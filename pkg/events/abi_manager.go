package events

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/mev-engine/l2-mev-strategy-engine/pkg/interfaces"
)

// ABIManagerImpl implements the ABIManager interface
type ABIManagerImpl struct {
	abis      map[string]*abi.ABI
	mu        sync.RWMutex
	protocols map[interfaces.Protocol]map[interfaces.ContractType]string
}

// NewABIManager creates a new ABI manager instance
func NewABIManager() *ABIManagerImpl {
	manager := &ABIManagerImpl{
		abis:      make(map[string]*abi.ABI),
		protocols: make(map[interfaces.Protocol]map[interfaces.ContractType]string),
	}
	
	// Initialize protocol mappings
	manager.initializeProtocolMappings()
	
	// Load default ABIs
	manager.loadDefaultABIs()
	
	return manager
}

// LoadABI loads an ABI for a specific protocol and contract type
func (m *ABIManagerImpl) LoadABI(protocol interfaces.Protocol, contractType interfaces.ContractType) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	key := m.getABIKey(protocol, contractType)
	if _, exists := m.abis[key]; exists {
		return nil // Already loaded
	}
	
	abiJSON, err := m.getABIJSON(protocol, contractType)
	if err != nil {
		return fmt.Errorf("failed to get ABI JSON for %s %s: %w", protocol.String(), contractType.String(), err)
	}
	
	parsedABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return fmt.Errorf("failed to parse ABI for %s %s: %w", protocol.String(), contractType.String(), err)
	}
	
	m.abis[key] = &parsedABI
	return nil
}

// GetABI returns the ABI for a specific protocol and contract type
func (m *ABIManagerImpl) GetABI(protocol interfaces.Protocol, contractType interfaces.ContractType) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	key := m.getABIKey(protocol, contractType)
	abiObj, exists := m.abis[key]
	if !exists {
		return nil, fmt.Errorf("ABI not found for %s %s", protocol.String(), contractType.String())
	}
	
	return json.Marshal(abiObj)
}

// IsEventSupported checks if an event signature is supported for a protocol
func (m *ABIManagerImpl) IsEventSupported(protocol interfaces.Protocol, eventSignature string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Check all contract types for this protocol
	if protocolMap, exists := m.protocols[protocol]; exists {
		for contractType := range protocolMap {
			key := m.getABIKey(protocol, contractType)
			if abiObj, exists := m.abis[key]; exists {
				for _, event := range abiObj.Events {
					if event.Sig == eventSignature {
						return true
					}
				}
			}
		}
	}
	
	return false
}

// GetEventSignature returns the event signature hash for a given event name
func (m *ABIManagerImpl) GetEventSignature(protocol interfaces.Protocol, eventName string) (common.Hash, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Check all contract types for this protocol
	if protocolMap, exists := m.protocols[protocol]; exists {
		for contractType := range protocolMap {
			key := m.getABIKey(protocol, contractType)
			if abiObj, exists := m.abis[key]; exists {
				if event, exists := abiObj.Events[eventName]; exists {
					return event.ID, nil
				}
			}
		}
	}
	
	return common.Hash{}, fmt.Errorf("event %s not found for protocol %s", eventName, protocol.String())
}

// getABIKey generates a unique key for protocol and contract type combination
func (m *ABIManagerImpl) getABIKey(protocol interfaces.Protocol, contractType interfaces.ContractType) string {
	return fmt.Sprintf("%s_%d", protocol.String(), int(contractType))
}

// initializeProtocolMappings sets up the mapping between protocols and their contract types
func (m *ABIManagerImpl) initializeProtocolMappings() {
	m.protocols[interfaces.ProtocolUniswapV2] = map[interfaces.ContractType]string{
		interfaces.ContractTypePair:   "uniswap_v2_pair",
		interfaces.ContractTypeRouter: "uniswap_v2_router",
	}
	
	m.protocols[interfaces.ProtocolUniswapV3] = map[interfaces.ContractType]string{
		interfaces.ContractTypePool:   "uniswap_v3_pool",
		interfaces.ContractTypeRouter: "uniswap_v3_router",
	}
	
	m.protocols[interfaces.ProtocolAerodrome] = map[interfaces.ContractType]string{
		interfaces.ContractTypePair:   "aerodrome_pair",
		interfaces.ContractTypeRouter: "aerodrome_router",
	}
	
	m.protocols[interfaces.ProtocolBaseBridge] = map[interfaces.ContractType]string{
		interfaces.ContractTypeBridge: "base_bridge",
	}
}

// loadDefaultABIs loads the default ABIs for all supported protocols
func (m *ABIManagerImpl) loadDefaultABIs() {
	protocols := []interfaces.Protocol{
		interfaces.ProtocolUniswapV2,
		interfaces.ProtocolUniswapV3,
		interfaces.ProtocolAerodrome,
		interfaces.ProtocolBaseBridge,
	}
	
	for _, protocol := range protocols {
		if contractTypes, exists := m.protocols[protocol]; exists {
			for contractType := range contractTypes {
				_ = m.LoadABI(protocol, contractType) // Ignore errors for now
			}
		}
	}
}

// getABIJSON returns the ABI JSON string for a protocol and contract type
func (m *ABIManagerImpl) getABIJSON(protocol interfaces.Protocol, contractType interfaces.ContractType) (string, error) {
	switch protocol {
	case interfaces.ProtocolUniswapV2:
		return m.getUniswapV2ABI(contractType)
	case interfaces.ProtocolUniswapV3:
		return m.getUniswapV3ABI(contractType)
	case interfaces.ProtocolAerodrome:
		return m.getAerodromeABI(contractType)
	case interfaces.ProtocolBaseBridge:
		return m.getBaseBridgeABI(contractType)
	default:
		return "", fmt.Errorf("unsupported protocol: %s", protocol.String())
	}
}