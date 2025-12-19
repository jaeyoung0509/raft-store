package api

import (
	"context"
	"testing"

	hashiraft "github.com/hashicorp/raft"
	pb "github.com/jaeyoung0509/go-store/internal/api/pb"
	"github.com/jaeyoung0509/go-store/internal/raft"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRaftServerApplyCommandErrors(t *testing.T) {
	tests := []struct {
		name     string
		applyErr error
		wantCode codes.Code
	}{
		{
			name:     "unknown command",
			applyErr: raft.ErrUnknownCommand,
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "not leader",
			applyErr: hashiraft.ErrNotLeader,
			wantCode: codes.Unavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNode := &MockNode{
				isLeader: true,
				data:     map[string][]byte{},
				applyErr: tt.applyErr,
			}
			server := NewRaftServer(mockNode)

			_, err := server.ApplyCommand(context.Background(), &pb.CommandRequest{
				Type:  "SET",
				Key:   "key",
				Value: "value",
			})
			if err == nil {
				t.Fatalf("expected error")
			}
			if status.Code(err) != tt.wantCode {
				t.Fatalf("expected code %v, got %v", tt.wantCode, status.Code(err))
			}
		})
	}
}

func TestRaftServerGetValueNotLeader(t *testing.T) {
	mockNode := &MockNode{
		isLeader: true,
		data:     map[string][]byte{},
		getErr:   hashiraft.ErrNotLeader,
	}
	server := NewRaftServer(mockNode)

	_, err := server.GetValue(context.Background(), &pb.GetRequest{Key: "key"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected code %v, got %v", codes.Unavailable, status.Code(err))
	}
}
