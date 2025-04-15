package raft

import (
	"encoding/binary"
	"fmt"

	"github.com/hashicorp/raft"
	"go.etcd.io/bbolt"
)

// BoltStore implements both LogStore and StableStore interfaces
type BoltStore struct {
	db *bbolt.DB
}

func NewBoltStore(path string) (*BoltStore, error) {
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open bbolt database: %v", err)
	}

	// Create buckets if they don't exist
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("logs"))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte("stable"))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create buckets: %v", err)
	}

	return &BoltStore{db: db}, nil
}

// LogStore interface implementation
func (s *BoltStore) FirstIndex() (uint64, error) {
	var index uint64
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("logs"))
		c := b.Cursor()
		if k, _ := c.First(); k != nil {
			index = binary.BigEndian.Uint64(k)
		}
		return nil
	})
	return index, err
}

func (s *BoltStore) LastIndex() (uint64, error) {
	var index uint64
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("logs"))
		c := b.Cursor()
		if k, _ := c.Last(); k != nil {
			index = binary.BigEndian.Uint64(k)
		}
		return nil
	})
	return index, err
}

func (s *BoltStore) GetLog(index uint64, log *raft.Log) error {
	return s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("logs"))
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, index)
		data := b.Get(key)
		if data == nil {
			return raft.ErrLogNotFound
		}
		return decodeLog(data, log)
	})
}

func (s *BoltStore) StoreLog(log *raft.Log) error {
	return s.StoreLogs([]*raft.Log{log})
}

func (s *BoltStore) StoreLogs(logs []*raft.Log) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("logs"))
		for _, log := range logs {
			key := make([]byte, 8)
			binary.BigEndian.PutUint64(key, log.Index)
			data, err := encodeLog(log)
			if err != nil {
				return err
			}
			if err := b.Put(key, data); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *BoltStore) DeleteRange(min, max uint64) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("logs"))
		c := b.Cursor()
		for k, _ := c.Seek(make([]byte, 8)); k != nil; k, _ = c.Next() {
			index := binary.BigEndian.Uint64(k)
			if index > max {
				break
			}
			if index >= min {
				if err := b.Delete(k); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// StableStore interface implementation
func (s *BoltStore) Set(key []byte, val []byte) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("stable"))
		return b.Put(key, val)
	})
}

func (s *BoltStore) Get(key []byte) ([]byte, error) {
	var val []byte
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("stable"))
		val = b.Get(key)
		return nil
	})
	return val, err
}

func (s *BoltStore) SetUint64(key []byte, val uint64) error {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, val)
	return s.Set(key, buf)
}

func (s *BoltStore) GetUint64(key []byte) (uint64, error) {
	val, err := s.Get(key)
	if err != nil {
		return 0, err
	}
	if val == nil {
		return 0, nil
	}
	return binary.BigEndian.Uint64(val), nil
}

// Helper functions for encoding/decoding logs
func encodeLog(log *raft.Log) ([]byte, error) {
	// index (8byte) + term(8byte) + type (1byte) + data
	buf := make([]byte, 17+len(log.Data))
	binary.BigEndian.PutUint64(buf[0:8], log.Index)
	binary.BigEndian.PutUint64(buf[8:16], uint64(log.Term))
	buf[16] = byte(log.Type)
	copy(buf[17:], log.Data)
	return buf, nil
}

func decodeLog(data []byte, log *raft.Log) error {
	if len(data) < 17 {
		return fmt.Errorf("invalid log data length")
	}
	log.Index = binary.BigEndian.Uint64(data[0:8])
	log.Term = uint64(binary.BigEndian.Uint64(data[8:16]))
	log.Type = raft.LogType(data[16])
	log.Data = data[17:]
	return nil
}
