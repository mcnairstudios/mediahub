package bolt

import (
	bbolt "go.etcd.io/bbolt"
)

var (
	bucketStreams       = []byte("streams")
	bucketSettings     = []byte("settings")
	bucketChannels     = []byte("channels")
	bucketGroups       = []byte("groups")
	bucketEPGSources   = []byte("epg_sources")
	bucketEPGPrograms  = []byte("epg_programs")
	bucketRecordings   = []byte("recordings")
	bucketUsers        = []byte("users")
	bucketSourceConfigs = []byte("source_configs")
)

type DB struct {
	db *bbolt.DB
}

func Open(path string) (*DB, error) {
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}

	buckets := [][]byte{
		bucketStreams, bucketSettings, bucketChannels, bucketGroups,
		bucketEPGSources, bucketEPGPrograms, bucketRecordings, bucketUsers,
		bucketSourceConfigs,
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		for _, name := range buckets {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return err
			}
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

func (d *DB) ChannelStore() *ChannelStore {
	return &ChannelStore{db: d.db}
}

func (d *DB) GroupStore() *GroupStore {
	return &GroupStore{db: d.db}
}

func (d *DB) EPGSourceStore() *EPGSourceStore {
	return &EPGSourceStore{db: d.db}
}

func (d *DB) ProgramStore() *ProgramStore {
	return &ProgramStore{db: d.db}
}

func (d *DB) RecordingStore() *RecordingStore {
	return &RecordingStore{db: d.db}
}

func (d *DB) UserStore() *UserStore {
	return &UserStore{db: d.db}
}

func (d *DB) SourceConfigStore() *SourceConfigStore {
	return &SourceConfigStore{db: d.db}
}
