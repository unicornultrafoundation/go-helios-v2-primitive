package config

import (
	"fmt"
	"time"

	"github.com/unicornultrafoundation/go-helios-v2-primitive/pkg/model"
)

// Config contains all configuration constants for benchmarking
var Config = struct {
	// Network configuration
	BasePort     uint16
	PortsPerNode uint16
	DefaultHost  string

	// Benchmark configuration
	BlockWaitTime    time.Duration
	ExtraWaitTime    time.Duration
	ProgressInterval time.Duration
	PollInterval     time.Duration
	TotalValidators  int

	// Fixed transaction and block sizes
	TransactionSize int
	BlockSize       int
}{
	BasePort:     15000,
	PortsPerNode: 3, // One port each for tx, block, and vertex
	DefaultHost:  "0.0.0.0",

	BlockWaitTime:    100 * time.Millisecond,
	ExtraWaitTime:    1 * time.Second,
	ProgressInterval: time.Second,
	PollInterval:     50 * time.Millisecond,
	TotalValidators:  10,

	TransactionSize: 2 * 1024,        // 2KB per transaction for synthetic benchmark
	BlockSize:       4 * 1024 * 1024, // 4MB block size limit
}

// CreateFixedSizeTransaction creates a transaction of exactly the specified size
func CreateFixedSizeTransaction(index int) model.Transaction {
	// Create base transaction with index
	base := fmt.Sprintf("tx%d-", index)

	// Create a byte slice of the required size
	txData := make([]byte, Config.TransactionSize)

	// Copy the base into the beginning of the transaction
	copy(txData, []byte(base))

	// Fill the rest with a repeating pattern for padding
	for i := len(base); i < Config.TransactionSize; i++ {
		txData[i] = byte('a' + (i % 26))
	}

	return txData
}
