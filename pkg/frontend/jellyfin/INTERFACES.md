# Jellyfin Frontend Interfaces

## Dependencies

| Interface | Package | Used For |
|-----------|---------|----------|
| `auth.Service` | pkg/auth | Login, ListUsers, token validation |
| `channel.Store` | pkg/channel | Channel listing, lookup by ID |
| `channel.GroupStore` | pkg/channel | Channel group listing (Jellyfin-enabled groups as UserViews) |
| `store.StreamStore` | pkg/store | Stream/VOD listing, lookup by ID |
| `epg.ProgramStore` | pkg/epg | Now-playing lookup for live TV channels, program guide |
| `favorite.Store` | pkg/favorite | Per-user favorites (mark/unmark, list) |
| `*tmdbcache.Cache` | pkg/cache/tmdb | Movie/series metadata enrichment (poster, overview, rating, genres, cast) |
| `*tmdb.Client` | pkg/tmdb | Person image lookup (profile path for cast/crew) |
| `*tmdb.ImageCache` | pkg/tmdb | TMDB image caching and serving |
| `*logocache.Cache` | pkg/logocache | Channel/stream logo caching and serving |

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
    Favorites:  favoriteStore,
    TMDBCache:  tmdbCache,
    TMDBClient: tmdbClient,
    ImageCache: tmdbImages,
    LogoCache:  logoCache,
    Log:        logger,
})
```

## Provided Interfaces

```go
func (s *Server) Handler() http.Handler
func (s *Server) ListenAndServe(addr string) error
```

## Route Map

### Web
| Method | Path | Handler |
|--------|------|---------|
| GET | / | Server landing page |
| GET | /web, /web/ | Redirect to /web/index.html |
| GET | /web/index.html | Client landing page |
| GET | /web/{file} | config.json, manifest.json |

### Public (no auth)
| Method | Path | Handler |
|--------|------|---------|
| GET | /System/Info/Public | Server discovery |
| GET | /System/Info | Full server info |
| GET | /System/Info/Storage | Storage info |
| GET/POST | /System/Ping | Health check |
| GET | /System/Endpoint | Network info |
| GET | /Branding/Configuration | Branding config |
| GET | /Branding/Css | Custom CSS |
| GET | /Branding/Splashscreen | 404 |
| GET | /QuickConnect/Enabled | Always false |
| POST | /QuickConnect/Initiate | Not supported |
| GET | /Users/Public | Public user list |
| POST | /Users/AuthenticateByName | Login |
| GET | /ScheduledTasks | Empty array |
| GET | /socket | WebSocket stub |
| POST | /ClientLog/Document | Log sink |

### Media (no auth, for image/video loading)
| Method | Path | Handler |
|--------|------|---------|
| GET/HEAD | /Items/{itemId}/Images/{imageType} | Image serving (TMDB, logo, channel) |
| GET | /Persons/{personId}/Images/{imageType} | Person image serving |
| GET/HEAD | /Videos/{itemId}/stream | Video stream |
| GET | /Videos/{itemId}/master.m3u8 | HLS master playlist |
| GET | /Videos/{itemId}/main.m3u8 | HLS media playlist |
| GET | /Videos/{itemId}/live.m3u8 | HLS live playlist |
| GET | /Videos/{itemId}/hls1/{playlistId}/{segment} | HLS segment |
| GET | /Playback/BitrateTest | Bitrate test |

### Authenticated
| Method | Path | Handler |
|--------|------|---------|
| GET | /Users/Me | Current user |
| GET | /Users/{userId} | User by ID |
| GET | /UserViews | Library views (Movies, TV Shows, groups) |
| GET | /Items | Content listing (filter by parentId, type, search, genre, favorites) |
| GET | /Items/Filters | Genre, year, rating filters |
| GET | /Items/{itemId} | Item detail (movie, episode, series, season, channel) |
| GET | /Items/Latest | Latest items |
| GET | /Items/Resume | Resume items (empty) |
| GET | /Items/Counts | Item counts |
| GET | /Items/Suggestions | Random suggestions |
| GET | /Items/{itemId}/Similar | Similar items by genre overlap |
| GET | /Items/{itemId}/Intros | Empty |
| POST | /Items/{itemId}/PlaybackInfo | Playback info (HLS URL, MediaStreams) |
| GET | /Shows/{seriesId}/Seasons | Season listing |
| GET | /Shows/{seriesId}/Episodes | Episode listing |
| GET | /LiveTv/Info | Live TV service info |
| GET | /LiveTv/Channels | Live TV channels with EPG |
| GET/POST | /LiveTv/Programs | Live TV programs (filter by type) |
| GET | /LiveTv/GuideInfo | Guide date range |
| POST/DELETE | /UserPlayedItems/{itemId} | Mark played/unplayed |
| POST/DELETE | /UserFavoriteItems/{itemId} | Mark/unmark favorite |
| GET | /Sessions | Active sessions |
| GET | /DisplayPreferences/{id} | Display preferences |
