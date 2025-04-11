package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/lewtran/go-helios-v2/pkg/model"
	"github.com/lewtran/go-helios-v2/pkg/node"
)

func main() {
	// Parse command line flags
	var nodeID uint64
	flag.Uint64Var(&nodeID, "id", 0, "Node ID")
	flag.Parse()

	if nodeID == 0 {
		log.Fatal("Node ID must be specified")
	}

	// Create and start node
	n := node.New(model.ID(nodeID))
	if err := n.Start(); err != nil {
		log.Fatalf("Failed to start node: %v", err)
	}
	defer n.Stop()

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
}
