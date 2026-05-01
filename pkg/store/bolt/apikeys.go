package bolt

import (
	"context"
	"encoding/json"

	"github.com/mcnairstudios/mediahub/pkg/auth"
	bbolt "go.etcd.io/bbolt"
)

type APIKeyStore struct {
	db *bbolt.DB
}

func (s *APIKeyStore) Create(_ context.Context, key *auth.APIKey) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketAPIKeys)
		data, err := json.Marshal(key)
		if err != nil {
			return err
		}
		return b.Put([]byte(key.ID), data)
	})
}

func (s *APIKeyStore) GetByKey(_ context.Context, key string) (*auth.APIKey, error) {
	var result *auth.APIKey
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketAPIKeys)
		return b.ForEach(func(_, v []byte) error {
			var ak auth.APIKey
			if err := json.Unmarshal(v, &ak); err != nil {
				return err
			}
			if ak.Key == key {
				result = &ak
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, auth.ErrAPIKeyNotFound
	}
	return result, nil
}

func (s *APIKeyStore) ListByUser(_ context.Context, userID string) ([]*auth.APIKey, error) {
	var result []*auth.APIKey
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketAPIKeys)
		return b.ForEach(func(_, v []byte) error {
			var ak auth.APIKey
			if err := json.Unmarshal(v, &ak); err != nil {
				return err
			}
			if ak.UserID == userID {
				result = append(result, &ak)
			}
			return nil
		})
	})
	return result, err
}

func (s *APIKeyStore) Delete(_ context.Context, id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketAPIKeys)
		if b.Get([]byte(id)) == nil {
			return auth.ErrAPIKeyNotFound
		}
		return b.Delete([]byte(id))
	})
}
