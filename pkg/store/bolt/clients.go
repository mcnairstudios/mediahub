package bolt

import (
	"context"
	"encoding/json"

	"github.com/mcnairstudios/mediahub/pkg/client"
	bbolt "go.etcd.io/bbolt"
)

type ClientStore struct {
	db *bbolt.DB
}

func (s *ClientStore) Get(_ context.Context, id string) (*client.Client, error) {
	var c *client.Client

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketClients)
		data := b.Get([]byte(id))
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
		return b.ForEach(func(_, v []byte) error {
			var c client.Client
			if err := json.Unmarshal(v, &c); err != nil {
				return err
			}
			result = append(result, c)
			return nil
		})
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
		return b.Put([]byte(c.ID), data)
	})
}

func (s *ClientStore) Update(_ context.Context, c *client.Client) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketClients)
		data, err := json.Marshal(c)
		if err != nil {
			return err
		}
		return b.Put([]byte(c.ID), data)
	})
}

func (s *ClientStore) Delete(_ context.Context, id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketClients)
		return b.Delete([]byte(id))
	})
}
