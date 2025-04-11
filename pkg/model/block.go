package model

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zeebo/blake3"
)

// Block represents a block in the blockchain
type Block struct {
	// hash is the hash of this block
	hash BlockHash `json:"hash"`

	// Height is the height of this block in the chain
	Height uint64 `json:"height"`

	// Transactions are the transactions in this block
	Transactions []Transaction `json:"transactions"`

	// Timestamp is when the block was created
	Timestamp time.Time `json:"timestamp"`
}

// NewBlock creates a new block with the given parameters
func NewBlock(height uint64, transactions []Transaction) *Block {
	b := &Block{
		Height:       height,
		Transactions: transactions,
		Timestamp:    time.Now(),
	}
	b.hash = b.calculateHash()
	return b
}

// calculateHash computes the hash of the block
func (b *Block) calculateHash() BlockHash {
	h := blake3.New()

	// Write height
	binary.Write(h, binary.BigEndian, b.Height)

	// Write transaction bytes
	for _, tx := range b.Transactions {
		h.Write(tx)
	}

	// Write timestamp
	binary.Write(h, binary.BigEndian, b.Timestamp.UnixNano())

	var hash BlockHash
	copy(hash[:], h.Sum(nil))
	return hash
}

// Hash returns a pointer to the block's hash
func (b *Block) Hash() *BlockHash {
	return &b.hash
}

// String returns a string representation of the block
func (b *Block) String() string {
	return fmt.Sprintf("Block(hash=%s, height=%d, tx_count=%d)",
		hex.EncodeToString(b.hash[:]),
		b.Height,
		len(b.Transactions),
	)
}

// MarshalBinary implements the encoding.BinaryMarshaler interface
func (b *Block) MarshalBinary() ([]byte, error) {
	return json.Marshal(b)
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface
func (b *Block) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, b)
}
