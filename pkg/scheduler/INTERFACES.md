# scheduler -- Interfaces

## Dependencies

### recording.Store

Used for reading and updating scheduled recording state. See `pkg/recording/INTERFACES.md`.

### StartFunc

```go
type StartFunc func(streamID, title string) error
```

Called when a scheduled recording's start time has arrived. Wired to the orchestrator's recording start logic.

### StopFunc

```go
type StopFunc func(streamID string) error
```

Called when a recording's scheduled stop time has arrived. Wired to the orchestrator's recording stop logic.

## Scheduler

```go
type Scheduler struct { ... }

func New(store recording.Store) *Scheduler
func (s *Scheduler) SetStartFunc(fn StartFunc)
func (s *Scheduler) SetStopFunc(fn StopFunc)
func (s *Scheduler) Start(ctx context.Context)
func (s *Scheduler) Stop()
func (s *Scheduler) Tick(ctx context.Context)
```

| Method | Description |
|--------|-------------|
| `New` | Create a scheduler backed by the given recording store |
| `SetStartFunc` | Wire the callback invoked to start a recording |
| `SetStopFunc` | Wire the callback invoked to stop a recording |
| `Start` | Begin the 30-second tick loop in a goroutine |
| `Stop` | Cancel the tick loop and wait for it to finish |
| `Tick` | Run one check cycle (exported for testing) |
