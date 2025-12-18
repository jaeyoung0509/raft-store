package raft

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	logger "github.com/jaeyoung0509/go-store/pkg/log"
)

func TestRaftNode(t *testing.T) {
	t.Log("[TEST] Starting RaftNode unit tests")
	tmpDir := t.TempDir()

	// Set logger
	logger.InitLogger(true)

	// Create test configuration
	config := NewNodeConfig("test1", "127.0.0.1:9001", tmpDir)
	config.Bootstrap = true
	config.InMemory = true
	t.Logf("[TEST] Created test configuration: %+v", config)

	// Initialize new Raft node
	t.Log("[TEST] Creating new test Raft node")
	node, err := NewRaftNode(config)
	if err != nil {
		t.Fatalf("[TEST] Failed to create node: %v", err)
	}
	defer func() {
		t.Log("[TEST] Performing test cleanup")
		if err := node.Shutdown(); err != nil {
			t.Fatalf("[TEST] Failed to shutdown node: %v", err)
		}
	}()

	// Verify initial cluster configuration
	confFuture := node.raft.GetConfiguration()
	if err := confFuture.Error(); err != nil {
		t.Fatalf("[TEST] Failed to get initial configuration: %v", err)
	}
	t.Logf("[TEST] Initial Raft configuration: %+v", confFuture.Configuration())

	// Wait for leader election
	t.Log("[TEST] Waiting for leader election")
	waitTime := 2 * time.Second
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(waitTime)
	leaderElected := false
	for {
		select {
		case <-timeout:
			t.Fatalf("[TEST] Leadership timeout after %v", waitTime)
		case <-ticker.C:
			if node.IsLeader() {
				t.Log("[TEST] Node became leader")
				leaderElected = true
			}
		}
		if leaderElected {
			break
		}
	}

	state := node.raft.State()
	leader := node.Leader()
	t.Logf("[TEST] Current state: %v, Leader: %v", state, leader)

	if !node.IsLeader() {
		t.Fatalf("[TEST] Node should be leader in single-node configuration, current state: %v", state)
	}
	t.Log("[TEST] Node is leader")

	t.Run("Apply Command", func(t *testing.T) {
		t.Log("[TEST] Testing command application")
		cmd := Command{
			Type:  "SET",
			Key:   "test-key",
			Value: []byte("test-value"),
		}
		t.Logf("[TEST] Created test command: %+v", cmd)

		data, err := json.Marshal(cmd)
		if err != nil {
			t.Fatalf("[TEST] Failed to marshal command: %v", err)
		}

		t.Log("[TEST] Applying command...")
		err = node.Apply(data, 500*time.Millisecond)
		if err != nil {
			t.Errorf("[TEST] Failed to apply command: %v", err)
		}
		t.Log("[TEST] Command applied successfully")

		value, found, err := node.Get(context.Background(), "test-key")
		if err != nil {
			t.Fatalf("[TEST] Failed to read value: %v", err)
		}
		if !found || string(value) != "test-value" {
			t.Fatalf("[TEST] Expected value %q, got %q (found=%v)", "test-value", string(value), found)
		}
	})
}
