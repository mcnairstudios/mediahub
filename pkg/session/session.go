package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"path/filepath"
	"sync"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/output"
)

type Session struct {
	ID         string
	StreamID   string
	StreamURL  string
	StreamName string
	OutputDir  string
	FanOut     *output.FanOut
	CreatedAt  time.Time
	Delivery   string

	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	seekFunc func(posMs int64)
	recorded bool
	closers  []io.Closer
}

func newSession(ctx context.Context, cancel context.CancelFunc, streamID, streamURL, streamName, outputDir string) *Session {
	return &Session{
		ID:         generateID(),
		StreamID:   streamID,
		StreamURL:  streamURL,
		StreamName: streamName,
		OutputDir:  filepath.Join(outputDir, streamID),
		FanOut:     output.NewFanOut(),
		CreatedAt:  time.Now(),
		ctx:        ctx,
		cancel:     cancel,
		done:       make(chan struct{}),
	}
}

func (s *Session) Context() context.Context {
	return s.ctx
}

func (s *Session) AddCloser(c io.Closer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closers = append(s.closers, c)
}

func (s *Session) SetRecorded(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recorded = v
}

func (s *Session) IsRecorded() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recorded
}

func (s *Session) Stop() {
	s.cancel()
	s.FanOut.Stop()
	s.mu.Lock()
	closers := s.closers
	s.closers = nil
	s.mu.Unlock()
	for _, c := range closers {
		c.Close()
	}
	close(s.done)
}

func (s *Session) Done() <-chan struct{} {
	return s.done
}

func (s *Session) SetSeekFunc(fn func(posMs int64)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seekFunc = fn
}

func (s *Session) Seek(posMs int64) {
	s.mu.Lock()
	fn := s.seekFunc
	s.mu.Unlock()

	if fn != nil {
		fn(posMs)
	}
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
