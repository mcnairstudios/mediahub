# API Interfaces

The API is a frontend plugin -- the first one, but just a plugin. It translates HTTP
into orchestrator calls. No business logic.

## Server

```go
type Server struct { ... }

func NewServer(deps OrchestratorDeps) *Server
func (s *Server) Handler() http.Handler
```

## OrchestratorDeps

All dependencies are explicit interfaces:

```go
type OrchestratorDeps struct {
    StreamStore       store.StreamStore
    ChannelStore      channel.Store
    SettingsStore     store.SettingsStore
    SourceConfigStore sourceconfig.Store
    ConnRegistry      *connectivity.Registry
    SessionMgr        *session.Manager
    Detector          *client.Detector
    OutputReg         *output.Registry
    SourceReg         *source.Registry
    RecordingStore    recording.Store
    ClientStore       client.Store
    AuthService       auth.Service
    EPGSourceStore    epg.SourceStore
    ProgramStore      epg.ProgramStore
    GroupStore        channel.GroupStore
    Strategy          func(strategy.Input, strategy.Output) strategy.Decision
    WGService         *wg.Service
    FavoriteStore     favorite.Store
    LogoCache         *logocache.Cache
    Activity          *activity.Service
    TMDBClient        *tmdb.Client
    TMDBImages        *tmdb.ImageCache
    Config            *config.Config
    StaticFS          fs.FS
    UserAgent         string
    BypassHeader      string
    BypassSecret      string
}
```

## Handler Contract

Every handler follows the same pattern:
1. Parse request (URL params, query params, JSON body)
2. Call orchestrator function or store method
3. Format JSON response

Handlers are methods on Server for access to deps. They contain ZERO business logic.

## Routes

| Method | Path | Auth | Handler |
|--------|------|------|---------|
| POST | /api/auth/login | Public | handleLogin |
| POST | /api/auth/refresh | Public | handleRefreshToken |
| GET | /api/streams | Auth | handleListStreams |
| GET | /api/streams/{id}/detail | Auth | handleStreamDetail |
| GET | /api/channels | Auth | handleListChannels |
| POST | /api/channels | Admin | handleCreateChannel |
| PUT | /api/channels/{id} | Admin | handleUpdateChannel |
| DELETE | /api/channels/{id} | Admin | handleDeleteChannel |
| POST | /api/channels/{id}/streams | Admin | handleAssignStreams |
| GET | /api/channel-groups | Auth | handleListGroups |
| POST | /api/channel-groups | Admin | handleCreateGroup |
| DELETE | /api/channel-groups/{id} | Admin | handleDeleteGroup |
| GET | /api/settings | Auth | handleGetSettings |
| PUT | /api/settings | Admin | handleUpdateSettings |
| GET | /api/epg/sources | Auth | handleListEPGSources |
| POST | /api/epg/sources | Admin | handleCreateEPGSource |
| PUT | /api/epg/sources/{id} | Admin | handleUpdateEPGSource |
| DELETE | /api/epg/sources/{id} | Admin | handleDeleteEPGSource |
| POST | /api/epg/sources/{id}/refresh | Admin | handleRefreshEPGSource |
| GET | /api/epg/now | Auth | handleEPGNow |
| GET | /api/epg/programs | Auth | handleEPGPrograms |
| GET | /api/dashboard/stats | Auth | handleDashboardStats |
| GET | /api/recordings | Auth | handleListRecordings |
| GET | /api/recordings/completed/{id} | Auth | handleGetRecording |
| POST | /api/recordings/completed/{id}/play | Auth | handlePlayRecording |
| DELETE | /api/recordings/completed/{id}/play | Auth | handleStopRecordingPlayback |
| POST | /api/recordings/completed/{id}/seek | Auth | handleSeekRecordingPlayback |
| GET | /api/recordings/completed/{id}/stream | Auth | handleStreamRecording |
| GET | /api/recordings/completed/{id}/play/hls/{path} | Auth | handleRecordingPlaybackServe |
| GET | /api/recordings/completed/{id}/play/mse/{path} | Auth | handleRecordingPlaybackServe |
| DELETE | /api/recordings/completed/{id} | Admin | handleDeleteRecording |
| POST | /api/recordings/schedule | Auth | handleScheduleRecording |
| GET | /api/recordings/schedule | Auth | handleListScheduledRecordings |
| DELETE | /api/recordings/schedule/{id} | Auth | handleCancelScheduledRecording |
| POST | /api/play/{streamID} | Auth | handleStartPlayback |
| DELETE | /api/play/{streamID} | Auth | handleStopPlayback |
| POST | /api/play/{streamID}/seek | Auth | handleSeek |
| POST | /api/play/{streamID}/record | Auth | handleStartRecording |
| DELETE | /api/play/{streamID}/record | Auth | handleStopRecording |
| GET | /api/play/{streamID}/hls/{path} | Auth | handlePlaybackServe |
| GET | /api/play/{streamID}/mse/{path} | Auth | handlePlaybackServe |
| GET | /api/sources | Auth | handleListSources |
| POST | /api/sources/m3u | Admin | handleCreateM3USource |
| PUT | /api/sources/m3u/{id} | Admin | handleUpdateM3USource |
| DELETE | /api/sources/m3u/{id} | Admin | handleDeleteM3USource |
| POST | /api/sources/tvpstreams | Admin | handleCreateTVPStreamsSource |
| PUT | /api/sources/tvpstreams/{id} | Admin | handleUpdateTVPStreamsSource |
| DELETE | /api/sources/tvpstreams/{id} | Admin | handleDeleteTVPStreamsSource |
| GET | /api/sources/tvpstreams/{id}/tls | Auth | handleTVPStreamsTLSStatus |
| POST | /api/sources/xtream | Admin | handleCreateXtreamSource |
| PUT | /api/sources/xtream/{id} | Admin | handleUpdateXtreamSource |
| DELETE | /api/sources/xtream/{id} | Admin | handleDeleteXtreamSource |
| GET | /api/sources/xtream/{id}/info | Admin | handleXtreamAccountInfo |
| POST | /api/sources/hdhr | Admin | handleCreateHDHRSource |
| PUT | /api/sources/hdhr/{id} | Admin | handleUpdateHDHRSource |
| DELETE | /api/sources/hdhr/{id} | Admin | handleDeleteHDHRSource |
| POST | /api/sources/hdhr/discover | Admin | handleHDHRDiscover |
| POST | /api/sources/hdhr/add-device | Admin | handleHDHRAddDevice |
| GET | /api/sources/hdhr/{id}/devices | Admin | handleHDHRDevices |
| POST | /api/sources/hdhr/{id}/scan | Admin | handleHDHRScan |
| POST | /api/sources/hdhr/{id}/retune | Admin | handleHDHRRetune |
| GET | /api/sources/hdhr/{id}/status | Auth | handleHDHRRetuneStatus |
| POST | /api/sources/hdhr/{id}/clear | Admin | handleHDHRClear |
| POST | /api/sources/satip | Admin | handleCreateSatIPSource |
| PUT | /api/sources/satip/{id} | Admin | handleUpdateSatIPSource |
| DELETE | /api/sources/satip/{id} | Admin | handleDeleteSatIPSource |
| POST | /api/sources/satip/{id}/scan | Admin | handleSatIPScan |
| GET | /api/sources/satip/{id}/status | Auth | handleSatIPScanStatus |
| POST | /api/sources/satip/{id}/clear | Admin | handleSatIPClear |
| POST | /api/sources/{sourceID}/refresh | Admin | handleRefreshSource |
| GET | /api/sources/{sourceID}/status | Auth | handleSourceStatus |
| GET | /api/users | Admin | handleListUsers |
| POST | /api/users | Admin | handleCreateUser |
| PUT | /api/users/{id} | Admin | handleUpdateUser |
| DELETE | /api/users/{id} | Admin | handleDeleteUser |
| PUT | /api/users/{id}/password | Auth | handleChangePassword |
| GET | /api/wireguard/profiles | Admin | handleListWGProfiles |
| POST | /api/wireguard/profiles | Admin | handleCreateWGProfile |
| PUT | /api/wireguard/profiles/{id} | Admin | handleUpdateWGProfile |
| DELETE | /api/wireguard/profiles/{id} | Admin | handleDeleteWGProfile |
| POST | /api/wireguard/profiles/{id}/activate | Admin | handleActivateWGProfile |
| POST | /api/wireguard/profiles/{id}/test | Admin | handleTestWGProfile |
| GET | /api/wireguard/status | Admin | handleWGStatus |
| GET | /api/favorites | Auth | handleListFavorites |
| POST | /api/favorites | Auth | handleAddFavorite |
| DELETE | /api/favorites/{streamID} | Auth | handleRemoveFavorite |
| GET | /api/favorites/check/{streamID} | Auth | handleCheckFavorite |
| GET | /api/tmdb/image | Auth | handleTMDBImage |
| GET | /api/clients | Auth | handleListClients |
| GET | /api/clients/{id} | Auth | handleGetClient |
| POST | /api/clients | Admin | handleCreateClient |
| PUT | /api/clients/{id} | Admin | handleUpdateClient |
| DELETE | /api/clients/{id} | Admin | handleDeleteClient |
| GET | /api/capabilities | Auth | handleCapabilities |
| GET | /api/activity | Admin | handleListActivity |
| GET | /api/output/playlist.m3u | Public | handleOutputM3U |
| GET | /api/output/epg.xml | Public | handleOutputEPG |
| GET | /channel/{id} | Public | handleChannelStream |
| POST | /api/probe | Admin | handleProbe |

## Relation to Other Frontend Plugins

```
Orchestrator (the brain -- stateless workflows)
    ^           ^           ^           ^
    API       Jellyfin     HDHR        DLNA
  (:8080)    (:8096)     (emulated)  (SSDP)
```

None know about each other. All call the same orchestrator.
