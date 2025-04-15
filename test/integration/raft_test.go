package integration

// import (
// 	"encoding/json"
// 	"fmt"
// 	"testing"
// 	"time"

// 	"github.com/jaeyoung0509/go-store/internal/raft"
// 	"github.com/jaeyoung0509/go-store/test/util"
// 	"github.com/stretchr/testify/require"
// )

// func waitForPeers(t *testing.T, node *raft.RaftNode, expected int) {
// 	require.Eventually(t, func() bool {
// 		config, err := node.GetConfiguration()
// 		if err != nil {
// 			t.Logf("[DEBUG] Error getting configuration: %v", err)
// 			return false
// 		}
// 		t.Logf("[DEBUG] Current configuration: %+v", config)
// 		return len(config.Servers) == expected
// 	}, 10*time.Second, 500*time.Millisecond, "Failed to wait for peers")

// }

// func TestRaftIntegration(t *testing.T) {
// 	cluster, err := util.SetupTestCluster(t, util.ModeIntegration)
// 	if err != nil {
// 		t.Fatalf("Failed to setup test cluster: %v", err)
// 	}
// 	defer cluster.Teardown(t)

// 	// 리더 노드 생성 및 시작
// 	leaderConfig := &raft.NodeConfig{
// 		ID:        "node1",
// 		Addr:      cluster.Nodes["node1"],
// 		DataDir:   "/tmp/raft-data",
// 		Bootstrap: true,
// 	}
// 	leader, err := raft.NewRaftNode(leaderConfig)
// 	if err != nil {
// 		t.Fatalf("Failed to create leader node: %v", err)
// 	}
// 	defer leader.Shutdown()

// 	// 리더가 안정화될 때까지 대기
// 	t.Log("Waiting for leader stabilization...")
// 	time.Sleep(5 * time.Second)

// 	// 리더 상태 확인
// 	if !leader.IsLeader() {
// 		t.Fatalf("Expected node1 to be leader")
// 	}

// 	// 피어 노드들 생성 및 시작
// 	peers := make(map[string]*raft.RaftNode)
// 	for i := 2; i <= 3; i++ {
// 		nodeID := fmt.Sprintf("node%d", i)
// 		peerConfig := &raft.NodeConfig{
// 			ID:        nodeID,
// 			Addr:      cluster.Nodes[nodeID],
// 			DataDir:   "/tmp/raft-data",
// 			Bootstrap: false,
// 		}
// 		peer, err := raft.NewRaftNode(peerConfig)
// 		if err != nil {
// 			t.Fatalf("Failed to create peer node %s: %v", nodeID, err)
// 		}
// 		defer peer.Shutdown()
// 		peers[nodeID] = peer

// 		// 피어 추가
// 		t.Logf("Adding peer %s to cluster...", nodeID)
// 		if err := leader.AddPeer(nodeID, cluster.Nodes[nodeID]); err != nil {
// 			t.Fatalf("Failed to add peer %s: %v", nodeID, err)
// 		}

// 		// 피어가 클러스터에 조인될 때까지 대기
// 		t.Logf("Waiting for peer %s to join cluster...", nodeID)
// 		time.Sleep(5 * time.Second)
// 	}

// 	// 클러스터 구성 확인
// 	config, err := leader.GetConfiguration()
// 	if err != nil {
// 		t.Fatalf("Failed to get cluster configuration: %v", err)
// 	}
// 	if len(config.Servers) != 3 {
// 		t.Fatalf("Expected 3 servers in cluster, got %d", len(config.Servers))
// 	}

// 	// Wait for log replication to complete
// 	for _, peer := range peers {
// 		waitForPeers(t, peer, 3)
// 	}

// 	// 리더 선출 확인
// 	t.Log("[TEST] Confirming leader election")
// 	require.Eventually(t, func() bool {
// 		for _, peer := range peers {
// 			if peer.IsLeader() {
// 				t.Logf("[DEBUG] Node %s state: %s, leader: %s", peer.GetID(), peer.GetState().String(), peer.Leader())
// 				return true
// 			}
// 		}
// 		return false
// 	}, 10*time.Second, 500*time.Millisecond, "Failed to elect new leader")

// 	// Test command application
// 	t.Run("Apply Command to Cluster", func(t *testing.T) {
// 		cmd := raft.Command{
// 			Type:  "SET",
// 			Key:   "test-key",
// 			Value: []byte("test-value"),
// 		}
// 		data, err := json.Marshal(cmd)
// 		require.NoError(t, err)

// 		t.Log("[TEST] Applying command to cluster")
// 		err = leader.Apply(data, 5*time.Second)
// 		require.NoError(t, err)
// 		t.Log("[TEST] Command applied successfully")
// 	})

// 	// Test leader failover
// 	t.Run("Leader Failover", func(t *testing.T) {
// 		oldLeaderID := leader.GetID()
// 		t.Log("[TEST] Shutting down current leader:", oldLeaderID)
// 		require.NoError(t, leader.Shutdown())

// 		// Wait for new leader election
// 		t.Log("[TEST] Waiting for new leader election")
// 		require.Eventually(t, func() bool {
// 			for id, peer := range peers {
// 				if id != oldLeaderID && peer.IsLeader() {
// 					t.Logf("[DEBUG] Node %s state: %s, leader: %s", peer.GetID(), peer.GetState().String(), peer.Leader())
// 					return true
// 				}
// 			}
// 			return false
// 		}, 10*time.Second, 500*time.Millisecond, "Failed to elect new leader")
// 	})
// }
