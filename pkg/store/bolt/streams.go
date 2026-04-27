package bolt

import (
	"context"
	"encoding/json"

	"github.com/mcnairstudios/mediahub/pkg/media"
	bbolt "go.etcd.io/bbolt"
)

type StreamStore struct {
	db *bbolt.DB
}

func (s *StreamStore) Get(_ context.Context, id string) (*media.Stream, error) {
	var stream *media.Stream

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketStreams)
		data := b.Get([]byte(id))
		if data == nil {
			return nil
		}
		stream = &media.Stream{}
		return json.Unmarshal(data, stream)
	})

	return stream, err
}

func (s *StreamStore) List(_ context.Context) ([]media.Stream, error) {
	var result []media.Stream

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketStreams)
		return b.ForEach(func(_, v []byte) error {
			var stream media.Stream
			if err := json.Unmarshal(v, &stream); err != nil {
				return err
			}
			result = append(result, stream)
			return nil
		})
	})

	return result, err
}

func (s *StreamStore) ListBySource(_ context.Context, sourceType, sourceID string) ([]media.Stream, error) {
	var result []media.Stream

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketStreams)
		return b.ForEach(func(_, v []byte) error {
			var stream media.Stream
			if err := json.Unmarshal(v, &stream); err != nil {
				return err
			}
			if stream.SourceType == sourceType && stream.SourceID == sourceID {
				result = append(result, stream)
			}
			return nil
		})
	})

	return result, err
}

func (s *StreamStore) BulkUpsert(_ context.Context, streams []media.Stream) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketStreams)
		for _, stream := range streams {
			data, err := json.Marshal(stream)
			if err != nil {
				return err
			}
			if err := b.Put([]byte(stream.ID), data); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *StreamStore) DeleteBySource(_ context.Context, sourceType, sourceID string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketStreams)

		var toDelete [][]byte
		err := b.ForEach(func(k, v []byte) error {
			var stream media.Stream
			if err := json.Unmarshal(v, &stream); err != nil {
				return err
			}
			if stream.SourceType == sourceType && stream.SourceID == sourceID {
				toDelete = append(toDelete, k)
			}
			return nil
		})
		if err != nil {
			return err
		}

		for _, k := range toDelete {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *StreamStore) DeleteStaleBySource(_ context.Context, sourceType, sourceID string, keepIDs []string) ([]string, error) {
	keep := make(map[string]struct{}, len(keepIDs))
	for _, id := range keepIDs {
		keep[id] = struct{}{}
	}

	var deleted []string

	err := s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketStreams)

		var toDelete [][]byte
		err := b.ForEach(func(k, v []byte) error {
			var stream media.Stream
			if err := json.Unmarshal(v, &stream); err != nil {
				return err
			}
			if stream.SourceType != sourceType || stream.SourceID != sourceID {
				return nil
			}
			if _, ok := keep[stream.ID]; !ok {
				toDelete = append(toDelete, k)
				deleted = append(deleted, stream.ID)
			}
			return nil
		})
		if err != nil {
			return err
		}

		for _, k := range toDelete {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})

	return deleted, err
}

func (s *StreamStore) Save() error {
	return nil
}
