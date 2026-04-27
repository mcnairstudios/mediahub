# API Interfaces

The API is a frontend plugin — the first one, but just a plugin. It translates HTTP
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
    StreamStore    store.StreamStore
    ChannelStore   channel.Store
    GroupStore     channel.GroupStore
    SettingsStore  store.SettingsStore
    EPGSourceStore epg.SourceStore
    EPGProgramStore epg.ProgramStore
    SessionMgr     *session.Manager
    Detector       *client.Detector
    OutputReg      *output.Registry
    SourceReg      *source.Registry
    RecordingStore recording.Store
    AuthService    auth.Service
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
| GET | /api/channels | Auth | handleListChannels |
| GET | /api/settings | Auth | handleGetSettings |
| PUT | /api/settings | Admin | handleUpdateSettings |
| GET | /api/epg/sources | Auth | handleListEPGSources |
| GET | /api/recordings | Auth | handleListRecordings |
| POST | /api/play/{streamID} | Auth | handleStartPlayback |
| DELETE | /api/play/{streamID} | Auth | handleStopPlayback |
| POST | /api/play/{streamID}/seek | Auth | handleSeek |
| POST | /api/play/{streamID}/record | Auth | handleStartRecording |
| DELETE | /api/play/{streamID}/record | Auth | handleStopRecording |
| POST | /api/sources/{sourceID}/refresh | Admin | handleRefreshSource |
| GET | /api/users | Admin | handleListUsers |
| POST | /api/users | Admin | handleCreateUser |

## Relation to Other Frontend Plugins

```
Orchestrator (the brain — stateless workflows)
    ↑           ↑           ↑           ↑
    API       Jellyfin     HDHR        DLNA
  (:8080)    (:8096)     (emulated)  (SSDP)
```

None know about each other. All call the same orchestrator.
