package api

import (
	"bytes"
	"context"
	"encoding/base64"
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
	data     map[string][]byte
	applyErr error
	getErr   error
}

func (m *MockNode) Start() error                  { return nil }
func (m *MockNode) Stop() error                   { return nil }
func (m *MockNode) IsLeader() bool                { return m.isLeader }
func (m *MockNode) Leader() string                { return "node1" }
func (m *MockNode) AddPeer(id, addr string) error { return nil }
func (m *MockNode) RemovePeer(id string) error    { return nil }
func (m *MockNode) Apply(data []byte, timeout time.Duration) error {
	if m.applyErr != nil {
		return m.applyErr
	}
	var cmd raft.Command
	if err := json.Unmarshal(data, &cmd); err != nil {
		return err
	}
	switch cmd.Type {
	case "SET":
		m.data[cmd.Key] = cmd.Value
	case "DELETE":
		delete(m.data, cmd.Key)
	}
	return nil
}
func (m *MockNode) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if m.getErr != nil {
		return nil, false, m.getErr
	}
	value, ok := m.data[key]
	if !ok {
		return nil, false, nil
	}
	return append([]byte(nil), value...), true, nil
}
func (m *MockNode) GetConfiguration() (*raft.Configuration, error) {
	return &raft.Configuration{}, nil
}
func (m *MockNode) Shutdown() error { return nil }

func TestServer(t *testing.T) {
	mockNode := &MockNode{
		isLeader: true,
		data: map[string][]byte{
			"test_key": []byte("test-value"),
		},
	}
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
		assertBody func(t *testing.T, rec *httptest.ResponseRecorder)
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
			assertBody: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp struct {
					Key      string `json:"key"`
					Value    string `json:"value"`
					Found    bool   `json:"found"`
					Encoding string `json:"encoding"`
				}
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if !resp.Found || resp.Encoding != "base64" || resp.Key != "test_key" {
					t.Fatalf("Unexpected response: %+v", resp)
				}
				value, err := base64.StdEncoding.DecodeString(resp.Value)
				if err != nil {
					t.Fatalf("Failed to decode value: %v", err)
				}
				if string(value) != "test-value" {
					t.Fatalf("Expected value %q, got %q", "test-value", string(value))
				}
			},
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
		{
			name:       "GET /key - not leader",
			method:     "GET",
			path:       "/key?key=test_key",
			wantStatus: http.StatusServiceUnavailable,
			assertBody: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp struct {
					Error  string `json:"error"`
					Leader string `json:"leader"`
				}
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if resp.Error != "not leader" || resp.Leader == "" {
					t.Fatalf("Unexpected response: %+v", resp)
				}
			},
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

			if tt.name == "GET /key - not leader" {
				mockNode.isLeader = false
			} else {
				mockNode.isLeader = true
			}

			// Use the mux to handle the request
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("Expected status %v, got %v", tt.wantStatus, rec.Code)
				t.Errorf("Response body: %v", rec.Body.String())
			}
			if tt.assertBody != nil {
				tt.assertBody(t, rec)
			}
		})
	}
}
