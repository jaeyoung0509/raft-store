package storage

import (
	"testing"
)

func TestMemoryStore(t *testing.T) {
	store := NewStore()

	t.Run("Put and Get", func(t *testing.T) {
		// Test storing and retrieving a key-value pair
		err := store.Put("key1", "value1")
		if err != nil {
			t.Errorf("Failed to store value: %v", err)
		}

		value, err := store.Get("key1")
		if err != nil {
			t.Errorf("Failed to retrieve value: %v", err)
		}
		if value != "value1" {
			t.Errorf("Expected value 'value1', got '%s'", value)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		// Test deleting a key-value pair
		err := store.Put("key2", "value2")
		if err != nil {
			t.Errorf("Failed to store value: %v", err)
		}

		err = store.Delete("key2")
		if err != nil {
			t.Errorf("Failed to delete key: %v", err)
		}

		// Verify key was deleted
		value, err := store.Get("key2")
		if err != nil {
			t.Errorf("Failed to check deleted key: %v", err)
		}
		if value != "" {
			t.Errorf("Expected empty string after deletion, got '%s'", value)
		}
	})
}
