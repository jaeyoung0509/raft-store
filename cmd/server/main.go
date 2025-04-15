package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/jaeyoung0509/go-store/internal/api"
	"github.com/jaeyoung0509/go-store/internal/api/pb"
	"github.com/jaeyoung0509/go-store/internal/raft"
	"google.golang.org/grpc"
)

func main() {
	// Parse command line flags
	nodeID := os.Getenv("NODE_ID")
	raftAddr := os.Getenv("RAFT_ADDR")
	apiAddr := os.Getenv("API_ADDR")
	grpcAddr := os.Getenv("GRPC_ADDR")
	dataDir := os.Getenv("RAFT_DATA_DIR")
	bootstrap := os.Getenv("BOOTSTRAP") == "true"

	if nodeID == "" || grpcAddr == "" {
		log.Fatal("Please provide both -id and -addr flags")
	}
	fmt.Printf("addr config, nodeID:%s, grpcAddr:%s, raftAddr:%s, bootStrap:%t", nodeID, grpcAddr, raftAddr, bootstrap)

	// Create and start Raft node
	config := raft.NewNodeConfig(nodeID, raftAddr, dataDir)
	config.Bootstrap = bootstrap // For now, assume we're bootstrapping

	node, err := raft.NewRaftNode(config)
	if err != nil {
		log.Fatalf("Failed to create Raft node: %v", err)
	}

	// Create and start API server
	server := api.NewServer(node, apiAddr)
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("Failed to start API server: %v", err)
		}
	}()

	// create grpc server
	go func() {
		ls, err := net.Listen("tcp", grpcAddr)
		if err != nil {
			log.Fatalf("Failed to listen on %s: %v", grpcAddr, err)
		}
		grpcServer := grpc.NewServer()
		pb.RegisterRaftServiceServer(grpcServer, api.NewRaftServer(node))

		log.Printf("gRPC server listening on %s", grpcAddr)
		if err := grpcServer.Serve(ls); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Graceful shutdown
	if err := server.Stop(); err != nil {
		log.Printf("Error stopping API server: %v", err)
	}
	if err := node.Stop(); err != nil {
		log.Printf("Error stopping Raft node: %v", err)
	}
}
