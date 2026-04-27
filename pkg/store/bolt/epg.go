package bolt

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/epg"
	bbolt "go.etcd.io/bbolt"
)

type EPGSourceStore struct {
	db *bbolt.DB
}

func (s *EPGSourceStore) Get(_ context.Context, id string) (*epg.Source, error) {
	var src *epg.Source

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEPGSources)
		data := b.Get([]byte(id))
		if data == nil {
			return nil
		}
		src = &epg.Source{}
		return json.Unmarshal(data, src)
	})

	return src, err
}

func (s *EPGSourceStore) List(_ context.Context) ([]epg.Source, error) {
	var result []epg.Source

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEPGSources)
		return b.ForEach(func(_, v []byte) error {
			var src epg.Source
			if err := json.Unmarshal(v, &src); err != nil {
				return err
			}
			result = append(result, src)
			return nil
		})
	})

	return result, err
}

func (s *EPGSourceStore) Create(_ context.Context, src *epg.Source) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEPGSources)
		data, err := json.Marshal(src)
		if err != nil {
			return err
		}
		return b.Put([]byte(src.ID), data)
	})
}

func (s *EPGSourceStore) Update(_ context.Context, src *epg.Source) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEPGSources)
		data, err := json.Marshal(src)
		if err != nil {
			return err
		}
		return b.Put([]byte(src.ID), data)
	})
}

func (s *EPGSourceStore) Delete(_ context.Context, id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEPGSources)
		return b.Delete([]byte(id))
	})
}

type ProgramStore struct {
	db *bbolt.DB
}

func programKey(channelID string, start time.Time) []byte {
	return []byte(channelID + "|" + start.UTC().Format(time.RFC3339Nano))
}

func (s *ProgramStore) NowPlaying(_ context.Context, channelID string) (*epg.Program, error) {
	var found *epg.Program

	now := time.Now()
	prefix := []byte(channelID + "|")

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEPGPrograms)
		c := b.Cursor()

		for k, v := c.Seek(prefix); k != nil && hasPrefix(k, prefix); k, v = c.Next() {
			var p epg.Program
			if err := json.Unmarshal(v, &p); err != nil {
				return err
			}
			if !now.Before(p.StartTime) && now.Before(p.EndTime) {
				found = &p
				return nil
			}
		}
		return nil
	})

	return found, err
}

func (s *ProgramStore) Range(_ context.Context, channelID string, start, end time.Time) ([]epg.Program, error) {
	var result []epg.Program

	prefix := []byte(channelID + "|")

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEPGPrograms)
		c := b.Cursor()

		for k, v := c.Seek(prefix); k != nil && hasPrefix(k, prefix); k, v = c.Next() {
			var p epg.Program
			if err := json.Unmarshal(v, &p); err != nil {
				return err
			}
			if p.StartTime.Before(end) && p.EndTime.After(start) {
				result = append(result, p)
			}
		}
		return nil
	})

	return result, err
}

func (s *ProgramStore) BulkInsert(_ context.Context, programs []epg.Program) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEPGPrograms)
		for _, p := range programs {
			data, err := json.Marshal(p)
			if err != nil {
				return err
			}
			if err := b.Put(programKey(p.ChannelID, p.StartTime), data); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *ProgramStore) DeleteBySource(_ context.Context, sourceID string) error {
	prefix := []byte(sourceID + "|")

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEPGPrograms)
		c := b.Cursor()

		var toDelete [][]byte
		for k, _ := c.Seek(prefix); k != nil && hasPrefix(k, prefix); k, _ = c.Next() {
			keyCopy := make([]byte, len(k))
			copy(keyCopy, k)
			toDelete = append(toDelete, keyCopy)
		}

		for _, k := range toDelete {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}

func hasPrefix(key, prefix []byte) bool {
	return strings.HasPrefix(string(key), string(prefix))
}
