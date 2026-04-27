# session -- Public API

No interfaces defined. `Manager` and `Session` are concrete structs.

## Manager

Manages active sessions keyed by stream ID. One session per stream.

| Method | Signature | Description |
|--------|-----------|-------------|
| `NewManager` | `(outputDir string) *Manager` | Create a manager with the given output directory |
| `GetOrCreate` | `(ctx context.Context, streamID, streamURL, streamName string) (*Session, bool, error)` | Return existing session or create a new one; bool indicates created |
| `Get` | `(streamID string) *Session` | Look up a session by stream ID |
| `Stop` | `(streamID string)` | Stop and remove a session |
| `StopAll` | `()` | Stop all active sessions |
| `ActiveCount` | `() int` | Return number of active sessions |
| `List` | `() []*Session` | Return all active sessions |
| `AddPlugin` | `(streamID string, plugin output.OutputPlugin) error` | Add an output plugin to an existing session |
| `RemovePlugin` | `(streamID string, mode output.DeliveryMode) error` | Remove an output plugin by delivery mode |

## Session

Represents a single active stream with fan-out delivery.

| Field | Type | Description |
|-------|------|-------------|
| `ID` | `string` | Unique session identifier |
| `StreamID` | `string` | Stream this session is playing |
| `StreamURL` | `string` | Source URL |
| `StreamName` | `string` | Display name |
| `OutputDir` | `string` | Directory for segments and metadata |
| `FanOut` | `*output.FanOut` | Packet distributor |
| `CreatedAt` | `time.Time` | When the session started |

| Method | Signature | Description |
|--------|-----------|-------------|
| `SetRecorded` | `(v bool)` | Mark session as being recorded |
| `IsRecorded` | `() bool` | Check if session is being recorded |
| `Stop` | `()` | Cancel context, stop fan-out, close done channel |
| `Done` | `() <-chan struct{}` | Channel closed when session stops |
| `SetSeekFunc` | `(fn func(posMs int64))` | Register the seek callback |
| `Seek` | `(posMs int64)` | Invoke the registered seek callback |
