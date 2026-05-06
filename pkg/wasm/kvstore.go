package wasm

import (
	"fmt"

	"go.etcd.io/bbolt"
)

// KVStore provides plugin-scoped key-value storage.
type KVStore interface {
	Get(pluginType, key string) (string, error)
	Set(pluginType, key, value string) error
	Delete(pluginType, key string) error
}

var pluginKVBucket = []byte("plugin_kv")

// BoltKVStore is a KVStore backed by bbolt.
type BoltKVStore struct {
	db *bbolt.DB
}

// NewBoltKVStore creates a KVStore using the given bolt database.
func NewBoltKVStore(db *bbolt.DB) (*BoltKVStore, error) {
	err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(pluginKVBucket)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("creating plugin_kv bucket: %w", err)
	}
	return &BoltKVStore{db: db}, nil
}

func compositeKey(pluginType, key string) []byte {
	return []byte(pluginType + ":" + key)
}

func (s *BoltKVStore) Get(pluginType, key string) (string, error) {
	var val string
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(pluginKVBucket)
		if b == nil {
			return nil
		}
		v := b.Get(compositeKey(pluginType, key))
		if v != nil {
			val = string(v)
		}
		return nil
	})
	return val, err
}

func (s *BoltKVStore) Set(pluginType, key, value string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(pluginKVBucket)
		if b == nil {
			return fmt.Errorf("plugin_kv bucket missing")
		}
		return b.Put(compositeKey(pluginType, key), []byte(value))
	})
}

func (s *BoltKVStore) Delete(pluginType, key string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(pluginKVBucket)
		if b == nil {
			return nil
		}
		return b.Delete(compositeKey(pluginType, key))
	})
}
