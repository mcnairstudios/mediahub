package bolt

import (
	"bytes"
	"context"
	"encoding/json"
	"log"

	"github.com/mcnairstudios/mediahub/pkg/channel"
	"github.com/mcnairstudios/mediahub/pkg/store/bolt/keyenc"
	bbolt "go.etcd.io/bbolt"
)

type channelKeyDef struct {
	Kind      string `key:"channels"`
	GroupID   string
	ChannelID string
}

var (
	channelSchema    = keyenc.NewSchema[channelKeyDef]()
	prefixChannel    = channelSchema.Prefix(channelKeyDef{})
	prefixChannelIdx = keyenc.ReversePrefix("channelidx")
)

type ChannelStore struct {
	db *bbolt.DB
}

func (s *ChannelStore) Get(_ context.Context, id string) (*channel.Channel, error) {
	var ch *channel.Channel

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChannels)
		fullKey := b.Get(keyenc.Reverse("channelidx", id))
		if fullKey == nil {
			return nil
		}
		data := b.Get(fullKey)
		if data == nil {
			return nil
		}
		ch = &channel.Channel{}
		return json.Unmarshal(data, ch)
	})

	return ch, err
}

func (s *ChannelStore) List(_ context.Context) ([]channel.Channel, error) {
	var result []channel.Channel

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChannels)
		c := b.Cursor()
		for k, v := c.Seek(prefixChannel); k != nil && bytes.HasPrefix(k, prefixChannel); k, v = c.Next() {
			var ch channel.Channel
			if json.Unmarshal(v, &ch) == nil {
				result = append(result, ch)
			}
		}
		return nil
	})

	return result, err
}

func (s *ChannelStore) ListByGroup(_ context.Context, groupID string) ([]channel.Channel, error) {
	var result []channel.Channel

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChannels)
		c := b.Cursor()
		prefix := channelSchema.Prefix(channelKeyDef{GroupID: groupID})
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			var ch channel.Channel
			if json.Unmarshal(v, &ch) == nil {
				result = append(result, ch)
			}
		}
		return nil
	})

	return result, err
}

func (s *ChannelStore) channelGroupID(ch *channel.Channel) string {
	if ch.GroupID != "" {
		return ch.GroupID
	}
	return "_ungrouped"
}

func (s *ChannelStore) putChannel(b *bbolt.Bucket, ch *channel.Channel) error {
	data, err := json.Marshal(ch)
	if err != nil {
		return err
	}
	key := channelSchema.Key(channelKeyDef{
		GroupID:   s.channelGroupID(ch),
		ChannelID: ch.ID,
	})
	if err := b.Put(key, data); err != nil {
		return err
	}
	return b.Put(keyenc.Reverse("channelidx", ch.ID), key)
}

func (s *ChannelStore) deleteChannel(b *bbolt.Bucket, id string) {
	idxKey := keyenc.Reverse("channelidx", id)
	fullKey := b.Get(idxKey)
	if fullKey != nil {
		b.Delete(fullKey)
	}
	b.Delete(idxKey)
}

func (s *ChannelStore) Create(_ context.Context, ch *channel.Channel) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChannels)
		return s.putChannel(b, ch)
	})
}

func (s *ChannelStore) Update(_ context.Context, ch *channel.Channel) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChannels)
		s.deleteChannel(b, ch.ID)
		return s.putChannel(b, ch)
	})
}

func (s *ChannelStore) Delete(_ context.Context, id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChannels)
		s.deleteChannel(b, id)
		return nil
	})
}

func (s *ChannelStore) AssignStreams(_ context.Context, channelID string, streamIDs []string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChannels)
		fullKey := b.Get(keyenc.Reverse("channelidx", channelID))
		if fullKey == nil {
			return nil
		}
		data := b.Get(fullKey)
		if data == nil {
			return nil
		}
		var ch channel.Channel
		if err := json.Unmarshal(data, &ch); err != nil {
			return err
		}
		ch.StreamIDs = make([]string, len(streamIDs))
		copy(ch.StreamIDs, streamIDs)
		return s.putChannel(b, &ch)
	})
}

func (s *ChannelStore) RemoveStreamMappings(_ context.Context, streamIDs []string) error {
	remove := make(map[string]struct{}, len(streamIDs))
	for _, id := range streamIDs {
		remove[id] = struct{}{}
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChannels)
		c := b.Cursor()

		var updates []channel.Channel
		for k, v := c.Seek(prefixChannel); k != nil && bytes.HasPrefix(k, prefixChannel); k, v = c.Next() {
			var ch channel.Channel
			if json.Unmarshal(v, &ch) != nil {
				continue
			}
			var filtered []string
			for _, sid := range ch.StreamIDs {
				if _, ok := remove[sid]; !ok {
					filtered = append(filtered, sid)
				}
			}
			ch.StreamIDs = filtered
			updates = append(updates, ch)
		}

		for _, ch := range updates {
			if err := s.putChannel(b, &ch); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *ChannelStore) migrateFromFlatKeys() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChannels)
		c := b.Cursor()

		k, _ := c.Seek(prefixChannelIdx)
		if k != nil && bytes.HasPrefix(k, prefixChannelIdx) {
			return nil
		}

		type migration struct {
			oldKey, newKey, idxKey, data []byte
		}
		var migrations []migration

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if bytes.HasPrefix(k, prefixChannel) || bytes.HasPrefix(k, prefixChannelIdx) {
				continue
			}
			var ch channel.Channel
			if json.Unmarshal(v, &ch) != nil || ch.ID == "" {
				continue
			}
			groupID := ch.GroupID
			if groupID == "" {
				groupID = "_ungrouped"
			}
			newKey := channelSchema.Key(channelKeyDef{
				GroupID:   groupID,
				ChannelID: ch.ID,
			})
			migrations = append(migrations, migration{
				oldKey: append([]byte{}, k...),
				newKey: newKey,
				idxKey: keyenc.Reverse("channelidx", ch.ID),
				data:   append([]byte{}, v...),
			})
		}

		if len(migrations) == 0 {
			return nil
		}

		log.Printf("channels: migrating %d channels to prefixed keys", len(migrations))
		for _, m := range migrations {
			b.Put(m.newKey, m.data)
			b.Put(m.idxKey, m.newKey)
			b.Delete(m.oldKey)
		}
		log.Printf("channels: migration complete")
		return nil
	})
}

type groupKeyDef struct {
	Kind    string `key:"groups"`
	GroupID string
}

var (
	groupSchema = keyenc.NewSchema[groupKeyDef]()
	prefixGroup = groupSchema.Prefix(groupKeyDef{})
)

type GroupStore struct {
	db *bbolt.DB
}

func (s *GroupStore) List(_ context.Context) ([]channel.Group, error) {
	var result []channel.Group

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketGroups)
		c := b.Cursor()
		for k, v := c.Seek(prefixGroup); k != nil && bytes.HasPrefix(k, prefixGroup); k, v = c.Next() {
			var g channel.Group
			if json.Unmarshal(v, &g) == nil {
				result = append(result, g)
			}
		}
		return nil
	})

	return result, err
}

func (s *GroupStore) Create(_ context.Context, g *channel.Group) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketGroups)
		data, err := json.Marshal(g)
		if err != nil {
			return err
		}
		key := groupSchema.Key(groupKeyDef{GroupID: g.ID})
		return b.Put(key, data)
	})
}

func (s *GroupStore) Delete(_ context.Context, id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketGroups)
		key := groupSchema.Key(groupKeyDef{GroupID: id})
		return b.Delete(key)
	})
}

func (s *GroupStore) migrateFromFlatKeys() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketGroups)
		c := b.Cursor()

		k, _ := c.Seek(prefixGroup)
		if k != nil && bytes.HasPrefix(k, prefixGroup) {
			return nil
		}

		type migration struct {
			oldKey, newKey, data []byte
		}
		var migrations []migration

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if bytes.HasPrefix(k, prefixGroup) {
				continue
			}
			var g channel.Group
			if json.Unmarshal(v, &g) != nil || g.ID == "" {
				continue
			}
			newKey := groupSchema.Key(groupKeyDef{GroupID: g.ID})
			migrations = append(migrations, migration{
				oldKey: append([]byte{}, k...),
				newKey: newKey,
				data:   append([]byte{}, v...),
			})
		}

		if len(migrations) == 0 {
			return nil
		}

		log.Printf("groups: migrating %d groups to prefixed keys", len(migrations))
		for _, m := range migrations {
			b.Put(m.newKey, m.data)
			b.Delete(m.oldKey)
		}
		log.Printf("groups: migration complete")
		return nil
	})
}
