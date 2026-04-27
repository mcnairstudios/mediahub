package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestJobRuns(t *testing.T) {
	var count atomic.Int64
	s := NewScheduler(nil)
	s.Add(Job{
		Name:     "counter",
		Interval: 10 * time.Millisecond,
		Fn: func(ctx context.Context) error {
			count.Add(1)
			return nil
		},
	})

	s.Start(context.Background())
	time.Sleep(50 * time.Millisecond)
	s.Stop()

	if c := count.Load(); c < 2 {
		t.Fatalf("expected job to run at least twice, got %d", c)
	}
}

func TestMultipleJobsRunIndependently(t *testing.T) {
	var a, b atomic.Int64
	s := NewScheduler(nil)
	s.Add(Job{
		Name:     "a",
		Interval: 10 * time.Millisecond,
		Fn: func(ctx context.Context) error {
			a.Add(1)
			return nil
		},
	})
	s.Add(Job{
		Name:     "b",
		Interval: 10 * time.Millisecond,
		Fn: func(ctx context.Context) error {
			b.Add(1)
			return nil
		},
	})

	s.Start(context.Background())
	time.Sleep(50 * time.Millisecond)
	s.Stop()

	if ac, bc := a.Load(), b.Load(); ac < 2 || bc < 2 {
		t.Fatalf("expected both jobs to run at least twice, got a=%d b=%d", ac, bc)
	}
}

func TestStopWaitsForCompletion(t *testing.T) {
	var done atomic.Bool
	s := NewScheduler(nil)
	s.Add(Job{
		Name:     "slow",
		Interval: time.Hour,
		Fn: func(ctx context.Context) error {
			time.Sleep(30 * time.Millisecond)
			done.Store(true)
			return nil
		},
	})

	s.Start(context.Background())
	s.Stop()

	if !done.Load() {
		t.Fatal("Stop returned before job completed")
	}
}

func TestJobErrorCallsLog(t *testing.T) {
	var loggedName string
	var loggedErr error
	logged := make(chan struct{}, 1)

	s := NewScheduler(func(name string, err error) {
		loggedName = name
		loggedErr = err
		select {
		case logged <- struct{}{}:
		default:
		}
	})

	testErr := errors.New("test error")
	s.Add(Job{
		Name:     "failing",
		Interval: 10 * time.Millisecond,
		Fn: func(ctx context.Context) error {
			return testErr
		},
	})

	s.Start(context.Background())

	select {
	case <-logged:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for error log")
	}

	s.Stop()

	if loggedName != "failing" {
		t.Fatalf("expected name 'failing', got %q", loggedName)
	}
	if !errors.Is(loggedErr, testErr) {
		t.Fatalf("expected test error, got %v", loggedErr)
	}
}

func TestRunningAndJobCount(t *testing.T) {
	s := NewScheduler(nil)

	if s.Running() {
		t.Fatal("expected not running before start")
	}
	if s.JobCount() != 0 {
		t.Fatalf("expected 0 jobs, got %d", s.JobCount())
	}

	s.Add(Job{
		Name:     "noop",
		Interval: time.Hour,
		Fn:       func(ctx context.Context) error { return nil },
	})

	if s.JobCount() != 1 {
		t.Fatalf("expected 1 job, got %d", s.JobCount())
	}

	s.Start(context.Background())

	if !s.Running() {
		t.Fatal("expected running after start")
	}

	s.Stop()

	if s.Running() {
		t.Fatal("expected not running after stop")
	}
}

func TestStartTwiceIsNoop(t *testing.T) {
	var count atomic.Int64
	s := NewScheduler(nil)
	s.Add(Job{
		Name:     "counter",
		Interval: 10 * time.Millisecond,
		Fn: func(ctx context.Context) error {
			count.Add(1)
			return nil
		},
	})

	s.Start(context.Background())
	s.Start(context.Background())
	time.Sleep(30 * time.Millisecond)
	s.Stop()

	if c := count.Load(); c < 1 {
		t.Fatal("job should have run")
	}
}

func TestStopBeforeStartIsSafe(t *testing.T) {
	s := NewScheduler(nil)
	s.Stop()

	if s.Running() {
		t.Fatal("should not be running")
	}
}
