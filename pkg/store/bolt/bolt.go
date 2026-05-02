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
	bucketFavorites    = []byte("favorites")
	bucketClients        = []byte("clients")
	bucketSourceProfiles = []byte("source_profiles")
	bucketProbeCache     = []byte("probe_cache")
	bucketTMDBQueue      = []byte("tmdb_queue")
	bucketTMDBBlobs      = []byte("tmdb_blobs")
	bucketImageQueue     = []byte("image_queue")
	bucketHDHRDevices    = []byte("hdhr_devices")
	bucketInvites        = []byte("invites")
	bucketAPIKeys        = []byte("api_keys")
	bucketScheduler      = []byte("scheduler")
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
		bucketSourceConfigs, bucketFavorites, bucketClients,
		bucketSourceProfiles, bucketProbeCache,
		bucketTMDBQueue, bucketTMDBBlobs, bucketImageQueue,
		bucketHDHRDevices,
		bucketInvites, bucketAPIKeys,
		bucketScheduler,
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

func (d *DB) BoltDB() *bbolt.DB {
	return d.db
}

func (d *DB) StreamStore() *StreamStore {
	return &StreamStore{db: d.db}
}

func (d *DB) SettingsStore() *SettingsStore {
	return &SettingsStore{db: d.db}
}

func (d *DB) ChannelStore() *ChannelStore {
	s := &ChannelStore{db: d.db}
	s.migrateFromFlatKeys()
	return s
}

func (d *DB) GroupStore() *GroupStore {
	s := &GroupStore{db: d.db}
	s.migrateFromFlatKeys()
	return s
}

func (d *DB) EPGSourceStore() *EPGSourceStore {
	return &EPGSourceStore{db: d.db}
}

func (d *DB) ProgramStore() *ProgramStore {
	s := &ProgramStore{db: d.db}
	s.migrateFromPipeKeys()
	return s
}

func (d *DB) RecordingStore() *RecordingStore {
	s := &RecordingStore{db: d.db}
	s.migrateFromFlatKeys()
	return s
}

func (d *DB) UserStore() *UserStore {
	return &UserStore{db: d.db}
}

func (d *DB) SourceConfigStore() *SourceConfigStore {
	return &SourceConfigStore{db: d.db}
}

func (d *DB) FavoriteStore() *FavoriteStore {
	s := &FavoriteStore{db: d.db}
	s.migrateFromNestedBuckets()
	return s
}

func (d *DB) ClientStore() *ClientStore {
	return &ClientStore{db: d.db}
}

func (d *DB) SourceProfileStore() *SourceProfileStore {
	return &SourceProfileStore{db: d.db}
}

func (d *DB) ProbeCacheStore() *ProbeCacheStore {
	return &ProbeCacheStore{db: d.db}
}

func (d *DB) HDHRDeviceStore() *HDHRDeviceStore {
	return &HDHRDeviceStore{db: d.db}
}

func (d *DB) InviteStore() *InviteStore {
	return &InviteStore{db: d.db}
}

func (d *DB) APIKeyStore() *APIKeyStore {
	return &APIKeyStore{db: d.db}
}

func (d *DB) ClearBucket(name string) error {
	bucketName := []byte(name)
	return d.db.Update(func(tx *bbolt.Tx) error {
		if err := tx.DeleteBucket(bucketName); err != nil {
			return err
		}
		_, err := tx.CreateBucket(bucketName)
		return err
	})
}

func (d *DB) ClearAll() error {
	buckets := []string{
		"streams", "settings", "channels", "groups",
		"epg_sources", "epg_programs", "recordings", "users",
		"source_configs", "favorites", "clients", "source_profiles",
		"probe_cache", "tmdb_queue", "tmdb_blobs", "image_queue",
		"hdhr_devices",
		"invites", "api_keys",
		"scheduler",
	}
	return d.db.Update(func(tx *bbolt.Tx) error {
		for _, name := range buckets {
			b := []byte(name)
			if err := tx.DeleteBucket(b); err != nil {
				continue
			}
			if _, err := tx.CreateBucket(b); err != nil {
				return err
			}
		}
		return nil
	})
}
