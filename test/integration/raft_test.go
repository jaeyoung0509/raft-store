//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jaeyoung0509/go-store/internal/api/pb"
	"github.com/jaeyoung0509/go-store/test/util"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// connectToLeader attempts to find and connect to the current cluster leader
func connectToLeader(t *testing.T, nodes map[string]string) pb.RaftServiceClient {
	var leaderClient pb.RaftServiceClient

	for retries := 0; retries < 5; retries++ {
		foundLeader := false
		for nodeID, addr := range nodes {
			// Establish gRPC connection with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

			conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
			if err != nil {
				t.Logf("[E2E] Failed to connect to %s (%s): %v", nodeID, addr, err)
				cancel()
				continue
			}

			client := pb.NewRaftServiceClient(conn)
			statusCtx, statusCancel := context.WithTimeout(context.Background(), 2*time.Second)
			statusResp, err := client.GetStatus(statusCtx, &pb.Empty{})
			statusCancel()

			if err != nil {
				// Connection established but failed to retrieve status
				grpcStatus, ok := status.FromError(err)
				if ok && grpcStatus.Code() == codes.Unavailable {
					t.Logf("[E2E] Node %s (%s) unavailable, retrying status: %v", nodeID, addr, err)
				} else {
					t.Logf("[E2E] Failed to get status from %s (%s): %v", nodeID, addr, err)
				}
				conn.Close()
				cancel()
				continue
			}

			t.Logf("[E2E] Status from node %s at %s - IsLeader: %v, LeaderId: %s", nodeID, addr, statusResp.IsLeader, statusResp.LeaderId)

			if statusResp.IsLeader {
				// Found leader! Close previous leader connection if any
				if leaderClient != nil {
					t.Logf("[E2E] Found new leader %s, closing previous potential leader connection.", addr)
				}
				leaderClient = client
				foundLeader = true
				t.Logf("[E2E] Confirmed leader at %s (%s)", nodeID, addr)
				cancel()
				break
			}

			// Close connection if not leader
			conn.Close()
			cancel()
		}

		if foundLeader {
			// Exit retry loop if leader is found
			return leaderClient
		}

		// Wait before retrying
		t.Logf("[E2E] Retrying to connect to leader... (%d/5)", retries+1)
		time.Sleep(3 * time.Second)
	}

	t.Fatal("No leader found among cluster nodes after retries")
	return nil
}

func TestRaftE2E(t *testing.T) {
	// Initialize test cluster environment
	cluster, err := util.SetupTestCluster(t, util.ModeE2E)
	if err != nil {
		t.Fatalf("Failed to setup test cluster: %v", err)
	}
	defer cluster.Teardown(t)

	// Allow time for initial cluster stabilization
	t.Log("[TEST] Waiting for initial cluster stabilization...")
	time.Sleep(15 * time.Second)

	// Establish connection with cluster leader
	client := connectToLeader(t, cluster.Nodes)
	if client == nil {
		t.Fatalf("Initial leader connection failed.")
	}
	ctx := context.Background()

	initialLeaderStatus, err := client.GetStatus(ctx, &pb.Empty{})
	if err != nil || !initialLeaderStatus.IsLeader {
		t.Fatalf("Connected client is not the leader or failed to get status.")
	}
	t.Logf("Initial leader confirmed: %s", initialLeaderStatus.LeaderId)

	// Add nodes to the cluster except for node1
	nodesToAdd := []string{}
	for id := range cluster.Nodes {
		if id != "node1" {
			nodesToAdd = append(nodesToAdd, id)
		}
	}

	for _, id := range nodesToAdd {
		// Use internal Raft address for AddPeer
		internalRaftAddr := fmt.Sprintf("%s:12000", id)

		t.Logf("[E2E] Attempting to add peer %s with internal address %s", id, internalRaftAddr)

		// Verify leader before adding peer
		currentLeaderClient := connectToLeader(t, cluster.Nodes)
		if currentLeaderClient == nil {
			t.Fatalf("Lost leader before adding peer %s", id)
		}

		addPeerCtx, addPeerCancel := context.WithTimeout(ctx, 10*time.Second)
		_, err = currentLeaderClient.AddPeer(addPeerCtx, &pb.PeerRequest{
			Id:   id,
			Addr: internalRaftAddr,
		})
		addPeerCancel()

		if err != nil {
			t.Fatalf("[E2E] Failed to execute AddPeer command for %s (%s): %v", id, internalRaftAddr, err)
		}
		t.Logf("[E2E] AddPeer command sent successfully for %s (%s)", id, internalRaftAddr)

		// Wait for cluster stabilization after adding peer
		t.Logf("[E2E] Waiting for cluster stabilization after adding peer %s...", id)
		time.Sleep(10 * time.Second)

		// Test ApplyCommand after stabilization
		require.Eventually(t, func() bool {
			applyClient := connectToLeader(t, cluster.Nodes)
			if applyClient == nil {
				t.Logf("[E2E] Still searching for leader after adding peer %s and waiting...", id)
				return false
			}

			applyCmdCtx, applyCmdCancel := context.WithTimeout(ctx, 5*time.Second)
			_, err = applyClient.ApplyCommand(applyCmdCtx, &pb.CommandRequest{
				Type:  "SET",
				Key:   fmt.Sprintf("e2e-key-%s", id),
				Value: fmt.Sprintf("e2e-value-%s", id),
			})
			applyCmdCancel()

			if err != nil {
				grpcStatus, _ := status.FromError(err)
				t.Logf("[E2E] ApplyCommand failed after adding %s (Code: %s): %v", id, grpcStatus.Code(), err)
				return false
			}

			t.Logf("[E2E] Successfully applied command after adding peer %s", id)
			return true
		}, 30*time.Second, 5*time.Second, fmt.Sprintf("ApplyCommand did not succeed within timeout after adding peer %s", id))

		t.Logf("[E2E] Successfully processed peer %s and applied command.", id)
	}

	// Verify final cluster state
	t.Logf("[E2E] All peers processed. Verifying final cluster state...")
	require.Eventually(t, func() bool {
		finalClient := connectToLeader(t, cluster.Nodes)
		if finalClient == nil {
			t.Logf("[E2E] Searching for leader for final cluster check...")
			return false
		}

		getClusterCtx, getClusterCancel := context.WithTimeout(ctx, 5*time.Second)
		clusterResp, err := finalClient.GetCluster(getClusterCtx, &pb.Empty{})
		getClusterCancel()

		if err != nil {
			t.Logf("[E2E] GetCluster failed during final check: %v", err)
			return false
		}

		t.Logf("[E2E] Final cluster members: %v (Expected: %d nodes)", clusterResp.ServerIds, len(cluster.Nodes))
		return len(clusterResp.ServerIds) == len(cluster.Nodes)
	}, 45*time.Second, 5*time.Second, "Cluster did not reach expected size in final check")

	t.Logf("[E2E] Test completed successfully. Final cluster has %d members.", len(cluster.Nodes))
}
