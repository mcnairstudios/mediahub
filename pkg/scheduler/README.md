# scheduler

EPG-triggered recording scheduler. Checks every 30 seconds for recordings that need to start or stop based on their scheduled times.

## Usage

```go
s := scheduler.New(recordingStore)
s.SetStartFunc(func(streamID, title string) error {
    return orchestrator.StartRecording(ctx, deps, streamID, title, "system", false)
})
s.SetStopFunc(func(streamID string) error {
    return orchestrator.StopRecording(ctx, deps, streamID)
})
s.Start(ctx)
defer s.Stop()
```

## Tick Logic

Runs every 30 seconds:

1. List recordings with status "scheduled"
2. For each where start time <= now: call start function, update status to "recording"
3. For each with status "recording" where stop time <= now: call stop function, update status to "completed"
4. If start/stop function returns error, recording status set to "failed"
