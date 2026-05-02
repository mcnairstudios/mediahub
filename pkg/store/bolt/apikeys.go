package bolt

import (
	"bytes"
	"context"
	"encoding/json"
	"log"

	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/store/bolt/keyenc"
	bbolt "go.etcd.io/bbolt"
)

type apiKeyKeyDef struct {
	Kind   string `key:"apikeys"`
	UserID string
	KeyID  string
}

var (
	apiKeySchema    = keyenc.NewSchema[apiKeyKeyDef]()
	prefixAPIKey    = apiKeySchema.Prefix(apiKeyKeyDef{})
	prefixAPIKeyIdx = keyenc.ReversePrefix("apikeyidx")
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
		k := apiKeySchema.Key(apiKeyKeyDef{UserID: key.UserID, KeyID: key.ID})
		if err := b.Put(k, data); err != nil {
			return err
		}
		return b.Put(keyenc.Reverse("apikeyidx", key.ID), k)
	})
}

func (s *APIKeyStore) GetByKey(_ context.Context, key string) (*auth.APIKey, error) {
	var result *auth.APIKey
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketAPIKeys)
		c := b.Cursor()
		for k, v := c.Seek(prefixAPIKey); k != nil && bytes.HasPrefix(k, prefixAPIKey); k, v = c.Next() {
			var ak auth.APIKey
			if err := json.Unmarshal(v, &ak); err != nil {
				return err
			}
			if ak.Key == key {
				result = &ak
				return nil
			}
		}
		return nil
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
		c := b.Cursor()
		prefix := apiKeySchema.Prefix(apiKeyKeyDef{UserID: userID})
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			var ak auth.APIKey
			if err := json.Unmarshal(v, &ak); err != nil {
				return err
			}
			result = append(result, &ak)
		}
		return nil
	})
	return result, err
}

func (s *APIKeyStore) Delete(_ context.Context, id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketAPIKeys)

		idxKey := keyenc.Reverse("apikeyidx", id)
		fullKey := b.Get(idxKey)
		if fullKey != nil {
			b.Delete(fullKey)
			b.Delete(idxKey)
			return nil
		}

		if b.Get([]byte(id)) == nil {
			return auth.ErrAPIKeyNotFound
		}
		return b.Delete([]byte(id))
	})
}

func (s *APIKeyStore) migrateFromFlatKeys() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketAPIKeys)
		c := b.Cursor()

		k, _ := c.Seek(prefixAPIKey)
		if k != nil && bytes.HasPrefix(k, prefixAPIKey) {
			return nil
		}

		type migration struct {
			oldKey, newKey, idxKey, data []byte
		}
		var migrations []migration

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if bytes.HasPrefix(k, prefixAPIKey) || bytes.HasPrefix(k, prefixAPIKeyIdx) {
				continue
			}
			var ak auth.APIKey
			if json.Unmarshal(v, &ak) != nil || ak.ID == "" || ak.UserID == "" {
				continue
			}
			newKey := apiKeySchema.Key(apiKeyKeyDef{UserID: ak.UserID, KeyID: ak.ID})
			migrations = append(migrations, migration{
				oldKey: append([]byte{}, k...),
				newKey: newKey,
				idxKey: keyenc.Reverse("apikeyidx", ak.ID),
				data:   append([]byte{}, v...),
			})
		}

		if len(migrations) == 0 {
			return nil
		}

		log.Printf("apikeys: migrating %d API keys to prefixed keys", len(migrations))
		for _, m := range migrations {
			b.Put(m.newKey, m.data)
			b.Put(m.idxKey, m.newKey)
			b.Delete(m.oldKey)
		}
		log.Printf("apikeys: migration complete")
		return nil
	})
}
