package util

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/compose"
)

type TestMode int

const (
	ModeIntegration TestMode = iota
	ModeE2E
)

type TestCluster struct {
	ComposeStack compose.ComposeStack
	Nodes        map[string]string
	Mode         TestMode
}

func SetupTestCluster(t *testing.T, mode TestMode) (*TestCluster, error) {
	t.Helper()
	os.Setenv("RAFT_DATA_DIR", "/tmp/raft-data")
	t.Log("RAFT_DATA_DIR =", os.Getenv("RAFT_DATA_DIR"))
	err := os.RemoveAll(os.Getenv("RAFT_DATA_DIR"))
	if err != nil {
		return nil, fmt.Errorf("failed to delete: %w", err)
	}

	// 먼저 기존 컨테이너들을 정리
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithVersion("1.43"), // Docker API 버전을 명시적으로 설정
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %v", err)
	}
	defer cli.Close()

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %v", err)
	}

	for _, container := range containers {
		// 테스트 관련 컨테이너만 정리
		if strings.Contains(container.Image, "go-store") {
			if err := cli.ContainerRemove(ctx, container.ID, types.ContainerRemoveOptions{
				Force: true,
			}); err != nil {
				t.Logf("[WARN] Failed to remove container %s: %v", container.ID, err)
			}
		}
	}

	composeFilePath := filepath.Join("..", "..", "docker-compose.yml")
	ctx = context.Background()

	// Create compose stack
	stack, err := compose.NewDockerCompose(composeFilePath)
	if err != nil {
		return nil, err
	}

	// Start the cluster
	t.Log("[TEST] Starting Raft cluster containers")
	if err := stack.Up(ctx, compose.Wait(true)); err != nil {
		return nil, fmt.Errorf("failed to start containers: %v", err)
	}

	// Wait for containers to be ready
	services := []string{"node1", "node2", "node3"}
	for _, service := range services {
		container, err := stack.ServiceContainer(ctx, service)
		if err != nil {
			return nil, fmt.Errorf("failed to get container for service %s: %v", service, err)
		}

		// Wait for container to be running
		statusCh := make(chan string)
		errCh := make(chan error)
		go func() {
			for {
				state, err := container.State(ctx)
				if err != nil {
					errCh <- err
					return
				}
				if state.Running {
					statusCh <- "running"
					return
				}
				if state.ExitCode != 0 {
					logs := getContainerLogs(ctx, container)
					errCh <- fmt.Errorf("container exited with code %d: %s", state.ExitCode, logs)
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
		}()

		select {
		case <-statusCh:
			t.Logf("[DEBUG] Container %s is running", service)
		case err := <-errCh:
			return nil, fmt.Errorf("container %s failed: %v", service, err)
		case <-time.After(30 * time.Second):
			logs := getContainerLogs(ctx, container)
			return nil, fmt.Errorf("timeout waiting for container %s: %s", service, logs)
		}
	}

	// Collect node information
	nodes := make(map[string]string)
	for _, service := range services {
		container, err := stack.ServiceContainer(ctx, service)
		if err != nil {
			t.Logf("[WARN] Failed to get container for service %s: %v", service, err)
			continue
		}
		var nodeAddr string

		grpcPort := "9090"
		mappedPort, err := container.MappedPort(ctx, nat.Port(grpcPort))
		if err != nil {
			t.Fatalf("[WARN] Failed to get mapped gRPC port for service %s: %v", service, err)
			continue
		}
		hostIp, err := container.Host(ctx)
		if err != nil {
			t.Fatalf("Failed to get host IP for container %s:%v", service, err)
		}
		nodeAddr = fmt.Sprintf("%s:%s", hostIp, mappedPort.Port())
		// if mode == ModeE2E {
		// 	// 컨테이너 내부 네트워크 내에서 접근 - 서비스 이름이 DNS로 동작함.
		// 	nodeAddr = fmt.Sprintf("%s:12000", service)
		// } else {
		// 	// Integration: 호스트에서 외부로 접근할 때, 매핑된 gRPC 포트를 사용
		// 	mappedPort, err := container.MappedPort(ctx, "9090")
		// 	if err != nil {
		// 		t.Fatalf("[WARN] Failed to get mapped port for service %s: %v", service, err)
		// 		continue
		// 	}
		// 	nodeAddr = fmt.Sprintf("localhost:%s", mappedPort.Port())
		// }

		nodes[service] = nodeAddr
		t.Logf("[DEBUG] Added node %s with address %s", service, nodeAddr)
	}

	return &TestCluster{
		ComposeStack: stack,
		Nodes:        nodes,
		Mode:         mode,
	}, nil
}

func getContainerLogs(ctx context.Context, container testcontainers.Container) string {
	logs, err := container.Logs(ctx)
	if err != nil {
		return fmt.Sprintf("failed to get logs: %v", err)
	}
	logBytes, err := io.ReadAll(logs)
	if err != nil {
		return fmt.Sprintf("failed to read logs: %v", err)
	}
	return string(logBytes)
}

func (c *TestCluster) Teardown(t *testing.T) {
	t.Helper()
	t.Log("[TEST] Tearing down the cluster")
	ctx := context.Background()
	err := c.ComposeStack.Down(ctx, compose.RemoveOrphans(true), compose.RemoveVolumes(true))
	if err != nil {
		t.Fatalf("Failed to teardown cluster: %v", err)
	}
}
