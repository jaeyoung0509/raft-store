package e2e

import (
	"context"
	"fmt" // fmt 패키지 import 확인
	"testing"
	"time"

	"github.com/jaeyoung0509/go-store/internal/api/pb"
	"github.com/jaeyoung0509/go-store/test/util"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes" // grpc/codes import 추가
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status" // grpc/status import 추가
)

// connectToLeader 함수는 그대로 유지 (호스트 gRPC 주소 사용)
func connectToLeader(t *testing.T, nodes map[string]string) pb.RaftServiceClient {
	var leaderClient pb.RaftServiceClient

	for retries := 0; retries < 5; retries++ {
		foundLeader := false
		for nodeID, addr := range nodes { // nodes 맵은 host:port 형태의 gRPC 주소를 가짐
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second) // 타임아웃 줄이기

			conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock()) // WithBlock 추가 고려
			if err != nil {
				t.Logf("[E2E] Failed to connect to %s (%s): %v", nodeID, addr, err)
				cancel()
				continue
			}

			client := pb.NewRaftServiceClient(conn)
			statusCtx, statusCancel := context.WithTimeout(context.Background(), 2*time.Second) // GetStatus 호출 타임아웃
			statusResp, err := client.GetStatus(statusCtx, &pb.Empty{})
			statusCancel() // GetStatus 컨텍스트 취소

			if err != nil {
				// 연결은 되었으나 상태 조회 실패 (노드가 아직 준비되지 않았거나, 일시적 문제)
				grpcStatus, ok := status.FromError(err)
				if ok && grpcStatus.Code() == codes.Unavailable {
					t.Logf("[E2E] Node %s (%s) unavailable, retrying status: %v", nodeID, addr, err)
				} else {
					t.Logf("[E2E] Failed to get status from %s (%s): %v", nodeID, addr, err)
				}
				conn.Close() // 연결 닫기
				cancel()
				continue
			}

			t.Logf("[E2E] Status from node %s at %s - IsLeader: %v, LeaderId: %s", nodeID, addr, statusResp.IsLeader, statusResp.LeaderId)

			if statusResp.IsLeader {
				// 리더 찾음! 기존 리더 연결은 닫고 새 연결 사용
				if leaderClient != nil {
					// 이전 루프에서 찾았던 임시 리더 연결일 수 있으므로 닫음
					// conn 변수는 현재 루프의 것이므로 직접 접근 불가, leaderClient의 연결 정보를 닫아야 함 (이 부분은 복잡하므로 일단 로깅만)
					t.Logf("[E2E] Found new leader %s, closing previous potential leader connection.", addr)
				}
				leaderClient = client
				foundLeader = true
				t.Logf("[E2E] Confirmed leader at %s (%s)", nodeID, addr)
				cancel() // 현재 연결 시도 컨텍스트 취소
				break    // 현재 리더 찾았으므로 노드 순회 중단
			}

			// 리더가 아니면 연결 닫기
			conn.Close()
			cancel() // 현재 연결 시도 컨텍스트 취소
		} // end node loop

		if foundLeader {
			// 최종 리더를 찾았으면 재시도 루프 종료
			return leaderClient
		}

		// 리더 못 찾았으면 잠시 대기 후 재시도
		t.Logf("[E2E] Retrying to connect to leader... (%d/5)", retries+1)
		time.Sleep(3 * time.Second) // 재시도 간격 늘리기

	} // end retry loop

	t.Fatal("No leader found among cluster nodes after retries")
	return nil // 리더 못 찾음
}

func TestRaftE2E(t *testing.T) {
	cluster, err := util.SetupTestCluster(t, util.ModeE2E) // cluster.Nodes는 localhost:3400X 형태의 주소를 가짐
	if err != nil {
		t.Fatalf("Failed to setup test cluster: %v", err)
	}
	defer cluster.Teardown(t) // 테스트 종료 시 정리하도록 defer 추가

	t.Log("[TEST] Waiting for initial cluster stabilization...")
	time.Sleep(15 * time.Second) // 초기 안정화 시간 증가

	// Connect to the leader using host-mapped gRPC address
	client := connectToLeader(t, cluster.Nodes)
	if client == nil {
		// connectToLeader에서 Fatal로 종료되지만, 방어적으로 체크
		t.Fatalf("Initial leader connection failed.")
	}
	ctx := context.Background()

	initialLeaderStatus, err := client.GetStatus(ctx, &pb.Empty{})
	if err != nil || !initialLeaderStatus.IsLeader {
		t.Fatalf("Connected client is not the leader or failed to get status.")
	}
	t.Logf("Initial leader confirmed: %s", initialLeaderStatus.LeaderId)

	// node1을 제외한 노드들을 클러스터에 추가
	nodesToAdd := []string{}
	for id := range cluster.Nodes {
		// docker-compose.yml 에서 bootstrap=true 인 노드를 식별하는 더 좋은 방법이 필요할 수 있음
		// 여기서는 간단히 node1을 제외한다고 가정
		if id != "node1" {
			nodesToAdd = append(nodesToAdd, id)
		}
	}

	for _, id := range nodesToAdd {
		// *** 중요 변경: AddPeer 호출 시 내부 Raft 주소 사용 ***
		internalRaftAddr := fmt.Sprintf("%s:12000", id) // 예: "node2:12000"

		t.Logf("[E2E] Attempting to add peer %s with internal address %s", id, internalRaftAddr)

		// AddPeer 호출 전에 리더가 여전히 유효한지 확인 (선택적)
		currentLeaderClient := connectToLeader(t, cluster.Nodes)
		if currentLeaderClient == nil {
			t.Fatalf("Lost leader before adding peer %s", id)
		}

		addPeerCtx, addPeerCancel := context.WithTimeout(ctx, 10*time.Second) // AddPeer 타임아웃 설정
		_, err = currentLeaderClient.AddPeer(addPeerCtx, &pb.PeerRequest{
			Id:   id,               // 예: "node2"
			Addr: internalRaftAddr, // 예: "node2:12000"
		})
		addPeerCancel() // AddPeer 컨텍스트 취소

		if err != nil {
			// AddPeer 호출 자체가 실패한 경우 (네트워크 문제, 리더 다운 등)
			t.Fatalf("[E2E] Failed to execute AddPeer command for %s (%s): %v", id, internalRaftAddr, err)
		}
		t.Logf("[E2E] AddPeer command sent successfully for %s (%s)", id, internalRaftAddr)

		// 피어 추가 후 클러스터 안정화 및 리더 재확인 시간 부여
		t.Logf("[E2E] Waiting for cluster stabilization after adding peer %s...", id)
		time.Sleep(10 * time.Second) // 안정화 시간 충분히 주기

		// 안정화 후 ApplyCommand 테스트
		require.Eventually(t, func() bool {
			// 매번 리더를 다시 찾아서 ApplyCommand 실행
			applyClient := connectToLeader(t, cluster.Nodes)
			if applyClient == nil {
				t.Logf("[E2E] Still searching for leader after adding peer %s and waiting...", id)
				return false // 리더 없음, 재시도
			}

			applyCmdCtx, applyCmdCancel := context.WithTimeout(ctx, 5*time.Second) // ApplyCommand 타임아웃
			_, err = applyClient.ApplyCommand(applyCmdCtx, &pb.CommandRequest{
				Type:  "SET",
				Key:   fmt.Sprintf("e2e-key-%s", id), // 노드별 다른 키 사용
				Value: fmt.Sprintf("e2e-value-%s", id),
			})
			applyCmdCancel() // ApplyCommand 컨텍스트 취소

			if err != nil {
				grpcStatus, _ := status.FromError(err)
				// 리더가 아니거나 (NotLeader), 불안정하여 처리 못하는 경우 (Unavailable) 등 로그 남기기
				t.Logf("[E2E] ApplyCommand failed after adding %s (Code: %s): %v", id, grpcStatus.Code(), err)
				return false // Apply 실패, 재시도
			}

			t.Logf("[E2E] Successfully applied command after adding peer %s", id)
			return true // Apply 성공
		}, 30*time.Second, 5*time.Second, fmt.Sprintf("ApplyCommand did not succeed within timeout after adding peer %s", id)) // 타임아웃 및 간격 늘리기

		t.Logf("[E2E] Successfully processed peer %s and applied command.", id)
	} // end loop for adding peers

	// 모든 피어 추가 후 최종 클러스터 상태 확인
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
		// 단순히 개수만 비교하는 것보다, 실제 노드 ID들이 모두 포함되었는지 확인하는 것이 더 정확할 수 있습니다.
		// memberSet := make(map[string]bool)
		// for _, serverId := range clusterResp.ServerIds {
		//  memberSet[serverId] = true
		// }
		// return len(memberSet) == len(cluster.Nodes)
		return len(clusterResp.ServerIds) == len(cluster.Nodes) // 현재는 개수만 비교

	}, 45*time.Second, 5*time.Second, "Cluster did not reach expected size in final check") // 최종 확인 타임아웃 늘리기

	t.Logf("[E2E] Test completed successfully. Final cluster has %d members.", len(cluster.Nodes))
}
