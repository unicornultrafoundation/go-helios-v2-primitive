package consensus

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/lewtran/go-helios-v2/pkg/model"
	"github.com/lewtran/go-helios-v2/pkg/network"
)

const (
	// MaxWave is the maximum number of waves in a round
	MaxWave = 4
)

// Coordinator handles the consensus protocol
type Coordinator struct {
	nodeID      model.ID
	committee   *model.Committee
	receiver    *network.Receiver
	sender      *network.Sender
	vertexCh    chan *model.Vertex
	blockCh     chan *model.Block
	broadcastCh chan *model.Vertex
	outputCh    chan *model.Vertex
	wg          sync.WaitGroup
	ctx         context.Context
	stop        context.CancelFunc
	dag         *DAG
	round       model.Round
	lastBlock   *model.BlockHash
}

// New creates a new consensus instance
func New(nodeID model.ID, committee *model.Committee) *Coordinator {
	ctx, stop := context.WithCancel(context.Background())

	// Create genesis vertex
	nodeKey := committee.GetNodeKey(nodeID)
	if nodeKey == nil {
		log.Fatalf("Failed to get node key for %d", nodeID)
	}
	genesis := model.NewVertex(*nodeKey, 0, model.NewBlock(0, nil), make(map[model.VertexHash]model.Round))

	return &Coordinator{
		nodeID:      nodeID,
		committee:   committee,
		vertexCh:    make(chan *model.Vertex, 1000),
		blockCh:     make(chan *model.Block, 1000),
		broadcastCh: make(chan *model.Vertex, 1000),
		outputCh:    make(chan *model.Vertex, 1000),
		ctx:         ctx,
		stop:        stop,
		dag:         NewDAG(genesis),
		round:       0,
	}
}

// Start starts the consensus process
func (c *Coordinator) Start() error {
	// Start network components
	addr := c.committee.GetNodeAddress(c.nodeID)
	if addr == nil {
		return fmt.Errorf("failed to get network address for node %d", c.nodeID)
	}

	// Create message handler
	handler := &messageHandler{coordinator: c}

	c.receiver = network.NewReceiver(addr, handler)
	c.sender = network.NewSender()

	if err := c.receiver.Start(); err != nil {
		return fmt.Errorf("failed to start receiver: %w", err)
	}

	// Start processing loop
	c.wg.Add(1)
	go c.run()

	return nil
}

// Stop stops the consensus process
func (c *Coordinator) Stop() {
	c.stop()
	c.wg.Wait()
	if c.receiver != nil {
		c.receiver.Stop()
	}
	if c.sender != nil {
		c.sender.Close()
	}
}

// GetBroadcastChannel returns the channel for broadcasting vertices
func (c *Coordinator) GetBroadcastChannel() chan *model.Vertex {
	return c.broadcastCh
}

// GetOutputChannel returns the channel for committed vertices
func (c *Coordinator) GetOutputChannel() chan *model.Vertex {
	return c.outputCh
}

// HandleVertex handles a new vertex from the network
func (c *Coordinator) HandleVertex(vertex *model.Vertex) error {
	select {
	case c.vertexCh <- vertex:
		return nil
	case <-c.ctx.Done():
		return fmt.Errorf("context cancelled")
	}
}

// HandleBlock handles a new block from the network
func (c *Coordinator) HandleBlock(block *model.Block) error {
	select {
	case c.blockCh <- block:
		return nil
	case <-c.ctx.Done():
		return fmt.Errorf("context cancelled")
	}
}

// GetVertexChannel returns the channel for vertices
func (c *Coordinator) GetVertexChannel() chan *model.Vertex {
	return c.vertexCh
}

// GetBlockChannel returns the channel for blocks
func (c *Coordinator) GetBlockChannel() chan *model.Block {
	return c.blockCh
}

// GetID returns the node ID
func (c *Coordinator) GetID() model.ID {
	return c.nodeID
}

// GetCommittee returns the committee
func (c *Coordinator) GetCommittee() *model.Committee {
	return c.committee
}

func (c *Coordinator) run() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		case vertex := <-c.vertexCh:
			if err := c.handleVertex(vertex); err != nil {
				log.Printf("Error handling vertex: %v", err)
			}
		case block := <-c.blockCh:
			if err := c.handleBlock(block); err != nil {
				log.Printf("Error handling block: %v", err)
			}
		}
	}
}

func (c *Coordinator) handleVertex(vertex *model.Vertex) error {
	// Add vertex to DAG
	c.dag.InsertVertex(vertex)

	// Check if we have enough vertices to create a block
	vertices := c.dag.GetVerticesInRound(c.round)
	if len(vertices) >= c.committee.Size() {
		// Create a new block with empty transactions for now
		// TODO: Add actual transactions
		block := model.NewBlock(uint64(c.round), nil)
		c.lastBlock = block.Hash()

		// Broadcast block to all validators
		data, err := block.MarshalBinary()
		if err != nil {
			return fmt.Errorf("failed to marshal block: %w", err)
		}

		// Send to all block receiver addresses
		addresses := c.committee.GetBlockReceiverAddresses()
		for _, addr := range addresses {
			if err := c.sender.Send(addr, data); err != nil {
				log.Printf("Failed to send block to %s: %v", addr, err)
			}
		}

		// Increment round
		c.round++
	}

	return nil
}

func (c *Coordinator) handleBlock(block *model.Block) error {
	// Update round if necessary
	if uint64(c.round) < block.Height {
		c.round = model.Round(block.Height)
	}

	// Update last block hash
	c.lastBlock = block.Hash()

	return nil
}

// messageHandler implements network.MessageHandler for both vertices and blocks
type messageHandler struct {
	coordinator *Coordinator
}

func (h *messageHandler) HandleMessage(data []byte) error {
	// Try to unmarshal as vertex first
	vertex := &model.Vertex{}
	if err := vertex.UnmarshalBinary(data); err == nil {
		return h.coordinator.HandleVertex(vertex)
	}

	// Try to unmarshal as block
	block := &model.Block{}
	if err := block.UnmarshalBinary(data); err == nil {
		return h.coordinator.HandleBlock(block)
	}

	return fmt.Errorf("unknown message type")
}
