package bolt

import (
	bbolt "go.etcd.io/bbolt"
)

var (
	bucketStreams  = []byte("streams")
	bucketSettings = []byte("settings")
)

type DB struct {
	db *bbolt.DB
}

func Open(path string) (*DB, error) {
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bucketStreams); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketSettings); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	return &DB{db: db}, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) StreamStore() *StreamStore {
	return &StreamStore{db: d.db}
}

func (d *DB) SettingsStore() *SettingsStore {
	return &SettingsStore{db: d.db}
}
