package tmdb

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	bbolt "go.etcd.io/bbolt"
)

var (
	bucketTMDBQueue  = []byte("tmdb_queue")
	bucketTMDBBlobs  = []byte("tmdb_blobs")
	bucketImageQueue = []byte("image_queue")
	bucketTMDBNames  = []byte("tmdb_names")
)

type Store struct {
	db *bbolt.DB
}

func NewStore(db *bbolt.DB) (*Store, error) {
	err := db.Update(func(tx *bbolt.Tx) error {
		for _, name := range [][]byte{bucketTMDBQueue, bucketTMDBBlobs, bucketImageQueue, bucketTMDBNames} {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func tmdbKey(id int) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(id))
	return buf
}

func tmdbKeyTyped(mediaType string, id int) []byte {
	prefix := byte('m')
	if mediaType == "series" {
		prefix = byte('s')
	}
	buf := make([]byte, 9)
	buf[0] = prefix
	binary.BigEndian.PutUint64(buf[1:], uint64(id))
	return buf
}

func (s *Store) EnqueueMetadata(entry QueueEntry) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		return b(tx, bucketTMDBQueue).Put(tmdbKey(entry.TMDBID), data)
	})
}

func (s *Store) PickOldestResolving() (*QueueEntry, error) {
	var entry *QueueEntry
	err := s.db.View(func(tx *bbolt.Tx) error {
		c := b(tx, bucketTMDBQueue).Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var e QueueEntry
			if err := json.Unmarshal(v, &e); err != nil {
				continue
			}
			if e.Status == "resolving" {
				if entry == nil || e.CreatedAt < entry.CreatedAt {
					cp := e
					entry = &cp
				}
			}
		}
		return nil
	})
	return entry, err
}

func (s *Store) UpdateQueueEntry(entry QueueEntry) error {
	return s.EnqueueMetadata(entry)
}

func (s *Store) DeleteQueueEntry(tmdbID int) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return b(tx, bucketTMDBQueue).Delete(tmdbKey(tmdbID))
	})
}

func (s *Store) QueueCount() (int, error) {
	var count int
	err := s.db.View(func(tx *bbolt.Tx) error {
		return b(tx, bucketTMDBQueue).ForEach(func(_, _ []byte) error {
			count++
			return nil
		})
	})
	return count, err
}

func (s *Store) WriteBlob(tmdbID int, data []byte) error {
	return s.WriteBlobTyped("movie", tmdbID, data)
}

func (s *Store) WriteBlobTyped(mediaType string, tmdbID int, data []byte) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return b(tx, bucketTMDBBlobs).Put(tmdbKeyTyped(mediaType, tmdbID), data)
	})
}

func (s *Store) GetBlob(tmdbID int) ([]byte, error) {
	return s.GetBlobTyped("movie", tmdbID)
}

func (s *Store) GetBlobTyped(mediaType string, tmdbID int) ([]byte, error) {
	var data []byte
	err := s.db.View(func(tx *bbolt.Tx) error {
		v := b(tx, bucketTMDBBlobs).Get(tmdbKeyTyped(mediaType, tmdbID))
		if v == nil {
			v = b(tx, bucketTMDBBlobs).Get(tmdbKey(tmdbID))
		}
		if v != nil {
			data = make([]byte, len(v))
			copy(data, v)
		}
		return nil
	})
	return data, err
}

func (s *Store) HasBlob(tmdbID int) (bool, error) {
	return s.HasBlobTyped("movie", tmdbID)
}

func (s *Store) HasBlobTyped(mediaType string, tmdbID int) (bool, error) {
	var exists bool
	err := s.db.View(func(tx *bbolt.Tx) error {
		exists = b(tx, bucketTMDBBlobs).Get(tmdbKeyTyped(mediaType, tmdbID)) != nil
		if !exists {
			exists = b(tx, bucketTMDBBlobs).Get(tmdbKey(tmdbID)) != nil
		}
		return nil
	})
	return exists, err
}

func (s *Store) DeleteBlob(tmdbID int) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b(tx, bucketTMDBBlobs).Delete(tmdbKey(tmdbID))
		b(tx, bucketTMDBBlobs).Delete(tmdbKeyTyped("movie", tmdbID))
		b(tx, bucketTMDBBlobs).Delete(tmdbKeyTyped("series", tmdbID))
		return nil
	})
}

func (s *Store) EnqueueImage(localPath string, entry ImageQueueEntry) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		return b(tx, bucketImageQueue).Put([]byte(localPath), data)
	})
}

func (s *Store) DequeueImage() (string, *ImageQueueEntry, error) {
	return s.PickImage()
}

func (s *Store) PickImage() (string, *ImageQueueEntry, error) {
	var localPath string
	var entry *ImageQueueEntry
	err := s.db.View(func(tx *bbolt.Tx) error {
		k, v := b(tx, bucketImageQueue).Cursor().First()
		if k == nil {
			return nil
		}
		localPath = string(k)
		var e ImageQueueEntry
		if err := json.Unmarshal(v, &e); err != nil {
			return fmt.Errorf("unmarshal image queue entry: %w", err)
		}
		entry = &e
		return nil
	})
	return localPath, entry, err
}

func (s *Store) DeleteImageEntry(localPath string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return b(tx, bucketImageQueue).Delete([]byte(localPath))
	})
}

func (s *Store) ImageQueueCount() (int, error) {
	var count int
	err := s.db.View(func(tx *bbolt.Tx) error {
		return b(tx, bucketImageQueue).ForEach(func(_, _ []byte) error {
			count++
			return nil
		})
	})
	return count, err
}

type nameIndexEntry struct {
	TMDBID    int    `json:"tmdb_id"`
	MediaType string `json:"media_type"`
}

func (s *Store) LookupName(name string) (tmdbID int, mediaType string, found bool) {
	normalized := NormalizeName(name)
	if normalized == "" {
		return 0, "", false
	}
	s.db.View(func(tx *bbolt.Tx) error {
		v := b(tx, bucketTMDBNames).Get([]byte(normalized))
		if v == nil {
			return nil
		}
		var entry nameIndexEntry
		if err := json.Unmarshal(v, &entry); err != nil {
			return nil
		}
		tmdbID = entry.TMDBID
		mediaType = entry.MediaType
		found = true
		return nil
	})
	return
}

func (s *Store) SetName(name string, tmdbID int, mediaType string) error {
	normalized := NormalizeName(name)
	if normalized == "" {
		return nil
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		data, err := json.Marshal(nameIndexEntry{
			TMDBID:    tmdbID,
			MediaType: mediaType,
		})
		if err != nil {
			return err
		}
		return b(tx, bucketTMDBNames).Put([]byte(normalized), data)
	})
}

var nameYearPattern = regexp.MustCompile(`\(\d{4}\)`)

func NormalizeName(name string) string {
	cleaned := nameYearPattern.ReplaceAllString(name, "")
	cleaned = strings.ToLower(cleaned)
	cleaned = strings.TrimSpace(cleaned)
	return cleaned
}

func b(tx *bbolt.Tx, name []byte) *bbolt.Bucket {
	return tx.Bucket(name)
}
