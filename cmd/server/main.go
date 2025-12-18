package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/jaeyoung0509/go-store/internal/api"
	"github.com/jaeyoung0509/go-store/internal/api/pb"
	"github.com/jaeyoung0509/go-store/internal/raft"
	logger "github.com/jaeyoung0509/go-store/pkg/log"
	"go.uber.org/zap"
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
	debug := os.Getenv("DEBUG") == "true"

	// Setup Logs
	logger.InitLogger(debug)
	defer func() {
		if err := logger.Sync(); err != nil {
			fmt.Fprintf(os.Stderr, "logger sync failed: %v\n", err)
		}
	}()

	logger.L().Info("Starting go-store",
		zap.String("nodeId", nodeID),
		zap.String("grpcAddr", grpcAddr),
		zap.String("raftAddr", raftAddr),
		zap.Bool("bootstrap", bootstrap),
	)

	if nodeID == "" || grpcAddr == "" {
		logger.L().Fatal("please provide both -id and -addr flags")
	}

	// Create and start Raft node
	config := raft.NewNodeConfig(nodeID, raftAddr, dataDir)
	config.Bootstrap = bootstrap // For now, assume we're bootstrapping

	node, err := raft.NewRaftNode(config)
	if err != nil {
		logger.L().Fatal("Failed to create Raft node",
			zap.Error(err),
		)
	}

	// Create and start API server
	server := api.NewServer(node, apiAddr)
	go func() {
		if err := server.Start(); err != nil {
			logger.L().Fatal("Failed to start API server",
				zap.Error(err))
		}
	}()

	// create grpc server
	go func() {
		ls, err := net.Listen("tcp", grpcAddr)
		if err != nil {
			logger.L().Fatal("Failed to listen on",
				zap.String("port", grpcAddr),
				zap.Error(err),
			)
		}
		grpcServer := grpc.NewServer()
		pb.RegisterRaftServiceServer(grpcServer, api.NewRaftServer(node))

		logger.L().Info("gRPC server listening on",
			zap.String("grpcAddr", grpcAddr))
		if err := grpcServer.Serve(ls); err != nil {
			logger.L().Fatal("Failed to serve grpc",
				zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Graceful shutdown
	if err := server.Stop(); err != nil {
		logger.L().Info("Error stopping API server",
			zap.Error(err),
		)
	}
	if err := node.Stop(); err != nil {
		logger.L().Info("Error stopping Raft node",
			zap.Error(err),
		)
	}
}
