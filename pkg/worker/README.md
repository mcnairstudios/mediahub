# worker

Ticker-based background job scheduler. Each registered job runs in its own goroutine on a fixed interval.

## Usage

```go
s := worker.NewScheduler(func(name string, err error) {
    log.Printf("job %s failed: %v", name, err)
})

s.Add(worker.Job{
    Name:     "epg-refresh",
    Interval: 6 * time.Hour,
    Fn:       func(ctx context.Context) error { return epgService.Refresh(ctx) },
})

s.Start(ctx)
defer s.Stop()
```

Jobs run immediately on `Start`, then repeat on their interval. `Stop` cancels all jobs and waits for in-flight executions to finish.

## Design

- Zero external dependencies
- Does not import any mediahub package -- fully generic
- Thread-safe: all state guarded by mutex
- Jobs are plain functions -- the caller wraps service/orchestrator calls
- Error callback receives the job name and error; nil errors are not reported
- `Start` is idempotent (second call is a no-op while running)
- `Stop` before `Start` is safe (no-op)
