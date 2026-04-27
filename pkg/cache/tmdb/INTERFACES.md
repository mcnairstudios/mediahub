# cache/tmdb -- Interfaces

## Cache

In-memory TMDB metadata cache. Implements `cache.Cache` for registry integration.

### cache.Cache methods

```go
func (c *Cache) Type() cache.CacheType
func (c *Cache) Get(ctx context.Context, key string) (any, bool)
func (c *Cache) Set(ctx context.Context, key string, value any) error
func (c *Cache) Delete(ctx context.Context, key string) error
func (c *Cache) Clear(ctx context.Context) error
```

| Method | Description |
|--------|-------------|
| `Type` | Returns `cache.CacheTMDB` |
| `Get` | Look up by key in movies first, then series. Returns the value and whether it was found. |
| `Set` | Store a `*Movie` or `*Series` by key. Other types are silently ignored. |
| `Delete` | Remove the key from both movies and series maps |
| `Clear` | Remove all cached entries |

### Typed helpers

```go
func (c *Cache) GetMovie(id string) (*Movie, bool)
func (c *Cache) GetSeries(id string) (*Series, bool)
func (c *Cache) SetMovie(id string, m *Movie)
func (c *Cache) SetSeries(id string, s *Series)
```

Direct access without type assertions. Use these when the caller knows the content type.

## Types

| Type | Key Fields |
|------|------------|
| `Movie` | ID, Title, Overview, PosterPath, BackdropPath, ReleaseDate, Rating, Genres, Runtime, Certification, Cast, Crew, CollectionID, CollectionName |
| `Series` | ID, Name, Overview, PosterPath, BackdropPath, FirstAirDate, Rating, Genres, Seasons |
| `Season` | SeasonNumber, Name, Overview, PosterPath, EpisodeCount, Episodes |
| `Episode` | EpisodeNumber, Name, Overview, StillPath, AirDate, Runtime |
| `CastMember` | Name, Character, ProfilePath, TMDBID |
| `CrewMember` | Name, Job, Department, ProfilePath, TMDBID |
