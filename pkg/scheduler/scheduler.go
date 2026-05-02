package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	bbolt "go.etcd.io/bbolt"
)

var bucketScheduler = []byte("scheduler")

type Entry struct {
	Name    string    `json:"name"`
	Type    string    `json:"type"`
	Spec    string    `json:"spec,omitempty"`
	At      time.Time `json:"at,omitempty"`
	Start   time.Time `json:"start,omitempty"`
	Stop    time.Time `json:"stop,omitempty"`
	Done    bool      `json:"done,omitempty"`
	Started bool      `json:"started,omitempty"`
	Stopped bool      `json:"stopped,omitempty"`
	LastRun time.Time `json:"last_run,omitempty"`
}

type recurringJob struct {
	name    string
	spec    string
	fn      func(ctx context.Context) error
	entryID cron.EntryID
}

type onceJob struct {
	name string
	at   time.Time
	fn   func(ctx context.Context) error
}

type betweenJob struct {
	name    string
	start   time.Time
	stop    time.Time
	startFn func(ctx context.Context) error
	stopFn  func(ctx context.Context) error
}

type forJob struct {
	name     string
	duration time.Duration
	fn       func(ctx context.Context) error
}

type Scheduler struct {
	db   *bbolt.DB
	log  func(string, error)
	cron *cron.Cron

	mu          sync.Mutex
	recurring   []recurringJob
	once        []onceJob
	between     []betweenJob
	forJobs     []forJob
	running     bool
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	runningJobs map[string]context.CancelFunc
}

func New(db *bbolt.DB, log func(string, error)) *Scheduler {
	return &Scheduler{
		db:          db,
		log:         log,
		cron:        cron.New(cron.WithSeconds()),
		runningJobs: make(map[string]context.CancelFunc),
	}
}

func (s *Scheduler) Every(name string, spec string, fn func(ctx context.Context) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recurring = append(s.recurring, recurringJob{name: name, spec: spec, fn: fn})
}

func (s *Scheduler) At(name string, at time.Time, fn func(ctx context.Context) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.once = append(s.once, onceJob{name: name, at: at, fn: fn})
	s.persistOnce(name, at, false)
}

func (s *Scheduler) For(name string, duration time.Duration, fn func(ctx context.Context) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.forJobs = append(s.forJobs, forJob{name: name, duration: duration, fn: fn})

	if s.running {
		s.startForJob(name, duration, fn)
	}
}

func (s *Scheduler) Between(name string, start, stop time.Time, startFn, stopFn func(ctx context.Context) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.between = append(s.between, betweenJob{name: name, start: start, stop: stop, startFn: startFn, stopFn: stopFn})
	s.persistBetween(name, start, stop, false, false)

	if s.running {
		s.startBetweenJob(name, start, stop, startFn, stopFn)
	}
}

func (s *Scheduler) Remove(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, j := range s.recurring {
		if j.name == name {
			if j.entryID != 0 {
				s.cron.Remove(j.entryID)
			}
			s.recurring = append(s.recurring[:i], s.recurring[i+1:]...)
			break
		}
	}
	for i, j := range s.once {
		if j.name == name {
			s.once = append(s.once[:i], s.once[i+1:]...)
			break
		}
	}
	for i, j := range s.between {
		if j.name == name {
			s.between = append(s.between[:i], s.between[i+1:]...)
			break
		}
	}
	for i, j := range s.forJobs {
		if j.name == name {
			s.forJobs = append(s.forJobs[:i], s.forJobs[i+1:]...)
			break
		}
	}

	if cancelFn, ok := s.runningJobs[name]; ok {
		cancelFn()
		delete(s.runningJobs, name)
	}

	s.deleteEntry(name)
}

func (s *Scheduler) List() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()

	var entries []Entry
	for _, j := range s.recurring {
		e := Entry{Name: j.name, Type: "recurring", Spec: j.spec}
		if persisted := s.loadEntry("cron:recurring:" + j.name); persisted != nil {
			e.LastRun = persisted.LastRun
		}
		entries = append(entries, e)
	}
	for _, j := range s.once {
		e := Entry{Name: j.name, Type: "once", At: j.at}
		if persisted := s.loadEntry("cron:once:" + j.name); persisted != nil {
			e.Done = persisted.Done
		}
		entries = append(entries, e)
	}
	for _, j := range s.between {
		e := Entry{Name: j.name, Type: "between", Start: j.start, Stop: j.stop}
		if persisted := s.loadEntry("cron:between:" + j.name); persisted != nil {
			e.Started = persisted.Started
			e.Stopped = persisted.Stopped
		}
		entries = append(entries, e)
	}
	return entries
}

func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}

	ctx, s.cancel = context.WithCancel(ctx)
	s.running = true

	for i := range s.recurring {
		j := &s.recurring[i]
		fn := j.fn
		name := j.name
		id, err := s.cron.AddFunc(j.spec, func() {
			if err := fn(ctx); err != nil && s.log != nil {
				s.log(name, err)
			}
			s.persistRecurring(name, j.spec)
		})
		if err != nil && s.log != nil {
			s.log(name, fmt.Errorf("failed to schedule: %w", err))
		}
		j.entryID = id
	}

	s.cron.Start()

	for _, j := range s.once {
		s.startOnceJob(ctx, j.name, j.at, j.fn)
	}

	for _, j := range s.between {
		s.startBetweenJob(j.name, j.start, j.stop, j.startFn, j.stopFn)
	}

	for _, j := range s.forJobs {
		s.startForJob(j.name, j.duration, j.fn)
	}

	s.recoverPersistedJobs(ctx)
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	cancel := s.cancel
	s.mu.Unlock()

	cronCtx := s.cron.Stop()
	<-cronCtx.Done()

	cancel()

	s.mu.Lock()
	for name, cancelFn := range s.runningJobs {
		cancelFn()
		delete(s.runningJobs, name)
	}
	s.mu.Unlock()

	s.wg.Wait()

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
}

func (s *Scheduler) startOnceJob(ctx context.Context, name string, at time.Time, fn func(ctx context.Context) error) {
	now := time.Now()
	if at.Before(now) || at.Equal(now) {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			if err := fn(ctx); err != nil && s.log != nil {
				s.log(name, err)
			}
			s.markOnceDone(name)
		}()
		return
	}

	delay := time.Until(at)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if err := fn(ctx); err != nil && s.log != nil {
				s.log(name, err)
			}
			s.markOnceDone(name)
		}
	}()
}

func (s *Scheduler) startBetweenJob(name string, start, stop time.Time, startFn, stopFn func(ctx context.Context) error) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		s.mu.Lock()
		if !s.running {
			s.mu.Unlock()
			return
		}
		jobCtx, jobCancel := context.WithCancel(context.Background())
		s.runningJobs[name] = jobCancel
		s.mu.Unlock()

		defer func() {
			jobCancel()
			s.mu.Lock()
			delete(s.runningJobs, name)
			s.mu.Unlock()
		}()

		now := time.Now()
		persisted := s.loadEntry("cron:between:" + name)
		alreadyStarted := persisted != nil && persisted.Started
		alreadyStopped := persisted != nil && persisted.Stopped

		if alreadyStopped {
			return
		}

		if !alreadyStarted {
			if start.After(now) {
				delay := time.Until(start)
				timer := time.NewTimer(delay)
				select {
				case <-jobCtx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
			}

			if err := startFn(jobCtx); err != nil && s.log != nil {
				s.log(name+":start", err)
			}
			s.markBetweenStarted(name, start, stop)
		}

		if stop.Before(time.Now()) || stop.Equal(time.Now()) {
			if err := stopFn(jobCtx); err != nil && s.log != nil {
				s.log(name+":stop", err)
			}
			s.markBetweenStopped(name, start, stop)
			return
		}

		delay := time.Until(stop)
		timer := time.NewTimer(delay)
		select {
		case <-jobCtx.Done():
			timer.Stop()
			return
		case <-timer.C:
			if err := stopFn(jobCtx); err != nil && s.log != nil {
				s.log(name+":stop", err)
			}
			s.markBetweenStopped(name, start, stop)
		}
	}()
}

func (s *Scheduler) startForJob(name string, duration time.Duration, fn func(ctx context.Context) error) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		s.mu.Lock()
		jobCtx, jobCancel := context.WithTimeout(context.Background(), duration)
		s.runningJobs[name] = jobCancel
		s.mu.Unlock()

		defer func() {
			jobCancel()
			s.mu.Lock()
			delete(s.runningJobs, name)
			s.mu.Unlock()
		}()

		if err := fn(jobCtx); err != nil && s.log != nil {
			s.log(name, err)
		}
	}()
}

func (s *Scheduler) recoverPersistedJobs(ctx context.Context) {
	if s.db == nil {
		return
	}

	s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketScheduler)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			key := string(k)
			if !strings.HasPrefix(key, "cron:once:") && !strings.HasPrefix(key, "cron:between:") {
				return nil
			}

			var e Entry
			if err := json.Unmarshal(v, &e); err != nil {
				return nil
			}

			if strings.HasPrefix(key, "cron:once:") && !e.Done {
				isRegistered := false
				for _, j := range s.once {
					if j.name == e.Name {
						isRegistered = true
						break
					}
				}
				if !isRegistered {
					return nil
				}
			}

			if strings.HasPrefix(key, "cron:between:") && !e.Stopped {
				isRegistered := false
				for _, j := range s.between {
					if j.name == e.Name {
						isRegistered = true
						break
					}
				}
				if !isRegistered {
					return nil
				}
			}

			return nil
		})
	})
}

func (s *Scheduler) persistRecurring(name, spec string) {
	if s.db == nil {
		return
	}
	e := Entry{Name: name, Type: "recurring", Spec: spec, LastRun: time.Now()}
	s.saveEntry("cron:recurring:"+name, &e)
}

func (s *Scheduler) persistOnce(name string, at time.Time, done bool) {
	if s.db == nil {
		return
	}
	e := Entry{Name: name, Type: "once", At: at, Done: done}
	s.saveEntry("cron:once:"+name, &e)
}

func (s *Scheduler) markOnceDone(name string) {
	if s.db == nil {
		return
	}
	e := s.loadEntry("cron:once:" + name)
	if e == nil {
		e = &Entry{Name: name, Type: "once"}
	}
	e.Done = true
	s.saveEntry("cron:once:"+name, e)
}

func (s *Scheduler) persistBetween(name string, start, stop time.Time, started, stopped bool) {
	if s.db == nil {
		return
	}
	e := Entry{Name: name, Type: "between", Start: start, Stop: stop, Started: started, Stopped: stopped}
	s.saveEntry("cron:between:"+name, &e)
}

func (s *Scheduler) markBetweenStarted(name string, start, stop time.Time) {
	if s.db == nil {
		return
	}
	e := s.loadEntry("cron:between:" + name)
	if e == nil {
		e = &Entry{Name: name, Type: "between", Start: start, Stop: stop}
	}
	e.Started = true
	s.saveEntry("cron:between:"+name, e)
}

func (s *Scheduler) markBetweenStopped(name string, start, stop time.Time) {
	if s.db == nil {
		return
	}
	e := s.loadEntry("cron:between:" + name)
	if e == nil {
		e = &Entry{Name: name, Type: "between", Start: start, Stop: stop}
	}
	e.Started = true
	e.Stopped = true
	s.saveEntry("cron:between:"+name, e)
}

func (s *Scheduler) saveEntry(key string, e *Entry) {
	if s.db == nil {
		return
	}
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	s.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketScheduler)
		if err != nil {
			return err
		}
		return b.Put([]byte(key), data)
	})
}

func (s *Scheduler) loadEntry(key string) *Entry {
	if s.db == nil {
		return nil
	}
	var e Entry
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketScheduler)
		if b == nil {
			return fmt.Errorf("no bucket")
		}
		data := b.Get([]byte(key))
		if data == nil {
			return fmt.Errorf("no entry")
		}
		return json.Unmarshal(data, &e)
	})
	if err != nil {
		return nil
	}
	return &e
}

func (s *Scheduler) deleteEntry(name string) {
	if s.db == nil {
		return
	}
	prefixes := []string{"cron:recurring:", "cron:once:", "cron:between:"}
	s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketScheduler)
		if b == nil {
			return nil
		}
		for _, prefix := range prefixes {
			b.Delete([]byte(prefix + name))
		}
		return nil
	})
}
