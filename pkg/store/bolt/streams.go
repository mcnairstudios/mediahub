package bolt

import (
	"bytes"
	"context"
	"encoding/json"
	"log"

	"github.com/mcnairstudios/mediahub/pkg/media"
	"github.com/mcnairstudios/mediahub/pkg/store/bolt/keyenc"
	bbolt "go.etcd.io/bbolt"
)

type streamKeyDef struct {
	Kind       string `key:"streams"`
	SourceType string
	SourceID   string
	VODType    string
	StreamID   string
}

const vodTypeLive = "_live"

func vodTypeKey(vodType string) string {
	if vodType == "" {
		return vodTypeLive
	}
	return vodType
}

var (
	streamSchema = keyenc.NewSchema[streamKeyDef]()
	prefixStream = streamSchema.Prefix(streamKeyDef{})
	prefixIdx    = keyenc.ReversePrefix("streamidx")
)

type StreamStore struct {
	db *bbolt.DB
}

func (s *StreamStore) Get(_ context.Context, id string) (*media.Stream, error) {
	var stream *media.Stream

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketStreams)
		fullKey := b.Get(keyenc.Reverse("streamidx", id))
		var data []byte
		if fullKey != nil {
			data = b.Get(fullKey)
		}
		if data == nil {
			data = b.Get([]byte(id))
		}
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
		return b.ForEach(func(k, v []byte) error {
			if bytes.HasPrefix(k, prefixIdx) {
				return nil
			}
			var stream media.Stream
			if json.Unmarshal(v, &stream) == nil {
				result = append(result, stream)
			}
			return nil
		})
	})

	return result, err
}

func (s *StreamStore) ListBySource(_ context.Context, sourceType, sourceID string) ([]media.Stream, error) {
	var result []media.Stream

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketStreams)
		c := b.Cursor()
		prefix := streamSchema.Prefix(streamKeyDef{SourceType: sourceType, SourceID: sourceID})
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			var stream media.Stream
			if json.Unmarshal(v, &stream) == nil {
				result = append(result, stream)
			}
		}
		if len(result) > 0 {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			if bytes.HasPrefix(k, prefixStream) || bytes.HasPrefix(k, prefixIdx) {
				return nil
			}
			var stream media.Stream
			if json.Unmarshal(v, &stream) == nil && stream.SourceType == sourceType && stream.SourceID == sourceID {
				result = append(result, stream)
			}
			return nil
		})
	})

	return result, err
}

func (s *StreamStore) ListBySourceAndType(_ context.Context, sourceType, sourceID, vodType string) ([]media.Stream, error) {
	var result []media.Stream
	target := vodTypeKey(vodType)

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketStreams)
		c := b.Cursor()
		prefix := streamSchema.Prefix(streamKeyDef{SourceType: sourceType, SourceID: sourceID, VODType: target})
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			var stream media.Stream
			if json.Unmarshal(v, &stream) == nil {
				result = append(result, stream)
			}
		}
		if len(result) > 0 {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			if bytes.HasPrefix(k, prefixStream) || bytes.HasPrefix(k, prefixIdx) {
				return nil
			}
			var stream media.Stream
			if json.Unmarshal(v, &stream) == nil && stream.SourceType == sourceType && stream.SourceID == sourceID && vodTypeKey(stream.VODType) == target {
				result = append(result, stream)
			}
			return nil
		})
	})

	return result, err
}

func (s *StreamStore) ListByVODType(_ context.Context, vodType string) ([]media.Stream, error) {
	var result []media.Stream
	target := vodTypeKey(vodType)

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketStreams)
		c := b.Cursor()
		for k, v := c.Seek(prefixStream); k != nil && bytes.HasPrefix(k, prefixStream); k, v = c.Next() {
			parsed := streamSchema.Parse(k)
			if parsed.VODType != target {
				continue
			}
			var stream media.Stream
			if json.Unmarshal(v, &stream) == nil {
				result = append(result, stream)
			}
		}
		if len(result) > 0 {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			if bytes.HasPrefix(k, prefixStream) || bytes.HasPrefix(k, prefixIdx) {
				return nil
			}
			var stream media.Stream
			if json.Unmarshal(v, &stream) == nil && vodTypeKey(stream.VODType) == target {
				result = append(result, stream)
			}
			return nil
		})
	})

	return result, err
}

func (s *StreamStore) CountBySourceAndType(_ context.Context, sourceType, sourceID, vodType string) (int, error) {
	count := 0
	err := s.db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket(bucketStreams).Cursor()
		prefix := streamSchema.Prefix(streamKeyDef{SourceType: sourceType, SourceID: sourceID, VODType: vodTypeKey(vodType)})
		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			count++
		}
		return nil
	})
	return count, err
}

func (s *StreamStore) CountBySource(_ context.Context, sourceType, sourceID string) (int, error) {
	count := 0
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketStreams)
		c := b.Cursor()
		prefix := streamSchema.Prefix(streamKeyDef{SourceType: sourceType, SourceID: sourceID})
		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			count++
		}
		if count > 0 {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			if bytes.HasPrefix(k, prefixStream) || bytes.HasPrefix(k, prefixIdx) {
				return nil
			}
			var partial struct {
				SourceType string `json:"source_type"`
				SourceID   string `json:"source_id"`
			}
			if json.Unmarshal(v, &partial) == nil && partial.SourceType == sourceType && partial.SourceID == sourceID {
				count++
			}
			return nil
		})
	})
	return count, err
}

func (s *StreamStore) BulkUpsert(_ context.Context, streams []media.Stream) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketStreams)
		for _, stream := range streams {
			data, err := json.Marshal(stream)
			if err != nil {
				return err
			}
			key := streamSchema.Key(streamKeyDef{
				SourceType: stream.SourceType,
				SourceID:   stream.SourceID,
				VODType:    vodTypeKey(stream.VODType),
				StreamID:   stream.ID,
			})
			if err := b.Put(key, data); err != nil {
				return err
			}
			if err := b.Put(keyenc.Reverse("streamidx", stream.ID), key); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *StreamStore) DeleteBySource(_ context.Context, sourceType, sourceID string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketStreams)
		c := b.Cursor()
		prefix := streamSchema.Prefix(streamKeyDef{SourceType: sourceType, SourceID: sourceID})

		var toDelete []struct{ key, id []byte }
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			var stream media.Stream
			if json.Unmarshal(v, &stream) == nil {
				toDelete = append(toDelete, struct{ key, id []byte }{
					key: append([]byte{}, k...),
					id:  []byte(stream.ID),
				})
			}
		}

		for _, d := range toDelete {
			b.Delete(d.key)
			b.Delete(keyenc.Reverse("streamidx", string(d.id)))
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
		c := b.Cursor()
		prefix := streamSchema.Prefix(streamKeyDef{SourceType: sourceType, SourceID: sourceID})

		var toDelete []struct{ key, id []byte }
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			var stream media.Stream
			if json.Unmarshal(v, &stream) != nil {
				continue
			}
			if _, ok := keep[stream.ID]; !ok {
				toDelete = append(toDelete, struct{ key, id []byte }{
					key: append([]byte{}, k...),
					id:  []byte(stream.ID),
				})
				deleted = append(deleted, stream.ID)
			}
		}

		for _, d := range toDelete {
			b.Delete(d.key)
			b.Delete(keyenc.Reverse("streamidx", string(d.id)))
		}
		return nil
	})

	return deleted, err
}

func (s *StreamStore) Save() error {
	return nil
}

func (s *StreamStore) MigrateFromFlatKeys() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketStreams)
		c := b.Cursor()

		type migration struct {
			oldKey, newKey, idxKey, data []byte
		}
		var migrations []migration

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if bytes.HasPrefix(k, prefixIdx) {
				continue
			}
			if bytes.HasPrefix(k, prefixStream) {
				parts := bytes.Split(k, []byte(":"))
				if len(parts) == 4 {
					var stream media.Stream
					if json.Unmarshal(v, &stream) != nil || stream.SourceType == "" || stream.SourceID == "" || stream.ID == "" {
						continue
					}
					newKey := streamSchema.Key(streamKeyDef{
						SourceType: stream.SourceType,
						SourceID:   stream.SourceID,
						VODType:    vodTypeKey(stream.VODType),
						StreamID:   stream.ID,
					})
					migrations = append(migrations, migration{
						oldKey: append([]byte{}, k...),
						newKey: newKey,
						idxKey: keyenc.Reverse("streamidx", stream.ID),
						data:   append([]byte{}, v...),
					})
				}
				continue
			}
			var stream media.Stream
			if json.Unmarshal(v, &stream) != nil || stream.SourceType == "" || stream.SourceID == "" || stream.ID == "" {
				continue
			}
			newKey := streamSchema.Key(streamKeyDef{
				SourceType: stream.SourceType,
				SourceID:   stream.SourceID,
				VODType:    vodTypeKey(stream.VODType),
				StreamID:   stream.ID,
			})
			migrations = append(migrations, migration{
				oldKey: append([]byte{}, k...),
				newKey: newKey,
				idxKey: keyenc.Reverse("streamidx", stream.ID),
				data:   append([]byte{}, v...),
			})
		}

		if len(migrations) == 0 {
			return nil
		}

		log.Printf("streams: migrating %d streams to 5-segment keys", len(migrations))
		for _, m := range migrations {
			b.Put(m.newKey, m.data)
			b.Put(m.idxKey, m.newKey)
			if !bytes.Equal(m.oldKey, m.newKey) {
				b.Delete(m.oldKey)
			}
		}
		log.Printf("streams: migration complete")
		return nil
	})
}
