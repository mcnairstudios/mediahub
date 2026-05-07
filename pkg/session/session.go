package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"path/filepath"
	"sync"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/av/demux"
	"github.com/mcnairstudios/mediahub/pkg/av/subtitle"
	"github.com/mcnairstudios/mediahub/pkg/media"
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

	Subtitles *subtitle.Collector

	demuxer   *demux.Demuxer
	probeInfo *media.ProbeResult

	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	stopOnce sync.Once
	seekFunc func(posMs int64)
	recorded bool
	closers  []io.Closer
	err      error
	finished bool

	// WebRTC SDP negotiation: pipeline deferred until WHEP offer arrives
	onSDPOffer       func(sdp string) (negotiatedCodec string, err error)
	pipelineDeferred bool
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
	s.stopOnce.Do(func() {
		s.cancel()
		s.waitFinished()
		s.FanOut.Stop()
		s.mu.Lock()
		closers := s.closers
		s.closers = nil
		s.mu.Unlock()
		for _, c := range closers {
			c.Close()
		}
		close(s.done)
	})
}

func (s *Session) waitFinished() {
	deadline := time.After(30 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		s.mu.Lock()
		done := s.finished
		s.mu.Unlock()
		if done {
			return
		}
		select {
		case <-deadline:
			return
		case <-ticker.C:
		}
	}
}

func (s *Session) Done() <-chan struct{} {
	return s.done
}

func (s *Session) SetSeekFunc(fn func(posMs int64)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seekFunc = fn
}

func (s *Session) SeekTo(posMs int64) {
	s.mu.Lock()
	fn := s.seekFunc
	s.mu.Unlock()

	if fn != nil {
		fn(posMs)
	}
}

func (s *Session) SetError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

func (s *Session) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *Session) MarkDone() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finished = true
}

func (s *Session) ClearFinished() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finished = false
	s.err = nil
}

func (s *Session) DrainClosers() []io.Closer {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.closers
	s.closers = nil
	return c
}

func (s *Session) IsFinished() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.finished
}

func (s *Session) SetDemuxer(d *demux.Demuxer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.demuxer = d
}

func (s *Session) Demuxer() *demux.Demuxer {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.demuxer
}

func (s *Session) SetProbeInfo(info *media.ProbeResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.probeInfo = info
}

func (s *Session) ProbeInfo() *media.ProbeResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.probeInfo
}

// SetOnSDPOffer registers a callback invoked when the browser sends a WHEP
// offer. The callback receives the raw SDP, starts the pipeline with the
// negotiated codec, and returns the chosen video codec name.
func (s *Session) SetOnSDPOffer(fn func(sdp string) (string, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onSDPOffer = fn
	s.pipelineDeferred = true
}

// OnSDPOffer invokes the registered SDP callback. Returns ("", nil) if none set.
func (s *Session) OnSDPOffer(sdp string) (string, error) {
	s.mu.Lock()
	fn := s.onSDPOffer
	s.mu.Unlock()
	if fn == nil {
		return "", nil
	}
	return fn(sdp)
}

// IsPipelineDeferred returns true if the pipeline start is deferred (WebRTC).
func (s *Session) IsPipelineDeferred() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pipelineDeferred
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
