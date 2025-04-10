package raft3d

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/raft-boltdb"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"
)

type RaftNode struct {
	Raft      *raft.Raft
	Printers  map[string]Printer
	Filaments map[string]Filament
	PrintJobs map[string]PrintJob
	store     map[string]string
}

func NewRaftNode(nodeID, raftDir, addr string) *RaftNode {
	rn := &RaftNode{
		Printers:  make(map[string]Printer),
		Filaments: make(map[string]Filament),
		PrintJobs: make(map[string]PrintJob),
		store:     make(map[string]string),
	}

	// Create raft directory
	if err := os.MkdirAll(raftDir, 0755); err != nil {
		log.Fatalf("Failed to create Raft directory: %v", err)
	}

	// Raft configuration
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(nodeID)

	switch nodeID {
	case "node1":
		config.ElectionTimeout = 150 * time.Millisecond
		config.HeartbeatTimeout = 50 * time.Millisecond
		config.LeaderLeaseTimeout = 25 * time.Millisecond

	case "node2":
		config.ElectionTimeout = 250 * time.Millisecond
		config.HeartbeatTimeout = 75 * time.Millisecond
		config.LeaderLeaseTimeout = 50 * time.Millisecond

	case "node3":
		config.ElectionTimeout = 350 * time.Millisecond
		config.HeartbeatTimeout = 100 * time.Millisecond
		config.LeaderLeaseTimeout = 75 * time.Millisecond

	case "node4":
		config.ElectionTimeout = 450 * time.Millisecond
		config.HeartbeatTimeout = 125 * time.Millisecond
		config.LeaderLeaseTimeout = 100 * time.Millisecond

	case "node5":
		config.ElectionTimeout = 550 * time.Millisecond
		config.HeartbeatTimeout = 150 * time.Millisecond
		config.LeaderLeaseTimeout = 125 * time.Millisecond
	}

	config.CommitTimeout = 50 * time.Millisecond

	// Log store, stable store, snapshot store
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(raftDir, "raft-log.bolt"))
	if err != nil {
		log.Fatalf("Failed to create log store: %v", err)
	}

	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(raftDir, "raft-stable.bolt"))
	if err != nil {
		log.Fatalf("Failed to create stable store: %v", err)
	}

	snapshotStore, err := raft.NewFileSnapshotStore(raftDir, 2, os.Stdout)
	if err != nil {
		log.Fatalf("Failed to create snapshot store: %v", err)
	}

	// Logger
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "raft",
		Output: os.Stdout,
		Level:  hclog.LevelFromString("DEBUG"),
	})

	// Transport
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to resolve TCP address: %v", err)
	}

	transport, err := raft.NewTCPTransportWithLogger(addr, tcpAddr, 3, 10*time.Second, logger)
	if err != nil {
		log.Fatalf("Failed to create transport: %v", err)
	}

	// Create Raft system
	hasState, err := raft.HasExistingState(logStore, stableStore, snapshotStore)
	if err != nil {
		log.Fatalf("Error checking raft state: %v", err)
	}

	ra, err := raft.NewRaft(config, rn, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		log.Fatalf("Failed to create Raft: %v", err)
	}

	// Bootstrap if no existing state
	if !hasState {
		cfg := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		if err := ra.BootstrapCluster(cfg).Error(); err != nil {
			log.Fatalf("Failed to bootstrap cluster: %v", err)
		}
	}

	go func() {
		for {
			time.Sleep(1 * time.Second)
			log.Printf("[Raft-%s] State: %s | Leader: %s",
				nodeID, ra.State(), ra.Leader())
		}
	}()

	rn.Raft = ra
	return rn
}
