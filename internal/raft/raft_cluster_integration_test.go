package raft

// package raft

// import (
// 	"context"
// 	"path/filepath"
// 	"testing"

// 	"google.golang.org/grpc/credentials/insecure"

// 	"github.com/jaeyoung0509/go-store/internal/api/pb"
// 	"github.com/stretchr/testify/require"
// 	"github.com/testcontainers/testcontainers-go/modules/compose"
// 	"google.golang.org/grpc"
// )

// type ClusterNode struct {
// 	ID       string
// 	GRPCAddr string
// 	Client   pb.RaftServiceClient
// 	Coon     *grpc.ClientConn
// }

// type RaftCluster struct {
// 	t       *testing.T
// 	Compose compose.ComposeStack
// 	Nodes   map[string]*ClusterNode
// }

// func StartTestCluster(t *testing.T) *RaftCluster {
// 	t.Helper()

// 	composeFilePath := filepath.Join("..", "..", "docker-compose.yml")
// 	ctx := context.Background()
// 	stack, err := compose.NewDockerCompose(composeFilePath)
// 	require.NoError(t, err)
// 	err = stack.Up(context.Background(), compose.Wait(true))
// 	require.NoError(t, err)

// 	// Static mapping for example purposes
// 	nodePorts := map[string]string{
// 		"nnode1": "localhost:9001",
// 		"nnode2": "localhost:9002",
// 		"nnode3": "localhost:9003",
// 	}

// 	nodes := make(map[string]*ClusterNode)
// 	for id, addr := range nodePorts {
// 		conn, err := grpc.DialContext(ctx, addr, grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()))
// 		require.NoError(t, err)
// 		client := pb.NewRaftServiceClient(conn)
// 		nodes[id] = &ClusterNode{
// 			ID:       id,
// 			GRPCAddr: addr,
// 			Client:   client,
// 		}
// 	}

// 	return &RaftCluster{
// 		t:       t,
// 		Compose: stack,
// 		Nodes:   nodes,
// 	}
// }

// func (c *RaftCluster) ShutDown() {
// 	for _, node := range c.Nodes {
// 		node.Coon.Close()
// 	}
// 	c.Compose.Down(context.Background(), compose.RemoveOrphans(true))
// }

// func (c *RaftCluster) StopNode(id string) {
// 	err := c.Compose.ServiceStop(context.Background(), id)
// }
