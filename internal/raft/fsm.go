package raft

import (
	"encoding/json"
	"io"
	"time"

	"github.com/hashicorp/raft"
	"github.com/vmihailenco/msgpack/v5"
)

// Command represents a command to be applied to the FSM
type Command struct {
	Type    string        `json:"type"`
	Key     string        `json:"key"`
	Value   []byte        `json:"value"`
	Timeout time.Duration `json:"timeout"`
}

// fsm implements the raft.FSM interface
type fsm struct {
	data map[string][]byte
}

func newFSM() *fsm {
	return &fsm{
		data: make(map[string][]byte),
	}
}

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

func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	return &fsmSnapshot{data: f.data}, nil
}

func (f *fsm) Restore(rc io.ReadCloser) error {
	f.data = make(map[string][]byte)
	return json.NewDecoder(rc).Decode(&f.data)
}

type fsmSnapshot struct {
	data map[string][]byte
}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	return json.NewEncoder(sink).Encode(f.data)
}

func (f *fsmSnapshot) Release() {}
