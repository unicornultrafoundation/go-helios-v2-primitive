package consensus

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unicornultrafoundation/go-helios-v2-primitive/pkg/model"
)

// ===== TEST HELPER FUNCTIONS =====

// createTestVertex creates a vertex for testing
func createVertexForTest(t *testing.T, nodeKey model.NodePublicKey, round model.Round, block *model.Block, parents map[model.VertexHash]model.Round) *model.Vertex {
	vertex := model.NewVertex(nodeKey, round, block, parents)
	require.NotNil(t, vertex, "Vertex creation should succeed")
	return vertex
}

// ===== CONSENSUS COORDINATOR TESTS =====

// TestCoordinatorCreation tests creating a new consensus coordinator
func TestCoordinatorCreation(t *testing.T) {
	// Create committee with 4 validators
	committee := CreateMockCommittee(4)

	// Create coordinator for node 1
	coordinator := New(1, committee)

	// Verify coordinator initialization
	assert.NotNil(t, coordinator, "Coordinator should not be nil")
	assert.Equal(t, model.ID(1), coordinator.nodeID, "Node ID should match")
	assert.Equal(t, committee, coordinator.committee, "Committee should match")
	assert.NotNil(t, coordinator.vertexCh, "Vertex channel should be initialized")
	assert.NotNil(t, coordinator.blockCh, "Block channel should be initialized")
	assert.NotNil(t, coordinator.broadcastCh, "Broadcast channel should be initialized")
	assert.NotNil(t, coordinator.outputCh, "Output channel should be initialized")
	assert.NotNil(t, coordinator.ctx, "Context should be initialized")
	assert.NotNil(t, coordinator.stop, "Stop function should be initialized")
	assert.NotNil(t, coordinator.dag, "DAG should be initialized")
	assert.Equal(t, model.Round(0), coordinator.round, "Round should start at 0")
}

// TestCoordinatorHandleVertex tests handling vertices
func TestCoordinatorHandleVertex(t *testing.T) {
	// Create committee with 4 validators
	committee := CreateMockCommittee(4)

	// Create coordinator directly (without mocking network)
	coordinator := New(1, committee)

	// Create a test vertex
	nodeKey := GenerateMockPublicKey(1)
	block := model.NewBlock(1, []model.Transaction{CreateTestTransaction(64)})
	vertex := createVertexForTest(t, nodeKey, 1, block, make(map[model.VertexHash]model.Round))

	// Handle vertex
	err := coordinator.HandleVertex(vertex)
	assert.NoError(t, err, "Handling vertex should not return an error")

	// Verify vertex is in coordinator's channel
	select {
	case receivedVertex := <-coordinator.vertexCh:
		assert.Equal(t, vertex, receivedVertex, "Received vertex should match")
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "Timed out waiting for vertex")
	}

	// Test cancellation
	coordinator.stop()
	err = coordinator.HandleVertex(vertex)
	assert.Error(t, err, "Handling vertex after stop should return an error")
}

// TestCoordinatorHandleBlock tests handling blocks
func TestCoordinatorHandleBlock(t *testing.T) {
	// Create committee with 4 validators
	committee := CreateMockCommittee(4)

	// Create coordinator directly (without mocking network)
	coordinator := New(1, committee)

	// Create a test block
	block := model.NewBlock(1, []model.Transaction{CreateTestTransaction(64)})

	// Handle block
	err := coordinator.HandleBlock(block)
	assert.NoError(t, err, "Handling block should not return an error")

	// Verify block is in coordinator's channel
	select {
	case receivedBlock := <-coordinator.blockCh:
		assert.Equal(t, block, receivedBlock, "Received block should match")
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "Timed out waiting for block")
	}

	// Test cancellation
	coordinator.stop()
	err = coordinator.HandleBlock(block)
	assert.Error(t, err, "Handling block after stop should return an error")
}

// TestCoordinatorGetChannels tests getting channels
func TestCoordinatorGetChannels(t *testing.T) {
	// Create committee with 4 validators
	committee := CreateMockCommittee(4)

	// Create coordinator
	coordinator := New(1, committee)

	// Test getting channels
	assert.Equal(t, coordinator.vertexCh, coordinator.GetVertexChannel(), "Vertex channel should match")
	assert.Equal(t, coordinator.blockCh, coordinator.GetBlockChannel(), "Block channel should match")
	assert.Equal(t, coordinator.outputCh, coordinator.GetOutputChannel(), "Output channel should match")
	assert.Equal(t, coordinator.broadcastCh, coordinator.GetBroadcastChannel(), "Broadcast channel should match")
}

// TestCoordinatorGetID tests getting ID
func TestCoordinatorGetID(t *testing.T) {
	// Create committee with 4 validators
	committee := CreateMockCommittee(4)

	// Create coordinator
	coordinator := New(1, committee)

	// Test getting ID
	assert.Equal(t, model.ID(1), coordinator.GetID(), "Node ID should match")
}

// TestCoordinatorGetCommittee tests getting committee
func TestCoordinatorGetCommittee(t *testing.T) {
	// Create committee with 4 validators
	committee := CreateMockCommittee(4)

	// Create coordinator
	coordinator := New(1, committee)

	// Test getting committee
	assert.Equal(t, committee, coordinator.GetCommittee(), "Committee should match")
}

// TestHandleBlockInternal tests the internal block handling logic
func TestHandleBlockInternal(t *testing.T) {
	// Create committee with 4 validators
	committee := CreateMockCommittee(4)

	t.Run("block height greater than current round", func(t *testing.T) {
		// Create coordinator with round 1
		coordinator := NewWithRound(1, committee, 1)
		assert.Equal(t, model.Round(1), coordinator.round, "Initial round should be 1")

		// Create a test block with height 5
		block := model.NewBlock(5, []model.Transaction{CreateTestTransaction(64)})

		// Handle block
		err := coordinator.handleBlock(block)
		assert.NoError(t, err, "Handling block should not return an error")

		// Verify round is updated
		assert.Equal(t, model.Round(5), coordinator.round, "Round should be updated to block height")

		// Verify last block hash is updated
		blockHash := block.Hash()
		assert.Equal(t, blockHash, coordinator.lastBlock, "Last block hash should match block hash")
	})

	t.Run("block height equal to current round", func(t *testing.T) {
		// Create coordinator with round 5
		coordinator := NewWithRound(1, committee, 5)
		assert.Equal(t, model.Round(5), coordinator.round, "Initial round should be 5")

		// Create a test block with same height as current round
		block := model.NewBlock(5, []model.Transaction{CreateTestTransaction(64)})

		// Handle block
		err := coordinator.handleBlock(block)
		assert.NoError(t, err, "Handling block should not return an error")

		// Verify round remains unchanged
		assert.Equal(t, model.Round(5), coordinator.round, "Round should remain unchanged")

		// Verify last block hash is updated
		blockHash := block.Hash()
		assert.Equal(t, blockHash, coordinator.lastBlock, "Last block hash should match block hash")
	})

	t.Run("block height less than current round", func(t *testing.T) {
		// Create coordinator with round 10
		coordinator := NewWithRound(1, committee, 10)
		assert.Equal(t, model.Round(10), coordinator.round, "Initial round should be 10")

		// Create a test block with lower height
		block := model.NewBlock(5, []model.Transaction{CreateTestTransaction(64)})

		// Handle block
		err := coordinator.handleBlock(block)
		assert.NoError(t, err, "Handling block should not return an error")

		// Verify round remains unchanged
		assert.Equal(t, model.Round(10), coordinator.round, "Round should remain unchanged")

		// Verify last block hash is updated
		blockHash := block.Hash()
		assert.Equal(t, blockHash, coordinator.lastBlock, "Last block hash should match block hash")
	})
}

// TestMessageHandler tests the message handler
func TestMessageHandler(t *testing.T) {
	// Create committee with 4 validators
	committee := CreateMockCommittee(4)

	// Create coordinator
	coordinator := New(1, committee)

	// Start the coordinator
	err := coordinator.Start()
	require.NoError(t, err, "Starting coordinator should not return an error")
	defer coordinator.Stop()

	// Add a short delay to ensure the coordinator has started
	time.Sleep(50 * time.Millisecond)

	// Create message handler
	handler := &messageHandler{coordinator: coordinator}

	// Create a test vertex
	nodeKey := GenerateMockPublicKey(1)
	block := model.NewBlock(1, []model.Transaction{CreateTestTransaction(64)})
	vertex := createVertexForTest(t, nodeKey, 1, block, make(map[model.VertexHash]model.Round))

	// Marshal vertex
	vertexData, err := vertex.MarshalBinary()
	require.NoError(t, err, "Marshaling vertex should not return an error")

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a mock message handler
	mockHandler := NewMockMessageHandler()
	mockHandler.HandleMessageFunc = func(data []byte) error {
		return handler.HandleMessage(data)
	}

	// Handle vertex message
	err = mockHandler.HandleMessage(vertexData)
	require.NoError(t, err, "Handling vertex message should not return an error")

	// Wait for vertex to be processed
	vertexProcessed := false
	for !vertexProcessed {
		select {
		case <-ctx.Done():
			t.Fatal("Timeout waiting for vertex processing")
		default:
			// Check if vertex is in DAG
			vertices := coordinator.dag.GetVerticesInRound(vertex.Round())
			for _, v := range vertices {
				if v.Hash() == vertex.Hash() {
					vertexProcessed = true
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Test block processing
	// Create a new block for the block handling test
	testBlock := model.NewBlock(1, []model.Transaction{CreateTestTransaction(64)})

	// Marshal block
	blockData, err := testBlock.MarshalBinary()
	require.NoError(t, err, "Marshaling block should not return an error")

	t.Logf("Sending block with height %d, hash %v", testBlock.Height, *testBlock.Hash())

	// Process block using HandleMessage
	err = mockHandler.HandleMessage(blockData)
	require.NoError(t, err, "Handling block message should not return an error")

	// Wait for block to be processed
	blockProcessed := false
	timeoutCh := time.After(2 * time.Second)
	for !blockProcessed {
		select {
		case <-timeoutCh:
			if coordinator.lastBlock != nil {
				t.Logf("Debug - coordinator.lastBlock: %v", *coordinator.lastBlock)
				t.Logf("Debug - testBlock.Hash(): %v", *testBlock.Hash())
			} else {
				t.Logf("Debug - coordinator.lastBlock is nil")
			}
			t.Fatal("Timeout waiting for block processing")
		default:
			// Check if block is in coordinator
			if coordinator.lastBlock != nil {
				t.Logf("Debug - coordinator.lastBlock: %v", *coordinator.lastBlock)
				t.Logf("Debug - testBlock.Hash(): %v", *testBlock.Hash())
				if *coordinator.lastBlock == *testBlock.Hash() {
					blockProcessed = true
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Verify coordinator state
	require.Equal(t, model.Round(testBlock.Height), coordinator.round, "Coordinator round should match block height")
	require.NotNil(t, coordinator.lastBlock, "Coordinator should have a last block")
	require.Equal(t, testBlock.Hash(), coordinator.lastBlock, "Coordinator's last block should match the processed block")
}
