# recording -- Interfaces

## Store

Persistence layer for recording state and scheduling.

```go
type Store interface {
    Get(ctx context.Context, id string) (*Recording, error)
    List(ctx context.Context, userID string, isAdmin bool) ([]Recording, error)
    Create(ctx context.Context, r *Recording) error
    Update(ctx context.Context, r *Recording) error
    Delete(ctx context.Context, id string) error
    ListByStatus(ctx context.Context, status Status) ([]Recording, error)
    ListScheduled(ctx context.Context) ([]Recording, error)
}
```

| Method | Description |
|--------|-------------|
| `Get` | Retrieve a recording by ID |
| `List` | Return recordings visible to the given user (admins see all) |
| `Create` | Persist a new recording |
| `Update` | Update recording state (status, file path, size, etc.) |
| `Delete` | Remove a recording by ID |
| `ListByStatus` | Return all recordings with the given status |
| `ListScheduled` | Return all recordings with status "scheduled" |
