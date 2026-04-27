package bolt

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/favorite"
	bbolt "go.etcd.io/bbolt"
)

type FavoriteStore struct {
	db *bbolt.DB
}

func (s *FavoriteStore) List(_ context.Context, userID string) ([]favorite.Favorite, error) {
	var result []favorite.Favorite

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketFavorites)
		userBucket := b.Bucket([]byte(userID))
		if userBucket == nil {
			return nil
		}
		return userBucket.ForEach(func(_, v []byte) error {
			var f favorite.Favorite
			if err := json.Unmarshal(v, &f); err != nil {
				return err
			}
			result = append(result, f)
			return nil
		})
	})

	return result, err
}

func (s *FavoriteStore) Add(_ context.Context, userID, streamID string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketFavorites)
		userBucket, err := b.CreateBucketIfNotExists([]byte(userID))
		if err != nil {
			return err
		}

		if existing := userBucket.Get([]byte(streamID)); existing != nil {
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
		return userBucket.Put([]byte(streamID), data)
	})
}

func (s *FavoriteStore) Remove(_ context.Context, userID, streamID string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketFavorites)
		userBucket := b.Bucket([]byte(userID))
		if userBucket == nil {
			return nil
		}
		return userBucket.Delete([]byte(streamID))
	})
}

func (s *FavoriteStore) IsFavorite(_ context.Context, userID, streamID string) (bool, error) {
	var exists bool

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketFavorites)
		userBucket := b.Bucket([]byte(userID))
		if userBucket == nil {
			return nil
		}
		exists = userBucket.Get([]byte(streamID)) != nil
		return nil
	})

	return exists, err
}
