package bolt

import (
	"context"
	"encoding/json"

	"github.com/mcnairstudios/mediahub/pkg/frontend/hdhr"
	bbolt "go.etcd.io/bbolt"
)

type HDHRDeviceStore struct {
	db *bbolt.DB
}

func (s *HDHRDeviceStore) Get(_ context.Context, id string) (*hdhr.Device, error) {
	var d *hdhr.Device

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketHDHRDevices)
		data := b.Get([]byte(id))
		if data == nil {
			return nil
		}
		d = &hdhr.Device{}
		return json.Unmarshal(data, d)
	})

	return d, err
}

func (s *HDHRDeviceStore) List(_ context.Context) ([]hdhr.Device, error) {
	var result []hdhr.Device

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketHDHRDevices)
		return b.ForEach(func(_, v []byte) error {
			var d hdhr.Device
			if err := json.Unmarshal(v, &d); err != nil {
				return err
			}
			result = append(result, d)
			return nil
		})
	})

	return result, err
}

func (s *HDHRDeviceStore) Create(_ context.Context, d *hdhr.Device) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketHDHRDevices)
		data, err := json.Marshal(d)
		if err != nil {
			return err
		}
		return b.Put([]byte(d.ID), data)
	})
}

func (s *HDHRDeviceStore) Update(_ context.Context, d *hdhr.Device) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketHDHRDevices)
		data, err := json.Marshal(d)
		if err != nil {
			return err
		}
		return b.Put([]byte(d.ID), data)
	})
}

func (s *HDHRDeviceStore) Delete(_ context.Context, id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketHDHRDevices)
		return b.Delete([]byte(id))
	})
}
