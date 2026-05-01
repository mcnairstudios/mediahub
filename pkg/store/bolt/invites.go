package bolt

import (
	"context"
	"encoding/json"

	"github.com/mcnairstudios/mediahub/pkg/auth"
	bbolt "go.etcd.io/bbolt"
)

type InviteStore struct {
	db *bbolt.DB
}

func (s *InviteStore) Create(_ context.Context, invite *auth.Invite) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketInvites)
		data, err := json.Marshal(invite)
		if err != nil {
			return err
		}
		return b.Put([]byte(invite.Token), data)
	})
}

func (s *InviteStore) Get(_ context.Context, token string) (*auth.Invite, error) {
	var invite *auth.Invite
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketInvites)
		data := b.Get([]byte(token))
		if data == nil {
			return auth.ErrInviteNotFound
		}
		var inv auth.Invite
		if err := json.Unmarshal(data, &inv); err != nil {
			return err
		}
		invite = &inv
		return nil
	})
	return invite, err
}

func (s *InviteStore) List(_ context.Context) ([]*auth.Invite, error) {
	var result []*auth.Invite
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketInvites)
		return b.ForEach(func(_, v []byte) error {
			var inv auth.Invite
			if err := json.Unmarshal(v, &inv); err != nil {
				return err
			}
			result = append(result, &inv)
			return nil
		})
	})
	return result, err
}

func (s *InviteStore) Update(_ context.Context, invite *auth.Invite) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketInvites)
		if b.Get([]byte(invite.Token)) == nil {
			return auth.ErrInviteNotFound
		}
		data, err := json.Marshal(invite)
		if err != nil {
			return err
		}
		return b.Put([]byte(invite.Token), data)
	})
}

func (s *InviteStore) Delete(_ context.Context, token string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketInvites)
		if b.Get([]byte(token)) == nil {
			return auth.ErrInviteNotFound
		}
		return b.Delete([]byte(token))
	})
}
