package transaction

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unicornultrafoundation/go-helios-v2-primitive/pkg/model"
)

// ===== TEST HELPERS =====

// MockMessageHandler is a test implementation of a message handler
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

// createFixedSizeTransaction creates a transaction with a specific size
func createFixedSizeTransaction(size int) model.Transaction {
	tx := make(model.Transaction, size)
	// Fill with some identifiable pattern
	for i := 0; i < size; i++ {
		tx[i] = byte(i % 256)
	}
	return tx
}

// createTestCommittee creates a committee with the specified number of validators
func createTestCommittee(t *testing.T, totalValidators int) *model.Committee {
	committee := &model.Committee{
		Validators: make(map[model.ID]*model.Validator),
	}

	basePort := uint16(10000)
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

// TestNew tests the creation of a new transaction coordinator
func TestNew(t *testing.T) {
	// Create test data
	nodeID := model.ID(1)
	committee := createTestCommittee(t, 3)

	// Create transaction coordinator
	coordinator := New(nodeID, committee)

	// Verify coordinator was created correctly
	assert.Equal(t, nodeID, coordinator.GetID(), "Coordinator should have the correct node ID")
	assert.Equal(t, committee, coordinator.GetCommittee(), "Coordinator should have the correct committee")
	assert.NotNil(t, coordinator.GetBlockChannel(), "Block channel should not be nil")
}

// TestGetBlockChannel tests getting the block channel
func TestGetBlockChannel(t *testing.T) {
	// Create test data
	nodeID := model.ID(1)
	committee := createTestCommittee(t, 3)

	// Create transaction coordinator
	coordinator := New(nodeID, committee)

	// Get block channel
	blockCh := coordinator.GetBlockChannel()

	// Verify channel is not nil
	assert.NotNil(t, blockCh, "Block channel should not be nil")
}

// TestEmptyCommittee tests handling of an empty committee
func TestEmptyCommittee(t *testing.T) {
	// Create test data
	nodeID := model.ID(1)

	// Create empty committee
	committee := &model.Committee{
		Validators: make(map[model.ID]*model.Validator),
	}

	// Create transaction coordinator
	coordinator := New(nodeID, committee)

	// Verify coordinator was created correctly despite empty committee
	assert.Equal(t, nodeID, coordinator.GetID(), "Coordinator should have the correct node ID")
	assert.Equal(t, committee, coordinator.GetCommittee(), "Coordinator should have the correct committee")
	assert.NotNil(t, coordinator.GetBlockChannel(), "Block channel should not be nil")
}

// TestGracefulShutdown tests that the coordinator shuts down gracefully
func TestGracefulShutdown(t *testing.T) {
	// Create test data
	nodeID := model.ID(1)
	committee := createTestCommittee(t, 3)

	// Create transaction coordinator
	coordinator := New(nodeID, committee)

	// Test stopping without starting - should not panic
	assert.NotPanics(t, func() {
		coordinator.Stop()
	}, "Stopping a non-started coordinator should not panic")
}

// ===== TRANSACTION HANDLING TESTS =====

// TestAddTransaction tests adding a transaction to the coordinator
func TestAddTransaction(t *testing.T) {
	// Create test data
	nodeID := model.ID(1)
	committee := createTestCommittee(t, 3)

	// Create transaction coordinator
	coordinator := New(nodeID, committee)

	// Create a transaction
	tx := createFixedSizeTransaction(1024) // 1KB transaction

	// Add transaction
	coordinator.AddTransaction(tx)

	// Check if transaction was added to the buffer
	assert.Equal(t, 1, len(coordinator.txBuffer), "Transaction buffer should contain one transaction")
	assert.Equal(t, tx, coordinator.txBuffer[0], "Transaction in buffer should match the added transaction")
}

// TestAddMultipleTransactions tests adding multiple transactions
func TestAddMultipleTransactions(t *testing.T) {
	// Create test data
	nodeID := model.ID(1)
	committee := createTestCommittee(t, 3)

	// Create transaction coordinator
	coordinator := New(nodeID, committee)

	// Number of transactions to add
	txCount := 100

	// Add multiple transactions
	for i := 0; i < txCount; i++ {
		tx := createFixedSizeTransaction(1024) // 1KB transaction
		coordinator.AddTransaction(tx)
	}

	// Check if all transactions were added to the buffer
	assert.Equal(t, txCount, len(coordinator.txBuffer), "Transaction buffer should contain all added transactions")
}

// TestCreateFixedSizeTransaction tests creating a transaction with a specific size
func TestCreateFixedSizeTransaction(t *testing.T) {
	sizes := []int{64, 128, 1024, 2048, 3 * 1024} // Test various sizes including 3KB

	for _, size := range sizes {
		t.Run(formatSize(size), func(t *testing.T) {
			tx := createFixedSizeTransaction(size)
			assert.Equal(t, size, len(tx), "Transaction should be the requested size")
		})
	}
}

// Helper function to format sizes
func formatSize(size int) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	}
	return fmt.Sprintf("%dKB", size/1024)
}

// ===== BLOCK CREATION TESTS =====

// TestBlockCreation tests the block creation process
func TestBlockCreation(t *testing.T) {
	// Create test data
	nodeID := model.ID(1)
	committee := createTestCommittee(t, 3)

	// Create transaction coordinator with custom context to control testing flow
	testCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	coordinator := New(nodeID, committee)
	coordinator.ctx = testCtx

	// Add transactions to trigger block creation
	txCount := 2000
	txSize := 1024 // 1KB
	for i := 0; i < txCount; i++ {
		tx := createFixedSizeTransaction(txSize)
		coordinator.AddTransaction(tx)
	}

	// Manually trigger block creation
	err := coordinator.createBlock()
	require.NoError(t, err, "Block creation should not return an error")

	// Check block was created and sent to channel
	select {
	case block := <-coordinator.GetBlockChannel():
		assert.NotNil(t, block, "Created block should not be nil")
		assert.True(t, len(block.Transactions) > 0, "Block should contain transactions")
		assert.Equal(t, uint64(0), block.Height, "First block should have height 0")
	default:
		t.Fatal("No block was sent to the channel")
	}

	// Check transaction buffer was updated
	assert.Less(t, len(coordinator.txBuffer), txCount, "Transaction buffer should have fewer transactions after block creation")
}

// TestBlockSizeLimits tests that blocks respect the size limit
func TestBlockSizeLimits(t *testing.T) {
	// Create test data
	nodeID := model.ID(1)
	committee := createTestCommittee(t, 3)

	// Create transaction coordinator
	coordinator := New(nodeID, committee)

	// Create transactions larger than the block limit
	largeSize := 2 * 1024 * 1024 // 2MB per transaction
	tx1 := createFixedSizeTransaction(largeSize)
	tx2 := createFixedSizeTransaction(largeSize)
	tx3 := createFixedSizeTransaction(largeSize)

	// Add to coordinator
	coordinator.AddTransaction(tx1)
	coordinator.AddTransaction(tx2)
	coordinator.AddTransaction(tx3)

	// Manually trigger block creation
	err := coordinator.createBlock()
	require.NoError(t, err, "Block creation should not return an error")

	// Check block was created and sent to channel
	select {
	case block := <-coordinator.GetBlockChannel():
		assert.NotNil(t, block, "Created block should not be nil")

		// Block should contain at least one transaction but not all three
		assert.True(t, len(block.Transactions) >= 1, "Block should contain at least one transaction")
		assert.True(t, len(block.Transactions) < 3, "Block should not contain all three transactions due to size limit")
	default:
		t.Fatal("No block was sent to the channel")
	}
}

// TestMultipleBlockCreation tests creating multiple blocks sequentially
func TestMultipleBlockCreation(t *testing.T) {
	// Create test data
	nodeID := model.ID(1)
	committee := createTestCommittee(t, 3)

	// Create transaction coordinator
	coordinator := New(nodeID, committee)

	// Add a large number of transactions
	txCount := 5000
	for i := 0; i < txCount; i++ {
		tx := createFixedSizeTransaction(1024) // 1KB each
		coordinator.AddTransaction(tx)
	}

	// Create blocks until buffer is empty
	blockCount := 0
	for len(coordinator.txBuffer) > 0 {
		err := coordinator.createBlock()
		require.NoError(t, err, "Block creation should not return an error")

		// Get the block from the channel
		select {
		case block := <-coordinator.GetBlockChannel():
			assert.NotNil(t, block, "Created block should not be nil")
			assert.Equal(t, uint64(blockCount), block.Height, "Block height should increment")
			blockCount++
		default:
			t.Fatal("No block was sent to the channel")
		}
	}

	// Verify multiple blocks were created
	assert.True(t, blockCount > 1, "Multiple blocks should have been created")
	assert.Empty(t, coordinator.txBuffer, "Transaction buffer should be empty after processing all transactions")
}

// ===== METRICS TESTS =====

// TestMetricsReporting tests the metrics reporting functionality
func TestMetricsReporting(t *testing.T) {
	// Create test data
	nodeID := model.ID(1)
	committee := createTestCommittee(t, 3)

	// Create transaction coordinator
	coordinator := New(nodeID, committee)

	// Add transactions
	txCount := 100
	for i := 0; i < txCount; i++ {
		tx := createFixedSizeTransaction(1024)
		coordinator.AddTransaction(tx)
	}

	// Create a block
	err := coordinator.createBlock()
	require.NoError(t, err, "Block creation should not return an error")

	// Get metrics
	metrics := coordinator.GetMetrics()

	// Verify metrics were updated
	assert.True(t, metrics.TxReceivedCount > 0 || metrics.TxProcessedCount > 0,
		"Transaction metrics should be updated")
	assert.True(t, metrics.BlockCount > 0, "Block count metric should be updated")
}

// TestBlockOverCapacity tests handling of blocks when the channel is at capacity
func TestBlockOverCapacity(t *testing.T) {
	// Create test data
	nodeID := model.ID(1)
	committee := createTestCommittee(t, 3)

	// Create transaction coordinator with a small channel capacity for testing
	coordinator := New(nodeID, committee)

	// Fill the block channel to capacity
	for i := 0; i < cap(coordinator.blockCh); i++ {
		block := model.NewBlock(uint64(i), []model.Transaction{
			createFixedSizeTransaction(10),
		})
		coordinator.blockCh <- block
	}

	// Add transactions
	for i := 0; i < 100; i++ {
		tx := createFixedSizeTransaction(1024)
		coordinator.AddTransaction(tx)
	}

	// Attempt to create a block
	err := coordinator.createBlock()

	// Since channel is full, we expect an error
	assert.Error(t, err, "Block creation should return an error when channel is full")

	// The droppedBlockCount metric should be incremented
	assert.True(t, coordinator.metrics.droppedBlockCount > 0, "Dropped block count should be incremented")
}

// TestCoordinatorRestart tests stopping and restarting a coordinator
func TestCoordinatorRestart(t *testing.T) {
	t.Skip("Skipping test that requires network operations")

	// Create test data
	nodeID := model.ID(1)
	committee := createTestCommittee(t, 3)

	// Create transaction coordinator
	coordinator := New(nodeID, committee)

	// Start coordinator
	err := coordinator.Start()
	require.NoError(t, err, "Starting coordinator should not return an error")

	// Stop coordinator
	coordinator.Stop()

	// Restart coordinator
	err = coordinator.Start()
	require.NoError(t, err, "Restarting coordinator should not return an error")
	defer coordinator.Stop()

	// Add a transaction after restart
	tx := createFixedSizeTransaction(1024)
	coordinator.AddTransaction(tx)

	// Verify transaction was added
	assert.Equal(t, 1, len(coordinator.txBuffer), "Transaction buffer should contain one transaction after restart")
}

// TestTxHandlerHandleMessage tests handling a transaction message
func TestTxHandlerHandleMessage(t *testing.T) {
	// Create test data
	nodeID := model.ID(1)
	committee := createTestCommittee(t, 3)

	// Create transaction coordinator
	coordinator := New(nodeID, committee)

	// Create a transaction handler
	handler := &txHandler{coordinator: coordinator}

	// Create a test transaction
	tx := createFixedSizeTransaction(1024)

	// Handle the message
	err := handler.HandleMessage(tx)
	require.NoError(t, err, "Handling message should not return an error")

	// Verify transaction was added to the buffer
	assert.Equal(t, 1, len(coordinator.txBuffer), "Transaction buffer should contain one transaction")
	assert.Equal(t, tx, coordinator.txBuffer[0], "Transaction in buffer should match the added transaction")
}
