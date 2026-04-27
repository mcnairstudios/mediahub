# Orchestrator Interfaces

The orchestrator is stateless. It coordinates workflows by calling domain packages.
Each workflow is a function with explicit deps -- no god struct, no global state.

## Deps Pattern

Each workflow group has a `Deps` struct listing exactly what it needs:

```go
type PlaybackDeps struct {
    StreamStore       store.StreamStore
    SettingsStore     store.SettingsStore
    SourceConfigStore sourceconfig.Store
    ConnRegistry      *connectivity.Registry
    SessionMgr        *session.Manager
    Detector          *client.Detector
    OutputReg         *output.Registry
    Strategy          func(strategy.Input, strategy.Output) strategy.Decision
    UserAgent         string
    PipelineRunner    PipelineRunner
}

type RecordingDeps struct {
    SessionMgr     *session.Manager
    RecordingStore recording.Store
    OutputReg      *output.Registry
    RecordDir      string
}

type RefreshDeps struct {
    SourceReg         *source.Registry
    SourceConfigStore sourceconfig.Store
}
```

## PlaybackResult

```go
type PlaybackResult struct {
    Session   *session.Session
    Plugin    output.OutputPlugin
    Servable  output.ServablePlugin
    Decision  strategy.Decision
    IsNew     bool
    Delivery  string
    ProbeInfo *media.ProbeResult
}
```

## Workflow Functions

```go
// Playback
func StartPlayback(ctx, deps PlaybackDeps, streamID, port, headers) -> (*PlaybackResult, error)
func StopPlayback(deps PlaybackDeps, streamID)
func Seek(deps PlaybackDeps, streamID, positionMs int64) -> error
func PlayRecording(ctx, deps PlaybackDeps, recordingID, filePath, title) -> (*PlaybackResult, error)
func StopRecordingPlayback(deps PlaybackDeps, recordingID)

// Recording
func StartRecording(ctx, deps RecordingDeps, streamID, title, userID, short) -> error
func StopRecording(ctx, deps RecordingDeps, streamID) -> error
func ScheduleRecording(ctx, deps RecordingDeps, recording) -> error

// Source Refresh
func RefreshSource(ctx, deps RefreshDeps, sourceType, sourceID) -> error
func RefreshAll(ctx, deps RefreshDeps) -> []error
```

## Contract
- Functions are the interface -- no struct methods
- Each function documents its preconditions and error cases
- Deps are injected per-call, not per-instance
- Orchestrator owns NO state
