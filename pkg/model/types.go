package model

const (
	DefaultChannelCapacity = 1000
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
