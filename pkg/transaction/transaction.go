package transaction

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lewtran/go-helios-v2/pkg/model"
	"github.com/lewtran/go-helios-v2/pkg/network"
)

// Metrics represents the current metrics of a transaction coordinator
type Metrics struct {
	TxReceivedCount  uint64
	TxProcessedCount uint64
	BlockCount       uint64
}

// Coordinator handles transaction processing and block creation
type Coordinator struct {
	nodeID    model.ID
	committee *model.Committee

	// Network handlers
	txReceiver  *network.Receiver
	blockSender *network.Sender

	// Transaction buffer - double buffer system
	activeBuffer []model.Transaction
	backupBuffer []model.Transaction
	bufferSize   int
	head         int
	tail         int
	txMutex      sync.RWMutex // Use RWMutex for better read concurrency

	// Pre-allocated transaction slices for block creation
	blockTransactions []model.Transaction

	// Block creation
	blockCh chan *model.Block

	// Synchronization
	wg   sync.WaitGroup
	ctx  context.Context
	stop context.CancelFunc

	// Metrics
	txReceivedCount   uint64
	txProcessedCount  uint64
	blockCreatedCount uint64
	txBufferSize      uint64
	lastMetricsTime   time.Time

	// Detailed metrics
	metrics struct {
		// Transaction metrics
		txReceivedCount  uint64 // Number of transactions received
		txReceiveTime    uint64 // Total time spent receiving transactions (ns)
		txBufferTime     uint64 // Total time spent in buffer operations (ns)
		txProcessedCount uint64 // Number of transactions processed
		txDroppedCount   uint64 // Number of transactions dropped (buffer full)

		// Block metrics
		blockCreationTime  uint64  // Total time spent creating blocks (ns)
		blockBroadcastTime uint64  // Total time spent broadcasting blocks (ns)
		blockSize          uint64  // Total size of blocks created
		blockCount         uint64  // Number of blocks created
		droppedBlockCount  uint64  // Number of blocks dropped (channel full)
		avgBlockSize       float64 // Average number of transactions per block

		// Buffer metrics
		bufferSwapCount   uint64 // Number of buffer swaps
		bufferFullCount   uint64 // Number of times buffer was full
		bufferSizeSum     uint64 // Sum of buffer sizes for averaging
		bufferSizeSamples uint64 // Number of buffer size samples

		// Lock metrics
		lockContentionCount uint64 // Number of lock contentions
		lockWaitTime        uint64 // Total time spent waiting for locks (ns)

		// Last metrics time
		lastMetricsTime time.Time
	}

	txBuffer    []model.Transaction
	blockHeight uint64
}

// New creates a new transaction coordinator
func New(nodeID model.ID, committee *model.Committee) *Coordinator {
	ctx, cancel := context.WithCancel(context.Background())
	bufferSize := 100000
	return &Coordinator{
		nodeID:            nodeID,
		committee:         committee,
		activeBuffer:      make([]model.Transaction, bufferSize),
		backupBuffer:      make([]model.Transaction, bufferSize),
		blockTransactions: make([]model.Transaction, 0, 1000), // Pre-allocate with max block size
		bufferSize:        bufferSize,
		head:              0,
		tail:              0,
		blockCh:           make(chan *model.Block, model.DefaultChannelCapacity),
		ctx:               ctx,
		stop:              cancel,
	}
}

// Start starts the transaction coordinator
func (c *Coordinator) Start() error {
	// Create transaction receiver
	txAddr := c.committee.GetTxReceiverAddress(c.nodeID)
	if txAddr == nil {
		return fmt.Errorf("failed to get transaction receiver address for node %d", c.nodeID)
	}

	log.Printf("Starting transaction receiver on %s", txAddr)
	c.txReceiver = network.NewReceiver(txAddr, &txHandler{coordinator: c})
	if err := c.txReceiver.Start(); err != nil {
		return fmt.Errorf("failed to start transaction receiver: %w", err)
	}

	// Create block sender
	c.blockSender = network.NewSender()

	// Start block creation loop
	c.wg.Add(1)
	go c.run()

	return nil
}

// Stop stops the transaction coordinator
func (c *Coordinator) Stop() {
	c.stop()
	if c.txReceiver != nil {
		c.txReceiver.Stop()
	}
	if c.blockSender != nil {
		c.blockSender.Close()
	}
	c.wg.Wait()
}

// GetBlockChannel returns the channel for created blocks
func (c *Coordinator) GetBlockChannel() <-chan *model.Block {
	return c.blockCh
}

func (c *Coordinator) run() {
	defer c.wg.Done()

	// Create a ticker for metrics
	metricsTicker := time.NewTicker(time.Second)
	defer metricsTicker.Stop()

	// Create a ticker for block creation
	blockTicker := time.NewTicker(300 * time.Millisecond) // Fixed 300ms interval for more predictable block creation
	defer blockTicker.Stop()

	for {
		select {
		case <-blockTicker.C:
			c.createBlock()

		case <-metricsTicker.C:
			c.printMetrics()

		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Coordinator) getBufferSize() int {
	if c.tail >= c.head {
		return c.tail - c.head
	}
	return c.bufferSize - c.head + c.tail
}

func (c *Coordinator) createBlock() error {
	start := time.Now()

	// Only create a block if we have transactions
	if len(c.txBuffer) == 0 {
		return nil
	}

	// Create a new block with transactions limited by max block size (2MB)
	const maxBlockSize = 2 * 1024 * 1024 // Maximum block size in bytes (2MB)
	var blockTxs []model.Transaction
	var totalSize int

	// Lock buffer for reading and writing
	c.txMutex.Lock()
	defer c.txMutex.Unlock()

	// Take transactions that fit within the max block size
	// Process as many transactions as possible within the size limit
	for i := 0; i < len(c.txBuffer); i++ {
		tx := c.txBuffer[i]
		txSize := len(tx)

		// Always include at least one transaction regardless of size
		if len(blockTxs) == 0 || (totalSize+txSize <= maxBlockSize) {
			blockTxs = append(blockTxs, tx)
			totalSize += txSize
		} else {
			// Stop processing if we've reached the size limit
			break
		}
	}

	// Number of transactions processed
	txProcessed := len(blockTxs)

	// Create a new block with the selected transactions
	block := model.NewBlock(c.blockHeight, blockTxs)
	c.blockHeight++

	// Update metrics
	c.metrics.blockCount++
	c.metrics.txProcessedCount += uint64(txProcessed)
	c.metrics.avgBlockSize = float64(c.metrics.txProcessedCount) / float64(c.metrics.blockCount)
	c.metrics.blockCreationTime = uint64(time.Since(start).Nanoseconds())

	// Remove the processed transactions from the buffer
	c.txBuffer = c.txBuffer[txProcessed:]

	// Try to send the block, don't block if channel is full
	select {
	case c.blockCh <- block:
		return nil
	default:
		c.metrics.droppedBlockCount++
		return fmt.Errorf("block channel is full")
	}
}

// AddTransaction adds a transaction to the buffer
func (c *Coordinator) AddTransaction(tx model.Transaction) {
	// Check if context is done
	select {
	case <-c.ctx.Done():
		return
	default:
	}

	start := time.Now()

	// Use read lock to check buffer size
	lockStart := time.Now()
	c.txMutex.RLock()
	atomic.AddUint64(&c.metrics.lockWaitTime, uint64(time.Since(lockStart).Nanoseconds()))

	bufferSize := len(c.txBuffer)
	if bufferSize >= c.bufferSize {
		c.txMutex.RUnlock()
		atomic.AddUint64(&c.metrics.bufferFullCount, 1)
		atomic.AddUint64(&c.metrics.txDroppedCount, 1)
		log.Printf("Failed to add transaction to buffer: buffer full")
		return
	}
	c.txMutex.RUnlock()

	// Upgrade to write lock only when we need to modify
	lockStart = time.Now()
	c.txMutex.Lock()
	atomic.AddUint64(&c.metrics.lockWaitTime, uint64(time.Since(lockStart).Nanoseconds()))

	// Double check after acquiring lock
	bufferSize = len(c.txBuffer)
	if bufferSize >= c.bufferSize {
		c.txMutex.Unlock()
		atomic.AddUint64(&c.metrics.bufferFullCount, 1)
		atomic.AddUint64(&c.metrics.txDroppedCount, 1)
		log.Printf("Failed to add transaction to buffer: buffer full")
		return
	}

	// Add transaction to buffer
	bufferStart := time.Now()
	c.txBuffer = append(c.txBuffer, tx)
	atomic.AddUint64(&c.metrics.txBufferTime, uint64(time.Since(bufferStart).Nanoseconds()))

	c.txMutex.Unlock()

	atomic.AddUint64(&c.metrics.txReceivedCount, 1)
	atomic.AddUint64(&c.metrics.txReceiveTime, uint64(time.Since(start).Nanoseconds()))
	atomic.StoreUint64(&c.txBufferSize, uint64(len(c.txBuffer)))

	if len(c.txBuffer)%1000 == 0 {
		log.Printf("Transaction added to buffer (size: %d) in %v", len(c.txBuffer), time.Since(start))
	}
}

func (c *Coordinator) printMetrics() {
	// Calculate time since last metrics print
	now := time.Now()
	duration := now.Sub(c.metrics.lastMetricsTime).Seconds()
	c.metrics.lastMetricsTime = now

	// Calculate averages
	avgTxReceiveTime := float64(c.metrics.txReceiveTime) / float64(max(c.metrics.txReceivedCount, 1))
	avgTxBufferTime := float64(c.metrics.txBufferTime) / float64(max(c.metrics.txReceivedCount, 1))
	avgBlockCreationTime := float64(c.metrics.blockCreationTime) / float64(max(c.metrics.blockCount, 1))
	avgBlockBroadcastTime := float64(c.metrics.blockBroadcastTime) / float64(max(c.metrics.blockCount, 1))
	avgLockWaitTime := float64(c.metrics.lockWaitTime) / float64(max(c.metrics.lockContentionCount, 1))
	avgBufferSize := float64(c.metrics.bufferSizeSum) / float64(max(c.metrics.bufferSizeSamples, 1))

	// Calculate rates
	txRate := float64(c.metrics.txReceivedCount) / duration
	blockRate := float64(c.metrics.blockCount) / duration

	// Log metrics
	log.Printf("Transaction Metrics: %d received (%.2f tx/s), %d processed, %d dropped, avg receive time: %.2f µs, avg buffer time: %.2f µs",
		c.metrics.txReceivedCount, txRate, c.metrics.txProcessedCount, c.metrics.txDroppedCount,
		avgTxReceiveTime/1000, avgTxBufferTime/1000)

	log.Printf("Block Metrics: %d created (%.2f blocks/s), %d dropped, avg size: %.2f tx/block, avg creation time: %.2f ms, avg broadcast time: %.2f ms",
		c.metrics.blockCount, blockRate, c.metrics.droppedBlockCount, c.metrics.avgBlockSize,
		avgBlockCreationTime/1e6, avgBlockBroadcastTime/1e6)

	log.Printf("Buffer Metrics: current size: %d, avg size: %.2f, swaps: %d, full count: %d",
		len(c.txBuffer), avgBufferSize, c.metrics.bufferSwapCount, c.metrics.bufferFullCount)

	log.Printf("Lock Metrics: contentions: %d, avg wait time: %.2f µs",
		c.metrics.lockContentionCount, avgLockWaitTime/1000)
}

// Helper function for safe division
func max(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

// txHandler implements network.MessageHandler for transactions
type txHandler struct {
	coordinator *Coordinator
}

func (h *txHandler) HandleMessage(data []byte) error {
	// Create a new transaction from the raw data
	tx := model.Transaction(data)

	// Add transaction to buffer
	h.coordinator.AddTransaction(tx)
	return nil
}

// GetMetrics returns the current transaction metrics
func (c *Coordinator) GetMetrics() struct {
	TxReceivedCount  uint64
	TxProcessedCount uint64
	BlockCount       uint64
} {
	return struct {
		TxReceivedCount  uint64
		TxProcessedCount uint64
		BlockCount       uint64
	}{
		TxReceivedCount:  atomic.LoadUint64(&c.metrics.txReceivedCount),
		TxProcessedCount: atomic.LoadUint64(&c.metrics.txProcessedCount),
		BlockCount:       atomic.LoadUint64(&c.metrics.blockCount),
	}
}

// PrintMetrics prints the current metrics
func (c *Coordinator) PrintMetrics() {
	c.printMetrics()
}

// GetID returns the node ID
func (c *Coordinator) GetID() model.ID {
	return c.nodeID
}

// GetCommittee returns the committee
func (c *Coordinator) GetCommittee() *model.Committee {
	return c.committee
}
