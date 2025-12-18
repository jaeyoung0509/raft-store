package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/hashicorp/raft"
	raftstore "github.com/jaeyoung0509/go-store/internal/raft"
)

// Server implements the HTTP API interface for Raft operations
type Server struct {
	raftNode raftstore.Node
	addr     string
}

// NewServer creates a new HTTP API server instance
func NewServer(node raftstore.Node, addr string) *Server {
	return &Server{
		raftNode: node,
		addr:     addr,
	}
}

// Start initializes and starts the HTTP server
func (s *Server) Start() error {
	http.HandleFunc("/key", s.handleKey)
	http.HandleFunc("/cluster", s.handleCluster)
	return http.ListenAndServe(s.addr, nil)
}

// Stop gracefully shuts down the HTTP server
func (s *Server) Stop() error {
	return nil // TODO: Implement graceful shutdown
}

// handleKey handles all key-value operations (GET/PUT/DELETE)
func (s *Server) handleKey(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGet(w, r)
	case http.MethodPut:
		s.handlePut(w, r)
	case http.MethodDelete:
		s.handleDelete(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGet processes GET requests for key retrieval
func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}

	if !s.raftNode.IsLeader() {
		s.writeNotLeader(w)
		return
	}

	value, found, err := s.raftNode.Get(r.Context(), key)
	if err != nil {
		if errors.Is(err, raft.ErrNotLeader) {
			s.writeNotLeader(w)
			return
		}
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, err.Error(), http.StatusGatewayTimeout)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := getResponse{
		Key:      key,
		Found:    found,
		Encoding: "base64",
	}
	if found {
		resp.Value = base64.StdEncoding.EncodeToString(value)
		s.writeJSON(w, http.StatusOK, resp)
		return
	}
	s.writeJSON(w, http.StatusNotFound, resp)
}

// handlePut processes PUT requests for key-value updates
func (s *Server) handlePut(w http.ResponseWriter, r *http.Request) {
	var cmd raftstore.Command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cmd.Type = "SET"
	cmd.Timeout = 5 * time.Second

	data, err := json.Marshal(cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.raftNode.Apply(data, 5*time.Second); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handleDelete processes DELETE requests for key removal
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}

	cmd := raftstore.Command{
		Type:    "DELETE",
		Key:     key,
		Timeout: 5 * time.Second,
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.raftNode.Apply(data, 5*time.Second); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handleCluster handles all cluster management operations
func (s *Server) handleCluster(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetCluster(w, r)
	case http.MethodPost:
		s.handleAddPeer(w, r)
	case http.MethodDelete:
		s.handleRemovePeer(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetCluster retrieves current cluster configuration
func (s *Server) handleGetCluster(w http.ResponseWriter, _ *http.Request) {
	config, err := s.raftNode.GetConfiguration()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.writeJSON(w, http.StatusOK, config)
}

type getResponse struct {
	Key      string `json:"key"`
	Value    string `json:"value,omitempty"`
	Found    bool   `json:"found"`
	Encoding string `json:"encoding,omitempty"`
}

type errorResponse struct {
	Error  string `json:"error"`
	Leader string `json:"leader,omitempty"`
}

func (s *Server) writeNotLeader(w http.ResponseWriter) {
	s.writeJSON(w, http.StatusServiceUnavailable, errorResponse{
		Error:  "not leader",
		Leader: s.raftNode.Leader(),
	})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}

// handleAddPeer processes requests to add new cluster members
func (s *Server) handleAddPeer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID   string `json:"id"`
		Addr string `json:"addr"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.raftNode.AddPeer(req.ID, req.Addr); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handleRemovePeer processes requests to remove cluster members
func (s *Server) handleRemovePeer(w http.ResponseWriter, r *http.Request) {
	peerID := r.URL.Query().Get("id")
	if peerID == "" {
		http.Error(w, "Peer ID is required", http.StatusBadRequest)
		return
	}

	if err := s.raftNode.RemovePeer(peerID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
