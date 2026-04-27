# recording

Recording types and store interface for mediahub.

Every playback session writes to a temp file (StatusActive). When the user presses record, the status transitions to StatusRecording and the file is preserved on completion (StatusCompleted). Scheduled recordings are created ahead of time with StatusScheduled and transition to StatusRecording when their EPG start time arrives.

## Status Lifecycle

```
StatusScheduled  -->  StatusRecording  -->  StatusCompleted
StatusActive     -->  StatusRecording  -->  StatusCompleted
                                       -->  StatusFailed
```

## Store Interface

The `Store` interface defines persistence operations. Implementations live outside this package (e.g. JSON file store, database store).

## Zero Dependencies

This package has no external dependencies. It defines only types and interfaces.
