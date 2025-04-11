package main

import (
	"encoding/binary"
	"flag"
	"log"
	"net"
	"time"
)

const (
	// DefaultTxSize is the size of each transaction in bytes
	DefaultTxSize = 64

	// DefaultTxCount is the default number of transactions to send
	DefaultTxCount = 1000

	// DefaultRate is the default transaction rate per second
	DefaultRate = 100
)

func main() {
	// Parse command line flags
	var (
		endpoint string
		txCount  uint64
		rate     uint64
		duration uint64
	)
	flag.StringVar(&endpoint, "endpoint", "127.0.0.1:1244", "Node endpoint to send transactions to")
	flag.Uint64Var(&txCount, "transactions", DefaultTxCount, "Number of transactions to send")
	flag.Uint64Var(&rate, "rate", DefaultRate, "Transaction rate per second")
	flag.Uint64Var(&duration, "duration", 0, "Duration in seconds (0 means no duration limit)")
	flag.Parse()

	// Connect to node
	conn, err := net.Dial("tcp", endpoint)
	if err != nil {
		log.Fatalf("Failed to connect to %s: %v", endpoint, err)
	}
	defer conn.Close()

	log.Printf("Starting transaction benchmark:")
	log.Printf("Endpoint: %s", endpoint)
	log.Printf("Transactions: %d", txCount)
	log.Printf("Rate: %d tx/s", rate)
	if duration > 0 {
		log.Printf("Duration: %d seconds", duration)
	}

	// Calculate interval between transactions
	interval := time.Second / time.Duration(rate)

	// Create ticker for rate limiting
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Create timer for duration if specified
	var timer *time.Timer
	if duration > 0 {
		timer = time.NewTimer(time.Duration(duration) * time.Second)
		defer timer.Stop()
	}

	// Send transactions
	var (
		sent   uint64
		start  = time.Now()
		txBuf  = make([]byte, DefaultTxSize)
		lenBuf = make([]byte, 4)
	)

	for sent < txCount {
		select {
		case <-ticker.C:
			// Create transaction
			binary.BigEndian.PutUint64(txBuf, sent)

			// Write transaction length
			binary.BigEndian.PutUint32(lenBuf, DefaultTxSize)
			if _, err := conn.Write(lenBuf); err != nil {
				log.Fatalf("Failed to write transaction length: %v", err)
			}

			// Write transaction data
			if _, err := conn.Write(txBuf); err != nil {
				log.Fatalf("Failed to write transaction: %v", err)
			}

			sent++
			if sent%1000 == 0 {
				log.Printf("Sent %d transactions", sent)
			}

		case <-timer.C:
			if timer != nil {
				goto done
			}
		}
	}

done:
	elapsed := time.Since(start)
	log.Printf("Benchmark complete:")
	log.Printf("Sent %d transactions in %v", sent, elapsed)
	log.Printf("Average TPS: %.2f", float64(sent)/elapsed.Seconds())
}
