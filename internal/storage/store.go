package storage

import (
	"encoding/json"
	"sync"
)

// Store defines the interface for key-value storage operations
type Store interface {
	Get(key string) (string, error)
	Put(key, value string) error
	Delete(key string) error
}

// MemoryStore provides an in-memory implementation of the Store interface
type MemoryStore struct {
	mu    sync.RWMutex
	store map[string]string
}

// NewStore creates a new instance of MemoryStore
func NewStore() *MemoryStore {
	return &MemoryStore{
		store: make(map[string]string),
	}
}

// Get retrieves the value associated with the given key
func (s *MemoryStore) Get(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if value, ok := s.store[key]; ok {
		return value, nil
	}
	return "", nil
}

// Put stores a key-value pair in the store
func (s *MemoryStore) Put(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.store[key] = value
	return nil
}

// Delete removes a key-value pair from the store
func (s *MemoryStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.store, key)
	return nil
}

// Command represents a storage operation to be processed
type Command struct {
	Op    string `json:"op"`    // Operation type (put/delete)
	Key   string `json:"key"`   // Key to operate on
	Value string `json:"value"` // Value for put operations
}

// ApplyCommand processes a command on the store
func (s *MemoryStore) ApplyCommand(cmd Command) error {
	switch cmd.Op {
	case "put":
		return s.Put(cmd.Key, cmd.Value)
	case "delete":
		return s.Delete(cmd.Key)
	default:
		return nil
	}
}

// EncodeCommand serializes a command to JSON format
func EncodeCommand(cmd Command) ([]byte, error) {
	return json.Marshal(cmd)
}

// DecodeCommand deserializes a command from JSON format
func DecodeCommand(data []byte) (Command, error) {
	var cmd Command
	err := json.Unmarshal(data, &cmd)
	return cmd, err
}
