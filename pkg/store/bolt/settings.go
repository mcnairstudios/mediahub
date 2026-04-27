package bolt

import (
	"context"

	bbolt "go.etcd.io/bbolt"
)

type SettingsStore struct {
	db *bbolt.DB
}

func (s *SettingsStore) Get(_ context.Context, key string) (string, error) {
	var val string

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSettings)
		data := b.Get([]byte(key))
		if data != nil {
			val = string(data)
		}
		return nil
	})

	return val, err
}

func (s *SettingsStore) Set(_ context.Context, key, value string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSettings)
		return b.Put([]byte(key), []byte(value))
	})
}

func (s *SettingsStore) List(_ context.Context) (map[string]string, error) {
	result := make(map[string]string)

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSettings)
		return b.ForEach(func(k, v []byte) error {
			result[string(k)] = string(v)
			return nil
		})
	})

	return result, err
}
