package bolt

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/store/bolt/keyenc"
	bbolt "go.etcd.io/bbolt"
)

type userKeyDef struct {
	Kind   string `key:"users"`
	UserID string
}

var (
	userSchema    = keyenc.NewSchema[userKeyDef]()
	prefixUser    = userSchema.Prefix(userKeyDef{})
	prefixUserIdx = keyenc.ReversePrefix("useridx")
)

type boltStoredUser struct {
	User         auth.User `json:"user"`
	PasswordHash string    `json:"password_hash"`
}

type UserStore struct {
	db *bbolt.DB
}

func (s *UserStore) Get(_ context.Context, id string) (*auth.User, error) {
	var user *auth.User

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		data := s.getData(b, id)
		if data == nil {
			return auth.ErrUserNotFound
		}
		var su boltStoredUser
		if err := json.Unmarshal(data, &su); err != nil {
			return err
		}
		u := su.User
		user = &u
		return nil
	})

	return user, err
}

func (s *UserStore) getData(b *bbolt.Bucket, id string) []byte {
	key := userSchema.Key(userKeyDef{UserID: id})
	data := b.Get(key)
	if data != nil {
		return data
	}
	return b.Get([]byte(id))
}

func (s *UserStore) GetByUsername(_ context.Context, username string) (*auth.User, error) {
	var user *auth.User

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		c := b.Cursor()
		for k, v := c.Seek(prefixUser); k != nil && bytes.HasPrefix(k, prefixUser); k, v = c.Next() {
			var su boltStoredUser
			if err := json.Unmarshal(v, &su); err != nil {
				return err
			}
			if su.User.Username == username {
				u := su.User
				user = &u
				return nil
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, auth.ErrUserNotFound
	}
	return user, nil
}

func (s *UserStore) GetByEmail(_ context.Context, email string) (*auth.User, error) {
	var user *auth.User

	lower := strings.ToLower(email)
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		c := b.Cursor()
		for k, v := c.Seek(prefixUser); k != nil && bytes.HasPrefix(k, prefixUser); k, v = c.Next() {
			var su boltStoredUser
			if err := json.Unmarshal(v, &su); err != nil {
				return err
			}
			if strings.ToLower(su.User.Email) == lower {
				u := su.User
				user = &u
				return nil
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, auth.ErrUserNotFound
	}
	return user, nil
}

func (s *UserStore) List(_ context.Context) ([]*auth.User, error) {
	var result []*auth.User

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		c := b.Cursor()
		for k, v := c.Seek(prefixUser); k != nil && bytes.HasPrefix(k, prefixUser); k, v = c.Next() {
			var su boltStoredUser
			if err := json.Unmarshal(v, &su); err != nil {
				return err
			}
			u := su.User
			result = append(result, &u)
		}
		return nil
	})

	return result, err
}

func (s *UserStore) putUser(b *bbolt.Bucket, su boltStoredUser) error {
	data, err := json.Marshal(su)
	if err != nil {
		return err
	}
	key := userSchema.Key(userKeyDef{UserID: su.User.ID})
	return b.Put(key, data)
}

func (s *UserStore) Create(_ context.Context, user *auth.User) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)

		var duplicate bool
		c := b.Cursor()
		for k, v := c.Seek(prefixUser); k != nil && bytes.HasPrefix(k, prefixUser); k, v = c.Next() {
			var su boltStoredUser
			if err := json.Unmarshal(v, &su); err != nil {
				return err
			}
			if su.User.Username == user.Username {
				duplicate = true
				break
			}
		}
		if duplicate {
			return auth.ErrUsernameExists
		}

		return s.putUser(b, boltStoredUser{User: *user})
	})
}

func (s *UserStore) Update(_ context.Context, user *auth.User) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		data := s.getData(b, user.ID)
		if data == nil {
			return auth.ErrUserNotFound
		}

		var existing bool
		c := b.Cursor()
		for k, v := c.Seek(prefixUser); k != nil && bytes.HasPrefix(k, prefixUser); k, v = c.Next() {
			var su boltStoredUser
			if err := json.Unmarshal(v, &su); err != nil {
				return err
			}
			if su.User.Username == user.Username && su.User.ID != user.ID {
				existing = true
				break
			}
		}
		if existing {
			return auth.ErrUsernameExists
		}

		var su boltStoredUser
		if err := json.Unmarshal(data, &su); err != nil {
			return err
		}
		su.User = *user
		return s.putUser(b, su)
	})
}

func (s *UserStore) Delete(_ context.Context, id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		key := userSchema.Key(userKeyDef{UserID: id})
		data := b.Get(key)
		if data == nil {
			data = b.Get([]byte(id))
			if data == nil {
				return auth.ErrUserNotFound
			}
			b.Delete([]byte(id))
			return nil
		}
		return b.Delete(key)
	})
}

func (s *UserStore) UpdatePassword(_ context.Context, id, hashedPassword string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		data := s.getData(b, id)
		if data == nil {
			return auth.ErrUserNotFound
		}
		var su boltStoredUser
		if err := json.Unmarshal(data, &su); err != nil {
			return err
		}
		su.PasswordHash = hashedPassword
		return s.putUser(b, su)
	})
}

func (s *UserStore) GetPasswordHash(_ context.Context, id string) (string, error) {
	var hash string

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		data := s.getData(b, id)
		if data == nil {
			return auth.ErrUserNotFound
		}
		var su boltStoredUser
		if err := json.Unmarshal(data, &su); err != nil {
			return err
		}
		hash = su.PasswordHash
		return nil
	})

	return hash, err
}

func (s *UserStore) migrateFromFlatKeys() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		c := b.Cursor()

		k, _ := c.Seek(prefixUser)
		if k != nil && bytes.HasPrefix(k, prefixUser) {
			return nil
		}

		type migration struct {
			oldKey, newKey, data []byte
		}
		var migrations []migration

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if bytes.HasPrefix(k, prefixUser) || bytes.HasPrefix(k, prefixUserIdx) {
				continue
			}
			var su boltStoredUser
			if json.Unmarshal(v, &su) != nil || su.User.ID == "" {
				continue
			}
			newKey := userSchema.Key(userKeyDef{UserID: su.User.ID})
			migrations = append(migrations, migration{
				oldKey: append([]byte{}, k...),
				newKey: newKey,
				data:   append([]byte{}, v...),
			})
		}

		if len(migrations) == 0 {
			return nil
		}

		log.Printf("users: migrating %d users to prefixed keys", len(migrations))
		for _, m := range migrations {
			b.Put(m.newKey, m.data)
			b.Delete(m.oldKey)
		}
		log.Printf("users: migration complete")
		return nil
	})
}
