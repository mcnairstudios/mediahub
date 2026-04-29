package bolt

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/media"
	bbolt "go.etcd.io/bbolt"
)

const probeCacheTTL = 24 * time.Hour

type probeCacheEntry struct {
	Result    *media.ProbeResult `json:"result"`
	StoredAt  time.Time          `json:"stored_at"`
}

type ProbeCacheStore struct {
	db *bbolt.DB
}

func (s *ProbeCacheStore) Get(url string) (*media.ProbeResult, error) {
	key := hashURL(url)
	var entry probeCacheEntry

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketProbeCache)
		data := b.Get([]byte(key))
		if data == nil {
			return nil
		}
		return json.Unmarshal(data, &entry)
	})
	if err != nil {
		return nil, err
	}

	if entry.Result == nil {
		return nil, nil
	}

	if time.Since(entry.StoredAt) > probeCacheTTL {
		_ = s.Delete(url)
		return nil, nil
	}

	return entry.Result, nil
}

func (s *ProbeCacheStore) Set(url string, result *media.ProbeResult) error {
	key := hashURL(url)
	entry := probeCacheEntry{
		Result:   result,
		StoredAt: time.Now(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketProbeCache)
		return b.Put([]byte(key), data)
	})
}

func (s *ProbeCacheStore) Delete(url string) error {
	key := hashURL(url)
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketProbeCache)
		return b.Delete([]byte(key))
	})
}

func hashURL(url string) string {
	h := sha256.Sum256([]byte(url))
	return hex.EncodeToString(h[:])
}
