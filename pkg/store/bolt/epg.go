package bolt

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/epg"
	"github.com/mcnairstudios/mediahub/pkg/store/bolt/keyenc"
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

type programKeyDef struct {
	Kind      string `key:"programs"`
	ChannelID string
	StartTime string
}

var (
	programSchema = keyenc.NewSchema[programKeyDef]()
	prefixProgram = programSchema.Prefix(programKeyDef{})
)

type ProgramStore struct {
	db *bbolt.DB
}

func (s *ProgramStore) NowPlaying(_ context.Context, channelID string) (*epg.Program, error) {
	var found *epg.Program

	now := time.Now()
	prefix := programSchema.Prefix(programKeyDef{ChannelID: channelID})

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEPGPrograms)
		c := b.Cursor()

		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
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

	prefix := programSchema.Prefix(programKeyDef{ChannelID: channelID})

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEPGPrograms)
		c := b.Cursor()

		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
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

func (s *ProgramStore) ListAll(_ context.Context) ([]epg.Program, error) {
	var result []epg.Program

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEPGPrograms)
		c := b.Cursor()
		for k, v := c.Seek(prefixProgram); k != nil && bytes.HasPrefix(k, prefixProgram); k, v = c.Next() {
			var p epg.Program
			if err := json.Unmarshal(v, &p); err != nil {
				return err
			}
			result = append(result, p)
		}
		return nil
	})

	return result, err
}

func (s *ProgramStore) ListChannelIDs(_ context.Context) ([]string, error) {
	seen := make(map[string]struct{})
	var result []string

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEPGPrograms)
		c := b.Cursor()
		for k, v := c.Seek(prefixProgram); k != nil && bytes.HasPrefix(k, prefixProgram); k, v = c.Next() {
			var p epg.Program
			if json.Unmarshal(v, &p) == nil {
				if _, ok := seen[p.ChannelID]; !ok {
					seen[p.ChannelID] = struct{}{}
					result = append(result, p.ChannelID)
				}
			}
		}
		return nil
	})

	return result, err
}

func (s *ProgramStore) BulkInsert(_ context.Context, programs []epg.Program) error {
	const batchSize = 5000
	for i := 0; i < len(programs); i += batchSize {
		end := i + batchSize
		if end > len(programs) {
			end = len(programs)
		}
		batch := programs[i:end]
		if err := s.db.Update(func(tx *bbolt.Tx) error {
			b := tx.Bucket(bucketEPGPrograms)
			for _, p := range batch {
				data, err := json.Marshal(p)
				if err != nil {
					return err
				}
				key := programSchema.Key(programKeyDef{
					ChannelID: p.ChannelID,
					StartTime: p.StartTime.UTC().Format(time.RFC3339Nano),
				})
				if err := b.Put(key, data); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *ProgramStore) DeleteBySource(_ context.Context, sourceID string) error {
	prefix := programSchema.Prefix(programKeyDef{ChannelID: sourceID})

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEPGPrograms)
		c := b.Cursor()

		var toDelete [][]byte
		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			toDelete = append(toDelete, append([]byte{}, k...))
		}

		for _, k := range toDelete {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *ProgramStore) migrateFromPipeKeys() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEPGPrograms)
		c := b.Cursor()

		k, _ := c.Seek(prefixProgram)
		if k != nil && bytes.HasPrefix(k, prefixProgram) {
			return nil
		}

		type migration struct {
			oldKey, newKey, data []byte
		}
		var migrations []migration

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if bytes.HasPrefix(k, prefixProgram) {
				continue
			}
			ks := string(k)
			pipeIdx := -1
			for i, ch := range ks {
				if ch == '|' {
					pipeIdx = i
					break
				}
			}
			if pipeIdx < 0 {
				continue
			}
			channelID := ks[:pipeIdx]
			startTime := ks[pipeIdx+1:]
			if channelID == "" || startTime == "" {
				continue
			}

			newKey := programSchema.Key(programKeyDef{
				ChannelID: channelID,
				StartTime: startTime,
			})
			migrations = append(migrations, migration{
				oldKey: append([]byte{}, k...),
				newKey: newKey,
				data:   append([]byte{}, v...),
			})
		}

		if len(migrations) == 0 {
			return nil
		}

		log.Printf("epg programs: migrating %d programs to prefixed keys", len(migrations))
		for _, m := range migrations {
			b.Put(m.newKey, m.data)
			b.Delete(m.oldKey)
		}
		log.Printf("epg programs: migration complete")
		return nil
	})
}
