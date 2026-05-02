package bolt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
	"github.com/mcnairstudios/mediahub/pkg/store/bolt/keyenc"
	bbolt "go.etcd.io/bbolt"
)

var ErrSourceConfigNotFound = errors.New("source config not found")

type sourceConfigKeyDef struct {
	Kind       string `key:"sourceconfigs"`
	SourceType string
	ConfigID   string
}

const sourceTypeUnknown = "_unknown"

func sourceTypeKey(t string) string {
	if t == "" {
		return sourceTypeUnknown
	}
	return t
}

var (
	sourceConfigSchema    = keyenc.NewSchema[sourceConfigKeyDef]()
	prefixSourceConfig    = sourceConfigSchema.Prefix(sourceConfigKeyDef{})
	prefixSourceConfigIdx = keyenc.ReversePrefix("srccfgidx")
)

type SourceConfigStore struct {
	db *bbolt.DB
}

func (s *SourceConfigStore) Get(_ context.Context, id string) (*sourceconfig.SourceConfig, error) {
	var sc *sourceconfig.SourceConfig

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSourceConfigs)
		idxKey := keyenc.Reverse("srccfgidx", id)
		fullKey := b.Get(idxKey)
		var data []byte
		if fullKey != nil {
			data = b.Get(fullKey)
		}
		if data == nil {
			data = b.Get([]byte(id))
		}
		if data == nil {
			return nil
		}
		sc = &sourceconfig.SourceConfig{}
		return json.Unmarshal(data, sc)
	})

	return sc, err
}

func (s *SourceConfigStore) List(_ context.Context) ([]sourceconfig.SourceConfig, error) {
	var result []sourceconfig.SourceConfig

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSourceConfigs)
		c := b.Cursor()
		for k, v := c.Seek(prefixSourceConfig); k != nil && bytes.HasPrefix(k, prefixSourceConfig); k, v = c.Next() {
			var sc sourceconfig.SourceConfig
			if err := json.Unmarshal(v, &sc); err != nil {
				return err
			}
			result = append(result, sc)
		}
		return nil
	})

	if result == nil {
		result = []sourceconfig.SourceConfig{}
	}
	return result, err
}

func (s *SourceConfigStore) ListByType(_ context.Context, sourceType string) ([]sourceconfig.SourceConfig, error) {
	var result []sourceconfig.SourceConfig

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSourceConfigs)
		c := b.Cursor()
		prefix := sourceConfigSchema.Prefix(sourceConfigKeyDef{SourceType: sourceTypeKey(sourceType)})
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			var sc sourceconfig.SourceConfig
			if err := json.Unmarshal(v, &sc); err != nil {
				return err
			}
			result = append(result, sc)
		}
		return nil
	})

	if result == nil {
		result = []sourceconfig.SourceConfig{}
	}
	return result, err
}

func (s *SourceConfigStore) putSourceConfig(b *bbolt.Bucket, sc *sourceconfig.SourceConfig) error {
	data, err := json.Marshal(sc)
	if err != nil {
		return err
	}
	key := sourceConfigSchema.Key(sourceConfigKeyDef{SourceType: sourceTypeKey(sc.Type), ConfigID: sc.ID})
	if err := b.Put(key, data); err != nil {
		return err
	}
	return b.Put(keyenc.Reverse("srccfgidx", sc.ID), key)
}

func (s *SourceConfigStore) deleteSourceConfig(b *bbolt.Bucket, id string) {
	idxKey := keyenc.Reverse("srccfgidx", id)
	fullKey := b.Get(idxKey)
	if fullKey != nil {
		b.Delete(fullKey)
	}
	b.Delete(idxKey)
	b.Delete([]byte(id))
}

func (s *SourceConfigStore) Create(_ context.Context, sc *sourceconfig.SourceConfig) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSourceConfigs)
		return s.putSourceConfig(b, sc)
	})
}

func (s *SourceConfigStore) Update(_ context.Context, sc *sourceconfig.SourceConfig) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSourceConfigs)
		idxKey := keyenc.Reverse("srccfgidx", sc.ID)
		fullKey := b.Get(idxKey)
		if fullKey == nil && b.Get([]byte(sc.ID)) == nil {
			return ErrSourceConfigNotFound
		}
		s.deleteSourceConfig(b, sc.ID)
		return s.putSourceConfig(b, sc)
	})
}

func (s *SourceConfigStore) Delete(_ context.Context, id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSourceConfigs)
		s.deleteSourceConfig(b, id)
		return nil
	})
}

func (s *SourceConfigStore) migrateFromFlatKeys() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSourceConfigs)
		c := b.Cursor()

		k, _ := c.Seek(prefixSourceConfigIdx)
		if k != nil && bytes.HasPrefix(k, prefixSourceConfigIdx) {
			return nil
		}

		type migration struct {
			oldKey, newKey, idxKey, data []byte
		}
		var migrations []migration

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if bytes.HasPrefix(k, prefixSourceConfig) || bytes.HasPrefix(k, prefixSourceConfigIdx) {
				continue
			}
			var sc sourceconfig.SourceConfig
			if json.Unmarshal(v, &sc) != nil || sc.ID == "" {
				continue
			}
			newKey := sourceConfigSchema.Key(sourceConfigKeyDef{SourceType: sourceTypeKey(sc.Type), ConfigID: sc.ID})
			migrations = append(migrations, migration{
				oldKey: append([]byte{}, k...),
				newKey: newKey,
				idxKey: keyenc.Reverse("srccfgidx", sc.ID),
				data:   append([]byte{}, v...),
			})
		}

		if len(migrations) == 0 {
			return nil
		}

		log.Printf("source_configs: migrating %d source configs to prefixed keys", len(migrations))
		for _, m := range migrations {
			b.Put(m.newKey, m.data)
			b.Put(m.idxKey, m.newKey)
			b.Delete(m.oldKey)
		}
		log.Printf("source_configs: migration complete")
		return nil
	})
}
