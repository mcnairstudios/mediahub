package scheduler

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	bbolt "go.etcd.io/bbolt"
)

func tempDB(t *testing.T) *bbolt.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatalf("open bolt: %v", err)
	}
	t.Cleanup(func() { db.Close(); os.Remove(path) })
	return db
}

func TestEveryRunsMultipleTimes(t *testing.T) {
	db := tempDB(t)
	var count atomic.Int64

	s := New(db, nil)
	s.Every("counter", "@every 1s", func(ctx context.Context) error {
		count.Add(1)
		return nil
	})

	s.Start(context.Background())
	time.Sleep(2500 * time.Millisecond)
	s.Stop()

	if c := count.Load(); c < 2 {
		t.Fatalf("expected at least 2 runs, got %d", c)
	}
}

func TestEveryErrorCallsLog(t *testing.T) {
	db := tempDB(t)
	var loggedName string
	logged := make(chan struct{}, 1)

	s := New(db, func(name string, err error) {
		loggedName = name
		select {
		case logged <- struct{}{}:
		default:
		}
	})
	s.Every("failing", "@every 1s", func(ctx context.Context) error {
		return context.DeadlineExceeded
	})

	s.Start(context.Background())
	select {
	case <-logged:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for error log")
	}
	s.Stop()

	if loggedName != "failing" {
		t.Fatalf("expected name 'failing', got %q", loggedName)
	}
}

func TestAtRunsImmediatelyWhenPast(t *testing.T) {
	db := tempDB(t)
	var ran atomic.Bool

	s := New(db, nil)
	s.At("past-job", time.Now().Add(-time.Minute), func(ctx context.Context) error {
		ran.Store(true)
		return nil
	})

	s.Start(context.Background())
	time.Sleep(100 * time.Millisecond)
	s.Stop()

	if !ran.Load() {
		t.Fatal("expected past At job to run immediately")
	}
}

func TestAtRunsAtScheduledTime(t *testing.T) {
	db := tempDB(t)
	var ran atomic.Bool

	s := New(db, nil)
	s.At("future-job", time.Now().Add(500*time.Millisecond), func(ctx context.Context) error {
		ran.Store(true)
		return nil
	})

	s.Start(context.Background())
	time.Sleep(200 * time.Millisecond)
	if ran.Load() {
		t.Fatal("At job ran too early")
	}
	time.Sleep(500 * time.Millisecond)
	s.Stop()

	if !ran.Load() {
		t.Fatal("At job did not run")
	}
}

func TestForRunsWithTimeout(t *testing.T) {
	db := tempDB(t)
	var expired atomic.Bool

	s := New(db, nil)
	s.Start(context.Background())

	s.For("timed-job", 200*time.Millisecond, func(ctx context.Context) error {
		<-ctx.Done()
		expired.Store(true)
		return nil
	})

	time.Sleep(400 * time.Millisecond)
	s.Stop()

	if !expired.Load() {
		t.Fatal("For job context should have expired")
	}
}

func TestBetweenStartsAndStops(t *testing.T) {
	db := tempDB(t)
	var started, stopped atomic.Bool

	s := New(db, nil)
	s.Start(context.Background())

	startAt := time.Now().Add(100 * time.Millisecond)
	stopAt := time.Now().Add(400 * time.Millisecond)

	s.Between("rec-1", startAt, stopAt,
		func(ctx context.Context) error {
			started.Store(true)
			return nil
		},
		func(ctx context.Context) error {
			stopped.Store(true)
			return nil
		},
	)

	time.Sleep(250 * time.Millisecond)
	if !started.Load() {
		t.Fatal("Between start should have fired")
	}
	if stopped.Load() {
		t.Fatal("Between stop should not have fired yet")
	}

	time.Sleep(300 * time.Millisecond)
	s.Stop()

	if !stopped.Load() {
		t.Fatal("Between stop should have fired")
	}
}

func TestBetweenPastStartRunsImmediately(t *testing.T) {
	db := tempDB(t)
	var started, stopped atomic.Bool

	s := New(db, nil)
	s.Start(context.Background())

	startAt := time.Now().Add(-time.Minute)
	stopAt := time.Now().Add(200 * time.Millisecond)

	s.Between("rec-past", startAt, stopAt,
		func(ctx context.Context) error {
			started.Store(true)
			return nil
		},
		func(ctx context.Context) error {
			stopped.Store(true)
			return nil
		},
	)

	time.Sleep(100 * time.Millisecond)
	if !started.Load() {
		t.Fatal("Between with past start should fire immediately")
	}

	time.Sleep(300 * time.Millisecond)
	s.Stop()

	if !stopped.Load() {
		t.Fatal("Between stop should have fired")
	}
}

func TestBetweenBothPastRunsBoth(t *testing.T) {
	db := tempDB(t)
	var started, stopped atomic.Bool

	s := New(db, nil)
	s.Start(context.Background())

	startAt := time.Now().Add(-2 * time.Minute)
	stopAt := time.Now().Add(-time.Minute)

	s.Between("rec-both-past", startAt, stopAt,
		func(ctx context.Context) error {
			started.Store(true)
			return nil
		},
		func(ctx context.Context) error {
			stopped.Store(true)
			return nil
		},
	)

	time.Sleep(200 * time.Millisecond)
	s.Stop()

	if !started.Load() {
		t.Fatal("start should have fired for past between")
	}
	if !stopped.Load() {
		t.Fatal("stop should have fired for past between")
	}
}

func TestRemoveCancelsBetween(t *testing.T) {
	db := tempDB(t)
	var stopped atomic.Bool

	s := New(db, nil)
	s.Start(context.Background())

	startAt := time.Now().Add(100 * time.Millisecond)
	stopAt := time.Now().Add(500 * time.Millisecond)

	s.Between("removable", startAt, stopAt,
		func(ctx context.Context) error { return nil },
		func(ctx context.Context) error {
			stopped.Store(true)
			return nil
		},
	)

	time.Sleep(50 * time.Millisecond)
	s.Remove("removable")
	time.Sleep(600 * time.Millisecond)
	s.Stop()

	if stopped.Load() {
		t.Fatal("stop should not fire after Remove")
	}
}

func TestListEntries(t *testing.T) {
	db := tempDB(t)
	s := New(db, nil)

	s.Every("refresh", "@every 1m", func(ctx context.Context) error { return nil })
	s.At("one-shot", time.Now().Add(time.Hour), func(ctx context.Context) error { return nil })
	s.Between("rec-1", time.Now(), time.Now().Add(time.Hour),
		func(ctx context.Context) error { return nil },
		func(ctx context.Context) error { return nil },
	)

	entries := s.List()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	types := map[string]bool{}
	for _, e := range entries {
		types[e.Type] = true
	}
	for _, expected := range []string{"recurring", "once", "between"} {
		if !types[expected] {
			t.Errorf("missing entry type %q", expected)
		}
	}
}

func TestStopBeforeStart(t *testing.T) {
	db := tempDB(t)
	s := New(db, nil)
	s.Stop()
}

func TestStartIdempotent(t *testing.T) {
	db := tempDB(t)
	var count atomic.Int64
	s := New(db, nil)
	s.Every("x", "@every 1s", func(ctx context.Context) error {
		count.Add(1)
		return nil
	})

	s.Start(context.Background())
	s.Start(context.Background())
	time.Sleep(1500 * time.Millisecond)
	s.Stop()

	if c := count.Load(); c > 3 {
		t.Fatalf("double start should be noop, got %d runs", c)
	}
}

func TestNilDBWorks(t *testing.T) {
	s := New(nil, nil)
	s.Every("no-db", "@every 1s", func(ctx context.Context) error { return nil })
	s.Start(context.Background())
	time.Sleep(1200 * time.Millisecond)
	s.Stop()
}

func TestConcurrentSafety(t *testing.T) {
	db := tempDB(t)
	s := New(db, nil)

	s.Every("bg", "@every 1s", func(ctx context.Context) error { return nil })
	s.Start(context.Background())

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.List()
		}()
	}
	wg.Wait()
	s.Stop()
}

func TestBoltPersistence(t *testing.T) {
	db := tempDB(t)
	var ran atomic.Bool

	s := New(db, nil)
	s.Every("persisted", "@every 1s", func(ctx context.Context) error {
		ran.Store(true)
		return nil
	})
	s.Start(context.Background())
	time.Sleep(1200 * time.Millisecond)
	s.Stop()

	if !ran.Load() {
		t.Fatal("job should have run")
	}

	e := s.loadEntry("cron:recurring:persisted")
	if e == nil {
		t.Fatal("expected persisted entry in bolt")
	}
	if e.LastRun.IsZero() {
		t.Fatal("expected non-zero last_run")
	}
}
