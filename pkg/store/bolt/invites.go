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

type inviteKeyDef struct {
	Kind  string `key:"invites"`
	Token string
}

var (
	inviteSchema = keyenc.NewSchema[inviteKeyDef]()
	prefixInvite = inviteSchema.Prefix(inviteKeyDef{})
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
		key := inviteSchema.Key(inviteKeyDef{Token: invite.Token})
		return b.Put(key, data)
	})
}

func (s *InviteStore) Get(_ context.Context, token string) (*auth.Invite, error) {
	var invite *auth.Invite
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketInvites)
		key := inviteSchema.Key(inviteKeyDef{Token: token})
		data := b.Get(key)
		if data == nil {
			data = b.Get([]byte(token))
		}
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
		c := b.Cursor()
		for k, v := c.Seek(prefixInvite); k != nil && bytes.HasPrefix(k, prefixInvite); k, v = c.Next() {
			var inv auth.Invite
			if err := json.Unmarshal(v, &inv); err != nil {
				return err
			}
			result = append(result, &inv)
		}
		return nil
	})
	return result, err
}

func (s *InviteStore) Update(_ context.Context, invite *auth.Invite) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketInvites)
		key := inviteSchema.Key(inviteKeyDef{Token: invite.Token})
		if b.Get(key) == nil {
			if b.Get([]byte(invite.Token)) == nil {
				return auth.ErrInviteNotFound
			}
			b.Delete([]byte(invite.Token))
		}
		data, err := json.Marshal(invite)
		if err != nil {
			return err
		}
		return b.Put(key, data)
	})
}

func (s *InviteStore) Delete(_ context.Context, token string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketInvites)
		key := inviteSchema.Key(inviteKeyDef{Token: token})
		if b.Get(key) != nil {
			return b.Delete(key)
		}
		if b.Get([]byte(token)) != nil {
			return b.Delete([]byte(token))
		}
		return auth.ErrInviteNotFound
	})
}

func (s *InviteStore) migrateFromFlatKeys() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketInvites)
		c := b.Cursor()

		k, _ := c.Seek(prefixInvite)
		if k != nil && bytes.HasPrefix(k, prefixInvite) {
			return nil
		}

		type migration struct {
			oldKey, newKey, data []byte
		}
		var migrations []migration

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if bytes.HasPrefix(k, prefixInvite) {
				continue
			}
			var inv auth.Invite
			if json.Unmarshal(v, &inv) != nil || inv.Token == "" {
				continue
			}
			newKey := inviteSchema.Key(inviteKeyDef{Token: inv.Token})
			migrations = append(migrations, migration{
				oldKey: append([]byte{}, k...),
				newKey: newKey,
				data:   append([]byte{}, v...),
			})
		}

		if len(migrations) == 0 {
			return nil
		}

		log.Printf("invites: migrating %d invites to prefixed keys", len(migrations))
		for _, m := range migrations {
			b.Put(m.newKey, m.data)
			b.Delete(m.oldKey)
		}
		log.Printf("invites: migration complete")
		return nil
	})
}
