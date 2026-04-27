# Jellyfin Frontend Interfaces

## Dependencies

| Interface | Package | Used For |
|-----------|---------|----------|
| `auth.Service` | pkg/auth | Login, ListUsers, token validation |
| `channel.Store` | pkg/channel | Channel listing, lookup by ID |
| `channel.GroupStore` | pkg/channel | Channel group listing (future: Jellyfin-enabled groups as UserViews) |
| `store.StreamStore` | pkg/store | Stream/VOD listing, lookup by ID |
| `epg.ProgramStore` | pkg/epg | Now-playing lookup for live TV channels |
| `*tmdb.Cache` | pkg/cache/tmdb | Movie/series metadata enrichment (poster, overview, rating, genres) |

## Server Construction

```go
srv := jellyfin.NewServer(jellyfin.ServerDeps{
    ServerName: "MediaHub",
    StateDir:   "/config",
    Auth:       authService,
    Channels:   channelStore,
    Groups:     groupStore,
    Streams:    streamStore,
    Programs:   programStore,
    TMDBCache:  tmdbCache,
    Log:        logger,
})
```

## Provided Interfaces

```go
func (s *Server) Handler() http.Handler
func (s *Server) ListenAndServe(addr string) error
```

## Route Map

### Public (no auth)
| Method | Path | Handler |
|--------|------|---------|
| GET | /System/Info/Public | Server discovery |
| GET | /System/Info | Full server info |
| GET/POST | /System/Ping | Health check |
| GET | /System/Endpoint | Network info |
| GET | /Branding/Configuration | Branding config |
| GET | /Branding/Css | Custom CSS |
| GET | /QuickConnect/Enabled | Always false |
| GET | /Users/Public | Public user list |
| POST | /Users/AuthenticateByName | Login |

### Authenticated
| Method | Path | Handler |
|--------|------|---------|
| GET | /Users/Me | Current user |
| GET | /Users/{userId} | User by ID |
| GET | /UserViews | Library views (Movies, TV Shows) |
| GET | /Items | Content listing (filter by parentId, type, search) |
| GET | /Items/{itemId} | Item detail |
| GET | /Items/Latest | Latest items |
| GET | /Items/Resume | Resume items (empty) |
| GET | /Items/Counts | Item counts |
| POST | /Items/{itemId}/PlaybackInfo | Playback info (HLS URL) |
| GET | /LiveTv/Channels | Live TV channel list |
| GET | /LiveTv/Info | Live TV service info |
| GET | /Sessions | Active sessions |
| GET | /DisplayPreferences/{id} | Display preferences |

### Media (no auth, for image loading)
| Method | Path | Handler |
|--------|------|---------|
| GET/HEAD | /Items/{itemId}/Images/{imageType} | Image serving (stub) |
