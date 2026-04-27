# channel -- Interfaces

## Store

CRUD and stream mapping for channels.

```go
type Store interface {
    Get(ctx context.Context, id string) (*Channel, error)
    List(ctx context.Context) ([]Channel, error)
    Create(ctx context.Context, ch *Channel) error
    Update(ctx context.Context, ch *Channel) error
    Delete(ctx context.Context, id string) error
    AssignStreams(ctx context.Context, channelID string, streamIDs []string) error
    RemoveStreamMappings(ctx context.Context, streamIDs []string) error
}
```

| Method | Description |
|--------|-------------|
| `Get` | Retrieve a channel by ID |
| `List` | Return all channels |
| `Create` | Persist a new channel |
| `Update` | Update an existing channel |
| `Delete` | Remove a channel by ID |
| `AssignStreams` | Set the ordered list of stream IDs for a channel |
| `RemoveStreamMappings` | Remove the given stream IDs from all channels |

---

## GroupStore

CRUD for channel groups.

```go
type GroupStore interface {
    List(ctx context.Context) ([]Group, error)
    Create(ctx context.Context, g *Group) error
    Delete(ctx context.Context, id string) error
}
```

| Method | Description |
|--------|-------------|
| `List` | Return all channel groups |
| `Create` | Persist a new group |
| `Delete` | Remove a group by ID |
