# Helios v2 Primitive

A high-performance consensus mechanism based on the Helios protocol, optimized for distributed systems.

## About

Helios v2 Primitive is a Go implementation of the Helios consensus protocol, providing fast finality and high throughput for blockchain and distributed systems. The implementation focuses on performance, security, and U2U Layer 2 scaling.

## Components and Architecture

### Core Components

1. **Consensus Engine**
   - Located in `pkg/consensus`
   - Implements the wave-based Helios consensus algorithm
   - Handles vertex creation, validation, and finalization
   - Manages state transitions between rounds and waves

2. **Transaction Manager**
   - Located in `pkg/transaction`
   - Processes incoming transactions
   - Maintains transaction buffers
   - Creates blocks from validated transactions

3. **Network Layer**
   - Handles message passing between nodes
   - Implements peer discovery and connection management
   - Provides reliable communication channels for consensus messages

4. **Model**
   - Located in `pkg/model`
   - Defines core data structures like Block, Vertex, Transaction
   - Implements serialization/deserialization of messages

5. **Vertex System**
   - Located in `pkg/vertex`
   - Manages the DAG (Directed Acyclic Graph) structure
   - Handles vertex creation, validation, and storage

### Architecture

The system follows a multi-layered architecture:

1. **Network Layer**: Handles communication between nodes
2. **Consensus Layer**: Implements the Helios consensus protocol
3. **Execution Layer**: Processes transactions and updates the state
4. **Storage Layer**: Persists blocks, vertices, and state

Nodes in the network participate in rounds and waves of consensus, creating vertices that reference parent vertices. Each vertex may contain a block of transactions. The consensus mechanism ensures that all honest nodes agree on the same set of vertices, providing Byzantine Fault Tolerance.

## Building the Project

### Prerequisites

- Go 1.19 or higher
- Git

### Build Instructions

1. Clone the repository:

```bash
git clone https://github.com/unicornultrafoundation/go-helios-v2-primitive.git
cd go-helios-v2-primitive
```

2. Build the project:

```bash
go build ./...
```

3. Run tests to ensure everything is working:

```bash
go test ./...
```

## Running the Benchmark

The project includes benchmarking tools to measure performance and scalability.

### Configuration

Benchmark settings can be configured in `cmd/benchmark/config/config.go`:

- `TotalValidators`: Number of validator nodes to simulate (default: 10)
- `TransactionSize`: Size of each transaction in bytes (default: 2KB)
- `BlockSize`: Maximum block size in bytes (default: 4MB)
- `BasePort`: Starting port number for validator communication

### Running Benchmarks

To run the multi-node transaction processing benchmark with 100,000 transactions:

```bash
go test -bench=BenchmarkMultiNodeTransactionProcessing -benchtime=100000x ./cmd/benchmark -timeout 5m
```

To run the single-node transaction processing benchmark:

```bash
go test -bench=BenchmarkTransactionProcessing -benchtime=100000x ./cmd/benchmark -timeout 5m
```

### Custom Parameters

You can modify the benchmark configuration in `cmd/benchmark/config/config.go` to test different scenarios:

- Adjust the number of validators to test scalability
- Change transaction size to test different payload sizes
- Modify block size to test block creation performance

## Latest Benchmark Results

### Multi-Node Performance (10 validators)

Tested on `Apple M2 Pro - 12 Cores (8+4) - 32GB Ram`

- **Transactions Processed**: 1,000,000
- **Transaction Size**: 2KB per transaction
- **Average Throughput**: ~68,000 transactions per second
- **Time per Operation**: ~14,285 ns/op
- **Transactions per Node**: 100,000 per node
- **Average Block Creation Time**: ~2 ms
- **Total Blocks Created**: ~2,000 blocks across all nodes
- **Average Transactions per Block**: ~500 transactions

The multi-node benchmark successfully processed all 1,000,000 transactions distributed across 10 nodes with efficient block creation.

### Single-Node Performance

- **Transactions Processed**: 40,960 (out of 100,000)
- **Transaction Size**: 2KB per transaction
- **Block Creation Rate**: ~20 blocks

The single-node benchmark demonstrated performance limitations when handling high transaction volume, processing only ~41% of the transactions within the time limit.

## Further Development

The Helios v2 Primitive implementation continues to be optimized for performance and scalability. Future work includes:

- Improved memory management
- Optimized block creation algorithms
- Enhanced network communication
- Support for dynamic validator sets

## License

MIT