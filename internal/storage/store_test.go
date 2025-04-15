package storage

import (
	"testing"
)

func TestMemoryStore(t *testing.T) {
	store := NewStore()

	t.Run("Put and Get", func(t *testing.T) {
		err := store.Put("key1", "value1")
		if err != nil {
			t.Errorf("Failed to put: %v", err)
		}

		value, err := store.Get("key1")
		if err != nil {
			t.Errorf("Failed to get: %v", err)
		}
		if value != "value1" {
			t.Errorf("Expected value1, got %s", value)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		err := store.Put("key2", "value2")
		if err != nil {
			t.Errorf("Failed to put: %v", err)
		}

		err = store.Delete("key2")
		if err != nil {
			t.Errorf("Failed to delete: %v", err)
		}

		value, err := store.Get("key2")
		if err != nil {
			t.Errorf("Failed to get: %v", err)
		}
		if value != "" {
			t.Errorf("Expected empty string, got %s", value)
		}
	})
}
