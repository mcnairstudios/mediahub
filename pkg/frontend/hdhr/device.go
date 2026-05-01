package hdhr

import "context"

type Device struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	DeviceUUID string  `json:"device_uuid"`
	Port      int      `json:"port"`
	GroupIDs  []string `json:"group_ids,omitempty"`
	IsEnabled bool     `json:"is_enabled"`
	MaxChannels int    `json:"max_channels"`
}

type DeviceStore interface {
	Get(ctx context.Context, id string) (*Device, error)
	List(ctx context.Context) ([]Device, error)
	Create(ctx context.Context, d *Device) error
	Update(ctx context.Context, d *Device) error
	Delete(ctx context.Context, id string) error
}
