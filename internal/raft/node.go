package raft

import (
	"time"

	"github.com/hashicorp/raft"
)

// Node represents a Raft node interface
type Node interface {
	// Start starts the Raft node
	Start() error

	// Stop stops the Raft node
	Stop() error

	// IsLeader returns true if this node is the current leader
	IsLeader() bool

	// Leader returns the current leader's address
	Leader() string

	// AddPeer adds a new peer to the cluster
	AddPeer(peerID string, addr string) error

	// RemovePeer removes a peer from the cluster
	RemovePeer(peerID string) error

	// Apply applies a command to the cluster
	Apply(data []byte, timeout time.Duration) error

	// GetConfiguration returns the current cluster configuration
	GetConfiguration() (*Configuration, error)

	// Shutdown gracefully shuts down the node
	Shutdown() error
}

// Configuration represents the cluster configuration
type Configuration struct {
	Servers []Server
}

// Server represents a server in the cluster
type Server struct {
	ID       string
	Addr     string
	Suffrage raft.ServerSuffrage
}

// NodeConfig holds the configuration for a Raft node
type NodeConfig struct {
	ID        string
	Addr      string
	DataDir   string
	Bootstrap bool
	InMemory  bool // 인메모리 전송 계층 사용 여부
}

// NewNodeConfig creates a new node configuration with default values
func NewNodeConfig(id, addr, dataAddr string) *NodeConfig {
	return &NodeConfig{
		ID:      id,
		Addr:    addr,
		DataDir: dataAddr,
	}
}
