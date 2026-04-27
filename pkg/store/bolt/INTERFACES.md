# store/bolt -- Interfaces

Bolt-backed implementations of all domain store interfaces. Each store is accessed via the `DB` struct.

## DB

```go
func Open(path string) (*DB, error)
func (d *DB) Close() error
```

Opens a bbolt database file. Creates all required buckets on open. Returns typed store accessors:

| Accessor | Returns | Implements |
|----------|---------|------------|
| `StreamStore()` | `*StreamStore` | `store.StreamStore` |
| `SettingsStore()` | `*SettingsStore` | `store.SettingsStore` |
| `ChannelStore()` | `*ChannelStore` | `channel.Store` |
| `GroupStore()` | `*GroupStore` | `channel.GroupStore` |
| `EPGSourceStore()` | `*EPGSourceStore` | `epg.SourceStore` |
| `ProgramStore()` | `*ProgramStore` | `epg.ProgramStore` |
| `RecordingStore()` | `*RecordingStore` | `recording.Store` |
| `UserStore()` | `*UserStore` | `auth.UserStore` |
| `SourceConfigStore()` | `*SourceConfigStore` | `sourceconfig.Store` |

## StreamStore

```go
Get(ctx, id) (*media.Stream, error)
List(ctx) ([]media.Stream, error)
ListBySource(ctx, sourceType, sourceID) ([]media.Stream, error)
BulkUpsert(ctx, streams) error
DeleteBySource(ctx, sourceType, sourceID) error
DeleteStaleBySource(ctx, sourceType, sourceID, keepIDs) ([]string, error)
Save() error  // no-op
```

## SettingsStore

```go
Get(ctx, key) (string, error)
Set(ctx, key, value) error
List(ctx) (map[string]string, error)
```

## ChannelStore

```go
Get(ctx, id) (*channel.Channel, error)
List(ctx) ([]channel.Channel, error)
Create(ctx, ch) error
Update(ctx, ch) error
Delete(ctx, id) error
AssignStreams(ctx, channelID, streamIDs) error
RemoveStreamMappings(ctx, streamIDs) error
```

## GroupStore

```go
List(ctx) ([]channel.Group, error)
Create(ctx, g) error
Delete(ctx, id) error
```

## EPGSourceStore

```go
Get(ctx, id) (*epg.Source, error)
List(ctx) ([]epg.Source, error)
Create(ctx, src) error
Update(ctx, src) error
Delete(ctx, id) error
```

## ProgramStore

```go
NowPlaying(ctx, channelID) (*epg.Program, error)
Range(ctx, channelID, start, end) ([]epg.Program, error)
BulkInsert(ctx, programs) error
DeleteBySource(ctx, sourceID) error
```

Programs are keyed by `channelID|startTime` for efficient cursor-based range queries.

## RecordingStore

```go
Get(ctx, id) (*recording.Recording, error)
List(ctx, userID, isAdmin) ([]recording.Recording, error)
Create(ctx, r) error
Update(ctx, r) error
Delete(ctx, id) error
ListByStatus(ctx, status) ([]recording.Recording, error)
ListScheduled(ctx) ([]recording.Recording, error)
```

`List` filters by userID unless isAdmin is true.

## UserStore

```go
Get(ctx, id) (*auth.User, error)
GetByUsername(ctx, username) (*auth.User, error)
List(ctx) ([]*auth.User, error)
Create(ctx, user) error
Delete(ctx, id) error
UpdatePassword(ctx, id, hashedPassword) error
GetPasswordHash(ctx, id) (string, error)
```

Password hashes are stored alongside the user in a wrapper struct, never exposed on `auth.User`.

## SourceConfigStore

```go
Get(ctx, id) (*sourceconfig.SourceConfig, error)
List(ctx) ([]sourceconfig.SourceConfig, error)
ListByType(ctx, sourceType) ([]sourceconfig.SourceConfig, error)
Create(ctx, sc) error
Update(ctx, sc) error
Delete(ctx, id) error
```
