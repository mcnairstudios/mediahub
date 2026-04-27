package bolt

import (
	"context"
	"encoding/json"

	"github.com/mcnairstudios/mediahub/pkg/auth"
	bbolt "go.etcd.io/bbolt"
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
		data := b.Get([]byte(id))
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

func (s *UserStore) GetByUsername(_ context.Context, username string) (*auth.User, error) {
	var user *auth.User

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		return b.ForEach(func(_, v []byte) error {
			var su boltStoredUser
			if err := json.Unmarshal(v, &su); err != nil {
				return err
			}
			if su.User.Username == username {
				u := su.User
				user = &u
			}
			return nil
		})
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
		return b.ForEach(func(_, v []byte) error {
			var su boltStoredUser
			if err := json.Unmarshal(v, &su); err != nil {
				return err
			}
			u := su.User
			result = append(result, &u)
			return nil
		})
	})

	return result, err
}

func (s *UserStore) Create(_ context.Context, user *auth.User) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)

		var duplicate bool
		err := b.ForEach(func(_, v []byte) error {
			var su boltStoredUser
			if err := json.Unmarshal(v, &su); err != nil {
				return err
			}
			if su.User.Username == user.Username {
				duplicate = true
			}
			return nil
		})
		if err != nil {
			return err
		}
		if duplicate {
			return auth.ErrUsernameExists
		}

		data, err := json.Marshal(boltStoredUser{User: *user})
		if err != nil {
			return err
		}
		return b.Put([]byte(user.ID), data)
	})
}

func (s *UserStore) Delete(_ context.Context, id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		data := b.Get([]byte(id))
		if data == nil {
			return auth.ErrUserNotFound
		}
		return b.Delete([]byte(id))
	})
}

func (s *UserStore) UpdatePassword(_ context.Context, id, hashedPassword string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		data := b.Get([]byte(id))
		if data == nil {
			return auth.ErrUserNotFound
		}
		var su boltStoredUser
		if err := json.Unmarshal(data, &su); err != nil {
			return err
		}
		su.PasswordHash = hashedPassword
		updated, err := json.Marshal(su)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), updated)
	})
}

func (s *UserStore) GetPasswordHash(_ context.Context, id string) (string, error) {
	var hash string

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		data := b.Get([]byte(id))
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
