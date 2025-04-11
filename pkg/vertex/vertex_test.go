package vertex

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unicornultrafoundation/go-helios-v2-primitive/pkg/model"
)

// ===== TEST HELPERS =====

// MockMessageHandler is a test implementation of network.MessageHandler
type MockMessageHandler struct {
	receivedMessages [][]byte
	mutex            sync.Mutex
}

func (m *MockMessageHandler) HandleMessage(data []byte) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.receivedMessages = append(m.receivedMessages, data)
	return nil
}

func (m *MockMessageHandler) GetReceivedMessages() [][]byte {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.receivedMessages
}

// mockBroadcastHandler is used to mock the broadcast handler functionality
type mockBroadcastHandler struct {
	wg          sync.WaitGroup
	broadcastCh chan *model.Vertex
	stopCh      chan struct{}
	receivedCh  chan *model.Vertex
}

func newMockBroadcastHandler() *mockBroadcastHandler {
	return &mockBroadcastHandler{
		broadcastCh: make(chan *model.Vertex, 100),
		stopCh:      make(chan struct{}),
		receivedCh:  make(chan *model.Vertex, 100),
	}
}

func (m *mockBroadcastHandler) start() {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for {
			select {
			case vertex := <-m.broadcastCh:
				m.receivedCh <- vertex
			case <-m.stopCh:
				return
			}
		}
	}()
}

func (m *mockBroadcastHandler) stop() {
	close(m.stopCh)
	m.wg.Wait()
}

// createTestVertex creates a vertex for testing
func createTestVertex(t *testing.T, owner model.NodePublicKey, round model.Round) *model.Vertex {
	block := model.NewBlock(1, []model.Transaction{
		[]byte("test transaction 1"),
		[]byte("test transaction 2"),
	})

	parents := make(map[model.VertexHash]model.Round)

	// If not genesis vertex, add a parent
	if round > 1 {
		parentVertex := createTestVertex(t, owner, round-1)
		parents[parentVertex.Hash()] = round - 1
	}

	return model.NewVertex(owner, round, block, parents)
}

// createTestCommittee creates a committee with the specified number of validators
func createTestCommittee(t *testing.T, totalValidators int) *model.Committee {
	committee := &model.Committee{
		Validators: make(map[model.ID]*model.Validator),
	}

	basePort := uint16(20000)
	portsPerNode := uint16(3)

	for i := 0; i < totalValidators; i++ {
		nodeID := model.ID(i + 1)

		// Create validator
		validator := &model.Validator{
			TxAddress: &net.TCPAddr{
				IP:   net.ParseIP("127.0.0.1"),
				Port: int(basePort + uint16(i)*portsPerNode),
			},
			BlockAddress: &net.TCPAddr{
				IP:   net.ParseIP("127.0.0.1"),
				Port: int(basePort + uint16(i)*portsPerNode + 1),
			},
			VertexAddress: &net.TCPAddr{
				IP:   net.ParseIP("127.0.0.1"),
				Port: int(basePort + uint16(i)*portsPerNode + 2),
			},
		}

		// Add to committee
		committee.Validators[nodeID] = validator
	}

	return committee
}

// ===== BASIC FUNCTIONALITY TESTS =====

// TestNew tests the creation of a new vertex coordinator
func TestNew(t *testing.T) {
	// Create test data
	nodeID := model.ID(1)
	committee := createTestCommittee(t, 3)

	// Create vertex coordinator
	coordinator := New(nodeID, committee)

	// Verify coordinator was created correctly
	assert.Equal(t, nodeID, coordinator.GetID(), "Coordinator should have the correct node ID")
	assert.Equal(t, committee, coordinator.GetCommittee(), "Coordinator should have the correct committee")
	assert.NotNil(t, coordinator.GetVertexChannel(), "Vertex channel should not be nil")
	assert.NotNil(t, coordinator.GetBroadcastChannel(), "Broadcast channel should not be nil")
}

// TestGetVertexChannel tests getting the vertex channel
func TestGetVertexChannel(t *testing.T) {
	// Create test data
	nodeID := model.ID(1)
	committee := createTestCommittee(t, 3)

	// Create vertex coordinator
	coordinator := New(nodeID, committee)

	// Get vertex channel
	vertexCh := coordinator.GetVertexChannel()

	// Verify channel is not nil
	assert.NotNil(t, vertexCh, "Vertex channel should not be nil")
}

// TestGetBroadcastChannel tests getting the broadcast channel
func TestGetBroadcastChannel(t *testing.T) {
	// Create test data
	nodeID := model.ID(1)
	committee := createTestCommittee(t, 3)

	// Create vertex coordinator
	coordinator := New(nodeID, committee)

	// Get broadcast channel
	broadcastCh := coordinator.GetBroadcastChannel()

	// Verify channel is not nil
	assert.NotNil(t, broadcastCh, "Broadcast channel should not be nil")
}

// TestEmptyCommittee tests handling of an empty committee
func TestEmptyCommittee(t *testing.T) {
	// Create test data
	nodeID := model.ID(1)

	// Create empty committee
	committee := &model.Committee{
		Validators: make(map[model.ID]*model.Validator),
	}

	// Create vertex coordinator
	coordinator := New(nodeID, committee)

	// Verify coordinator was created correctly despite empty committee
	assert.Equal(t, nodeID, coordinator.GetID(), "Coordinator should have the correct node ID")
	assert.Equal(t, committee, coordinator.GetCommittee(), "Coordinator should have the correct committee")
	assert.NotNil(t, coordinator.GetVertexChannel(), "Vertex channel should not be nil")
	assert.NotNil(t, coordinator.GetBroadcastChannel(), "Broadcast channel should not be nil")
}

// TestGracefulShutdown tests that the coordinator shuts down gracefully
func TestGracefulShutdown(t *testing.T) {
	// Create test data
	nodeID := model.ID(1)
	committee := createTestCommittee(t, 3)

	// Create vertex coordinator
	coordinator := New(nodeID, committee)

	// Test stopping without starting - should not panic
	assert.NotPanics(t, func() {
		coordinator.Stop()
	}, "Stopping a non-started coordinator should not panic")
}

// TestStartupErrorHandling tests error handling during coordinator startup
func TestStartupErrorHandling(t *testing.T) {
	t.Skip("Skipping test that would attempt to bind to invalid ports")

	// Create test data with very unlikely port to cause startup failure
	nodeID := model.ID(1)

	// Create a committee with an invalid port that should fail
	committee := &model.Committee{
		Validators: make(map[model.ID]*model.Validator),
	}

	// Add a validator with an invalid port
	committee.Validators[nodeID] = &model.Validator{
		VertexAddress: &net.TCPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 1, // Port < 1024 usually requires privileges
		},
	}

	// Create vertex coordinator
	coordinator := New(nodeID, committee)

	// Start coordinator - should fail because of privileged port
	err := coordinator.Start()
	assert.Error(t, err, "Starting coordinator with privileged port should return an error")
}

// ===== BROADCAST CHANNEL TESTS =====

// TestBroadcastChannelWorks tests the broadcast channel functions correctly
func TestBroadcastChannelWorks(t *testing.T) {
	// Create test data
	nodeID := model.ID(1)
	committee := createTestCommittee(t, 3)

	// Create vertex coordinator without starting it
	coordinator := New(nodeID, committee)

	// Create a test vertex
	var owner model.NodePublicKey
	copy(owner[:], []byte("test owner"))
	vertex := createTestVertex(t, owner, 1)

	// We'll create a goroutine to listen to the broadcast channel
	broadcastReceived := make(chan bool, 1)

	go func() {
		select {
		case v := <-coordinator.broadcastCh:
			// Verify it's the right vertex
			if v.Hash() == vertex.Hash() {
				broadcastReceived <- true
			} else {
				broadcastReceived <- false
			}
		case <-time.After(time.Second):
			broadcastReceived <- false
		}
	}()

	// Send vertex to broadcast channel
	coordinator.GetBroadcastChannel() <- vertex

	// Wait for the goroutine to process it
	result := <-broadcastReceived
	assert.True(t, result, "Broadcast channel should receive the correct vertex")
}

// TestBroadcastHandling tests the broadcast handling functionality
func TestBroadcastHandling(t *testing.T) {
	// Create mock broadcast handler
	mock := newMockBroadcastHandler()
	mock.start()
	defer mock.stop()

	// Create test vertices
	var owner model.NodePublicKey
	copy(owner[:], []byte("test owner"))

	// Create multiple vertices to broadcast
	vertices := make([]*model.Vertex, 5)
	for i := 0; i < 5; i++ {
		vertices[i] = createTestVertex(t, owner, model.Round(i+1))
	}

	// Send vertices to broadcast channel
	for _, v := range vertices {
		mock.broadcastCh <- v
	}

	// Wait for processing with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Check that all vertices were received
	for i := 0; i < len(vertices); i++ {
		select {
		case received := <-mock.receivedCh:
			assert.NotNil(t, received, "Received vertex should not be nil")

			// Find the matching vertex
			var matched bool
			for _, original := range vertices {
				if received.Hash() == original.Hash() {
					matched = true
					break
				}
			}
			assert.True(t, matched, "Received vertex should match one of the original vertices")

		case <-ctx.Done():
			t.Fatalf("Timed out waiting for vertex %d", i)
		}
	}
}

// TestConcurrentBroadcasts tests handling concurrent broadcasts
func TestConcurrentBroadcasts(t *testing.T) {
	// Create mock broadcast handler
	mock := newMockBroadcastHandler()
	mock.start()
	defer mock.stop()

	// Number of goroutines and vertices per goroutine
	numGoroutines := 5
	verticesPerGoroutine := 10
	expectedTotal := numGoroutines * verticesPerGoroutine

	// Synchronization
	var wg sync.WaitGroup

	// Launch goroutines to send vertices concurrently
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Create unique owner for this goroutine
			var owner model.NodePublicKey
			copy(owner[:], []byte("test owner"+string(rune(id))))

			// Send vertices
			for i := 0; i < verticesPerGoroutine; i++ {
				vertex := createTestVertex(t, owner, model.Round(i+1))
				mock.broadcastCh <- vertex

				// Small sleep to avoid overwhelming the channel
				time.Sleep(time.Millisecond)
			}
		}(g)
	}

	// Wait for all goroutines to finish sending
	wg.Wait()

	// Create a counting channel to track received vertices
	received := make(chan struct{}, expectedTotal)

	// Collect received vertices with timeout
	go func() {
		for i := 0; i < expectedTotal; i++ {
			select {
			case <-mock.receivedCh:
				received <- struct{}{}
			case <-time.After(2 * time.Second):
				return
			}
		}
	}()

	// Wait with timeout for all expected vertices
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Count received vertices
	count := 0
	for {
		select {
		case <-received:
			count++
			if count == expectedTotal {
				// All vertices received
				return
			}
		case <-ctx.Done():
			t.Fatalf("Timed out waiting for vertices. Got %d out of %d expected", count, expectedTotal)
			return
		}
	}
}

// ===== VERTEX HANDLER TESTS =====

// TestVertexHandlerHandleMessage tests handling a vertex message
func TestVertexHandlerHandleMessage(t *testing.T) {
	// Create test data
	nodeID := model.ID(1)
	committee := createTestCommittee(t, 3)

	// Create vertex coordinator
	coordinator := New(nodeID, committee)

	// Create a vertex handler
	handler := &vertexHandler{coordinator: coordinator}

	// Create a test vertex
	var owner model.NodePublicKey
	copy(owner[:], []byte("test owner"))
	vertex := createTestVertex(t, owner, 1)

	// Marshal the vertex
	vertexData, err := vertex.MarshalBinary()
	require.NoError(t, err, "Marshaling vertex should not return an error")

	// Handle the message
	err = handler.HandleMessage(vertexData)
	require.NoError(t, err, "Handling message should not return an error")

	// Create a context with timeout for the test
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Verify the vertex was sent to the vertex channel
	select {
	case receivedVertex := <-coordinator.vertexCh:
		// Verify the received vertex
		assert.NotNil(t, receivedVertex, "Received vertex should not be nil")

		// Compare vertex fields individually
		assert.Equal(t, vertex.Round(), receivedVertex.Round(), "Round should match")
		assert.Equal(t, vertex.Owner(), receivedVertex.Owner(), "Owner should match")
		assert.Equal(t, vertex.Block().Hash(), receivedVertex.Block().Hash(), "Block hash should match")
		assert.Equal(t, vertex.Block().Height, receivedVertex.Block().Height, "Block height should match")
		assert.Equal(t, vertex.Block().Transactions, receivedVertex.Block().Transactions, "Block transactions should match")
		assert.Equal(t, vertex.Parents(), receivedVertex.Parents(), "Parents should match")
		assert.Equal(t, vertex.Wave, receivedVertex.Wave, "Wave should match")
		// Compare timestamps with a reasonable tolerance (1 second)
		assert.WithinDuration(t, vertex.Timestamp, receivedVertex.Timestamp, time.Second, "Timestamp should be within 1 second")

	case <-ctx.Done():
		t.Fatal("Timed out waiting for vertex from vertex channel")
	}
}

// TestVertexHandling tests the full vertex handling flow
func TestVertexHandling(t *testing.T) {
	// Create test data
	nodeID := model.ID(1)
	committee := createTestCommittee(t, 3)

	// Create vertex coordinator without starting it (to avoid network operations)
	coordinator := New(nodeID, committee)

	// Create a vertex handler
	handler := &vertexHandler{coordinator: coordinator}

	// Create a test vertex
	var owner model.NodePublicKey
	copy(owner[:], []byte("test owner"))
	vertex := createTestVertex(t, owner, 1)

	// Marshal the vertex
	vertexData, err := vertex.MarshalBinary()
	require.NoError(t, err, "Marshaling vertex should not return an error")

	// Create a go routine to listen for vertices
	vertexCh := coordinator.GetVertexChannel()
	vertexReceived := make(chan *model.Vertex, 1)

	go func() {
		select {
		case v := <-vertexCh:
			vertexReceived <- v
		case <-time.After(2 * time.Second):
			// Timeout
		}
	}()

	// Handle the message
	err = handler.HandleMessage(vertexData)
	require.NoError(t, err, "Handling message should not return an error")

	// Wait for vertex to be received
	select {
	case receivedVertex := <-vertexReceived:
		// Verify the received vertex
		assert.NotNil(t, receivedVertex, "Received vertex should not be nil")

		// Compare vertex fields individually
		assert.Equal(t, vertex.Round(), receivedVertex.Round(), "Round should match")
		assert.Equal(t, vertex.Owner(), receivedVertex.Owner(), "Owner should match")
		assert.Equal(t, vertex.Block().Hash(), receivedVertex.Block().Hash(), "Block hash should match")
		assert.Equal(t, vertex.Block().Height, receivedVertex.Block().Height, "Block height should match")
		assert.Equal(t, vertex.Block().Transactions, receivedVertex.Block().Transactions, "Block transactions should match")
		assert.Equal(t, vertex.Parents(), receivedVertex.Parents(), "Parents should match")
		assert.Equal(t, vertex.Wave, receivedVertex.Wave, "Wave should match")
		// Compare timestamps with a reasonable tolerance (1 second)
		assert.WithinDuration(t, vertex.Timestamp, receivedVertex.Timestamp, time.Second, "Timestamp should be within 1 second")

	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for vertex to be received")
	}
}

// TestVertexHandlerInvalidData tests handling an invalid vertex message
func TestVertexHandlerInvalidData(t *testing.T) {
	// Create test data
	nodeID := model.ID(1)
	committee := createTestCommittee(t, 3)

	// Create vertex coordinator
	coordinator := New(nodeID, committee)

	// Create a vertex handler
	handler := &vertexHandler{coordinator: coordinator}

	// Create invalid vertex data
	invalidData := []byte("this is not a valid vertex")

	// Handle the message
	err := handler.HandleMessage(invalidData)

	// Should return an error for invalid data
	assert.Error(t, err, "Handling invalid message should return an error")
}

// TestVertexChannelProcessing tests the processing of vertex channel messages
func TestVertexChannelProcessing(t *testing.T) {
	// Create test data
	nodeID := model.ID(1)
	committee := createTestCommittee(t, 3)

	// Create vertex coordinator
	coordinator := New(nodeID, committee)

	// Create a test vertex
	var owner model.NodePublicKey
	copy(owner[:], []byte("test vertex"))
	vertex := createTestVertex(t, owner, 1)

	// Set up a handler to listen to the vertex channel
	done := make(chan bool)

	// Create a goroutine to listen to vertex channel
	var receivedVertex *model.Vertex
	vertexCh := coordinator.GetVertexChannel()

	// Set up the handler to listen to the vertex channel
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		select {
		case v := <-vertexCh:
			receivedVertex = v
			done <- true
		case <-ctx.Done():
			done <- false
		}
	}()

	// Manually handle a vertex as if it came from the network
	vertexData, err := vertex.MarshalBinary()
	require.NoError(t, err, "Marshaling vertex should not return an error")

	// Call handler directly
	handler := &vertexHandler{coordinator: coordinator}
	err = handler.HandleMessage(vertexData)
	require.NoError(t, err, "Handling message should not return an error")

	// Wait for verification
	result := <-done
	assert.True(t, result, "Should receive vertex on the vertex channel")
	assert.NotNil(t, receivedVertex, "Received vertex should not be nil")

	// Compare vertex fields individually
	assert.Equal(t, vertex.Round(), receivedVertex.Round(), "Round should match")
	assert.Equal(t, vertex.Owner(), receivedVertex.Owner(), "Owner should match")
	assert.Equal(t, vertex.Block().Hash(), receivedVertex.Block().Hash(), "Block hash should match")
	assert.Equal(t, vertex.Block().Height, receivedVertex.Block().Height, "Block height should match")
	assert.Equal(t, vertex.Block().Transactions, receivedVertex.Block().Transactions, "Block transactions should match")
	assert.Equal(t, vertex.Parents(), receivedVertex.Parents(), "Parents should match")
	assert.Equal(t, vertex.Wave, receivedVertex.Wave, "Wave should match")
	// Compare timestamps with a reasonable tolerance (1 second)
	assert.WithinDuration(t, vertex.Timestamp, receivedVertex.Timestamp, time.Second, "Timestamp should be within 1 second")
}
