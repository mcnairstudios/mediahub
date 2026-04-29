package bolt

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/mcnairstudios/mediahub/pkg/sourceprofile"
	bbolt "go.etcd.io/bbolt"
)

var ErrSourceProfileNotFound = errors.New("source profile not found")

type SourceProfileStore struct {
	db *bbolt.DB
}

func (s *SourceProfileStore) Get(_ context.Context, id string) (*sourceprofile.Profile, error) {
	var p *sourceprofile.Profile

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSourceProfiles)
		data := b.Get([]byte(id))
		if data == nil {
			return nil
		}
		p = &sourceprofile.Profile{}
		return json.Unmarshal(data, p)
	})

	return p, err
}

func (s *SourceProfileStore) List(_ context.Context) ([]sourceprofile.Profile, error) {
	var result []sourceprofile.Profile

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSourceProfiles)
		return b.ForEach(func(_, v []byte) error {
			var p sourceprofile.Profile
			if err := json.Unmarshal(v, &p); err != nil {
				return err
			}
			result = append(result, p)
			return nil
		})
	})

	if result == nil {
		result = []sourceprofile.Profile{}
	}
	return result, err
}

func (s *SourceProfileStore) Create(_ context.Context, p *sourceprofile.Profile) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSourceProfiles)
		data, err := json.Marshal(p)
		if err != nil {
			return err
		}
		return b.Put([]byte(p.ID), data)
	})
}

func (s *SourceProfileStore) Update(_ context.Context, p *sourceprofile.Profile) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSourceProfiles)
		existing := b.Get([]byte(p.ID))
		if existing == nil {
			return ErrSourceProfileNotFound
		}
		data, err := json.Marshal(p)
		if err != nil {
			return err
		}
		return b.Put([]byte(p.ID), data)
	})
}

func (s *SourceProfileStore) Delete(_ context.Context, id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSourceProfiles)
		return b.Delete([]byte(id))
	})
}
