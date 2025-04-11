package node

import (
	"fmt"
	"net"
	"sync"

	"github.com/unicornultrafoundation/go-helios-v2-primitive/pkg/network"
)

// MockAddress implements net.Addr for testing
type MockAddress struct {
	network string
	address string
}

func (ma *MockAddress) Network() string { return ma.network }
func (ma *MockAddress) String() string  { return ma.address }

// MockMessageHandler implements network.MessageHandler for testing
type MockMessageHandler struct {
	mu       sync.RWMutex
	messages [][]byte
}

func NewMockMessageHandler() *MockMessageHandler {
	return &MockMessageHandler{
		messages: make([][]byte, 0),
	}
}

func (h *MockMessageHandler) HandleMessage(data []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, data)
	return nil
}

func (h *MockMessageHandler) GetMessages() [][]byte {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([][]byte, len(h.messages))
	copy(result, h.messages)
	return result
}

// MockNetwork is a mock implementation of network components for testing
type MockNetwork struct {
	mu sync.RWMutex

	// Network components
	receiver *network.Receiver
	sender   *network.Sender
	handler  *MockMessageHandler
	mockAddr *MockAddress

	// Track received messages for verification
	receivedMessages [][]byte
}

// findAvailablePort finds an available port on localhost
func findAvailablePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// NewMockNetwork creates a new mock network for testing
func NewMockNetwork(address string) *MockNetwork {
	// If address contains port 0, find an available port
	if address == "127.0.0.1:0" {
		port, err := findAvailablePort()
		if err != nil {
			panic(err)
		}
		address = fmt.Sprintf("127.0.0.1:%d", port)
	}

	mockAddr := &MockAddress{
		network: "tcp",
		address: address,
	}

	handler := NewMockMessageHandler()
	receiver := network.NewReceiver(mockAddr, handler)
	sender := network.NewSender()

	return &MockNetwork{
		receiver:         receiver,
		sender:           sender,
		handler:          handler,
		mockAddr:         mockAddr,
		receivedMessages: make([][]byte, 0),
	}
}

// GetReceiver returns the mock receiver
func (mn *MockNetwork) GetReceiver() *network.Receiver {
	return mn.receiver
}

// GetSender returns the mock sender
func (mn *MockNetwork) GetSender() *network.Sender {
	return mn.sender
}

// GetAddress returns the mock address
func (mn *MockNetwork) GetAddress() net.Addr {
	return mn.mockAddr
}

// GetReceivedMessages returns all messages received by the mock network
func (mn *MockNetwork) GetReceivedMessages() [][]byte {
	return mn.handler.GetMessages()
}

// Close closes all network components
func (mn *MockNetwork) Close() error {
	if err := mn.receiver.Stop(); err != nil {
		return err
	}
	return mn.sender.Close()
}
