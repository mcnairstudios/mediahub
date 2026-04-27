package store

import (
	"fmt"
	"path/filepath"

	boltstore "github.com/mcnairstudios/mediahub/pkg/store/bolt"
)

type BackendType string

const (
	BackendMemory BackendType = "memory"
	BackendBolt   BackendType = "bolt"
)

type Factory struct {
	dataDir string
}

func NewFactory(dataDir string) *Factory {
	return &Factory{dataDir: dataDir}
}

func (f *Factory) StreamStore(backend BackendType) (StreamStore, error) {
	switch backend {
	case BackendMemory:
		return NewMemoryStreamStore(), nil
	case BackendBolt:
		db, err := boltstore.Open(filepath.Join(f.dataDir, "mediahub.db"))
		if err != nil {
			return nil, fmt.Errorf("open bolt db: %w", err)
		}
		return db.StreamStore(), nil
	default:
		return nil, fmt.Errorf("unknown backend: %q", backend)
	}
}

func (f *Factory) SettingsStore(backend BackendType) (SettingsStore, error) {
	switch backend {
	case BackendMemory:
		return NewMemorySettingsStore(), nil
	case BackendBolt:
		db, err := boltstore.Open(filepath.Join(f.dataDir, "mediahub.db"))
		if err != nil {
			return nil, fmt.Errorf("open bolt db: %w", err)
		}
		return db.SettingsStore(), nil
	default:
		return nil, fmt.Errorf("unknown backend: %q", backend)
	}
}
