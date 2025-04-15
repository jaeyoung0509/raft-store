package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jaeyoung0509/go-store/internal/raft"
)

// MockNode implements raft.Node interface for testing
type MockNode struct {
	isLeader bool
}

func (m *MockNode) Start() error                                   { return nil }
func (m *MockNode) Stop() error                                    { return nil }
func (m *MockNode) IsLeader() bool                                 { return m.isLeader }
func (m *MockNode) Leader() string                                 { return "node1" }
func (m *MockNode) AddPeer(id, addr string) error                  { return nil }
func (m *MockNode) RemovePeer(id string) error                     { return nil }
func (m *MockNode) Apply(data []byte, timeout time.Duration) error { return nil }
func (m *MockNode) GetConfiguration() (*raft.Configuration, error) {
	return &raft.Configuration{}, nil
}
func (m *MockNode) Shutdown() error { return nil }

func TestServer(t *testing.T) {
	mockNode := &MockNode{isLeader: true}
	server := NewServer(mockNode, ":8080")

	// Create a new ServeMux and register handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/key", server.handleKey)
	mux.HandleFunc("/cluster", server.handleCluster)

	tests := []struct {
		name       string
		method     string
		path       string
		body       interface{}
		wantStatus int
		wantError  bool
	}{
		{
			name:   "PUT /key - success",
			method: "PUT",
			path:   "/key",
			body: struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			}{
				Key:   "test-key",
				Value: "dGVzdC12YWx1ZQ==", // base64 encoded "test-value"
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "GET /key - success",
			method:     "GET",
			path:       "/key?key=test_key",
			wantStatus: http.StatusOK,
		},
		{
			name:       "GET /key - missing key",
			method:     "GET",
			path:       "/key",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "GET /cluster - success",
			method:     "GET",
			path:       "/cluster",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request

			if tt.body != nil {
				bodyBytes, err := json.Marshal(tt.body)
				if err != nil {
					t.Fatalf("Failed to marshal request body: %v", err)
				}
				req = httptest.NewRequest(tt.method, tt.path, bytes.NewBuffer(bodyBytes))
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}

			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			// Use the mux to handle the request
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("Expected status %v, got %v", tt.wantStatus, rec.Code)
				t.Errorf("Response body: %v", rec.Body.String())
			}
		})
	}
}
