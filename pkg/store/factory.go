package store

import (
	"fmt"
	"path/filepath"

	"github.com/mcnairstudios/mediahub/pkg/auth"
	"github.com/mcnairstudios/mediahub/pkg/channel"
	"github.com/mcnairstudios/mediahub/pkg/epg"
	"github.com/mcnairstudios/mediahub/pkg/recording"
	"github.com/mcnairstudios/mediahub/pkg/sourceconfig"
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

func (f *Factory) ChannelStore(backend BackendType) (channel.Store, error) {
	switch backend {
	case BackendMemory:
		return NewMemoryChannelStore(), nil
	case BackendBolt:
		db, err := boltstore.Open(filepath.Join(f.dataDir, "mediahub.db"))
		if err != nil {
			return nil, fmt.Errorf("open bolt db: %w", err)
		}
		return db.ChannelStore(), nil
	default:
		return nil, fmt.Errorf("unknown backend: %q", backend)
	}
}

func (f *Factory) GroupStore(backend BackendType) (channel.GroupStore, error) {
	switch backend {
	case BackendMemory:
		return NewMemoryGroupStore(), nil
	case BackendBolt:
		db, err := boltstore.Open(filepath.Join(f.dataDir, "mediahub.db"))
		if err != nil {
			return nil, fmt.Errorf("open bolt db: %w", err)
		}
		return db.GroupStore(), nil
	default:
		return nil, fmt.Errorf("unknown backend: %q", backend)
	}
}

func (f *Factory) EPGSourceStore(backend BackendType) (epg.SourceStore, error) {
	switch backend {
	case BackendMemory:
		return NewMemoryEPGSourceStore(), nil
	case BackendBolt:
		db, err := boltstore.Open(filepath.Join(f.dataDir, "mediahub.db"))
		if err != nil {
			return nil, fmt.Errorf("open bolt db: %w", err)
		}
		return db.EPGSourceStore(), nil
	default:
		return nil, fmt.Errorf("unknown backend: %q", backend)
	}
}

func (f *Factory) ProgramStore(backend BackendType) (epg.ProgramStore, error) {
	switch backend {
	case BackendMemory:
		return NewMemoryProgramStore(), nil
	case BackendBolt:
		db, err := boltstore.Open(filepath.Join(f.dataDir, "mediahub.db"))
		if err != nil {
			return nil, fmt.Errorf("open bolt db: %w", err)
		}
		return db.ProgramStore(), nil
	default:
		return nil, fmt.Errorf("unknown backend: %q", backend)
	}
}

func (f *Factory) RecordingStore(backend BackendType) (recording.Store, error) {
	switch backend {
	case BackendMemory:
		return NewMemoryRecordingStore(), nil
	case BackendBolt:
		db, err := boltstore.Open(filepath.Join(f.dataDir, "mediahub.db"))
		if err != nil {
			return nil, fmt.Errorf("open bolt db: %w", err)
		}
		return db.RecordingStore(), nil
	default:
		return nil, fmt.Errorf("unknown backend: %q", backend)
	}
}

func (f *Factory) SourceConfigStore(backend BackendType) (sourceconfig.Store, error) {
	switch backend {
	case BackendMemory:
		return sourceconfig.NewMemoryStore(), nil
	case BackendBolt:
		db, err := boltstore.Open(filepath.Join(f.dataDir, "mediahub.db"))
		if err != nil {
			return nil, fmt.Errorf("open bolt db: %w", err)
		}
		return db.SourceConfigStore(), nil
	default:
		return nil, fmt.Errorf("unknown backend: %q", backend)
	}
}

func (f *Factory) UserStore(backend BackendType) (auth.UserStore, error) {
	switch backend {
	case BackendMemory:
		return auth.NewMemoryUserStore(), nil
	case BackendBolt:
		db, err := boltstore.Open(filepath.Join(f.dataDir, "mediahub.db"))
		if err != nil {
			return nil, fmt.Errorf("open bolt db: %w", err)
		}
		return db.UserStore(), nil
	default:
		return nil, fmt.Errorf("unknown backend: %q", backend)
	}
}
