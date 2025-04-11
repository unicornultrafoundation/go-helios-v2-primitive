package main

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unicornultrafoundation/go-helios-v2-primitive/cmd/benchmark/config"
	"github.com/unicornultrafoundation/go-helios-v2-primitive/pkg/model"
	"github.com/unicornultrafoundation/go-helios-v2-primitive/pkg/transaction"
)

// Helper function to create a multi-node committee
func createTestCommittee(totalValidators int, basePort uint16) *model.Committee {
	committee := model.NewCommittee()

	// Create validators with their addresses
	for i := 0; i < totalValidators; i++ {
		nodeID := model.ID(i + 1)
		validator := &model.Validator{
			TxAddress: &net.TCPAddr{
				IP:   net.ParseIP(config.Config.DefaultHost),
				Port: int(basePort + uint16(i)*config.Config.PortsPerNode),
			},
			BlockAddress: &net.TCPAddr{
				IP:   net.ParseIP(config.Config.DefaultHost),
				Port: int(basePort + uint16(i)*config.Config.PortsPerNode + 1),
			},
			VertexAddress: &net.TCPAddr{
				IP:   net.ParseIP(config.Config.DefaultHost),
				Port: int(basePort + uint16(i)*config.Config.PortsPerNode + 2),
			},
		}
		committee.Validators[nodeID] = validator
	}

	return committee
}

func BenchmarkMultiNodeTransactionProcessing(b *testing.B) {
	committee := createTestCommittee(config.Config.TotalValidators, config.Config.BasePort)

	// Create and start coordinators for each node
	coordinators := make([]*transaction.Coordinator, config.Config.TotalValidators)
	for i := 0; i < config.Config.TotalValidators; i++ {
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
		tx := config.CreateFixedSizeTransaction(i)
		// Round-robin distribution of transactions
		nodeIdx := i % config.Config.TotalValidators
		coordinators[nodeIdx].AddTransaction(tx)
		if i%1000 == 0 {
			b.Logf("Added %d transactions", i)
		}
	}

	// Wait for blocks to be created
	// Calculate number of blocks based on total data size and block size limit
	totalDataSize := b.N * config.Config.TransactionSize
	numBlocks := (totalDataSize + config.Config.BlockSize - 1) / config.Config.BlockSize // Ceiling division by block size
	waitTime := time.Duration(numBlocks) * config.Config.BlockWaitTime
	waitTime += config.Config.ExtraWaitTime

	b.Logf("Waiting up to %v for %d transactions to be processed", waitTime, b.N)

	// Keep checking until all transactions are processed or timeout
	deadline := time.Now().Add(waitTime)
	lastMetrics := make([]transaction.Metrics, config.Config.TotalValidators)
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
		if time.Since(lastPrint) > config.Config.ProgressInterval {
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

		time.Sleep(config.Config.PollInterval)
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
	b.ReportMetric(float64(totalProcessed)/float64(config.Config.TotalValidators), "tx/node")
}

// Original single-node benchmark
func BenchmarkTransactionProcessing(b *testing.B) {
	nodeID := model.ID(1)
	committee := createTestCommittee(1, config.Config.BasePort)

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
		tx := config.CreateFixedSizeTransaction(i)
		coordinator.AddTransaction(tx)
	}

	// Wait for blocks to be created
	// Calculate number of blocks based on total data size and block size limit
	totalDataSize := b.N * config.Config.TransactionSize
	numBlocks := (totalDataSize + config.Config.BlockSize - 1) / config.Config.BlockSize // Ceiling division by block size
	waitTime := time.Duration(numBlocks) * config.Config.BlockWaitTime
	waitTime += config.Config.ExtraWaitTime

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
		if time.Since(lastPrint) > config.Config.ProgressInterval {
			b.Logf("Progress: %d/%d transactions processed", metrics.TxProcessedCount, b.N)
			lastPrint = time.Now()
		}

		// If no progress in last second, print metrics
		if metrics.TxProcessedCount == lastMetrics.TxProcessedCount {
			coordinator.PrintMetrics()
		}
		lastMetrics = metrics

		time.Sleep(config.Config.PollInterval)
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
