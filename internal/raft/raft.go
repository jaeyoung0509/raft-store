package raft

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
)

// RaftNode implements the Node interface
type RaftNode struct {
	raft   *raft.Raft
	config *NodeConfig
}

// NewRaftNode creates a new Raft node
func NewRaftNode(config *NodeConfig) (*RaftNode, error) {
	fmt.Printf("[DEBUG] Starting NewRaftNode with config: %+v\n", config)

	// Create Raft Directory
	raftDir := filepath.Join(config.DataDir, config.ID)
	fmt.Printf("[DEBUG] Creating raft directory: %s\n", raftDir)
	if err := os.MkdirAll(raftDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create raft directory: %v", err)
	}

	// Create the store
	storePath := filepath.Join(raftDir, "raft.db")
	boltStore, err := NewBoltStore(storePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create bolt store: %v", err)
	}

	// Create the snapshot store
	snapshotStore, err := raft.NewFileSnapshotStore(raftDir, 3, os.Stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot store: %v", err)
	}

	// Create FSM
	fsm := newFSM()

	// Create transport layer
	var transport raft.Transport
	if config.InMemory {
		fmt.Printf("[DEBUG] Setting up InmemTransport\n")
		addr, memTransport := raft.NewInmemTransportWithTimeout(
			raft.ServerAddress(config.Addr),
			150*time.Microsecond, // 타임아웃 증가
		)
		fmt.Printf("[DEBUG] Created InmemTransport with addr: %s\n", addr)
		config.Addr = string(addr)
		transport = memTransport

		// 자기 자신과 연결 설정
		memTransport.Connect(addr, memTransport)
	} else {
		addr, err := net.ResolveTCPAddr("tcp", config.Addr)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve tcp address: %v", err)
		}

		// Increase maxPool for better connection handling
		// Increase timeout for better network resilience
		transport, err = raft.NewTCPTransport(
			config.Addr,
			addr,
			5,              // increased from 3
			30*time.Second, // increased from 10
			os.Stdout,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create transport: %v", err)
		}
	}

	fmt.Printf("[DEBUG] Creating Raft configuration\n")
	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(config.ID)

	raftConfig.HeartbeatTimeout = 1 * time.Second
	raftConfig.ElectionTimeout = 1 * time.Second
	raftConfig.LeaderLeaseTimeout = 1 * time.Second
	raftConfig.CommitTimeout = 2 * time.Second
	raftConfig.SnapshotInterval = 24 * time.Hour
	raftConfig.TrailingLogs = 10000
	raftConfig.ShutdownOnRemove = false

	// 디버깅을 위한 로그 설정
	raftConfig.Logger = hclog.New(&hclog.LoggerOptions{
		Name:   fmt.Sprintf("raft-%s", config.ID),
		Output: os.Stdout,
		Level:  hclog.Debug,
	})

	fmt.Printf("[DEBUG] Updated Raft configuration: %+v\n", raftConfig)

	// Bootstrap the cluster if needed
	if config.Bootstrap {
		fmt.Printf("[DEBUG] Checking for existing state\n")
		hasState, err := raft.HasExistingState(boltStore, boltStore, snapshotStore)
		if err != nil {
			return nil, fmt.Errorf("failed to check for existing state: %v", err)
		}
		fmt.Printf("[DEBUG] Has existing state: %v\n", hasState)

		if !hasState {
			fmt.Printf("[DEBUG] No existing state, bootstrapping cluster\n")
			configuration := raft.Configuration{
				Servers: []raft.Server{
					{
						Suffrage: raft.Voter,
						ID:       raft.ServerID(config.ID),
						Address:  raft.ServerAddress(config.Addr),
					},
				},
			}
			fmt.Printf("[DEBUG] Bootstrap configuration: %+v\n", configuration)
			if err := raft.BootstrapCluster(raftConfig, boltStore, boltStore, snapshotStore, transport, configuration); err != nil {
				if err != raft.ErrCantBootstrap {
					return nil, fmt.Errorf("failed to bootstrap cluster: %v", err)
				}
				fmt.Printf("[DEBUG] Cluster already bootstrapped\n")
			}
		}

		// 부트스트랩 후 잠시 대기
		time.Sleep(1000 * time.Millisecond)
	}

	fmt.Printf("[DEBUG] Creating new Raft instance\n")
	r, err := raft.NewRaft(raftConfig, fsm, boltStore, boltStore, snapshotStore, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to create raft: %v", err)
	}

	node := &RaftNode{raft: r, config: config}

	if config.Bootstrap {
		fmt.Printf("[DEBUG] Waiting for leader election in bootstrap mode\n")
		maxAttempts := 100
		for i := 0; i < maxAttempts; i++ {
			state := r.State()
			leader := r.Leader()
			fmt.Printf("[DEBUG] Attempt %d - State: %v, Leader: %v\n", i+1, state, leader)

			if state == raft.Leader {
				fmt.Printf("[DEBUG] Node became leader\n")
				return node, nil
			}

			if state == raft.Follower && leader != "" {
				fmt.Printf("[DEBUG] Found leader: %v\n", leader)
				return node, nil
			}

			time.Sleep(300 * time.Millisecond)
		}
		return nil, fmt.Errorf("failed to elect leader after %d attempts", maxAttempts)
	}

	return node, nil
}

// Start starts the Raft node
func (n *RaftNode) Start() error {
	// No need to bootstrap here anymore - it's done in NewRaftNode
	return nil
}

// Stop stops the Raft node
func (n *RaftNode) Stop() error {
	return n.raft.Shutdown().Error()
}

// IsLeader returns true if this node is the current leader
func (n *RaftNode) IsLeader() bool {
	return n.raft.State() == raft.Leader
}

// Leader returns the current leader's address
func (n *RaftNode) Leader() string {
	return string(n.raft.Leader())
}

// AddPeer adds a new peer to the cluster
func (n *RaftNode) AddPeer(peerID string, addr string) error {
	f := n.raft.AddVoter(raft.ServerID(peerID), raft.ServerAddress(addr), 0, 0)
	return f.Error()
}

// RemovePeer removes a peer from the cluster
func (n *RaftNode) RemovePeer(peerID string) error {
	f := n.raft.RemoveServer(raft.ServerID(peerID), 0, 0)
	return f.Error()
}

// Apply applies a command to the cluster
func (n *RaftNode) Apply(data []byte, timeout time.Duration) error {
	f := n.raft.Apply(data, timeout)
	return f.Error()
}

// GetConfiguration returns the current cluster configuration
func (n *RaftNode) GetConfiguration() (*Configuration, error) {
	future := n.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return nil, err
	}

	config := &Configuration{
		Servers: make([]Server, len(future.Configuration().Servers)),
	}

	for i, server := range future.Configuration().Servers {
		config.Servers[i] = Server{
			ID:       string(server.ID),
			Addr:     string(server.Address),
			Suffrage: server.Suffrage,
		}
	}

	return config, nil
}

// Shutdown gracefully shuts down the node
func (n *RaftNode) Shutdown() error {
	return n.raft.Shutdown().Error()
}

func (n *RaftNode) GetID() string {
	return n.config.ID
}

func (n *RaftNode) GetAddr() string {
	return n.config.Addr
}

func (n *RaftNode) GetState() raft.RaftState {
	return n.raft.State()
}
