package model

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"

	"github.com/oasisprotocol/curve25519-voi/primitives/ed25519"
	"github.com/zeebo/blake3"
)

// Validator represents a node in the committee
type Validator struct {
	Address       net.Addr      `json:"address"`
	TxAddress     net.Addr      `json:"tx_address"`
	BlockAddress  net.Addr      `json:"block_address"`
	VertexAddress net.Addr      `json:"vertex_address"`
	PublicKey     NodePublicKey `json:"public_key"`
}

// NewValidator creates a new validator with the given parameters
func NewValidator(keypairHex string, port, txPort, blockPort, vertexPort uint16) (*Validator, error) {
	// Parse keypair
	keypairBytes, err := hex.DecodeString(keypairHex)
	if err != nil {
		return nil, err
	}

	// Create ed25519 keypair
	keypair := ed25519.NewKeyFromSeed(keypairBytes[:32])

	// Create public key hash
	hasher := blake3.New()
	hasher.Write(keypair.Public().(ed25519.PublicKey))
	publicKeyHash := hasher.Sum(nil)

	var publicKey NodePublicKey
	copy(publicKey[:], publicKeyHash[:32])

	// Create addresses
	addr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort("0.0.0.0", fmt.Sprintf("%d", port)))
	if err != nil {
		return nil, err
	}

	txAddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort("0.0.0.0", fmt.Sprintf("%d", txPort)))
	if err != nil {
		return nil, err
	}

	blockAddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort("0.0.0.0", fmt.Sprintf("%d", blockPort)))
	if err != nil {
		return nil, err
	}

	vertexAddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort("0.0.0.0", fmt.Sprintf("%d", vertexPort)))
	if err != nil {
		return nil, err
	}

	return &Validator{
		Address:       addr,
		TxAddress:     txAddr,
		BlockAddress:  blockAddr,
		VertexAddress: vertexAddr,
		PublicKey:     publicKey,
	}, nil
}

// Committee represents a group of validators
type Committee struct {
	Validators map[ID]*Validator `json:"validators"`
}

// NewCommittee creates a new committee with 7 random validators
func NewCommittee() *Committee {
	committee := &Committee{
		Validators: make(map[ID]*Validator),
	}

	// Generate genesis validators
	basePort := uint16(15000) // Starting port number
	numValidators := 7

	for i := 1; i <= numValidators; i++ {
		id := ID(i)

		// Generate random seed for ed25519 keypair
		seed := make([]byte, 32)
		if _, err := rand.Read(seed); err != nil {
			panic(fmt.Sprintf("Failed to generate random seed: %v", err))
		}

		// Convert the seed to hex string for the validator
		keypairHex := hex.EncodeToString(seed) + hex.EncodeToString(make([]byte, 32)) // Add padding to match expected length

		// Assign sequential ports for each validator
		portBase := basePort + uint16((i-1)*10)
		txPort := portBase
		blockPort := portBase + 1
		vertexPort := portBase + 2

		validator, err := NewValidator(keypairHex, txPort, txPort, blockPort, vertexPort)
		if err != nil {
			panic(fmt.Sprintf("Failed to create validator: %v", err))
		}

		committee.Validators[id] = validator
	}

	return committee
}

// Size returns the number of validators in the committee
func (c *Committee) Size() int {
	return len(c.Validators)
}

// QuorumThreshold returns the minimum number of validators required for consensus
func (c *Committee) QuorumThreshold() int {
	return c.Size() - 1
}

// GetNodeAddress returns the address of a specific validator
func (c *Committee) GetNodeAddress(id ID) net.Addr {
	if v, ok := c.Validators[id]; ok {
		return v.Address
	}
	return nil
}

// GetNodeAddresses returns all validator addresses
func (c *Committee) GetNodeAddresses() []net.Addr {
	addresses := make([]net.Addr, 0, len(c.Validators))
	for _, v := range c.Validators {
		addresses = append(addresses, v.Address)
	}
	return addresses
}

// GetTxReceiverAddress returns the transaction address of a specific validator
func (c *Committee) GetTxReceiverAddress(id ID) net.Addr {
	if v, ok := c.Validators[id]; ok {
		return v.TxAddress
	}
	return nil
}

// GetTxReceiverAddresses returns all validator transaction addresses
func (c *Committee) GetTxReceiverAddresses() []net.Addr {
	addresses := make([]net.Addr, 0, len(c.Validators))
	for _, v := range c.Validators {
		addresses = append(addresses, v.TxAddress)
	}
	return addresses
}

// GetBlockReceiverAddress returns the block address of a specific validator
func (c *Committee) GetBlockReceiverAddress(id ID) net.Addr {
	if v, ok := c.Validators[id]; ok {
		return v.BlockAddress
	}
	return nil
}

// GetBlockReceiverAddresses returns all validator block addresses
func (c *Committee) GetBlockReceiverAddresses() []net.Addr {
	addresses := make([]net.Addr, 0, len(c.Validators))
	for _, v := range c.Validators {
		addresses = append(addresses, v.BlockAddress)
	}
	return addresses
}

// GetNodeAddressesButMe returns all validator addresses except the specified one
func (c *Committee) GetNodeAddressesButMe(id ID) []net.Addr {
	addresses := make([]net.Addr, 0, len(c.Validators)-1)
	for vid, v := range c.Validators {
		if vid != id {
			addresses = append(addresses, v.Address)
		}
	}
	return addresses
}

// GetNodesKeys returns all validator public keys
func (c *Committee) GetNodesKeys() []NodePublicKey {
	keys := make([]NodePublicKey, 0, len(c.Validators))
	for _, v := range c.Validators {
		keys = append(keys, v.PublicKey)
	}
	return keys
}

// GetNodeKey returns the public key of a specific validator
func (c *Committee) GetNodeKey(id ID) *NodePublicKey {
	if v, ok := c.Validators[id]; ok {
		return &v.PublicKey
	}
	return nil
}

// GetVertexReceiverAddress returns the vertex receiver address for the given node
func (c *Committee) GetVertexReceiverAddress(nodeID ID) net.Addr {
	if v, ok := c.Validators[nodeID]; ok {
		return v.VertexAddress
	}
	return nil
}

// GetVertexReceiverAddresses returns all vertex receiver addresses except the given node
func (c *Committee) GetVertexReceiverAddresses() []net.Addr {
	addresses := make([]net.Addr, 0, len(c.Validators))
	for _, validator := range c.Validators {
		addresses = append(addresses, validator.VertexAddress)
	}
	return addresses
}
