package raft

import (
	"encoding/json"
	"io"
	"time"

	"github.com/hashicorp/raft"
	"github.com/vmihailenco/msgpack/v5"
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
	if err := msgpack.Unmarshal(log.Data, &cmd); err != nil {
		return err
	}

	switch cmd.Type {
	case "SET":
		f.data[cmd.Key] = cmd.Value
		return nil
	case "DELETE":
		delete(f.data, cmd.Key)
		return nil
	default:
		return ErrUnknownCommand
	}
}

// Snapshot returns a snapshot of the current state machine
func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	return &fsmSnapshot{data: f.data}, nil
}

// Restore restores the state machine from a snapshot
func (f *fsm) Restore(rc io.ReadCloser) error {
	f.data = make(map[string][]byte)
	return json.NewDecoder(rc).Decode(&f.data)
}

// fsmSnapshot implements the FSMSnapshot interface for state persistence
type fsmSnapshot struct {
	data map[string][]byte
}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	return json.NewEncoder(sink).Encode(f.data)
}

func (f *fsmSnapshot) Release() {}
