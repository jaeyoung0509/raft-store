package raft

// Package raft implements a Raft consensus node and related components

import "errors"

var (
	// ErrUnknownCommand is returned when trying to apply an unrecognized command type
	ErrUnknownCommand = errors.New("unknown command type")
)
