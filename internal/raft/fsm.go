package raft

import (
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/hashicorp/raft"
)

// Command represents an operation to be replicated through the Raft cluster
type Command struct {
	Type    string        `json:"type"`    // Operation type (SET, DELETE, etc.)
	Key     string        `json:"key"`     // Key to operate on
	Value   []byte        `json:"value"`   // Value for SET operations
	Timeout time.Duration `json:"timeout"` // Operation timeout
}

// fsm implements the raft.FSM interface for state machine replication
type fsm struct {
	mu   sync.RWMutex
	data map[string][]byte
}

// newFSM creates a new finite state machine instance
func newFSM() *fsm {
	return &fsm{
		data: make(map[string][]byte),
	}
}

// Apply handles incoming log entries and applies them to the state machine
func (f *fsm) Apply(log *raft.Log) interface{} {
	var cmd Command
	if err := json.Unmarshal(log.Data, &cmd); err != nil {
		return err
	}

	switch cmd.Type {
	case "SET":
		f.mu.Lock()
		f.data[cmd.Key] = append([]byte(nil), cmd.Value...)
		f.mu.Unlock()
		return nil
	case "DELETE":
		f.mu.Lock()
		delete(f.data, cmd.Key)
		f.mu.Unlock()
		return nil
	default:
		return ErrUnknownCommand
	}
}

// Snapshot returns a snapshot of the current state machine
func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	// Copy the map and values to avoid sharing mutable state with concurrent writers.
	snapshot := make(map[string][]byte, len(f.data))
	for key, value := range f.data {
		snapshot[key] = append([]byte(nil), value...)
	}
	f.mu.RUnlock()

	return &fsmSnapshot{data: snapshot}, nil
}

// Restore restores the state machine from a snapshot
func (f *fsm) Restore(rc io.ReadCloser) error {
	defer rc.Close()

	data := make(map[string][]byte)
	if err := json.NewDecoder(rc).Decode(&data); err != nil {
		return err
	}

	f.mu.Lock()
	f.data = data
	f.mu.Unlock()
	return nil
}

// fsmSnapshot implements the FSMSnapshot interface for state persistence
type fsmSnapshot struct {
	data map[string][]byte
}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	if err := json.NewEncoder(sink).Encode(f.data); err != nil {
		_ = sink.Cancel()
		return err
	}
	if err := sink.Close(); err != nil {
		_ = sink.Cancel()
		return err
	}
	return nil
}

func (f *fsmSnapshot) Release() {}

// Get returns a copy of the value for a key.
func (f *fsm) Get(key string) ([]byte, bool) {
	f.mu.RLock()
	value, ok := f.data[key]
	f.mu.RUnlock()
	if !ok {
		return nil, false
	}
	return append([]byte(nil), value...), true
}
