package consensus

import (
	"sync"

	"github.com/unicornultrafoundation/go-helios-v2-primitive/pkg/model"
)

// State represents the consensus state
type State struct {
	mu sync.RWMutex

	// DAG state
	dag *DAG

	// Current round
	currentRound model.Round

	// Delivered vertices
	deliveredVertices map[model.VertexHash]struct{}

	// Buffer for vertices
	buffer []*model.Vertex

	// Blocks to propose
	blocksToPropose []*model.Block
}

// NewState creates a new consensus state
func NewState(genesis *model.Vertex) *State {
	state := &State{
		dag:               NewDAG(genesis),
		currentRound:      0, // Start at round 0
		deliveredVertices: make(map[model.VertexHash]struct{}),
		buffer:            make([]*model.Vertex, 0),
		blocksToPropose:   make([]*model.Block, 0),
	}

	// Mark genesis vertex as delivered
	state.deliveredVertices[genesis.Hash()] = struct{}{}

	return state
}

// GetCurrentRound returns the current round
func (s *State) GetCurrentRound() model.Round {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentRound
}

// AdvanceRound advances to the next round
func (s *State) AdvanceRound() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentRound++
	s.processBuffer() // Try to process buffered vertices after advancing
}

// AddVertex adds a vertex to the state
func (s *State) AddVertex(vertex *model.Vertex) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If vertex is already delivered, ignore it
	if _, ok := s.deliveredVertices[vertex.Hash()]; ok {
		return
	}

	// Convert parents map to slice
	parentHashes := make([]model.VertexHash, 0, len(vertex.Parents()))
	for hash := range vertex.Parents() {
		parentHashes = append(parentHashes, hash)
	}

	// If vertex is for current round or earlier and all parents are present, add it
	if vertex.Round() <= s.currentRound && s.dag.ContainsVertices(parentHashes) {
		s.dag.InsertVertex(vertex)
		s.deliveredVertices[vertex.Hash()] = struct{}{}
		s.processBuffer()
		return
	}

	// Otherwise, add to buffer
	s.buffer = append(s.buffer, vertex)
}

// processBuffer processes vertices in the buffer
func (s *State) processBuffer() {
	processed := true
	for processed {
		processed = false
		remaining := make([]*model.Vertex, 0)

		for _, v := range s.buffer {
			parentHashes := make([]model.VertexHash, 0, len(v.Parents()))
			for hash := range v.Parents() {
				parentHashes = append(parentHashes, hash)
			}

			if v.Round() <= s.currentRound && s.dag.ContainsVertices(parentHashes) {
				s.dag.InsertVertex(v)
				s.deliveredVertices[v.Hash()] = struct{}{}
				processed = true
			} else {
				remaining = append(remaining, v)
			}
		}

		s.buffer = remaining
	}
}

// GetVertexCountInRound returns the number of vertices in a round
func (s *State) GetVertexCountInRound(round model.Round) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dag.GetVertexCountInRound(round)
}

// GetVerticesInRound returns all vertices in a round
func (s *State) GetVerticesInRound(round model.Round) []*model.Vertex {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dag.GetVerticesInRound(round)
}

// GetStrongParents returns strong parents for the current round
func (s *State) GetStrongParents() []model.VertexHash {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dag.GetStrongParents(s.currentRound)
}

// AddBlockToPropose adds a block to be proposed
func (s *State) AddBlockToPropose(block *model.Block) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blocksToPropose = append(s.blocksToPropose, block)
}

// GetNextBlockToPropose returns and removes the next block to propose
func (s *State) GetNextBlockToPropose() *model.Block {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.blocksToPropose) == 0 {
		return nil
	}
	block := s.blocksToPropose[0]
	s.blocksToPropose = s.blocksToPropose[1:]
	return block
}

// HasBlocksToPropose returns true if there are blocks to propose
func (s *State) HasBlocksToPropose() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.blocksToPropose) > 0
}

// IsVertexDelivered returns true if a vertex has been delivered
func (s *State) IsVertexDelivered(hash model.VertexHash) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.deliveredVertices[hash]
	return ok
}

// MarkVertexDelivered marks a vertex as delivered
func (s *State) MarkVertexDelivered(hash model.VertexHash) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deliveredVertices[hash] = struct{}{}
}

// GetWaveVertexLeader returns the leader vertex for a wave
func (s *State) GetWaveVertexLeader(wave model.Wave) *model.Vertex {
	s.mu.RLock()
	defer s.mu.RUnlock()
	round := s.GetRoundForWave(wave)
	vertices := s.dag.GetVerticesInRound(round)
	if len(vertices) == 0 {
		return nil
	}
	// Return the first vertex in the round as leader
	return vertices[0]
}

// GetRoundForWave returns the round number for a wave
func (s *State) GetRoundForWave(wave model.Wave) model.Round {
	return model.Round(wave * MaxWave)
}

// IsLastRoundInWave returns true if the current round is the last in a wave
func (s *State) IsLastRoundInWave() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentRound%MaxWave == MaxWave-1
}

// GetOrderedVertices returns vertices in order for a wave
func (s *State) GetOrderedVertices(wave model.Wave) []*model.Vertex {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get start and end rounds for the wave
	startRound := s.GetRoundForWave(wave)
	endRound := startRound + MaxWave - 1

	// Collect vertices from all rounds in the wave
	var result []*model.Vertex
	for round := startRound; round <= endRound; round++ {
		vertices := s.dag.GetVerticesInRound(round)
		if vertices != nil {
			result = append(result, vertices...)
		}
	}

	return result
}
