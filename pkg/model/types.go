package model

import (
	"encoding/base64"
	"fmt"
)

const (
	DefaultChannelCapacity = 10000
)

type (
	Round uint64
	Wave  uint64
	ID    uint32

	// NodePublicKey represents a node's public key as a 32-byte array
	NodePublicKey [32]byte

	// VertexHash represents a vertex's hash as a 32-byte array
	VertexHash [32]byte

	// BlockHash represents a block's hash as a 32-byte array
	BlockHash [32]byte

	// Transaction represents a raw transaction as a byte slice
	Transaction []byte
)

// MarshalJSON implements the json.Marshaler interface for NodePublicKey
func (k NodePublicKey) MarshalJSON() ([]byte, error) {
	return []byte(`"` + base64.StdEncoding.EncodeToString(k[:]) + `"`), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for NodePublicKey
func (k *NodePublicKey) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("invalid NodePublicKey format")
	}
	decoded, err := base64.StdEncoding.DecodeString(string(data[1 : len(data)-1]))
	if err != nil {
		return fmt.Errorf("invalid base64 encoding: %v", err)
	}
	copy(k[:], decoded)
	return nil
}

// MarshalJSON implements the json.Marshaler interface for VertexHash
func (h VertexHash) MarshalJSON() ([]byte, error) {
	return []byte(`"` + base64.StdEncoding.EncodeToString(h[:]) + `"`), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for VertexHash
func (h *VertexHash) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("invalid VertexHash format")
	}
	decoded, err := base64.StdEncoding.DecodeString(string(data[1 : len(data)-1]))
	if err != nil {
		return fmt.Errorf("invalid base64 encoding: %v", err)
	}
	copy(h[:], decoded)
	return nil
}

// MarshalJSON implements the json.Marshaler interface for BlockHash
func (h BlockHash) MarshalJSON() ([]byte, error) {
	return []byte(`"` + base64.StdEncoding.EncodeToString(h[:]) + `"`), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for BlockHash
func (h *BlockHash) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("invalid BlockHash format")
	}
	decoded, err := base64.StdEncoding.DecodeString(string(data[1 : len(data)-1]))
	if err != nil {
		return fmt.Errorf("invalid base64 encoding: %v", err)
	}
	copy(h[:], decoded)
	return nil
}
