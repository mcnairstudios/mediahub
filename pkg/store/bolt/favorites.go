package bolt

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/favorite"
	"github.com/mcnairstudios/mediahub/pkg/store/bolt/keyenc"
	bbolt "go.etcd.io/bbolt"
)

type favoriteKeyDef struct {
	Kind     string `key:"favorites"`
	UserID   string
	StreamID string
}

var (
	favoriteSchema = keyenc.NewSchema[favoriteKeyDef]()
	prefixFavorite = favoriteSchema.Prefix(favoriteKeyDef{})
)

type FavoriteStore struct {
	db *bbolt.DB
}

func (s *FavoriteStore) List(_ context.Context, userID string) ([]favorite.Favorite, error) {
	var result []favorite.Favorite

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketFavorites)
		c := b.Cursor()
		prefix := favoriteSchema.Prefix(favoriteKeyDef{UserID: userID})
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			var f favorite.Favorite
			if json.Unmarshal(v, &f) == nil {
				result = append(result, f)
			}
		}
		return nil
	})

	return result, err
}

func (s *FavoriteStore) Add(_ context.Context, userID, streamID string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketFavorites)
		key := favoriteSchema.Key(favoriteKeyDef{UserID: userID, StreamID: streamID})

		if existing := b.Get(key); existing != nil {
			return nil
		}

		f := favorite.Favorite{
			StreamID: streamID,
			UserID:   userID,
			AddedAt:  time.Now(),
		}
		data, err := json.Marshal(f)
		if err != nil {
			return err
		}
		return b.Put(key, data)
	})
}

func (s *FavoriteStore) Remove(_ context.Context, userID, streamID string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketFavorites)
		key := favoriteSchema.Key(favoriteKeyDef{UserID: userID, StreamID: streamID})
		return b.Delete(key)
	})
}

func (s *FavoriteStore) IsFavorite(_ context.Context, userID, streamID string) (bool, error) {
	var exists bool

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketFavorites)
		key := favoriteSchema.Key(favoriteKeyDef{UserID: userID, StreamID: streamID})
		exists = b.Get(key) != nil
		return nil
	})

	return exists, err
}

func (s *FavoriteStore) migrateFromNestedBuckets() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketFavorites)
		c := b.Cursor()

		k, _ := c.Seek(prefixFavorite)
		if k != nil && bytes.HasPrefix(k, prefixFavorite) {
			return nil
		}

		type migration struct {
			userID string
			data   []struct {
				streamID string
				value    []byte
			}
		}
		var migrations []migration

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if v != nil {
				continue
			}
			userID := string(k)
			userBucket := b.Bucket(k)
			if userBucket == nil {
				continue
			}

			m := migration{userID: userID}
			userBucket.ForEach(func(streamKey, streamVal []byte) error {
				m.data = append(m.data, struct {
					streamID string
					value    []byte
				}{
					streamID: string(streamKey),
					value:    append([]byte{}, streamVal...),
				})
				return nil
			})
			if len(m.data) > 0 {
				migrations = append(migrations, m)
			}
		}

		if len(migrations) == 0 {
			return nil
		}

		total := 0
		for _, m := range migrations {
			total += len(m.data)
		}
		log.Printf("favorites: migrating %d favorites from %d user buckets to prefixed keys", total, len(migrations))

		for _, m := range migrations {
			for _, d := range m.data {
				newKey := favoriteSchema.Key(favoriteKeyDef{
					UserID:   m.userID,
					StreamID: d.streamID,
				})
				b.Put(newKey, d.value)
			}
			b.DeleteBucket([]byte(m.userID))
		}

		log.Printf("favorites: migration complete")
		return nil
	})
}
