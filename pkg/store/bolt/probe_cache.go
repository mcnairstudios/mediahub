package bolt

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/store/bolt/keyenc"
	bbolt "go.etcd.io/bbolt"
)

const probeCacheTTL = 24 * time.Hour

type probeCacheKeyDef struct {
	Kind    string `key:"probecache"`
	URLHash string
}

var (
	probeCacheSchema = keyenc.NewSchema[probeCacheKeyDef]()
	prefixProbeCache = probeCacheSchema.Prefix(probeCacheKeyDef{})
)

type probeCacheEntry struct {
	Result   *media.ProbeResult `json:"result"`
	StoredAt time.Time          `json:"stored_at"`
}

type ProbeCacheStore struct {
	db *bbolt.DB
}

func (s *ProbeCacheStore) Get(url string) (*media.ProbeResult, error) {
	h := hashURL(url)
	var entry probeCacheEntry

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketProbeCache)
		key := probeCacheSchema.Key(probeCacheKeyDef{URLHash: h})
		data := b.Get(key)
		if data == nil {
			data = b.Get([]byte(h))
		}
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
	h := hashURL(url)
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
		key := probeCacheSchema.Key(probeCacheKeyDef{URLHash: h})
		return b.Put(key, data)
	})
}

func (s *ProbeCacheStore) Delete(url string) error {
	h := hashURL(url)
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketProbeCache)
		key := probeCacheSchema.Key(probeCacheKeyDef{URLHash: h})
		b.Delete(key)
		b.Delete([]byte(h))
		return nil
	})
}

func hashURL(url string) string {
	h := sha256.Sum256([]byte(url))
	return hex.EncodeToString(h[:])
}

func (s *ProbeCacheStore) migrateFromFlatKeys() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketProbeCache)
		c := b.Cursor()

		k, _ := c.Seek(prefixProbeCache)
		if k != nil && bytes.HasPrefix(k, prefixProbeCache) {
			return nil
		}

		type migration struct {
			oldKey, newKey, data []byte
		}
		var migrations []migration

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if bytes.HasPrefix(k, prefixProbeCache) {
				continue
			}
			keyStr := string(k)
			if len(keyStr) != 64 {
				continue
			}
			newKey := probeCacheSchema.Key(probeCacheKeyDef{URLHash: keyStr})
			migrations = append(migrations, migration{
				oldKey: append([]byte{}, k...),
				newKey: newKey,
				data:   append([]byte{}, v...),
			})
		}

		if len(migrations) == 0 {
			return nil
		}

		log.Printf("probe_cache: migrating %d entries to prefixed keys", len(migrations))
		for _, m := range migrations {
			b.Put(m.newKey, m.data)
			b.Delete(m.oldKey)
		}
		log.Printf("probe_cache: migration complete")
		return nil
	})
}
