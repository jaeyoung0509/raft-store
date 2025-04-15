package api

import (
	"context"
	"encoding/json"
	"time"

	pb "github.com/jaeyoung0509/go-store/internal/api/pb"
	"github.com/jaeyoung0509/go-store/internal/raft"
)

type RaftServer struct {
	pb.UnimplementedRaftServiceServer
	raftNode raft.Node
}

func NewRaftServer(node raft.Node) *RaftServer {
	return &RaftServer{
		raftNode: node,
	}
}

func (s *RaftServer) GetStatus(ctx context.Context, _ *pb.Empty) (*pb.StatusResponse, error) {
	isLeader := s.raftNode.IsLeader()
	leader := s.raftNode.Leader()
	return &pb.StatusResponse{
		IsLeader: isLeader,
		LeaderId: leader,
	}, nil
}

func (s *RaftServer) AddPeer(ctx context.Context, req *pb.PeerRequest) (*pb.PeerResponse, error) {
	if err := s.raftNode.AddPeer(req.Id, req.Addr); err != nil {
		return nil, err
	}
	return &pb.PeerResponse{}, nil
}
func (s *RaftServer) RemovePeer(ctx context.Context, req *pb.PeerID) (*pb.PeerResponse, error) {
	err := s.raftNode.RemovePeer(req.Id)
	if err != nil {
		return nil, err
	}
	return &pb.PeerResponse{}, nil
}

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
