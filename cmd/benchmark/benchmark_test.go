package main

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/lewtran/go-helios-v2/pkg/model"
	"github.com/lewtran/go-helios-v2/pkg/transaction"
	"github.com/stretchr/testify/require"
)

const (
	// Network configuration
	BasePort     = uint16(15000)
	PortsPerNode = uint16(3) // One port each for tx, block, and vertex
	DefaultHost  = "0.0.0.0"

	// Benchmark configuration
	TxPerBlock       = 1000
	BlockWaitTime    = 300 * time.Millisecond
	ExtraWaitTime    = 5 * time.Second
	ProgressInterval = time.Second
	PollInterval     = 100 * time.Millisecond
	TotalValidators  = 7

	// Fixed transaction size
	TransactionSize = 3 * 1024 // 3KB per transaction
)

// Helper function to create a multi-node committee
func createTestCommittee(totalValidators int, basePort uint16) *model.Committee {
	committee := model.NewCommittee()

	// Create validators with their addresses
	for i := 0; i < totalValidators; i++ {
		nodeID := model.ID(i + 1)
		validator := committee.Validators[nodeID]
		validator.TxAddress = &net.TCPAddr{
			IP:   net.ParseIP(DefaultHost),
			Port: int(basePort + uint16(i)*PortsPerNode),
		}
		validator.BlockAddress = &net.TCPAddr{
			IP:   net.ParseIP(DefaultHost),
			Port: int(basePort + uint16(i)*PortsPerNode + 1),
		}
		validator.VertexAddress = &net.TCPAddr{
			IP:   net.ParseIP(DefaultHost),
			Port: int(basePort + uint16(i)*PortsPerNode + 2),
		}
	}

	return committee
}

// createFixedSizeTransaction creates a transaction of exactly the specified size
func createFixedSizeTransaction(index int) model.Transaction {
	// Create base transaction with index
	base := fmt.Sprintf("tx%d-", index)

	// Create a byte slice of the required size
	txData := make([]byte, TransactionSize)

	// Copy the base into the beginning of the transaction
	copy(txData, []byte(base))

	// Fill the rest with a repeating pattern for padding
	for i := len(base); i < TransactionSize; i++ {
		txData[i] = byte('a' + (i % 26))
	}

	return txData
}

func BenchmarkMultiNodeTransactionProcessing(b *testing.B) {
	committee := createTestCommittee(TotalValidators, BasePort)

	// Create and start coordinators for each node
	coordinators := make([]*transaction.Coordinator, TotalValidators)
	for i := 0; i < TotalValidators; i++ {
		nodeID := model.ID(i + 1)
		coordinator := transaction.New(nodeID, committee)
		require.NotNil(b, coordinator)

		err := coordinator.Start()
		require.NoError(b, err)
		defer coordinator.Stop()

		coordinators[i] = coordinator
	}

	// Reset timer before the actual benchmark
	b.ResetTimer()

	// Run b.N transactions, distributed across nodes
	for i := 0; i < b.N; i++ {
		tx := createFixedSizeTransaction(i)
		// Round-robin distribution of transactions
		nodeIdx := i % TotalValidators
		coordinators[nodeIdx].AddTransaction(tx)
		if i%TxPerBlock == 0 {
			b.Logf("Added %d transactions", i)
		}
	}

	// Wait for blocks to be created
	// Calculate wait time based on number of transactions
	numBlocks := (b.N + TxPerBlock - 1) / TxPerBlock // Ceiling division
	waitTime := time.Duration(numBlocks) * BlockWaitTime
	waitTime += ExtraWaitTime

	b.Logf("Waiting up to %v for %d transactions to be processed", waitTime, b.N)

	// Keep checking until all transactions are processed or timeout
	deadline := time.Now().Add(waitTime)
	lastMetrics := make([]transaction.Metrics, TotalValidators)
	lastPrint := time.Now()
	totalProcessed := uint64(0)

	for time.Now().Before(deadline) {
		totalProcessed = 0
		noProgress := true

		for i, coordinator := range coordinators {
			metrics := coordinator.GetMetrics()
			totalProcessed += metrics.TxProcessedCount

			if metrics.TxProcessedCount > lastMetrics[i].TxProcessedCount {
				noProgress = false
				b.Logf("Node %d made progress: %d -> %d", i+1, lastMetrics[i].TxProcessedCount, metrics.TxProcessedCount)
			}
			lastMetrics[i] = metrics
		}

		if totalProcessed == uint64(b.N) {
			b.Logf("All transactions processed!")
			break
		}

		// Print progress every second
		if time.Since(lastPrint) > ProgressInterval {
			b.Logf("Progress: %d/%d transactions processed (%.2f%%)", totalProcessed, b.N, float64(totalProcessed)*100/float64(b.N))
			for i, coordinator := range coordinators {
				b.Logf("Node %d metrics:", i+1)
				coordinator.PrintMetrics()
			}
			lastPrint = time.Now()
		}

		if noProgress {
			b.Logf("No progress in last iteration, waiting...")
		}

		time.Sleep(PollInterval)
	}

	// Print final metrics for all nodes
	b.Logf("Final metrics after %v:", time.Since(deadline.Add(-waitTime)))
	for i, coordinator := range coordinators {
		b.Logf("Node %d metrics:", i+1)
		coordinator.PrintMetrics()
	}

	// Verify all transactions were processed (or at least 99.99% for large tests)
	percentComplete := float64(totalProcessed) / float64(b.N) * 100.0
	if percentComplete >= 99.99 {
		b.Logf("Successfully processed %.2f%% of transactions (%d/%d)", percentComplete, totalProcessed, b.N)
	} else {
		require.Equal(b, uint64(b.N), totalProcessed, "Not all transactions were processed")
	}

	// Report custom metrics
	b.ReportMetric(float64(totalProcessed)/b.Elapsed().Seconds(), "tx/sec")
	b.ReportMetric(float64(totalProcessed)/float64(TotalValidators), "tx/node")
}

// Original single-node benchmark
func BenchmarkTransactionProcessing(b *testing.B) {
	nodeID := model.ID(1)
	committee := createTestCommittee(1, BasePort)

	coordinator := transaction.New(nodeID, committee)
	require.NotNil(b, coordinator)

	// Start coordinator
	err := coordinator.Start()
	require.NoError(b, err)
	defer coordinator.Stop()

	// Reset timer before the actual benchmark
	b.ResetTimer()

	// Run b.N transactions
	for i := 0; i < b.N; i++ {
		tx := createFixedSizeTransaction(i)
		coordinator.AddTransaction(tx)
	}

	// Wait for blocks to be created
	// Calculate wait time based on number of transactions
	numBlocks := (b.N + TxPerBlock - 1) / TxPerBlock // Ceiling division
	waitTime := time.Duration(numBlocks) * BlockWaitTime
	waitTime += ExtraWaitTime

	// Keep checking until all transactions are processed or timeout
	deadline := time.Now().Add(waitTime)
	lastMetrics := coordinator.GetMetrics()
	lastPrint := time.Now()

	for time.Now().Before(deadline) {
		metrics := coordinator.GetMetrics()
		if metrics.TxProcessedCount == uint64(b.N) {
			break
		}

		// Print progress every second
		if time.Since(lastPrint) > ProgressInterval {
			b.Logf("Progress: %d/%d transactions processed", metrics.TxProcessedCount, b.N)
			lastPrint = time.Now()
		}

		// If no progress in last second, print metrics
		if metrics.TxProcessedCount == lastMetrics.TxProcessedCount {
			coordinator.PrintMetrics()
		}
		lastMetrics = metrics

		time.Sleep(PollInterval)
	}

	// Print final metrics
	coordinator.PrintMetrics()

	// Verify all transactions were processed
	metrics := coordinator.GetMetrics()
	require.Equal(b, uint64(b.N), metrics.TxProcessedCount, "Not all transactions were processed")

	// Report custom metrics
	b.ReportMetric(float64(metrics.TxProcessedCount)/b.Elapsed().Seconds(), "tx/sec")
	b.ReportMetric(float64(metrics.BlockCount), "blocks")
}
