package bolt

import (
	"bytes"
	"context"
	"encoding/json"
	"log"

	"github.com/mcnairstudios/mediahub/pkg/client"
	"github.com/mcnairstudios/mediahub/pkg/store/bolt/keyenc"
	bbolt "go.etcd.io/bbolt"
)

type clientKeyDef struct {
	Kind     string `key:"clients"`
	ClientID string
}

var (
	clientSchema = keyenc.NewSchema[clientKeyDef]()
	prefixClient = clientSchema.Prefix(clientKeyDef{})
)

type ClientStore struct {
	db *bbolt.DB
}

func (s *ClientStore) Get(_ context.Context, id string) (*client.Client, error) {
	var c *client.Client

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketClients)
		key := clientSchema.Key(clientKeyDef{ClientID: id})
		data := b.Get(key)
		if data == nil {
			data = b.Get([]byte(id))
		}
		if data == nil {
			return nil
		}
		c = &client.Client{}
		return json.Unmarshal(data, c)
	})

	return c, err
}

func (s *ClientStore) List(_ context.Context) ([]client.Client, error) {
	var result []client.Client

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketClients)
		c := b.Cursor()
		for k, v := c.Seek(prefixClient); k != nil && bytes.HasPrefix(k, prefixClient); k, v = c.Next() {
			var cl client.Client
			if err := json.Unmarshal(v, &cl); err != nil {
				return err
			}
			result = append(result, cl)
		}
		return nil
	})

	return result, err
}

func (s *ClientStore) Create(_ context.Context, c *client.Client) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketClients)
		data, err := json.Marshal(c)
		if err != nil {
			return err
		}
		key := clientSchema.Key(clientKeyDef{ClientID: c.ID})
		return b.Put(key, data)
	})
}

func (s *ClientStore) Update(_ context.Context, c *client.Client) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketClients)
		data, err := json.Marshal(c)
		if err != nil {
			return err
		}
		key := clientSchema.Key(clientKeyDef{ClientID: c.ID})
		return b.Put(key, data)
	})
}

func (s *ClientStore) Delete(_ context.Context, id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketClients)
		key := clientSchema.Key(clientKeyDef{ClientID: id})
		b.Delete(key)
		b.Delete([]byte(id))
		return nil
	})
}

func (s *ClientStore) migrateFromFlatKeys() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketClients)
		c := b.Cursor()

		k, _ := c.Seek(prefixClient)
		if k != nil && bytes.HasPrefix(k, prefixClient) {
			return nil
		}

		type migration struct {
			oldKey, newKey, data []byte
		}
		var migrations []migration

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if bytes.HasPrefix(k, prefixClient) {
				continue
			}
			var cl client.Client
			if json.Unmarshal(v, &cl) != nil || cl.ID == "" {
				continue
			}
			newKey := clientSchema.Key(clientKeyDef{ClientID: cl.ID})
			migrations = append(migrations, migration{
				oldKey: append([]byte{}, k...),
				newKey: newKey,
				data:   append([]byte{}, v...),
			})
		}

		if len(migrations) == 0 {
			return nil
		}

		log.Printf("clients: migrating %d clients to prefixed keys", len(migrations))
		for _, m := range migrations {
			b.Put(m.newKey, m.data)
			b.Delete(m.oldKey)
		}
		log.Printf("clients: migration complete")
		return nil
	})
}
