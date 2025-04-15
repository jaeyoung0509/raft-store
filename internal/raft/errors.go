package raft

import "errors"

var (
	// ErrUnknownCommand is returned when an unknown command type is received
	ErrUnknownCommand = errors.New("unknown command type")
)
