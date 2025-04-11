package vertex

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/unicornultrafoundation/go-helios-v2-primitive/pkg/model"
	"github.com/unicornultrafoundation/go-helios-v2-primitive/pkg/network"
)

// Coordinator handles vertex broadcasting and receiving
type Coordinator struct {
	nodeID      model.ID
	committee   *model.Committee
	receiver    *network.Receiver
	sender      *network.Sender
	vertexCh    chan *model.Vertex
	broadcastCh chan *model.Vertex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// New creates a new vertex coordinator
func New(nodeID model.ID, committee *model.Committee) *Coordinator {
	ctx, cancel := context.WithCancel(context.Background())
	return &Coordinator{
		nodeID:      nodeID,
		committee:   committee,
		vertexCh:    make(chan *model.Vertex, 100),
		broadcastCh: make(chan *model.Vertex, 100),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start starts the vertex coordinator
func (c *Coordinator) Start() error {
	// Create receiver
	addr := c.committee.GetVertexReceiverAddress(c.nodeID)
	if addr == nil {
		return fmt.Errorf("failed to get vertex receiver address for node %d", c.nodeID)
	}

	log.Printf("Starting vertex receiver on %s", addr)
	c.receiver = network.NewReceiver(addr, &vertexHandler{coordinator: c})
	if err := c.receiver.Start(); err != nil {
		return fmt.Errorf("failed to start vertex receiver: %w", err)
	}

	// Create sender
	c.sender = network.NewSender()

	// Start broadcast handler
	c.wg.Add(1)
	go c.handleBroadcast()

	return nil
}

// Stop stops the vertex coordinator
func (c *Coordinator) Stop() {
	c.cancel()
	c.wg.Wait()

	if c.receiver != nil {
		c.receiver.Stop()
	}
	if c.sender != nil {
		c.sender.Close()
	}
}

// GetVertexChannel returns the channel for receiving vertices
func (c *Coordinator) GetVertexChannel() <-chan *model.Vertex {
	return c.vertexCh
}

// GetBroadcastChannel returns the channel for broadcasting vertices
func (c *Coordinator) GetBroadcastChannel() chan<- *model.Vertex {
	return c.broadcastCh
}

// GetID returns the node ID
func (c *Coordinator) GetID() model.ID {
	return c.nodeID
}

// GetCommittee returns the committee
func (c *Coordinator) GetCommittee() *model.Committee {
	return c.committee
}

// handleBroadcast handles vertex broadcasting
func (c *Coordinator) handleBroadcast() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		case vertex := <-c.broadcastCh:
			data, err := vertex.MarshalBinary()
			if err != nil {
				log.Printf("Error marshaling vertex: %v", err)
				continue
			}

			// Broadcast to all validators
			addresses := c.committee.GetVertexReceiverAddresses()
			if err := c.sender.Broadcast(addresses, data); err != nil {
				log.Printf("Error broadcasting vertex: %v", err)
			}
		}
	}
}

// vertexHandler handles vertex messages
type vertexHandler struct {
	coordinator *Coordinator
}

// HandleMessage handles a vertex message
func (h *vertexHandler) HandleMessage(data []byte) error {
	vertex := &model.Vertex{}
	if err := vertex.UnmarshalBinary(data); err != nil {
		return err
	}

	select {
	case h.coordinator.vertexCh <- vertex:
		return nil
	case <-h.coordinator.ctx.Done():
		return h.coordinator.ctx.Err()
	}
}
