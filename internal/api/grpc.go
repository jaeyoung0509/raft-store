package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	hashiraft "github.com/hashicorp/raft"
	pb "github.com/jaeyoung0509/go-store/internal/api/pb"
	"github.com/jaeyoung0509/go-store/internal/raft"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RaftServer implements the gRPC service interface for Raft operations
type RaftServer struct {
	pb.UnimplementedRaftServiceServer
	raftNode raft.Node
}

// NewRaftServer creates a new gRPC server instance
func NewRaftServer(node raft.Node) *RaftServer {
	return &RaftServer{
		raftNode: node,
	}
}

// GetStatus returns the current node status including leader state
func (s *RaftServer) GetStatus(ctx context.Context, _ *pb.Empty) (*pb.StatusResponse, error) {
	isLeader := s.raftNode.IsLeader()
	leader := s.raftNode.Leader()
	return &pb.StatusResponse{
		IsLeader: isLeader,
		LeaderId: leader,
	}, nil
}

// AddPeer adds a new node to the Raft cluster
func (s *RaftServer) AddPeer(ctx context.Context, req *pb.PeerRequest) (*pb.PeerResponse, error) {
	if err := s.raftNode.AddPeer(req.Id, req.Addr); err != nil {
		return nil, err
	}
	return &pb.PeerResponse{}, nil
}

// RemovePeer removes a node from the Raft cluster
func (s *RaftServer) RemovePeer(ctx context.Context, req *pb.PeerID) (*pb.PeerResponse, error) {
	err := s.raftNode.RemovePeer(req.Id)
	if err != nil {
		return nil, err
	}
	return &pb.PeerResponse{}, nil
}

// GetCluster returns the current cluster configuration
func (s *RaftServer) GetCluster(ctx context.Context, _ *pb.Empty) (*pb.ClusterConfig, error) {
	config, err := s.raftNode.GetConfiguration()
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(config.Servers))
	for _, srv := range config.Servers {
		ids = append(ids, srv.ID)
	}
	return &pb.ClusterConfig{
		ServerIds: ids,
	}, nil
}

// ApplyCommand applies a new command to the Raft cluster
func (s *RaftServer) ApplyCommand(ctx context.Context, req *pb.CommandRequest) (*pb.CommandResponse, error) {
	cmd := raft.Command{
		Type:    req.Type,
		Key:     req.Key,
		Value:   []byte(req.Value),
		Timeout: 5 * time.Second,
	}
	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}

	if err := s.raftNode.Apply(data, cmd.Timeout); err != nil {
		return nil, err
	}

	return &pb.CommandResponse{
		Result: "OK",
	}, nil
}

// GetValue returns the current value for a key using a linearizable read.
func (s *RaftServer) GetValue(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	if req.GetKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "key is required")
	}

	value, found, err := s.raftNode.Get(ctx, req.GetKey())
	if err != nil {
		if errors.Is(err, hashiraft.ErrNotLeader) {
			leader := s.raftNode.Leader()
			return nil, status.Error(codes.Unavailable, fmt.Sprintf("not leader (leader: %s)", leader))
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, status.FromContextError(err).Err()
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.GetResponse{
		Value: value,
		Found: found,
	}, nil
}
