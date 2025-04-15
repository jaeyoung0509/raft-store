package storage

import (
	"encoding/json"
	"sync"
)

// Store represents the key-value store interface
type Store interface {
	Get(key string) (string, error)
	Put(key, value string) error
	Delete(key string) error
}

// MemoryStore is an in-memory implementation of the Store interface
type MemoryStore struct {
	mu    sync.RWMutex
	store map[string]string
}

// NewStore creates a new memory store
func NewStore() *MemoryStore {
	return &MemoryStore{
		store: make(map[string]string),
	}
}

// Get retrieves a value for a given key
func (s *MemoryStore) Get(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if value, ok := s.store[key]; ok {
		return value, nil
	}
	return "", nil
}

// Put stores a key-value pair
func (s *MemoryStore) Put(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.store[key] = value
	return nil
}

// Delete removes a key-value pair
func (s *MemoryStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.store, key)
	return nil
}

// Command represents a storage operation
type Command struct {
	Op    string `json:"op"`
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

// ApplyCommand applies a command to the store
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

// EncodeCommand encodes a command to JSON
func EncodeCommand(cmd Command) ([]byte, error) {
	return json.Marshal(cmd)
}

// DecodeCommand decodes a command from JSON
func DecodeCommand(data []byte) (Command, error) {
	var cmd Command
	err := json.Unmarshal(data, &cmd)
	return cmd, err
}
