package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/mcnairstudios/mediahub/pkg/recording"
)

type StartFunc func(streamID, title string) error
type StopFunc func(streamID string) error

type Scheduler struct {
	store   recording.Store
	startFn StartFunc
	stopFn  StopFunc
	ticker  *time.Ticker
	cancel  context.CancelFunc
	mu      sync.Mutex
	running bool
	wg      sync.WaitGroup
}

func New(store recording.Store) *Scheduler {
	return &Scheduler{
		store: store,
	}
}

func (s *Scheduler) SetStartFunc(fn StartFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startFn = fn
}

func (s *Scheduler) SetStopFunc(fn StopFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopFn = fn
}

func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}

	ctx, s.cancel = context.WithCancel(ctx)
	s.ticker = time.NewTicker(30 * time.Second)
	s.running = true

	s.wg.Add(1)
	go s.loop(ctx)
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	cancel := s.cancel
	s.mu.Unlock()

	cancel()
	s.wg.Wait()

	s.mu.Lock()
	s.ticker.Stop()
	s.running = false
	s.mu.Unlock()
}

func (s *Scheduler) loop(ctx context.Context) {
	defer s.wg.Done()

	s.Tick(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.ticker.C:
			s.Tick(ctx)
		}
	}
}

func (s *Scheduler) Tick(ctx context.Context) {
	now := time.Now()

	s.startDueRecordings(ctx, now)
	s.stopExpiredRecordings(ctx, now)
}

func (s *Scheduler) startDueRecordings(ctx context.Context, now time.Time) {
	scheduled, err := s.store.ListScheduled(ctx)
	if err != nil {
		return
	}

	s.mu.Lock()
	startFn := s.startFn
	s.mu.Unlock()

	if startFn == nil {
		return
	}

	for i := range scheduled {
		rec := &scheduled[i]
		if !rec.ScheduledStart.IsZero() && !rec.ScheduledStart.After(now) {
			if err := startFn(rec.StreamID, rec.Title); err != nil {
				rec.Status = recording.StatusFailed
				s.store.Update(ctx, rec)
				continue
			}
			rec.Status = recording.StatusRecording
			rec.StartedAt = now
			s.store.Update(ctx, rec)
		}
	}
}

func (s *Scheduler) stopExpiredRecordings(ctx context.Context, now time.Time) {
	active, err := s.store.ListByStatus(ctx, recording.StatusRecording)
	if err != nil {
		return
	}

	s.mu.Lock()
	stopFn := s.stopFn
	s.mu.Unlock()

	if stopFn == nil {
		return
	}

	for i := range active {
		rec := &active[i]
		if !rec.ScheduledStop.IsZero() && !rec.ScheduledStop.After(now) {
			if err := stopFn(rec.StreamID); err != nil {
				rec.Status = recording.StatusFailed
				s.store.Update(ctx, rec)
				continue
			}
			rec.Status = recording.StatusCompleted
			rec.StoppedAt = now
			s.store.Update(ctx, rec)
		}
	}
}
