# epg -- Interfaces

## SourceStore

CRUD for EPG sources (XMLTV feeds).

```go
type SourceStore interface {
    Get(ctx context.Context, id string) (*Source, error)
    List(ctx context.Context) ([]Source, error)
    Create(ctx context.Context, s *Source) error
    Update(ctx context.Context, s *Source) error
    Delete(ctx context.Context, id string) error
}
```

| Method | Description |
|--------|-------------|
| `Get` | Retrieve an EPG source by ID |
| `List` | Return all EPG sources |
| `Create` | Persist a new EPG source |
| `Update` | Update an existing EPG source |
| `Delete` | Remove an EPG source by ID |

---

## ProgramStore

Query and bulk-manage EPG program data.

```go
type ProgramStore interface {
    NowPlaying(ctx context.Context, channelID string) (*Program, error)
    Range(ctx context.Context, channelID string, start, end time.Time) ([]Program, error)
    ListAll(ctx context.Context) ([]Program, error)
    BulkInsert(ctx context.Context, programs []Program) error
    DeleteBySource(ctx context.Context, sourceID string) error
}
```

| Method | Description |
|--------|-------------|
| `NowPlaying` | Return the currently airing program for a channel |
| `Range` | Return all programs for a channel within a time window |
| `ListAll` | Return all programs (used for XMLTV EPG output) |
| `BulkInsert` | Insert programs in bulk (used during EPG refresh) |
| `DeleteBySource` | Remove all programs belonging to an EPG source |
