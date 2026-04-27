# Orchestrator Interfaces

The orchestrator is stateless. It coordinates workflows by calling domain packages.
Each workflow is a function with explicit deps — no god struct, no global state.

## Deps Pattern

Each workflow group has a `Deps` struct listing exactly what it needs:

```go
type PlaybackDeps struct {
    StreamStore  store.StreamStore
    SessionMgr   *session.Manager
    Detector     *client.Detector
    OutputReg    *output.Registry
    Strategy     func(strategy.Input, strategy.Output) strategy.Decision
}

type RecordingDeps struct {
    SessionMgr     *session.Manager
    RecordingStore recording.Store
    OutputReg      *output.Registry
}

type RefreshDeps struct {
    SourceReg *source.Registry
}
```

## Workflow Functions

```go
// Playback
func StartPlayback(ctx, deps, streamID, port, headers) → (*PlaybackResult, error)
func StopPlayback(deps, streamID)
func Seek(deps, streamID, positionMs) → error

// Recording
func StartRecording(ctx, deps, streamID, title, userID, short) → error
func StopRecording(ctx, deps, streamID) → error
func ScheduleRecording(ctx, deps, recording) → error

// Source Refresh
func RefreshSource(ctx, deps, sourceID) → error
func RefreshAll(ctx, deps) → []error
```

## Contract
- Functions are the interface — no struct methods
- Each function documents its preconditions and error cases
- Deps are injected per-call, not per-instance
- Orchestrator owns NO state
