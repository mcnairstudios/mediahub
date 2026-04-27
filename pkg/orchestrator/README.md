# pkg/orchestrator

Workflow coordination between API handlers and domain packages. Each function is a self-contained workflow that takes its dependencies explicitly via a Deps struct. No state lives here — all state is in the domain packages.

## Workflows

### Playback (`playback.go`)
- `StartPlayback` — resolve stream, detect client, run strategy, create/join session, attach output plugin
- `StopPlayback` — stop session and clean up
- `Seek` — delegate seek to active session

### Recording (`recording.go`)
- `StartRecording` — attach recording plugin to active session, create recording record
- `StopRecording` — remove recording plugin, mark recording completed
- `ScheduleRecording` — create a scheduled recording for future execution

### Refresh (`refresh.go`)
- `RefreshSource` — trigger refresh on a single source by type and ID
- `RefreshAll` — refresh all registered source types, collect errors

## Design

- Functions, not methods on a god struct
- Each Deps struct declares exactly what the workflow needs
- No global state, no singletons
- Tests use real in-memory stores and mock plugins
