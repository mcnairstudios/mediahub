# worker -- Interfaces

## Types

### Job

```go
type Job struct {
    Name     string
    Interval time.Duration
    Fn       func(ctx context.Context) error
}
```

A named function that runs on a fixed interval. `Fn` receives the scheduler's context -- when `Stop` is called or the parent context is cancelled, the context is done.

### Scheduler

```go
type Scheduler struct { /* unexported fields */ }
```

Manages a set of jobs, each running in its own goroutine.

## Constructor

```go
func NewScheduler(log func(string, error)) *Scheduler
```

Creates a scheduler. The `log` callback is invoked when a job returns a non-nil error. Pass nil to silently discard errors.

## Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| Add | `(s *Scheduler) Add(job Job)` | Register a job. Must be called before Start. |
| Start | `(s *Scheduler) Start(ctx context.Context)` | Launch all jobs. Idempotent while running. |
| Stop | `(s *Scheduler) Stop()` | Cancel all jobs and wait for completion. Safe to call before Start. |
| Running | `(s *Scheduler) Running() bool` | Whether the scheduler is active. |
| JobCount | `(s *Scheduler) JobCount() int` | Number of registered jobs. |

## Thread Safety

All methods are safe for concurrent use. Internal state is protected by a mutex. Job functions run in separate goroutines and must handle their own concurrency if needed.

## Error Handling

Job errors are passed to the log callback provided at construction. A failing job does not stop the scheduler -- it continues running on its interval.
