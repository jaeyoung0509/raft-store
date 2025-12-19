package raft

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	logger "github.com/jaeyoung0509/go-store/pkg/log"
	"go.uber.org/zap"
)

// RaftNode represents a node in the Raft cluster
type RaftNode struct {
	raft   *raft.Raft
	config *NodeConfig
	fsm    *fsm
}

// NewRaftNode creates and initializes a new Raft node with the given configuration
func NewRaftNode(config *NodeConfig) (*RaftNode, error) {
	logger.L().Debug(" Starting NewRaftNode with config",
		zap.Any("config", config),
	)

	// Create and initialize Raft directory
	raftDir := filepath.Join(config.DataDir, config.ID)
	logger.L().Debug(" Creating raft directory", zap.String("directory", raftDir))
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
		logger.L().Debug(" Setting up InmemTransport\n")
		addr, memTransport := raft.NewInmemTransportWithTimeout(
			raft.ServerAddress(config.Addr),
			150*time.Microsecond, // 타임아웃 증가
		)
		logger.L().Debug(" Created InmemTransport with addr", zap.String("addr", string(addr)))
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

	logger.L().Debug(" Creating Raft configuration\n")
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

	logger.L().Debug(" Updated Raft configuration", zap.Any("config", raftConfig))

	// Bootstrap the cluster if needed
	if config.Bootstrap {
		logger.L().Debug(" Checking for existing state")
		hasState, err := raft.HasExistingState(boltStore, boltStore, snapshotStore)
		if err != nil {
			return nil, fmt.Errorf("failed to check for existing state: %v", err)
		}
		logger.L().Debug(" Has existing state", zap.Bool("state", hasState))

		if !hasState {
			logger.L().Debug(" No existing state, bootstrapping cluster")
			configuration := raft.Configuration{
				Servers: []raft.Server{
					{
						Suffrage: raft.Voter,
						ID:       raft.ServerID(config.ID),
						Address:  raft.ServerAddress(config.Addr),
					},
				},
			}
			logger.L().Debug(" Bootstrap configuration", zap.Any("configuration", configuration))
			if err := raft.BootstrapCluster(raftConfig, boltStore, boltStore, snapshotStore, transport, configuration); err != nil {
				if err != raft.ErrCantBootstrap {
					return nil, fmt.Errorf("failed to bootstrap cluster: %v", err)
				}
				logger.L().Debug(" Cluster already bootstrapped")
			}
		}

		// 부트스트랩 후 잠시 대기
		time.Sleep(1000 * time.Millisecond)
	}

	logger.L().Debug(" Creating new Raft instance")
	r, err := raft.NewRaft(raftConfig, fsm, boltStore, boltStore, snapshotStore, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to create raft: %v", err)
	}

	node := &RaftNode{raft: r, config: config, fsm: fsm}

	if config.Bootstrap {
		logger.L().Debug(" Waiting for leader election in bootstrap mode\n")
		maxAttempts := 100
		for i := 0; i < maxAttempts; i++ {
			state := r.State()
			leader := r.Leader()
			logger.L().Debug("attempts",
				zap.Int("Attempt", i+1),
				zap.Any("State", state),
				zap.Any("leader", leader),
			)

			if state == raft.Leader {
				logger.L().Debug(" Node became leader\n")
				return node, nil
			}

			if state == raft.Follower && leader != "" {
				logger.L().Debug("Found leader", zap.Any("leader", leader))
				return node, nil
			}

			time.Sleep(300 * time.Millisecond)
		}
		return nil, fmt.Errorf("failed to elect leader after %d attempts", maxAttempts)
	}

	return node, nil
}

// Start initializes and starts the Raft node
func (n *RaftNode) Start() error {
	// No need to bootstrap here anymore - it's done in NewRaftNode
	return nil
}

// Stop gracefully stops the Raft node
func (n *RaftNode) Stop() error {
	return n.raft.Shutdown().Error()
}

// IsLeader checks if this node is the current cluster leader
func (n *RaftNode) IsLeader() bool {
	return n.raft.State() == raft.Leader
}

// Leader returns the address of the current leader node
func (n *RaftNode) Leader() string {
	return string(n.raft.Leader())
}

// AddPeer adds a new voting member to the Raft cluster
func (n *RaftNode) AddPeer(peerID string, addr string) error {
	f := n.raft.AddVoter(raft.ServerID(peerID), raft.ServerAddress(addr), 0, 0)
	return f.Error()
}

// RemovePeer removes a member from the Raft cluster
func (n *RaftNode) RemovePeer(peerID string) error {
	f := n.raft.RemoveServer(raft.ServerID(peerID), 0, 0)
	return f.Error()
}

// Apply submits a new command to be applied to the cluster state machine
func (n *RaftNode) Apply(data []byte, timeout time.Duration) error {
	f := n.raft.Apply(data, timeout)
	if err := f.Error(); err != nil {
		return err
	}
	// FSM.Apply can return an error via Response even when the raft future succeeds.
	if resp := f.Response(); resp != nil {
		if err, ok := resp.(error); ok {
			return err
		}
	}
	return nil
}

// Get performs a linearizable read by waiting for a barrier before reading FSM state.
func (n *RaftNode) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if n.raft == nil || n.fsm == nil {
		return nil, false, fmt.Errorf("raft node not initialized")
	}
	if !n.IsLeader() {
		return nil, false, raft.ErrNotLeader
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}

	timeout := 5 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
		if timeout <= 0 {
			return nil, false, context.DeadlineExceeded
		}
	}

	// Barrier ensures all prior log entries are applied before the read.
	if err := n.raft.Barrier(timeout).Error(); err != nil {
		return nil, false, err
	}

	value, found := n.fsm.Get(key)
	return value, found, nil
}

// GetConfiguration retrieves the current cluster configuration
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

// Shutdown performs a graceful shutdown of the node
func (n *RaftNode) Shutdown() error {
	return n.raft.Shutdown().Error()
}

// GetID returns the node's unique identifier
func (n *RaftNode) GetID() string {
	return n.config.ID
}

// GetAddr returns the node's network address
func (n *RaftNode) GetAddr() string {
	return n.config.Addr
}

// GetState returns the current Raft state of the node
func (n *RaftNode) GetState() raft.RaftState {
	return n.raft.State()
}
