package raft

import (
	"time"

	"github.com/hashicorp/raft"
)

// Node defines the interface for interacting with a Raft cluster node
type Node interface {
	Start() error
	Stop() error
	IsLeader() bool
	Leader() string
	AddPeer(peerID string, addr string) error
	RemovePeer(peerID string) error
	Apply(data []byte, timeout time.Duration) error
	GetConfiguration() (*Configuration, error)
	Shutdown() error
}

// Configuration represents the current cluster membership configuration
type Configuration struct {
	Servers []Server
}

// Server represents a single member in the Raft cluster
type Server struct {
	ID       string              // Unique identifier for the server
	Addr     string              // Network address of the server
	Suffrage raft.ServerSuffrage // Voting status of the server
}

// NodeConfig contains all configuration parameters for a Raft node
type NodeConfig struct {
	ID        string // Unique identifier for this node
	Addr      string // Network address for Raft communication
	DataDir   string // Directory for persistent storage
	Bootstrap bool   // Whether to bootstrap as a new cluster
	InMemory  bool   // Whether to use in-memory transport (for testing)
}

// NewNodeConfig creates a new configuration with default values
func NewNodeConfig(id, addr, dataAddr string) *NodeConfig {
	return &NodeConfig{
		ID:      id,
		Addr:    addr,
		DataDir: dataAddr,
	}
}
