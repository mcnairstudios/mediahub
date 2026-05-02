package source

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// BaseSource provides shared state and methods for source plugins.
// Embed this in concrete source types to eliminate duplicated Info(),
// SetRefreshResult(), SetError(), Streams(), DeleteStreams(), and Clear()
// implementations.
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

// NewBaseSource creates a BaseSource with the given identity fields.
func NewBaseSource(id, name string, typ SourceType, isEnabled bool, maxStreams int) BaseSource {
	return BaseSource{
		id:        id,
		name:      name,
		typ:       typ,
		isEnabled: isEnabled,
		maxStreams: maxStreams,
	}
}

// Info returns the current source metadata under a read lock.
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

// Type returns the source type identifier.
func (b *BaseSource) Type() SourceType {
	return b.typ
}

// SetRefreshResult updates stream count and clears the error after a
// successful refresh.
func (b *BaseSource) SetRefreshResult(count int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.streamCount = count
	now := time.Now()
	b.lastRefreshed = &now
	b.lastError = ""
}

// SetRefreshed marks the source as refreshed without changing the stream count.
func (b *BaseSource) SetRefreshed() {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.lastRefreshed = &now
	b.lastError = ""
}

// SetError records an error string.
func (b *BaseSource) SetError(msg string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lastError = msg
}

// AddStreamCount atomically adds to the stream count (used by background
// episode fetchers).
func (b *BaseSource) AddStreamCount(delta int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.streamCount += delta
}

// ClearState resets stream count, error, and any extra state marker. Callers
// should delete streams from the store before calling this.
func (b *BaseSource) ClearState() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.streamCount = 0
	b.lastError = ""
}

// ID returns the source identifier.
func (b *BaseSource) ID() string { return b.id }

// Name returns the source name.
func (b *BaseSource) Name() string { return b.name }

// HTTPClientFor returns the WireGuard client if useWG is true and wgClient
// is non-nil, otherwise returns the default client. If defaultClient is nil,
// http.DefaultClient is returned.
func HTTPClientFor(defaultClient, wgClient *http.Client, useWG bool) *http.Client {
	if useWG && wgClient != nil {
		return wgClient
	}
	if defaultClient != nil {
		return defaultClient
	}
	return http.DefaultClient
}
