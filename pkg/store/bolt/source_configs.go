package bolt

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
	bbolt "go.etcd.io/bbolt"
)

var ErrSourceConfigNotFound = errors.New("source config not found")

type SourceConfigStore struct {
	db *bbolt.DB
}

func (s *SourceConfigStore) Get(_ context.Context, id string) (*sourceconfig.SourceConfig, error) {
	var sc *sourceconfig.SourceConfig

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSourceConfigs)
		data := b.Get([]byte(id))
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
		return b.ForEach(func(k, v []byte) error {
			var sc sourceconfig.SourceConfig
			if err := json.Unmarshal(v, &sc); err != nil {
				return err
			}
			result = append(result, sc)
			return nil
		})
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
		return b.ForEach(func(k, v []byte) error {
			var sc sourceconfig.SourceConfig
			if err := json.Unmarshal(v, &sc); err != nil {
				return err
			}
			if sc.Type == sourceType {
				result = append(result, sc)
			}
			return nil
		})
	})

	if result == nil {
		result = []sourceconfig.SourceConfig{}
	}
	return result, err
}

func (s *SourceConfigStore) Create(_ context.Context, sc *sourceconfig.SourceConfig) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSourceConfigs)
		data, err := json.Marshal(sc)
		if err != nil {
			return err
		}
		return b.Put([]byte(sc.ID), data)
	})
}

func (s *SourceConfigStore) Update(_ context.Context, sc *sourceconfig.SourceConfig) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSourceConfigs)
		existing := b.Get([]byte(sc.ID))
		if existing == nil {
			return ErrSourceConfigNotFound
		}
		data, err := json.Marshal(sc)
		if err != nil {
			return err
		}
		return b.Put([]byte(sc.ID), data)
	})
}

func (s *SourceConfigStore) Delete(_ context.Context, id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSourceConfigs)
		return b.Delete([]byte(id))
	})
}
