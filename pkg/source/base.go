package source

import (
	"context"
	"net/http"
	"sync"
	"time"
)

type BaseSource struct {
	mu            sync.RWMutex
	id            string
	name          string
	typ           SourceType
	isEnabled     bool
	streamCount   int
	lastRefreshed *time.Time
	lastError     string
	maxStreams    int
}

func NewBaseSource(id, name string, typ SourceType, isEnabled bool, maxStreams int) BaseSource {
	return BaseSource{
		id:        id,
		name:      name,
		typ:       typ,
		isEnabled: isEnabled,
		maxStreams: maxStreams,
	}
}

func (b *BaseSource) Info(_ context.Context) SourceInfo {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return SourceInfo{
		ID:                  b.id,
		Type:                b.typ,
		Name:                b.name,
		IsEnabled:           b.isEnabled,
		StreamCount:         b.streamCount,
		LastRefreshed:       b.lastRefreshed,
		LastError:           b.lastError,
		MaxConcurrentStreams: b.maxStreams,
	}
}

func (b *BaseSource) Type() SourceType {
	return b.typ
}

func (b *BaseSource) SetRefreshResult(count int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.streamCount = count
	now := time.Now()
	b.lastRefreshed = &now
	b.lastError = ""
}

func (b *BaseSource) SetRefreshed() {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.lastRefreshed = &now
	b.lastError = ""
}

func (b *BaseSource) SetError(msg string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lastError = msg
}

func (b *BaseSource) AddStreamCount(delta int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.streamCount += delta
}

func (b *BaseSource) ClearState() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.streamCount = 0
	b.lastError = ""
}

func (b *BaseSource) ID() string { return b.id }

func (b *BaseSource) Name() string { return b.name }

func HTTPClientFor(defaultClient, wgClient *http.Client, useWG bool) *http.Client {
	if useWG && wgClient != nil {
		return wgClient
	}
	if defaultClient != nil {
		return defaultClient
	}
	return http.DefaultClient
}
