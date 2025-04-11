package model

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/zeebo/blake3"
)

// Vertex represents a vertex in the DAG
type Vertex struct {
	// hash is the unique identifier of this vertex
	hash VertexHash `json:"hash"`

	// owner is the node that created this vertex
	owner NodePublicKey `json:"owner"`

	// block contains the transactions for this vertex
	block *Block `json:"block"`

	// parents maps parent vertex hashes to their rounds
	parents map[VertexHash]Round `json:"parents"`

	// round is the round number of this vertex
	round Round `json:"round"`

	// Wave is the consensus wave this vertex was created in
	Wave Wave `json:"wave"`

	// Timestamp is when the vertex was created
	Timestamp time.Time `json:"timestamp"`
}

// NewVertex creates a new vertex with the given parameters
func NewVertex(owner NodePublicKey, round Round, block *Block, parents map[VertexHash]Round) *Vertex {
	v := &Vertex{
		owner:     owner,
		round:     round,
		block:     block,
		parents:   parents,
		Wave:      1,
		Timestamp: time.Now(),
	}
	v.hash = v.calculateHash()
	return v
}

// Genesis creates genesis vertices for each node
func Genesis(nodes []NodePublicKey) []*Vertex {
	vertices := make([]*Vertex, len(nodes))
	for i, owner := range nodes {
		vertices[i] = NewVertex(owner, 1, NewBlock(0, nil), make(map[VertexHash]Round))
	}
	return vertices
}

// AddParent adds a parent vertex hash and its round
func (v *Vertex) AddParent(parentHash VertexHash, round Round) {
	v.parents[parentHash] = round
}

// GetStrongParents returns parents from the previous round
func (v *Vertex) GetStrongParents() map[VertexHash]Round {
	strongParents := make(map[VertexHash]Round)
	for hash, round := range v.parents {
		if v.isPreviousRound(round) {
			strongParents[hash] = round
		}
	}
	return strongParents
}

// GetAllParents returns all parents
func (v *Vertex) GetAllParents() map[VertexHash]Round {
	parents := make(map[VertexHash]Round)
	for hash, round := range v.parents {
		parents[hash] = round
	}
	return parents
}

// IsWeakParent checks if a vertex hash is a weak parent
func (v *Vertex) IsWeakParent(hash VertexHash) bool {
	if round, ok := v.parents[hash]; ok {
		return !v.isPreviousRound(round)
	}
	return false
}

// Round returns the vertex's round number
func (v *Vertex) Round() Round {
	return v.round
}

// Parents returns the vertex's parents
func (v *Vertex) Parents() map[VertexHash]Round {
	return v.parents
}

// Owner returns the vertex's owner
func (v *Vertex) Owner() NodePublicKey {
	return v.owner
}

// Hash returns the vertex's hash
func (v *Vertex) Hash() VertexHash {
	return v.hash
}

// Block returns the vertex's block
func (v *Vertex) Block() *Block {
	return v.block
}

// String returns a string representation of the vertex
func (v *Vertex) String() string {
	return fmt.Sprintf("Vertex (%d, %s) [owner: %s]",
		v.round,
		base64.StdEncoding.EncodeToString(v.hash[:]),
		base64.StdEncoding.EncodeToString(v.owner[:]))
}

// calculateHash computes the hash of the vertex
func (v *Vertex) calculateHash() VertexHash {
	h := blake3.New()

	// Write owner
	h.Write(v.owner[:])

	// Write round
	binary.Write(h, binary.BigEndian, v.round)

	// Write block hash
	h.Write(v.block.Hash()[:])

	// Write parents in sorted order for deterministic hashing
	parentHashes := make([]VertexHash, 0, len(v.parents))
	for hash := range v.parents {
		parentHashes = append(parentHashes, hash)
	}
	sort.Slice(parentHashes, func(i, j int) bool {
		for k := 0; k < 32; k++ {
			if parentHashes[i][k] != parentHashes[j][k] {
				return parentHashes[i][k] < parentHashes[j][k]
			}
		}
		return false
	})
	for _, hash := range parentHashes {
		h.Write(hash[:])
		binary.Write(h, binary.BigEndian, v.parents[hash])
	}

	var hash VertexHash
	copy(hash[:], h.Sum(nil))
	return hash
}

// isPreviousRound checks if a round number is the previous round
func (v *Vertex) isPreviousRound(round Round) bool {
	return v.round-round == 1
}

// MarshalBinary implements the encoding.BinaryMarshaler interface
func (v *Vertex) MarshalBinary() ([]byte, error) {
	return json.Marshal(v)
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface
func (v *Vertex) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, v)
}
