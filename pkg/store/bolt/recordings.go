package bolt

import (
	"bytes"
	"context"
	"encoding/json"
	"log"

	"github.com/mcnairstudios/mediahub/pkg/recording"
	"github.com/mcnairstudios/mediahub/pkg/store/bolt/keyenc"
	bbolt "go.etcd.io/bbolt"
)

type recordingKeyDef struct {
	Kind   string `key:"recordings"`
	Status string
	ID     string
}

var (
	recordingSchema    = keyenc.NewSchema[recordingKeyDef]()
	prefixRecording    = recordingSchema.Prefix(recordingKeyDef{})
	prefixRecordingIdx = keyenc.ReversePrefix("recordingidx")
)

type RecordingStore struct {
	db *bbolt.DB
}

func (s *RecordingStore) Get(_ context.Context, id string) (*recording.Recording, error) {
	var r *recording.Recording

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketRecordings)
		fullKey := b.Get(keyenc.Reverse("recordingidx", id))
		if fullKey == nil {
			return nil
		}
		data := b.Get(fullKey)
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
		c := b.Cursor()
		for k, v := c.Seek(prefixRecording); k != nil && bytes.HasPrefix(k, prefixRecording); k, v = c.Next() {
			var r recording.Recording
			if json.Unmarshal(v, &r) != nil {
				continue
			}
			if isAdmin || r.UserID == userID {
				result = append(result, r)
			}
		}
		return nil
	})

	return result, err
}

func recordingStatus(r *recording.Recording) string {
	if r.Status == "" {
		return "_none"
	}
	return string(r.Status)
}

func (s *RecordingStore) putRecording(b *bbolt.Bucket, r *recording.Recording) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	key := recordingSchema.Key(recordingKeyDef{
		Status: recordingStatus(r),
		ID:     r.ID,
	})
	if err := b.Put(key, data); err != nil {
		return err
	}
	return b.Put(keyenc.Reverse("recordingidx", r.ID), key)
}

func (s *RecordingStore) deleteRecording(b *bbolt.Bucket, id string) {
	idxKey := keyenc.Reverse("recordingidx", id)
	fullKey := b.Get(idxKey)
	if fullKey != nil {
		b.Delete(fullKey)
	}
	b.Delete(idxKey)
}

func (s *RecordingStore) Create(_ context.Context, r *recording.Recording) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketRecordings)
		return s.putRecording(b, r)
	})
}

func (s *RecordingStore) Update(_ context.Context, r *recording.Recording) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketRecordings)
		s.deleteRecording(b, r.ID)
		return s.putRecording(b, r)
	})
}

func (s *RecordingStore) Delete(_ context.Context, id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketRecordings)
		s.deleteRecording(b, id)
		return nil
	})
}

func (s *RecordingStore) ListByStatus(_ context.Context, status recording.Status) ([]recording.Recording, error) {
	var result []recording.Recording

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketRecordings)
		c := b.Cursor()
		prefix := recordingSchema.Prefix(recordingKeyDef{Status: string(status)})
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			var r recording.Recording
			if json.Unmarshal(v, &r) == nil {
				result = append(result, r)
			}
		}
		return nil
	})

	return result, err
}

func (s *RecordingStore) ListScheduled(_ context.Context) ([]recording.Recording, error) {
	var result []recording.Recording

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketRecordings)
		c := b.Cursor()
		prefix := recordingSchema.Prefix(recordingKeyDef{Status: string(recording.StatusScheduled)})
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			var r recording.Recording
			if json.Unmarshal(v, &r) == nil {
				result = append(result, r)
			}
		}
		return nil
	})

	return result, err
}

func (s *RecordingStore) migrateFromFlatKeys() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketRecordings)
		c := b.Cursor()

		k, _ := c.Seek(prefixRecordingIdx)
		if k != nil && bytes.HasPrefix(k, prefixRecordingIdx) {
			return nil
		}

		type migration struct {
			oldKey, newKey, idxKey, data []byte
		}
		var migrations []migration

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if bytes.HasPrefix(k, prefixRecording) || bytes.HasPrefix(k, prefixRecordingIdx) {
				continue
			}
			var r recording.Recording
			if json.Unmarshal(v, &r) != nil || r.ID == "" {
				continue
			}
			status := string(r.Status)
			if status == "" {
				status = "_none"
			}
			newKey := recordingSchema.Key(recordingKeyDef{
				Status: status,
				ID:     r.ID,
			})
			migrations = append(migrations, migration{
				oldKey: append([]byte{}, k...),
				newKey: newKey,
				idxKey: keyenc.Reverse("recordingidx", r.ID),
				data:   append([]byte{}, v...),
			})
		}

		if len(migrations) == 0 {
			return nil
		}

		log.Printf("recordings: migrating %d recordings to prefixed keys", len(migrations))
		for _, m := range migrations {
			b.Put(m.newKey, m.data)
			b.Put(m.idxKey, m.newKey)
			b.Delete(m.oldKey)
		}
		log.Printf("recordings: migration complete")
		return nil
	})
}
