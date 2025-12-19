package raft

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hashicorp/raft"
)

func TestFSM(t *testing.T) {
	t.Log("[TEST] Starting FSM test")
	f := newFSM()
	cases := []struct {
		cmd  Command
		desc string
	}{
		{
			cmd: Command{
				Type:  "SET",
				Key:   "key1",
				Value: []byte("value1"),
			},
			desc: "Set command",
		},
		{
			cmd: Command{
				Type:  "DELETE",
				Key:   "key1",
				Value: nil,
			},
		},
	}
	for _, c := range cases {
		t.Logf("[TEST] Running case: %s", c.desc)
		data, err := json.Marshal(c.cmd)
		if err != nil {
			t.Fatalf("[TEST] Failed to marshal command: %v", err)
		}
		log := &raft.Log{Data: data}
		if err := f.Apply(log); err != nil {
			t.Fatalf("[TEST] Apply failed: %v", err)
		}
		switch c.cmd.Type {
		case "SET":
			if string(f.data[c.cmd.Key]) != string(c.cmd.Value) {
				t.Fatalf("[TEST] Expected value '%s', got '%s'", c.cmd.Value, f.data[c.cmd.Key])
			}
		case "DELETE":
			if _, exists := f.data[c.cmd.Key]; exists {
				t.Fatalf("[TEST] Expected key '%s' to be deleted, but it still exists", c.cmd.Key)
			}
		default:
			t.Fatalf("[TEST] Unexpected command type: %s", c.cmd.Type)
		}
	}
}

type mockSnapshotSink struct {
	bytes.Buffer
	closed   bool
	canceled bool
	writeErr error
	closeErr error
}

func (m *mockSnapshotSink) Write(p []byte) (int, error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	return m.Buffer.Write(p)
}

func (m *mockSnapshotSink) Close() error {
	m.closed = true
	return m.closeErr
}

func (m *mockSnapshotSink) Cancel() error {
	m.canceled = true
	return nil
}

func (m *mockSnapshotSink) ID() string {
	return "mock"
}

func TestFSMSnapshotPersistClosesSink(t *testing.T) {
	f := newFSM()
	f.mu.Lock()
	f.data["key"] = []byte("value")
	f.mu.Unlock()

	snapshot, err := f.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	sink := &mockSnapshotSink{}
	if err := snapshot.Persist(sink); err != nil {
		t.Fatalf("Persist failed: %v", err)
	}
	if !sink.closed {
		t.Fatalf("expected sink to be closed")
	}
	if sink.canceled {
		t.Fatalf("did not expect sink to be canceled")
	}
}

func TestFSMSnapshotPersistCancelsOnError(t *testing.T) {
	f := newFSM()
	f.mu.Lock()
	f.data["key"] = []byte("value")
	f.mu.Unlock()

	snapshot, err := f.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	writeErr := errors.New("write failed")
	sink := &mockSnapshotSink{writeErr: writeErr}
	if err := snapshot.Persist(sink); err == nil {
		t.Fatalf("expected persist error")
	}
	if !sink.canceled {
		t.Fatalf("expected sink to be canceled")
	}
	if sink.closed {
		t.Fatalf("did not expect sink to be closed")
	}
}
