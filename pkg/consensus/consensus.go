package consensus

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/unicornultrafoundation/go-helios-v2-primitive/pkg/model"
	"github.com/unicornultrafoundation/go-helios-v2-primitive/pkg/network"
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
	mu          sync.RWMutex
}

// New creates a new consensus instance
func New(nodeID model.ID, committee *model.Committee) *Coordinator {
	return NewWithRound(nodeID, committee, 0)
}

// NewWithRound creates a new consensus instance with a specified initial round
func NewWithRound(nodeID model.ID, committee *model.Committee, initialRound model.Round) *Coordinator {
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
		round:       initialRound,
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

// HandleVertex handles a vertex
func (c *Coordinator) HandleVertex(vertex *model.Vertex) error {
	// Check if context is cancelled first
	select {
	case <-c.ctx.Done():
		return fmt.Errorf("context cancelled")
	default:
		// Context is not cancelled, proceed
	}

	// Try to send the vertex to the channel
	select {
	case c.vertexCh <- vertex:
		return nil
	case <-c.ctx.Done():
		return fmt.Errorf("context cancelled")
	}
}

// HandleBlock handles a block
func (c *Coordinator) HandleBlock(block *model.Block) error {
	// Check if context is cancelled first
	select {
	case <-c.ctx.Done():
		return fmt.Errorf("context cancelled")
	default:
		// Context is not cancelled, proceed
	}

	log.Printf("HandleBlock called with height %d, hash %v", block.Height, *block.Hash())

	// Try to send the block to the channel
	select {
	case c.blockCh <- block:
		log.Printf("Block sent to channel for processing")
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
	log.Printf("Coordinator run loop started")

	for {
		select {
		case <-c.ctx.Done():
			log.Printf("Coordinator context done, exiting run loop")
			return
		case vertex := <-c.vertexCh:
			log.Printf("Processing vertex from channel in round %d", vertex.Round())
			if err := c.handleVertex(vertex); err != nil {
				log.Printf("Error handling vertex: %v", err)
			}
		case block := <-c.blockCh:
			log.Printf("Processing block from channel with height %d", block.Height)
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
	c.mu.Lock()
	defer c.mu.Unlock()

	// Log block processing
	log.Printf("Processing block: height=%d, hash=%v", block.Height, block.Hash())

	// Update last block hash regardless of block height
	c.lastBlock = block.Hash()

	// Log last block update
	log.Printf("Updated last block: %v", c.lastBlock)

	// Update round if block height is greater than current round
	if block.Height >= uint64(c.round) {
		log.Printf("Updating round from %d to %d", c.round, block.Height)
		c.round = model.Round(block.Height)
	}

	return nil
}

// messageHandler implements network.MessageHandler for both vertices and blocks
type messageHandler struct {
	coordinator *Coordinator
}

func (h *messageHandler) HandleMessage(data []byte) error {
	// Try to unmarshal as vertex first
	vertex := &model.Vertex{}
	vertexErr := vertex.UnmarshalBinary(data)
	if vertexErr == nil {
		log.Printf("Received vertex message for round %d", vertex.Round())
		return h.coordinator.HandleVertex(vertex)
	}
	log.Printf("Vertex unmarshal error: %v", vertexErr)

	// Try to unmarshal as block
	block := &model.Block{}
	blockErr := block.UnmarshalBinary(data)
	if blockErr == nil {
		log.Printf("Received block message with height %d", block.Height)
		return h.coordinator.HandleBlock(block)
	}
	log.Printf("Block unmarshal error: %v", blockErr)

	// Log the data for debugging
	if len(data) > 50 {
		log.Printf("Message data (first 50 bytes): %x", data[:50])
	} else {
		log.Printf("Message data: %x", data)
	}

	return fmt.Errorf("unknown message type: %v, %v", vertexErr, blockErr)
}
