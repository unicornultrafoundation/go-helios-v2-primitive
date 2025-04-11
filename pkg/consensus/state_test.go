package consensus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unicornultrafoundation/go-helios-v2-primitive/pkg/model"
)

// ===== STATE TESTS =====

// TestStateCreation tests creating a new consensus state
func TestStateCreation(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := model.NewVertex(nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create state
	state := NewState(genesis)

	// Verify state is created correctly
	assert.NotNil(t, state, "State should not be nil")
	assert.NotNil(t, state.dag, "State should have a DAG")
	assert.Equal(t, model.Round(0), state.currentRound, "State should start at round 0")
	assert.Equal(t, 1, len(state.deliveredVertices), "State should have one delivered vertex (genesis)")
	assert.Equal(t, 0, len(state.buffer), "State buffer should be empty")
	assert.Equal(t, 0, len(state.blocksToPropose), "State should have no blocks to propose")

	// Check that genesis vertex is marked as delivered
	assert.True(t, state.IsVertexDelivered(genesis.Hash()), "Genesis vertex should be marked as delivered")
}

// TestGetCurrentRound tests getting the current round
func TestGetCurrentRound(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := model.NewVertex(nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create state
	state := NewState(genesis)

	// Check current round
	assert.Equal(t, model.Round(0), state.GetCurrentRound(), "Current round should be 0")
}

// TestAdvanceRound tests advancing the round
func TestAdvanceRound(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := model.NewVertex(nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create state
	state := NewState(genesis)

	// Advance round
	state.AdvanceRound()

	// Check current round
	assert.Equal(t, model.Round(1), state.GetCurrentRound(), "Current round should be 1")

	// Advance again
	state.AdvanceRound()

	// Check current round
	assert.Equal(t, model.Round(2), state.GetCurrentRound(), "Current round should be 2")
}

// TestAddVertex tests adding a vertex to the state
func TestAddVertex(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := model.NewVertex(nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create state
	state := NewState(genesis)

	// Advance to round 1
	state.AdvanceRound()

	// Create a vertex in round 1
	block1 := model.NewBlock(1, []model.Transaction{CreateTestTransaction(64)})
	parents := map[model.VertexHash]model.Round{
		genesis.Hash(): 0,
	}
	vertex1 := model.NewVertex(nodeKey, 1, block1, parents)

	// Add vertex to state
	state.AddVertex(vertex1)

	// Check that vertex is in DAG
	count := state.GetVertexCountInRound(1)
	assert.Equal(t, 1, count, "Round 1 should have 1 vertex")

	// Check that vertex is marked as delivered
	assert.True(t, state.IsVertexDelivered(vertex1.Hash()), "Vertex should be marked as delivered")

	// Create a vertex in round 2 (should go to buffer since we're still at round 1)
	block2 := model.NewBlock(2, []model.Transaction{CreateTestTransaction(64)})
	parents2 := map[model.VertexHash]model.Round{
		vertex1.Hash(): 1,
	}
	vertex2 := model.NewVertex(nodeKey, 2, block2, parents2)

	// Add vertex to state
	state.AddVertex(vertex2)

	// Verify it's in the buffer
	count = state.GetVertexCountInRound(2)
	assert.Equal(t, 0, count, "Round 2 should have 0 vertices since it should be buffered")
	assert.False(t, state.IsVertexDelivered(vertex2.Hash()), "Vertex should not be marked as delivered yet")

	// Advance to round 2 - vertex from round 2 should now be processed from buffer
	state.AdvanceRound()
	count = state.GetVertexCountInRound(2)
	assert.Equal(t, 1, count, "Round 2 should now have 1 vertex")
	assert.True(t, state.IsVertexDelivered(vertex2.Hash()), "Vertex should now be marked as delivered")
}

// TestBufferProcessing tests processing vertices from the buffer
func TestBufferProcessing(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := model.NewVertex(nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create state
	state := NewState(genesis)

	// Create vertices for rounds 1-5
	var vertices []*model.Vertex
	vertices = append(vertices, genesis)

	for round := model.Round(1); round <= 5; round++ {
		block := model.NewBlock(uint64(round), []model.Transaction{CreateTestTransaction(64)})
		parents := map[model.VertexHash]model.Round{
			vertices[len(vertices)-1].Hash(): round - 1,
		}
		vertex := model.NewVertex(nodeKey, round, block, parents)
		vertices = append(vertices, vertex)
	}

	// Add vertices in reverse order to state (all but genesis which is already there)
	for i := len(vertices) - 1; i > 0; i-- {
		state.AddVertex(vertices[i])
	}

	// Verify only genesis is processed initially
	for round := model.Round(1); round <= 5; round++ {
		count := state.GetVertexCountInRound(round)
		assert.Equal(t, 0, count, "Round should have 0 vertices before advancing")
	}

	// Advance rounds one by one and check that vertices are processed in order
	for round := model.Round(1); round <= 5; round++ {
		state.AdvanceRound()
		assert.Equal(t, round, state.GetCurrentRound(), "Current round should be advanced")

		// Check that this round's vertex is now processed
		count := state.GetVertexCountInRound(round)
		assert.Equal(t, 1, count, "Round should have 1 vertex after advancing")
		assert.True(t, state.IsVertexDelivered(vertices[round].Hash()), "Vertex should be marked as delivered")
	}
}

// TestStateGetVertexCountInRound tests getting vertex count in a round
func TestStateGetVertexCountInRound(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := model.NewVertex(nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create state
	state := NewState(genesis)

	// Check count in round 0
	count := state.GetVertexCountInRound(0)
	assert.Equal(t, 1, count, "Round 0 should have 1 vertex")

	// Advance to round 1
	state.AdvanceRound()

	// Check count in round 1
	count = state.GetVertexCountInRound(1)
	assert.Equal(t, 0, count, "Round 1 should have 0 vertices")

	// Add a vertex in round 1
	block1 := model.NewBlock(1, []model.Transaction{CreateTestTransaction(64)})
	parents := map[model.VertexHash]model.Round{
		genesis.Hash(): 0,
	}
	vertex1 := model.NewVertex(nodeKey, 1, block1, parents)
	state.AddVertex(vertex1)

	// Check count in round 1 again
	count = state.GetVertexCountInRound(1)
	assert.Equal(t, 1, count, "Round 1 should now have 1 vertex")
}

// TestStateGetVerticesInRound tests getting vertices in a round
func TestStateGetVerticesInRound(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := model.NewVertex(nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create state
	state := NewState(genesis)

	// Check vertices in round 0
	vertices := state.GetVerticesInRound(0)
	require.Equal(t, 1, len(vertices), "Round 0 should have 1 vertex")

	// Advance to round 1
	state.AdvanceRound()

	// Check vertices in round 1
	vertices = state.GetVerticesInRound(1)
	assert.Nil(t, vertices, "Round 1 should not have any vertices")

	// Add a vertex in round 1
	block1 := model.NewBlock(1, []model.Transaction{CreateTestTransaction(64)})
	parents := map[model.VertexHash]model.Round{
		genesis.Hash(): 0,
	}
	vertex1 := model.NewVertex(nodeKey, 1, block1, parents)
	state.AddVertex(vertex1)

	// Check vertices in round 1 again
	vertices = state.GetVerticesInRound(1)
	require.Equal(t, 1, len(vertices), "Round 1 should now have 1 vertex")
	assert.Equal(t, vertex1.Hash(), vertices[0].Hash(), "Vertex hash should match")
}

// TestStateGetStrongParents tests getting strong parents
func TestStateGetStrongParents(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := model.NewVertex(nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create state
	state := NewState(genesis)

	// Check strong parents at round 0
	parents := state.GetStrongParents()
	assert.Nil(t, parents, "Round 0 should not have strong parents")

	// Add a vertex in round 1
	block1 := model.NewBlock(1, []model.Transaction{CreateTestTransaction(64)})
	parentMap := map[model.VertexHash]model.Round{
		genesis.Hash(): 0,
	}
	vertex1 := model.NewVertex(nodeKey, 1, block1, parentMap)
	state.AddVertex(vertex1)

	// Advance to round 1
	state.AdvanceRound()

	// Check strong parents at round 1
	parents = state.GetStrongParents()
	require.Equal(t, 1, len(parents), "Round 1 should have 1 strong parent")
	assert.Equal(t, genesis.Hash(), parents[0], "Strong parent should be genesis")
}

// TestAddBlockToPropose tests adding and retrieving blocks to propose
func TestAddBlockToPropose(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := model.NewVertex(nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create state
	state := NewState(genesis)

	// Check if there are blocks to propose
	assert.False(t, state.HasBlocksToPropose(), "State should not have blocks to propose")
	assert.Nil(t, state.GetNextBlockToPropose(), "GetNextBlockToPropose should return nil")

	// Add a block to propose
	block1 := model.NewBlock(1, []model.Transaction{CreateTestTransaction(64)})
	state.AddBlockToPropose(block1)

	// Check if there are blocks to propose
	assert.True(t, state.HasBlocksToPropose(), "State should have blocks to propose")

	// Get the block to propose
	block := state.GetNextBlockToPropose()
	assert.NotNil(t, block, "Block should not be nil")
	assert.Equal(t, block1.Hash(), block.Hash(), "Block hash should match")

	// Check that block was removed from queue
	assert.False(t, state.HasBlocksToPropose(), "State should not have blocks to propose anymore")
}

// TestVertexDelivery tests vertex delivery tracking
func TestVertexDelivery(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := model.NewVertex(nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create state
	state := NewState(genesis)

	// Create a vertex
	block1 := model.NewBlock(1, []model.Transaction{CreateTestTransaction(64)})
	parentMap := map[model.VertexHash]model.Round{
		genesis.Hash(): 0,
	}
	vertex1 := model.NewVertex(nodeKey, 1, block1, parentMap)

	// Check initial state
	assert.False(t, state.IsVertexDelivered(vertex1.Hash()), "Vertex should not be delivered yet")

	// Mark vertex as delivered
	state.MarkVertexDelivered(vertex1.Hash())

	// Check updated state
	assert.True(t, state.IsVertexDelivered(vertex1.Hash()), "Vertex should be marked as delivered")
}

// TestStateGetWaveVertexLeader tests getting the leader vertex for a wave
func TestStateGetWaveVertexLeader(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := model.NewVertex(nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create state
	state := NewState(genesis)

	// Genesis is treated as round 0, so there's no wave leader yet
	leader := state.GetWaveVertexLeader(1)
	assert.Nil(t, leader, "Wave 1 should not have a leader yet")

	// Create a vertex for wave 1
	wave1Round := model.Round(MaxWave)
	block1 := model.NewBlock(uint64(wave1Round), []model.Transaction{CreateTestTransaction(64)})
	parentMap := map[model.VertexHash]model.Round{
		genesis.Hash(): 0,
	}
	vertex1 := model.NewVertex(nodeKey, wave1Round, block1, parentMap)

	// Advance to wave 1's round
	for i := model.Round(0); i < wave1Round; i++ {
		state.AdvanceRound()
	}

	// Add vertex to state
	state.AddVertex(vertex1)

	// Check wave leader
	leader = state.GetWaveVertexLeader(1)
	require.NotNil(t, leader, "Wave 1 should have a leader")
	assert.Equal(t, vertex1.Hash(), leader.Hash(), "Leader hash should match")
}

// TestGetRoundForWave tests getting the round number for a wave
func TestGetRoundForWave(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := model.NewVertex(nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create state
	state := NewState(genesis)

	// Check rounds for different waves
	assert.Equal(t, model.Round(0), state.GetRoundForWave(0), "Wave 0 should correspond to round 0")
	assert.Equal(t, model.Round(MaxWave), state.GetRoundForWave(1), "Wave 1 should correspond to round MaxWave")
	assert.Equal(t, model.Round(2*MaxWave), state.GetRoundForWave(2), "Wave 2 should correspond to round 2*MaxWave")
}

// TestIsLastRoundInWave tests checking if the current round is the last in a wave
func TestIsLastRoundInWave(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := model.NewVertex(nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create state
	state := NewState(genesis)

	// Round 0 is not the last in wave
	assert.False(t, state.IsLastRoundInWave(), "Round 0 should not be the last in a wave")

	// Advance to last round in wave 1
	for round := model.Round(1); round <= MaxWave-1; round++ {
		state.AdvanceRound()
	}

	// Check if it's the last round in wave
	assert.True(t, state.IsLastRoundInWave(), "Should be the last round in wave")

	// Advance to first round in next wave
	state.AdvanceRound()

	// Check again
	assert.False(t, state.IsLastRoundInWave(), "Should not be the last round in wave")
}

// TestGetOrderedVertices tests ordering vertices by wave
func TestGetOrderedVertices(t *testing.T) {
	// Create a genesis vertex
	nodeKey := GenerateMockPublicKey(1)
	genesisBlock := model.NewBlock(0, nil)
	genesis := model.NewVertex(nodeKey, 0, genesisBlock, make(map[model.VertexHash]model.Round))

	// Create state
	state := NewState(genesis)

	// Without a leader for wave 1, ordered vertices should be nil
	ordered := state.GetOrderedVertices(1)
	assert.Nil(t, ordered, "Ordered vertices should be nil without a leader")

	// Set up wave 1 directly
	wave1Round := model.Round(MaxWave)
	state.currentRound = wave1Round

	// Create a leader vertex for wave 1
	leaderVertex := model.NewVertex(
		nodeKey,
		wave1Round,
		model.NewBlock(uint64(wave1Round), []model.Transaction{CreateTestTransaction(64)}),
		map[model.VertexHash]model.Round{genesis.Hash(): 0},
	)

	// Create a follower vertex in the same wave
	followerVertex := model.NewVertex(
		nodeKey,
		wave1Round,
		model.NewBlock(uint64(wave1Round), []model.Transaction{CreateTestTransaction(32)}),
		map[model.VertexHash]model.Round{genesis.Hash(): 0},
	)

	// Add vertices directly to the DAG
	state.dag.InsertVertex(leaderVertex)
	state.dag.InsertVertex(followerVertex)
	state.deliveredVertices[leaderVertex.Hash()] = struct{}{}
	state.deliveredVertices[followerVertex.Hash()] = struct{}{}

	// Get ordered vertices
	ordered = state.GetOrderedVertices(1)
	require.NotNil(t, ordered, "Ordered vertices should not be nil")
	require.Equal(t, 2, len(ordered), "Should include both vertices")

	// Verify that both vertices are in the result, without assuming a specific order
	hashes := []model.VertexHash{ordered[0].Hash(), ordered[1].Hash()}
	assert.Contains(t, hashes, leaderVertex.Hash(), "Result should contain the leader vertex")
	assert.Contains(t, hashes, followerVertex.Hash(), "Result should contain the follower vertex")
}
