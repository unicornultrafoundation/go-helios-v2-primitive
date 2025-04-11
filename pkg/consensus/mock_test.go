package consensus

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/unicornultrafoundation/go-helios-v2-primitive/pkg/model"
	"github.com/unicornultrafoundation/go-helios-v2-primitive/pkg/network"
)

// ReceiverInterface defines the interface for a network receiver
type ReceiverInterface interface {
	Start() error
	Stop() error
}

// SenderInterface defines the interface for a network sender
type SenderInterface interface {
	Send(address net.Addr, data []byte) error
	Broadcast(addresses []net.Addr, data []byte) error
	Close() error
}

// MockAddress implements net.Addr for testing
type MockAddress struct {
	network string
	address string
}

func NewMockAddress(network, address string) *MockAddress {
	return &MockAddress{
		network: network,
		address: address,
	}
}

func (a *MockAddress) Network() string {
	return a.network
}

func (a *MockAddress) String() string {
	return a.address
}

// MockMessageHandler implements network.MessageHandler
type MockMessageHandler struct {
	HandleMessageFunc func(data []byte) error
	ReceivedMessages  [][]byte
	mu                sync.Mutex
}

func NewMockMessageHandler() *MockMessageHandler {
	return &MockMessageHandler{
		ReceivedMessages: make([][]byte, 0),
		HandleMessageFunc: func(data []byte) error {
			return nil
		},
	}
}

func (m *MockMessageHandler) HandleMessage(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ReceivedMessages = append(m.ReceivedMessages, append([]byte{}, data...))
	return m.HandleMessageFunc(data)
}

func (m *MockMessageHandler) GetReceivedMessages() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ReceivedMessages
}

// MockReceiver mocks network.Receiver
type MockReceiver struct {
	address            net.Addr
	handler            network.MessageHandler
	startFunc          func() error
	stopFunc           func() error
	startCalled        bool
	stopCalled         bool
	mu                 sync.RWMutex
	activeConnections  int
	simulateStartError bool
}

func NewMockReceiver(address net.Addr, handler network.MessageHandler) *MockReceiver {
	return &MockReceiver{
		address: address,
		handler: handler,
		startFunc: func() error {
			return nil
		},
		stopFunc: func() error {
			return nil
		},
	}
}

func (r *MockReceiver) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.startCalled = true
	if r.simulateStartError {
		return fmt.Errorf("simulated start error")
	}
	return r.startFunc()
}

func (r *MockReceiver) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopCalled = true
	return r.stopFunc()
}

func (r *MockReceiver) IsStartCalled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.startCalled
}

func (r *MockReceiver) IsStopCalled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stopCalled
}

func (r *MockReceiver) SetStartError(simulate bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.simulateStartError = simulate
}

// MockSender mocks network.Sender
type MockSender struct {
	sendFunc            func(address net.Addr, data []byte) error
	broadcastFunc       func(addresses []net.Addr, data []byte) error
	closeFunc           func() error
	sentMessages        map[string][][]byte
	broadcastedMessages [][][]byte
	closeCalled         bool
	mu                  sync.RWMutex
	simulateSendError   bool
}

func NewMockSender() *MockSender {
	return &MockSender{
		sentMessages: make(map[string][][]byte),
		sendFunc: func(address net.Addr, data []byte) error {
			return nil
		},
		broadcastFunc: func(addresses []net.Addr, data []byte) error {
			return nil
		},
		closeFunc: func() error {
			return nil
		},
	}
}

func (s *MockSender) Send(address net.Addr, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.simulateSendError {
		return fmt.Errorf("simulated send error")
	}

	key := address.String()
	if _, ok := s.sentMessages[key]; !ok {
		s.sentMessages[key] = make([][]byte, 0)
	}
	s.sentMessages[key] = append(s.sentMessages[key], append([]byte{}, data...))

	return s.sendFunc(address, data)
}

func (s *MockSender) Broadcast(addresses []net.Addr, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Record addresses and data
	addressCopy := make([]net.Addr, len(addresses))
	copy(addressCopy, addresses)
	dataCopy := append([]byte{}, data...)

	msgs := [][]byte{dataCopy}
	s.broadcastedMessages = append(s.broadcastedMessages, msgs)

	// Also record in sentMessages for each address
	for _, addr := range addresses {
		key := addr.String()
		if _, ok := s.sentMessages[key]; !ok {
			s.sentMessages[key] = make([][]byte, 0)
		}
		s.sentMessages[key] = append(s.sentMessages[key], dataCopy)
	}

	return s.broadcastFunc(addresses, data)
}

func (s *MockSender) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeCalled = true
	return s.closeFunc()
}

func (s *MockSender) GetSentMessages(address string) [][]byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if msgs, ok := s.sentMessages[address]; ok {
		return msgs
	}
	return nil
}

func (s *MockSender) GetAllSentMessages() map[string][][]byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create a deep copy to avoid concurrent access issues
	result := make(map[string][][]byte)
	for addr, msgs := range s.sentMessages {
		result[addr] = make([][]byte, len(msgs))
		for i, msg := range msgs {
			result[addr][i] = append([]byte{}, msg...)
		}
	}

	return result
}

func (s *MockSender) GetBroadcastedMessages() [][][]byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.broadcastedMessages
}

func (s *MockSender) IsCloseCalled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closeCalled
}

func (s *MockSender) SetSendError(simulate bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.simulateSendError = simulate
}

// Helper functions for testing
func CreateMockCommittee(numValidators int) *model.Committee {
	committee := &model.Committee{
		Validators: make(map[model.ID]*model.Validator),
	}

	for i := 1; i <= numValidators; i++ {
		id := model.ID(i)
		baseAddr := fmt.Sprintf("127.0.0.1:%d", 10000+i)

		validator := &model.Validator{
			Address:       NewMockAddress("tcp", baseAddr),
			TxAddress:     NewMockAddress("tcp", fmt.Sprintf("127.0.0.1:%d", 20000+i)),
			BlockAddress:  NewMockAddress("tcp", fmt.Sprintf("127.0.0.1:%d", 30000+i)),
			VertexAddress: NewMockAddress("tcp", fmt.Sprintf("127.0.0.1:%d", 40000+i)),
			PublicKey:     GenerateMockPublicKey(id),
		}

		committee.Validators[id] = validator
	}

	return committee
}

// Generate mock public key for testing
func GenerateMockPublicKey(id model.ID) model.NodePublicKey {
	var key model.NodePublicKey
	binary := []byte(fmt.Sprintf("node-%d-public-key", id))
	copy(key[:], binary)
	return key
}

// createTestTransaction creates a transaction with fixed size
func CreateTestTransaction(size int) model.Transaction {
	tx := make(model.Transaction, size)
	// Fill with some identifiable pattern
	for i := 0; i < size; i++ {
		tx[i] = byte(i % 256)
	}
	return tx
}

// Helper function to wait for a specific condition with timeout
func WaitForCondition(condition func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}
