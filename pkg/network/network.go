package network

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// DefaultBufferSize is the default size for network buffers
	DefaultBufferSize = 1024 * 1024 // 1MB

	// DefaultDialTimeout is the default timeout for establishing connections
	DefaultDialTimeout = 5 * time.Second

	// DefaultReadTimeout is the default timeout for reading from connections
	DefaultReadTimeout = 30 * time.Second

	// DefaultWriteTimeout is the default timeout for writing to connections
	DefaultWriteTimeout = 30 * time.Second

	// MaxMessageSize is the maximum size of a message
	MaxMessageSize = 1024 * 1024 // 1MB
)

var (
	// Buffer pool for message data
	bufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, MaxMessageSize)
		},
	}
)

// MessageHandler is the interface that must be implemented to handle network messages
type MessageHandler interface {
	HandleMessage(data []byte) error
}

// Receiver listens for incoming connections and handles messages
type Receiver struct {
	address  net.Addr
	handler  MessageHandler
	listener net.Listener
	wg       sync.WaitGroup

	// Worker pool for message processing
	workerPool chan struct{}
	numWorkers int

	// Active connections
	conns    map[string]net.Conn
	connsMux sync.RWMutex

	// Metrics
	activeConnections int64
}

// NewReceiver creates a new network receiver
func NewReceiver(address net.Addr, handler MessageHandler) *Receiver {
	return &Receiver{
		address:    address,
		handler:    handler,
		workerPool: make(chan struct{}, 20),
		numWorkers: 20,
		conns:      make(map[string]net.Conn),
	}
}

// Start starts the receiver
func (r *Receiver) Start() error {
	var err error
	r.listener, err = net.Listen(r.address.Network(), r.address.String())
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}

	r.wg.Add(1)
	go r.acceptLoop()

	return nil
}

// Stop stops the receiver
func (r *Receiver) Stop() error {
	if r.listener != nil {
		if err := r.listener.Close(); err != nil {
			return fmt.Errorf("failed to close listener: %w", err)
		}
	}
	r.wg.Wait()
	return nil
}

func (r *Receiver) acceptLoop() {
	defer r.wg.Done()

	for {
		conn, err := r.listener.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				// Log error but continue accepting
				continue
			}
			return
		}

		r.wg.Add(1)
		go r.handleConnection(conn)
	}
}

func (r *Receiver) handleConnection(conn net.Conn) {
	defer func() {
		r.removeConnection(conn)
		conn.Close()
		r.wg.Done()
		atomic.AddInt64(&r.activeConnections, -1)
	}()

	// Add connection to active connections
	r.addConnection(conn)
	atomic.AddInt64(&r.activeConnections, 1)

	// Pre-allocate header buffer
	header := make([]byte, 4)

	for {
		// Set read deadline
		conn.SetReadDeadline(time.Now().Add(DefaultReadTimeout))

		// Read message length
		if _, err := io.ReadFull(conn, header); err != nil {
			if err != io.EOF {
				// Log error but continue reading
				continue
			}
			return
		}
		length := binary.BigEndian.Uint32(header)

		// Validate message length
		if length > MaxMessageSize {
			// Message too large, skip it
			continue
		}

		// Get buffer from pool
		buf := bufferPool.Get().([]byte)
		if cap(buf) < int(length) {
			buf = make([]byte, 0, length)
		}
		buf = buf[:length]

		// Read message data
		if _, err := io.ReadFull(conn, buf); err != nil {
			bufferPool.Put(buf)
			continue
		}

		// Process message in worker pool
		select {
		case r.workerPool <- struct{}{}: // Acquire worker
			go func(data []byte) {
				defer func() {
					bufferPool.Put(data)
					<-r.workerPool // Release worker
				}()
				if err := r.handler.HandleMessage(data); err != nil {
					// Log error but continue processing
				}
			}(buf)
		default:
			// No workers available, process in current goroutine
			if err := r.handler.HandleMessage(buf); err != nil {
				// Log error but continue processing
			}
			bufferPool.Put(buf)
		}
	}
}

func (r *Receiver) addConnection(conn net.Conn) {
	r.connsMux.Lock()
	r.conns[conn.RemoteAddr().String()] = conn
	r.connsMux.Unlock()
}

func (r *Receiver) removeConnection(conn net.Conn) {
	r.connsMux.Lock()
	delete(r.conns, conn.RemoteAddr().String())
	r.connsMux.Unlock()
}

// Sender sends messages to remote nodes
type Sender struct {
	conns    map[string]net.Conn
	connsMux sync.RWMutex

	// Worker pool for broadcasting
	workerPool chan struct{}
	numWorkers int

	// Connection pool
	idleTimeout time.Duration
	maxIdle     int
	idleConns   map[string][]net.Conn
	idleMux     sync.Mutex
}

// NewSender creates a new network sender
func NewSender() *Sender {
	s := &Sender{
		conns:       make(map[string]net.Conn),
		workerPool:  make(chan struct{}, 20),
		numWorkers:  20,
		idleTimeout: 30 * time.Second,
		maxIdle:     5,
		idleConns:   make(map[string][]net.Conn),
	}

	// Start idle connection cleanup
	go s.cleanupIdleConnections()

	return s
}

// Send sends a message to a remote node
func (s *Sender) Send(address net.Addr, data []byte) error {
	conn, err := s.getConnection(address)
	if err != nil {
		return err
	}

	// Set write deadline
	conn.SetWriteDeadline(time.Now().Add(DefaultWriteTimeout))

	// Write message length
	if err := binary.Write(conn, binary.BigEndian, uint32(len(data))); err != nil {
		s.closeConnection(address)
		return fmt.Errorf("failed to write message length: %w", err)
	}

	// Write message data
	if _, err := conn.Write(data); err != nil {
		s.closeConnection(address)
		return fmt.Errorf("failed to write message data: %w", err)
	}

	return nil
}

// Broadcast sends a message to multiple remote nodes
func (s *Sender) Broadcast(addresses []net.Addr, data []byte) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(addresses))

	for _, addr := range addresses {
		wg.Add(1)
		go func(addr net.Addr) {
			defer wg.Done()

			select {
			case s.workerPool <- struct{}{}: // Acquire worker
				defer func() { <-s.workerPool }() // Release worker
				if err := s.Send(addr, data); err != nil {
					errCh <- err
				}
			default:
				// No workers available, send in current goroutine
				if err := s.Send(addr, data); err != nil {
					errCh <- err
				}
			}
		}(addr)
	}

	wg.Wait()
	close(errCh)

	// Collect errors
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to broadcast to some nodes: %v", errs)
	}
	return nil
}

func (s *Sender) getConnection(address net.Addr) (net.Conn, error) {
	s.connsMux.RLock()
	conn, ok := s.conns[address.String()]
	s.connsMux.RUnlock()

	if ok {
		// Check if connection is still alive
		if err := conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
			s.closeConnection(address)
		} else {
			conn.SetWriteDeadline(time.Time{}) // Clear deadline
			return conn, nil
		}
	}

	s.connsMux.Lock()
	defer s.connsMux.Unlock()

	// Double check after acquiring write lock
	conn, ok = s.conns[address.String()]
	if ok {
		// Check if connection is still alive
		if err := conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
			s.closeConnection(address)
		} else {
			conn.SetWriteDeadline(time.Time{}) // Clear deadline
			return conn, nil
		}
	}

	// Create new connection
	conn, err := net.DialTimeout(address.Network(), address.String(), DefaultDialTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", address, err)
	}

	s.conns[address.String()] = conn
	return conn, nil
}

func (s *Sender) closeConnection(address net.Addr) {
	s.connsMux.Lock()
	defer s.connsMux.Unlock()

	if conn, ok := s.conns[address.String()]; ok {
		conn.Close()
		delete(s.conns, address.String())
	}
}

// Close closes all connections
func (s *Sender) Close() error {
	s.connsMux.Lock()
	defer s.connsMux.Unlock()

	for _, conn := range s.conns {
		conn.Close()
	}
	s.conns = make(map[string]net.Conn)
	return nil
}

func (s *Sender) cleanupIdleConnections() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.idleMux.Lock()
		for addr, conns := range s.idleConns {
			var remaining []net.Conn
			for _, conn := range conns {
				if err := conn.SetWriteDeadline(time.Now()); err != nil {
					conn.Close()
				} else {
					remaining = append(remaining, conn)
				}
			}
			if len(remaining) > s.maxIdle {
				// Close excess connections
				for _, conn := range remaining[s.maxIdle:] {
					conn.Close()
				}
				remaining = remaining[:s.maxIdle]
			}
			if len(remaining) == 0 {
				delete(s.idleConns, addr)
			} else {
				s.idleConns[addr] = remaining
			}
		}
		s.idleMux.Unlock()
	}
}
