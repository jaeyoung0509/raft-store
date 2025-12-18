//go:build integration
// +build integration

// Package util provides testing utilities for Raft cluster tests
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

// TestMode defines the type of test being run
type TestMode int

const (
	// ModeIntegration indicates integration test mode
	ModeIntegration TestMode = iota
	// ModeE2E indicates end-to-end test mode
	ModeE2E
)

// TestCluster represents a test Raft cluster environment
type TestCluster struct {
	ComposeStack compose.ComposeStack // Docker compose stack
	Nodes        map[string]string    // Node ID to address mapping
	Mode         TestMode             // Test execution mode
}

// SetupTestCluster initializes a test Raft cluster using Docker Compose
func SetupTestCluster(t *testing.T, mode TestMode) (*TestCluster, error) {
	t.Helper()
	// Setup data directory on host and ensure container data dir is deterministic.
	hostDataDir := "/tmp/raft-data"
	_ = os.Setenv("RAFT_DATA_DIR", "/data")
	t.Log("RAFT_DATA_DIR (host) =", hostDataDir)
	err := os.RemoveAll(hostDataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to delete data directory: %w", err)
	}
	if err := os.MkdirAll(hostDataDir, 0o777); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	if err := os.Chmod(hostDataDir, 0o777); err != nil {
		return nil, fmt.Errorf("failed to chmod data directory: %w", err)
	}

	services := []string{"node1", "node2", "node3"}
	for _, service := range services {
		nodeDir := filepath.Join(hostDataDir, service)
		if err := os.MkdirAll(nodeDir, 0o777); err != nil {
			return nil, fmt.Errorf("failed to create node directory %s: %w", nodeDir, err)
		}
		if err := os.Chmod(nodeDir, 0o777); err != nil {
			return nil, fmt.Errorf("failed to chmod node directory %s: %w", nodeDir, err)
		}
	}

	// Clean up existing containers
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithVersion("1.43"), // Docker API version explicitly set
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
		// Remove only test-related containers
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

		nodes[service] = nodeAddr
		t.Logf("[DEBUG] Added node %s with address %s", service, nodeAddr)
	}

	return &TestCluster{
		ComposeStack: stack,
		Nodes:        nodes,
		Mode:         mode,
	}, nil
}

// getContainerLogs retrieves logs from a test container
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

// Teardown cleans up the test cluster environment
func (c *TestCluster) Teardown(t *testing.T) {
	t.Helper()
	t.Log("[TEST] Tearing down the cluster")
	ctx := context.Background()
	err := c.ComposeStack.Down(ctx, compose.RemoveOrphans(true), compose.RemoveVolumes(true))
	if err != nil {
		t.Fatalf("Failed to teardown cluster: %v", err)
	}
}
