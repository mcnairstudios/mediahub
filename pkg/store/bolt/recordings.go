package bolt

import (
	"context"
	"encoding/json"

	"github.com/mcnairstudios/mediahub/pkg/recording"
	bbolt "go.etcd.io/bbolt"
)

type RecordingStore struct {
	db *bbolt.DB
}

func (s *RecordingStore) Get(_ context.Context, id string) (*recording.Recording, error) {
	var r *recording.Recording

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketRecordings)
		data := b.Get([]byte(id))
		if data == nil {
			return nil
		}
		r = &recording.Recording{}
		return json.Unmarshal(data, r)
	})

	return r, err
}

func (s *RecordingStore) List(_ context.Context, userID string, isAdmin bool) ([]recording.Recording, error) {
	var result []recording.Recording

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketRecordings)
		return b.ForEach(func(_, v []byte) error {
			var r recording.Recording
			if err := json.Unmarshal(v, &r); err != nil {
				return err
			}
			if isAdmin || r.UserID == userID {
				result = append(result, r)
			}
			return nil
		})
	})

	return result, err
}

func (s *RecordingStore) Create(_ context.Context, r *recording.Recording) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketRecordings)
		data, err := json.Marshal(r)
		if err != nil {
			return err
		}
		return b.Put([]byte(r.ID), data)
	})
}

func (s *RecordingStore) Update(_ context.Context, r *recording.Recording) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketRecordings)
		data, err := json.Marshal(r)
		if err != nil {
			return err
		}
		return b.Put([]byte(r.ID), data)
	})
}

func (s *RecordingStore) Delete(_ context.Context, id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketRecordings)
		return b.Delete([]byte(id))
	})
}

func (s *RecordingStore) ListByStatus(_ context.Context, status recording.Status) ([]recording.Recording, error) {
	var result []recording.Recording

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketRecordings)
		return b.ForEach(func(_, v []byte) error {
			var r recording.Recording
			if err := json.Unmarshal(v, &r); err != nil {
				return err
			}
			if r.Status == status {
				result = append(result, r)
			}
			return nil
		})
	})

	return result, err
}

func (s *RecordingStore) ListScheduled(_ context.Context) ([]recording.Recording, error) {
	var result []recording.Recording

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketRecordings)
		return b.ForEach(func(_, v []byte) error {
			var r recording.Recording
			if err := json.Unmarshal(v, &r); err != nil {
				return err
			}
			if r.Status == recording.StatusScheduled {
				result = append(result, r)
			}
			return nil
		})
	})

	return result, err
}
