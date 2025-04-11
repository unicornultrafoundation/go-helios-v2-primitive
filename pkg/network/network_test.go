package network

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===== TEST HELPERS =====

// MockMessageHandler is a mock implementation of MessageHandler
type MockMessageHandler struct {
	receivedMessages [][]byte
	mutex            sync.Mutex
	errorToReturn    error
}

func NewMockMessageHandler() *MockMessageHandler {
	return &MockMessageHandler{
		receivedMessages: make([][]byte, 0),
	}
}

func (m *MockMessageHandler) HandleMessage(data []byte) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.receivedMessages = append(m.receivedMessages, append([]byte{}, data...))
	return m.errorToReturn
}

func (m *MockMessageHandler) GetReceivedMessages() [][]byte {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.receivedMessages
}

func (m *MockMessageHandler) SetError(err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.errorToReturn = err
}

func (m *MockMessageHandler) Reset() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.receivedMessages = make([][]byte, 0)
	m.errorToReturn = nil
}

// createTestMessage creates a test message of the specified size
func createTestMessage(size int) []byte {
	message := make([]byte, size)
	for i := 0; i < size; i++ {
		message[i] = byte(i % 256)
	}
	return message
}

// waitForMessages waits for the handler to receive a specific number of messages
func waitForMessages(handler *MockMessageHandler, count int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if len(handler.GetReceivedMessages()) >= count {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// ===== BASIC FUNCTIONALITY TESTS =====

// TestReceiverCreation tests creating a new Receiver
func TestReceiverCreation(t *testing.T) {
	handler := NewMockMessageHandler()
	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}

	receiver := NewReceiver(addr, handler)

	assert.NotNil(t, receiver, "Receiver should not be nil")
	assert.Equal(t, addr, receiver.address, "Receiver should have the correct address")
	assert.Equal(t, handler, receiver.handler, "Receiver should have the correct handler")
	assert.NotNil(t, receiver.workerPool, "Worker pool should not be nil")
	assert.NotNil(t, receiver.conns, "Connections map should not be nil")
}

// TestReceiverStartStop tests starting and stopping a Receiver
func TestReceiverStartStop(t *testing.T) {
	handler := NewMockMessageHandler()
	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}

	receiver := NewReceiver(addr, handler)

	// Start the receiver
	err := receiver.Start()
	require.NoError(t, err, "Starting receiver should not error")

	// Verify the listener is active
	assert.NotNil(t, receiver.listener, "Listener should not be nil after starting")

	// Stop the receiver
	err = receiver.Stop()
	require.NoError(t, err, "Stopping receiver should not error")
}

// TestSenderCreation tests creating a new Sender
func TestSenderCreation(t *testing.T) {
	sender := NewSender()

	assert.NotNil(t, sender, "Sender should not be nil")
	assert.NotNil(t, sender.conns, "Connections map should not be nil")
	assert.NotNil(t, sender.workerPool, "Worker pool should not be nil")
	assert.NotNil(t, sender.idleConns, "Idle connections map should not be nil")
}

// TestSenderClose tests closing a Sender
func TestSenderClose(t *testing.T) {
	sender := NewSender()

	err := sender.Close()
	require.NoError(t, err, "Closing sender should not error")
}

// ===== INTEGRATION TESTS =====

// TestSendReceive tests sending and receiving messages between Sender and Receiver
func TestSendReceive(t *testing.T) {
	// Setup receiver
	handler := NewMockMessageHandler()
	receiverAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
	receiver := NewReceiver(receiverAddr, handler)

	err := receiver.Start()
	require.NoError(t, err, "Starting receiver should not error")
	defer receiver.Stop()

	// Get the actual address after binding to port 0
	actualAddr := receiver.listener.Addr()

	// Setup sender
	sender := NewSender()
	defer sender.Close()

	// Create test message
	message := createTestMessage(1024) // 1KB message

	// Send message
	err = sender.Send(actualAddr, message)
	require.NoError(t, err, "Sending message should not error")

	// Wait for message to be received
	success := waitForMessages(handler, 1, 2*time.Second)
	require.True(t, success, "Message should be received within timeout")

	// Verify message content
	receivedMessages := handler.GetReceivedMessages()
	require.Len(t, receivedMessages, 1, "Should receive exactly one message")
	assert.True(t, bytes.Equal(message, receivedMessages[0]), "Received message should match sent message")
}

// TestBroadcast tests broadcasting messages to multiple receivers
func TestBroadcast(t *testing.T) {
	// Number of receivers to create
	receiverCount := 3

	// Setup receivers
	var receivers []*Receiver
	var addresses []net.Addr
	var handlers []*MockMessageHandler

	for i := 0; i < receiverCount; i++ {
		handler := NewMockMessageHandler()
		addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
		receiver := NewReceiver(addr, handler)

		err := receiver.Start()
		require.NoError(t, err, "Starting receiver should not error")
		defer receiver.Stop()

		// Get the actual address after binding to port 0
		actualAddr := receiver.listener.Addr()

		receivers = append(receivers, receiver)
		addresses = append(addresses, actualAddr)
		handlers = append(handlers, handler)
	}

	// Setup sender
	sender := NewSender()
	defer sender.Close()

	// Create test message
	message := createTestMessage(1024) // 1KB message

	// Broadcast message
	err := sender.Broadcast(addresses, message)
	require.NoError(t, err, "Broadcasting message should not error")

	// Wait for messages to be received by all receivers
	for i, handler := range handlers {
		success := waitForMessages(handler, 1, 2*time.Second)
		require.True(t, success, fmt.Sprintf("Receiver %d should receive message within timeout", i))

		// Verify message content
		receivedMessages := handler.GetReceivedMessages()
		require.Len(t, receivedMessages, 1, fmt.Sprintf("Receiver %d should receive exactly one message", i))
		assert.True(t, bytes.Equal(message, receivedMessages[0]),
			fmt.Sprintf("Receiver %d's message should match sent message", i))
	}
}

// TestLargeMessage tests sending a large message
func TestLargeMessage(t *testing.T) {
	// Setup receiver
	handler := NewMockMessageHandler()
	receiverAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
	receiver := NewReceiver(receiverAddr, handler)

	err := receiver.Start()
	require.NoError(t, err, "Starting receiver should not error")
	defer receiver.Stop()

	// Get the actual address after binding to port 0
	actualAddr := receiver.listener.Addr()

	// Setup sender
	sender := NewSender()
	defer sender.Close()

	// Create large message (just under MaxMessageSize)
	message := createTestMessage(MaxMessageSize - 100)

	// Send message
	err = sender.Send(actualAddr, message)
	require.NoError(t, err, "Sending large message should not error")

	// Wait for message to be received
	success := waitForMessages(handler, 1, 5*time.Second)
	require.True(t, success, "Large message should be received within timeout")

	// Verify message content
	receivedMessages := handler.GetReceivedMessages()
	require.Len(t, receivedMessages, 1, "Should receive exactly one message")
	assert.Equal(t, len(message), len(receivedMessages[0]), "Received message should have same length as sent message")
	assert.True(t, bytes.Equal(message, receivedMessages[0]), "Received message should match sent message")
}

// TestTooLargeMessage tests handling messages larger than MaxMessageSize
func TestTooLargeMessage(t *testing.T) {
	// Setup mock networking for testing
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	// Setup receiver with mock connection
	handler := NewMockMessageHandler()
	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
	receiver := NewReceiver(addr, handler)

	// Start handling connection in a goroutine
	go receiver.handleConnection(server)

	// Try to send an oversized message header
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, MaxMessageSize+1)
	_, err := client.Write(header)
	require.NoError(t, err, "Writing header should not error")

	// Give some time for processing
	time.Sleep(100 * time.Millisecond)

	// Verify no message was processed
	receivedMessages := handler.GetReceivedMessages()
	assert.Len(t, receivedMessages, 0, "Oversized message should be rejected")
}

// TestMultipleMessages tests sending multiple messages
func TestMultipleMessages(t *testing.T) {
	// Setup receiver
	handler := NewMockMessageHandler()
	receiverAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
	receiver := NewReceiver(receiverAddr, handler)

	err := receiver.Start()
	require.NoError(t, err, "Starting receiver should not error")
	defer receiver.Stop()

	// Get the actual address after binding to port 0
	actualAddr := receiver.listener.Addr()

	// Setup sender
	sender := NewSender()
	defer sender.Close()

	// Number of messages to send
	messageCount := 10

	// Send multiple messages
	for i := 0; i < messageCount; i++ {
		message := createTestMessage(1024 + i) // Slightly different sized messages
		err := sender.Send(actualAddr, message)
		require.NoError(t, err, fmt.Sprintf("Sending message %d should not error", i))
	}

	// Wait for all messages to be received
	success := waitForMessages(handler, messageCount, 5*time.Second)
	require.True(t, success, "All messages should be received within timeout")

	// Verify message count
	receivedMessages := handler.GetReceivedMessages()
	assert.Len(t, receivedMessages, messageCount, "Should receive all sent messages")
}

// TestConnectionPooling tests that the sender reuses connections
func TestConnectionPooling(t *testing.T) {
	// Setup receiver
	handler := NewMockMessageHandler()
	receiverAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
	receiver := NewReceiver(receiverAddr, handler)

	err := receiver.Start()
	require.NoError(t, err, "Starting receiver should not error")
	defer receiver.Stop()

	// Get the actual address after binding to port 0
	actualAddr := receiver.listener.Addr()

	// Setup sender
	sender := NewSender()
	defer sender.Close()

	// Send first message
	message1 := createTestMessage(1024)
	err = sender.Send(actualAddr, message1)
	require.NoError(t, err, "Sending first message should not error")

	// Send second message
	message2 := createTestMessage(2048)
	err = sender.Send(actualAddr, message2)
	require.NoError(t, err, "Sending second message should not error")

	// Verify connection was pooled
	sender.connsMux.RLock()
	assert.Equal(t, 1, len(sender.conns), "Should have one pooled connection")
	sender.connsMux.RUnlock()

	// Wait for messages to be received
	success := waitForMessages(handler, 2, 2*time.Second)
	require.True(t, success, "Both messages should be received within timeout")
}

// TestMessageHandlerError tests handling errors from message handlers
func TestMessageHandlerError(t *testing.T) {
	// Setup receiver with error-returning handler
	handler := NewMockMessageHandler()
	handler.SetError(fmt.Errorf("test error"))

	receiverAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
	receiver := NewReceiver(receiverAddr, handler)

	err := receiver.Start()
	require.NoError(t, err, "Starting receiver should not error")
	defer receiver.Stop()

	// Get the actual address after binding to port 0
	actualAddr := receiver.listener.Addr()

	// Setup sender
	sender := NewSender()
	defer sender.Close()

	// Send message
	message := createTestMessage(1024)
	err = sender.Send(actualAddr, message)
	require.NoError(t, err, "Sending message should not error despite handler error")

	// Wait for message processing
	time.Sleep(500 * time.Millisecond)

	// Verify message was received by handler despite error
	receivedMessages := handler.GetReceivedMessages()
	assert.Len(t, receivedMessages, 1, "Message should be received by handler even with error")
}

// TestConnectionClosing tests handling closed connections
func TestConnectionClosing(t *testing.T) {
	// Setup receiver
	handler := NewMockMessageHandler()
	receiverAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
	receiver := NewReceiver(receiverAddr, handler)

	err := receiver.Start()
	require.NoError(t, err, "Starting receiver should not error")
	defer receiver.Stop()

	// Get the actual address after binding to port 0
	actualAddr := receiver.listener.Addr()

	// Setup sender
	sender := NewSender()

	// Send message
	message := createTestMessage(1024)
	err = sender.Send(actualAddr, message)
	require.NoError(t, err, "Sending message should not error")

	// Close sender
	err = sender.Close()
	require.NoError(t, err, "Closing sender should not error")

	// Wait for message to be received
	success := waitForMessages(handler, 1, 2*time.Second)
	require.True(t, success, "Message should be received despite sender closing")
}
