package bolt

import (
	"context"
	"encoding/json"

	"github.com/mcnairstudios/mediahub/pkg/channel"
	bbolt "go.etcd.io/bbolt"
)

type ChannelStore struct {
	db *bbolt.DB
}

func (s *ChannelStore) Get(_ context.Context, id string) (*channel.Channel, error) {
	var ch *channel.Channel

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChannels)
		data := b.Get([]byte(id))
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
		return b.ForEach(func(_, v []byte) error {
			var ch channel.Channel
			if err := json.Unmarshal(v, &ch); err != nil {
				return err
			}
			result = append(result, ch)
			return nil
		})
	})

	return result, err
}

func (s *ChannelStore) Create(_ context.Context, ch *channel.Channel) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChannels)
		data, err := json.Marshal(ch)
		if err != nil {
			return err
		}
		return b.Put([]byte(ch.ID), data)
	})
}

func (s *ChannelStore) Update(_ context.Context, ch *channel.Channel) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChannels)
		data, err := json.Marshal(ch)
		if err != nil {
			return err
		}
		return b.Put([]byte(ch.ID), data)
	})
}

func (s *ChannelStore) Delete(_ context.Context, id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChannels)
		return b.Delete([]byte(id))
	})
}

func (s *ChannelStore) AssignStreams(_ context.Context, channelID string, streamIDs []string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChannels)
		data := b.Get([]byte(channelID))
		if data == nil {
			return nil
		}
		var ch channel.Channel
		if err := json.Unmarshal(data, &ch); err != nil {
			return err
		}
		ch.StreamIDs = make([]string, len(streamIDs))
		copy(ch.StreamIDs, streamIDs)
		updated, err := json.Marshal(ch)
		if err != nil {
			return err
		}
		return b.Put([]byte(channelID), updated)
	})
}

func (s *ChannelStore) RemoveStreamMappings(_ context.Context, streamIDs []string) error {
	remove := make(map[string]struct{}, len(streamIDs))
	for _, id := range streamIDs {
		remove[id] = struct{}{}
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketChannels)

		var updates []channel.Channel
		err := b.ForEach(func(_, v []byte) error {
			var ch channel.Channel
			if err := json.Unmarshal(v, &ch); err != nil {
				return err
			}
			var filtered []string
			for _, sid := range ch.StreamIDs {
				if _, ok := remove[sid]; !ok {
					filtered = append(filtered, sid)
				}
			}
			ch.StreamIDs = filtered
			updates = append(updates, ch)
			return nil
		})
		if err != nil {
			return err
		}

		for _, ch := range updates {
			data, err := json.Marshal(ch)
			if err != nil {
				return err
			}
			if err := b.Put([]byte(ch.ID), data); err != nil {
				return err
			}
		}
		return nil
	})
}

type GroupStore struct {
	db *bbolt.DB
}

func (s *GroupStore) List(_ context.Context) ([]channel.Group, error) {
	var result []channel.Group

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketGroups)
		return b.ForEach(func(_, v []byte) error {
			var g channel.Group
			if err := json.Unmarshal(v, &g); err != nil {
				return err
			}
			result = append(result, g)
			return nil
		})
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
		return b.Put([]byte(g.ID), data)
	})
}

func (s *GroupStore) Delete(_ context.Context, id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketGroups)
		return b.Delete([]byte(id))
	})
}
