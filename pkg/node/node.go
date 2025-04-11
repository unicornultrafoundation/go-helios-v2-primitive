package node

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"github.com/unicornultrafoundation/go-helios-v2-primitive/pkg/consensus"
	"github.com/unicornultrafoundation/go-helios-v2-primitive/pkg/model"
	"github.com/unicornultrafoundation/go-helios-v2-primitive/pkg/transaction"
	"github.com/unicornultrafoundation/go-helios-v2-primitive/pkg/vertex"
)

// Node represents a node in the Helios network
type Node struct {
	id        model.ID
	committee *model.Committee

	// Components
	consensus   *consensus.Coordinator
	transaction *transaction.Coordinator
	vertex      *vertex.Coordinator

	// Synchronization
	wg   sync.WaitGroup
	ctx  context.Context
	stop context.CancelFunc

	// State
	state *consensus.State

	// Node metrics
	running           bool
	mu                sync.RWMutex
	committedVertices uint64
}

// New creates a new node
func New(id model.ID) *Node {
	committee := model.NewCommittee()
	ctx, cancel := context.WithCancel(context.Background())

	// Create genesis vertex for state
	nodeKey := committee.GetNodeKey(id)
	if nodeKey == nil {
		log.Fatalf("Failed to get node key for %d", id)
	}
	genesis := model.NewVertex(*nodeKey, 0, model.NewBlock(0, nil), make(map[model.VertexHash]model.Round))

	return &Node{
		id:        id,
		committee: committee,
		ctx:       ctx,
		stop:      cancel,
		state:     consensus.NewState(genesis),
	}
}

// Start starts the node and all its components
func (n *Node) Start() error {
	// Check if already running
	if n.IsRunning() {
		return fmt.Errorf("node %d is already running", n.id)
	}

	log.Printf("Starting node %d", n.id)

	// Create components
	n.consensus = consensus.New(n.id, n.committee)
	n.transaction = transaction.New(n.id, n.committee)
	n.vertex = vertex.New(n.id, n.committee)

	// Start transaction coordinator
	if err := n.transaction.Start(); err != nil {
		return fmt.Errorf("failed to start transaction coordinator: %w", err)
	}

	// Start vertex coordinator
	if err := n.vertex.Start(); err != nil {
		n.transaction.Stop()
		return fmt.Errorf("failed to start vertex coordinator: %w", err)
	}

	// Start consensus
	if err := n.consensus.Start(); err != nil {
		n.transaction.Stop()
		n.vertex.Stop()
		return fmt.Errorf("failed to start consensus: %w", err)
	}

	// Connect components
	if err := n.connectComponents(); err != nil {
		n.consensus.Stop()
		n.transaction.Stop()
		n.vertex.Stop()
		return fmt.Errorf("failed to connect components: %w", err)
	}

	// Start output handler
	n.wg.Add(1)
	go n.handleOutput()

	// Update state
	n.SetRunning(true)

	return nil
}

// Stop stops the node and all its components
func (n *Node) Stop() {
	log.Printf("Stopping node %d", n.id)

	n.stop()
	n.consensus.Stop()
	n.transaction.Stop()
	n.vertex.Stop()
	n.wg.Wait()

	// Update state
	n.SetRunning(false)
}

// SetRunning sets the running state of the node
func (n *Node) SetRunning(running bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.running = running
}

// IsRunning returns whether the node is currently running
func (n *Node) IsRunning() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.running
}

// IncrementCommittedVertices increments the count of committed vertices
func (n *Node) IncrementCommittedVertices() {
	atomic.AddUint64(&n.committedVertices, 1)
}

// GetCommittedVertices returns the number of committed vertices
func (n *Node) GetCommittedVertices() uint64 {
	return atomic.LoadUint64(&n.committedVertices)
}

func (n *Node) connectComponents() error {
	// Connect transaction coordinator to consensus
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		blockCh := n.transaction.GetBlockChannel()
		for {
			select {
			case block, ok := <-blockCh:
				if !ok {
					return
				}
				if err := n.consensus.HandleBlock(block); err != nil {
					log.Printf("Error handling block: %v", err)
				}
			case <-n.ctx.Done():
				return
			}
		}
	}()

	// Connect vertex coordinator to consensus
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		vertexCh := n.vertex.GetVertexChannel()
		for {
			select {
			case vertex, ok := <-vertexCh:
				if !ok {
					return
				}
				if err := n.consensus.HandleVertex(vertex); err != nil {
					log.Printf("Error handling vertex: %v", err)
				}
			case <-n.ctx.Done():
				return
			}
		}
	}()

	// Connect consensus to vertex coordinator for broadcasting
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		broadcastCh := n.consensus.GetBroadcastChannel()
		vertexBroadcastCh := n.vertex.GetBroadcastChannel()
		for {
			select {
			case vertex, ok := <-broadcastCh:
				if !ok {
					return
				}
				select {
				case vertexBroadcastCh <- vertex:
				case <-n.ctx.Done():
					return
				}
			case <-n.ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (n *Node) handleOutput() {
	defer n.wg.Done()

	outputCh := n.consensus.GetOutputChannel()
	for {
		select {
		case vertex, ok := <-outputCh:
			if !ok {
				return
			}
			log.Printf("Node %d: Committed vertex %s", n.id, vertex)
			n.IncrementCommittedVertices()
		case <-n.ctx.Done():
			return
		}
	}
}

// GetState returns the current state of the node
func (n *Node) GetState() *consensus.State {
	return n.state
}
