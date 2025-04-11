package consensus

import (
	"sync"

	"github.com/lewtran/go-helios-v2/pkg/model"
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

	// Convert parents map to slice
	parentHashes := make([]model.VertexHash, 0, len(vertex.Parents()))
	for hash := range vertex.Parents() {
		parentHashes = append(parentHashes, hash)
	}

	// Add to buffer if not ready
	if vertex.Round() > s.currentRound || !s.dag.ContainsVertices(parentHashes) {
		s.buffer = append(s.buffer, vertex)
		return
	}

	// Add to DAG
	s.dag.InsertVertex(vertex)

	// Mark vertex as delivered
	s.deliveredVertices[vertex.Hash()] = struct{}{}

	// Process buffer
	s.processBuffer()
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
	if len(vertices) > 0 {
		return vertices[0]
	}
	return nil
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

// GetOrderedVertices returns vertices ordered by wave
func (s *State) GetOrderedVertices(wave model.Wave) []*model.Vertex {
	s.mu.Lock()
	defer s.mu.Unlock()

	leader := s.GetWaveVertexLeader(wave)
	if leader == nil {
		return nil
	}

	round := s.GetRoundForWave(wave)
	if !s.dag.IsLinkedWithOthersInRound(leader, round) {
		return nil
	}

	ordered := make([]*model.Vertex, 0)
	ordered = append(ordered, leader)

	// Add all vertices linked to the leader
	for round, vertices := range s.dag.GetVertices() {
		if round > 0 {
			for _, vertex := range vertices {
				if !s.IsVertexDelivered(vertex.Hash()) && s.dag.IsLinked(vertex, leader) {
					ordered = append(ordered, vertex)
					s.MarkVertexDelivered(vertex.Hash())
				}
			}
		}
	}

	return ordered
}

// getLeadersToCommit returns leaders that need to be committed
func (s *State) getLeadersToCommit(fromWave model.Wave, currentLeader *model.Vertex) []*model.Vertex {
	toCommit := []*model.Vertex{currentLeader}
	current := currentLeader

	for wave := fromWave; wave > 0; wave-- {
		prevLeader := s.dag.GetWaveVertexLeader(wave)
		if prevLeader != nil && s.dag.IsStronglyLinked(current, prevLeader) {
			toCommit = append(toCommit, prevLeader)
			current = prevLeader
		}
	}

	return toCommit
}

// orderVertices orders vertices based on leaders
func (s *State) orderVertices(leaders []*model.Vertex) []*model.Vertex {
	ordered := make([]*model.Vertex, 0)

	for _, leader := range leaders {
		for round, vertices := range s.dag.GetVertices() {
			if round > 0 {
				for _, vertex := range vertices {
					if !s.IsVertexDelivered(vertex.Hash()) && s.dag.IsLinked(vertex, leader) {
						ordered = append(ordered, vertex)
						s.MarkVertexDelivered(vertex.Hash())
					}
				}
			}
		}
	}

	return ordered
}
