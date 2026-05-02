package bolt

import (
	"bytes"
	"context"
	"log"
	"strings"

	"github.com/mcnairstudios/mediahub/pkg/store/bolt/keyenc"
	bbolt "go.etcd.io/bbolt"
)

type settingsKeyDef struct {
	Kind string `key:"settings"`
	Key  string
}

var (
	settingsSchema = keyenc.NewSchema[settingsKeyDef]()
	prefixSettings = settingsSchema.Prefix(settingsKeyDef{})
)

type SettingsStore struct {
	db *bbolt.DB
}

func (s *SettingsStore) Get(_ context.Context, key string) (string, error) {
	var val string

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSettings)
		k := settingsSchema.Key(settingsKeyDef{Key: key})
		data := b.Get(k)
		if data == nil {
			data = b.Get([]byte(key))
		}
		if data != nil {
			val = string(data)
		}
		return nil
	})

	return val, err
}

func (s *SettingsStore) Set(_ context.Context, key, value string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSettings)
		k := settingsSchema.Key(settingsKeyDef{Key: key})
		return b.Put(k, []byte(value))
	})
}

func (s *SettingsStore) List(_ context.Context) (map[string]string, error) {
	result := make(map[string]string)

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSettings)
		c := b.Cursor()
		for k, v := c.Seek(prefixSettings); k != nil && bytes.HasPrefix(k, prefixSettings); k, v = c.Next() {
			parsed := settingsSchema.Parse(k)
			result[parsed.Key] = string(v)
		}
		return nil
	})

	return result, err
}

func (s *SettingsStore) migrateFromFlatKeys() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSettings)
		c := b.Cursor()

		k, _ := c.Seek(prefixSettings)
		if k != nil && bytes.HasPrefix(k, prefixSettings) {
			return nil
		}

		type migration struct {
			oldKey, newKey, data []byte
		}
		var migrations []migration

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if bytes.HasPrefix(k, prefixSettings) {
				continue
			}
			keyStr := string(k)
			if strings.Contains(keyStr, ":") {
				continue
			}
			newKey := settingsSchema.Key(settingsKeyDef{Key: keyStr})
			migrations = append(migrations, migration{
				oldKey: append([]byte{}, k...),
				newKey: newKey,
				data:   append([]byte{}, v...),
			})
		}

		if len(migrations) == 0 {
			return nil
		}

		log.Printf("settings: migrating %d settings to prefixed keys", len(migrations))
		for _, m := range migrations {
			b.Put(m.newKey, m.data)
			b.Delete(m.oldKey)
		}
		log.Printf("settings: migration complete")
		return nil
	})
}
