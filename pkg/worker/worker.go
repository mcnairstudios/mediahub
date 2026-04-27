package worker

import (
	"context"
	"sync"
	"time"
)

type Job struct {
	Name     string
	Interval time.Duration
	Fn       func(ctx context.Context) error
}

type Scheduler struct {
	jobs    []Job
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
	mu      sync.Mutex
	log     func(name string, err error)
}

func NewScheduler(log func(string, error)) *Scheduler {
	return &Scheduler{
		log: log,
	}
}

func (s *Scheduler) Add(job Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs = append(s.jobs, job)
}

func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}

	ctx, s.cancel = context.WithCancel(ctx)
	s.running = true

	for _, job := range s.jobs {
		s.wg.Add(1)
		go s.run(ctx, job)
	}
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
	s.running = false
	s.mu.Unlock()
}

func (s *Scheduler) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Scheduler) JobCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.jobs)
}

func (s *Scheduler) run(ctx context.Context, job Job) {
	defer s.wg.Done()

	s.exec(ctx, job)

	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.exec(ctx, job)
		}
	}
}

func (s *Scheduler) exec(ctx context.Context, job Job) {
	if err := job.Fn(ctx); err != nil && s.log != nil {
		s.log(job.Name, err)
	}
}
