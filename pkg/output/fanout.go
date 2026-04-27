package output

import (
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"
)

var errFanOutStopped = errors.New("fanout is stopped")

// FanOut distributes packets to multiple OutputPlugins. Errors from individual
// plugins are collected but do not prevent delivery to other plugins. Plugins
// can be added and removed at runtime (e.g., recording starts mid-stream).
type FanOut struct {
	plugins []OutputPlugin
	mu      sync.RWMutex
	stopped bool
}

// NewFanOut creates a FanOut with the given initial plugins.
func NewFanOut(plugins ...OutputPlugin) *FanOut {
	p := make([]OutputPlugin, len(plugins))
	copy(p, plugins)
	return &FanOut{plugins: p}
}

// PushVideo sends a video packet to all plugins. Returns the first error
// encountered, but always delivers to every plugin regardless of errors.
func (f *FanOut) PushVideo(data []byte, pts, dts int64, keyframe bool) error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.stopped {
		return errFanOutStopped
	}

	var firstErr error
	for _, p := range f.plugins {
		if err := safePushVideo(p, data, pts, dts, keyframe); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// PushAudio sends an audio packet to all plugins.
func (f *FanOut) PushAudio(data []byte, pts, dts int64) error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.stopped {
		return errFanOutStopped
	}

	var firstErr error
	for _, p := range f.plugins {
		if err := safePushAudio(p, data, pts, dts); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// PushSubtitle sends a subtitle packet to all plugins.
func (f *FanOut) PushSubtitle(data []byte, pts int64, duration int64) error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.stopped {
		return errFanOutStopped
	}

	var firstErr error
	for _, p := range f.plugins {
		if err := safePushSubtitle(p, data, pts, duration); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// EndOfStream signals end of stream to all plugins.
func (f *FanOut) EndOfStream() {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for _, p := range f.plugins {
		safeEndOfStream(p)
	}
}

// ResetForSeek signals a seek reset to all plugins.
func (f *FanOut) ResetForSeek() {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for _, p := range f.plugins {
		safeResetForSeek(p)
	}
}

// Stop stops all plugins and marks the FanOut as stopped. Subsequent pushes
// return an error.
func (f *FanOut) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.stopped = true
	for _, p := range f.plugins {
		safeStop(p)
	}
}

// Add appends a plugin to the FanOut. The plugin will receive all subsequent
// packets but not any previously sent ones.
func (f *FanOut) Add(p OutputPlugin) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.plugins = append(f.plugins, p)
}

// Remove removes and stops the first plugin matching the given mode.
func (f *FanOut) Remove(mode DeliveryMode) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i, p := range f.plugins {
		if p.Mode() == mode {
			p.Stop()
			f.plugins = append(f.plugins[:i], f.plugins[i+1:]...)
			return
		}
	}
}

// PluginCount returns the number of active plugins.
func (f *FanOut) PluginCount() int {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return len(f.plugins)
}

// Plugins returns a snapshot of the current plugins.
func (f *FanOut) Plugins() []OutputPlugin {
	f.mu.RLock()
	defer f.mu.RUnlock()

	out := make([]OutputPlugin, len(f.plugins))
	copy(out, f.plugins)
	return out
}

// Status returns the status of all active plugins.
func (f *FanOut) Status() []PluginStatus {
	f.mu.RLock()
	defer f.mu.RUnlock()

	statuses := make([]PluginStatus, len(f.plugins))
	for i, p := range f.plugins {
		statuses[i] = p.Status()
	}
	return statuses
}

func safePushVideo(p OutputPlugin, data []byte, pts, dts int64, keyframe bool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("fanout: panic in %s PushVideo: %v", p.Mode(), r)
			log.Error().Str("plugin", string(p.Mode())).Interface("panic", r).Msg("recovered panic in PushVideo")
		}
	}()
	return p.PushVideo(data, pts, dts, keyframe)
}

func safePushAudio(p OutputPlugin, data []byte, pts, dts int64) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("fanout: panic in %s PushAudio: %v", p.Mode(), r)
			log.Error().Str("plugin", string(p.Mode())).Interface("panic", r).Msg("recovered panic in PushAudio")
		}
	}()
	return p.PushAudio(data, pts, dts)
}

func safePushSubtitle(p OutputPlugin, data []byte, pts int64, duration int64) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("fanout: panic in %s PushSubtitle: %v", p.Mode(), r)
			log.Error().Str("plugin", string(p.Mode())).Interface("panic", r).Msg("recovered panic in PushSubtitle")
		}
	}()
	return p.PushSubtitle(data, pts, duration)
}

func safeEndOfStream(p OutputPlugin) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Str("plugin", string(p.Mode())).Interface("panic", r).Msg("recovered panic in EndOfStream")
		}
	}()
	p.EndOfStream()
}

func safeResetForSeek(p OutputPlugin) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Str("plugin", string(p.Mode())).Interface("panic", r).Msg("recovered panic in ResetForSeek")
		}
	}()
	p.ResetForSeek()
}

func safeStop(p OutputPlugin) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Str("plugin", string(p.Mode())).Interface("panic", r).Msg("recovered panic in Stop")
		}
	}()
	p.Stop()
}
