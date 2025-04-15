package raft

import (
	"testing"

	"github.com/hashicorp/raft"
	"github.com/vmihailenco/msgpack/v5"
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
		data, err := msgpack.Marshal(c.cmd)
		if err != nil {
			t.Fatalf("[TEST] Failed to marshal command: %v", err)
		}
		log := &raft.Log{Data: data}
		if err := f.Apply(log); err != nil {
			t.Fatalf("[TEST] Apply failed: %v", err)
		}
		if c.cmd.Type == "SET" {
			if string(f.data[c.cmd.Key]) != string(c.cmd.Value) {
				t.Fatalf("[TEST] Expected value '%s', got '%s'", c.cmd.Value, f.data[c.cmd.Key])
			}
		} else if c.cmd.Type == "DELETE" {
			if _, exists := f.data[c.cmd.Key]; exists {
				t.Fatalf("[TEST] Expected key '%s' to be deleted, but it still exists", c.cmd.Key)
			}
		}
	}
}
