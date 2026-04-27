# pkg/api

REST API server for mediahub. Thin translation layer: parse HTTP requests, call orchestrator/service functions, format JSON responses.

## Design

- Uses Go 1.22+ `http.ServeMux` with method-based routing (`GET /api/streams`, `POST /api/auth/login`)
- No external router dependencies
- Handlers contain zero business logic — they delegate to orchestrator functions and stores
- Auth middleware from `pkg/middleware` handles JWT validation and role checks

## Routes

### Public
- `POST /api/auth/login` — authenticate, returns access token
- `POST /api/auth/refresh` — refresh an expired token

### Authenticated
- `GET /api/streams` — list all streams
- `GET /api/channels` — list all channels
- `GET /api/settings` — get all settings
- `GET /api/epg/sources` — list EPG sources
- `GET /api/recordings` — list recordings (filtered by user unless admin)

### Playback (authenticated)
- `POST /api/play/{streamID}` — start playback session
- `DELETE /api/play/{streamID}` — stop playback session
- `POST /api/play/{streamID}/seek` — seek within session
- `POST /api/play/{streamID}/record` — start recording on active session
- `DELETE /api/play/{streamID}/record` — stop recording

### Admin only
- `PUT /api/settings` — update settings
- `POST /api/sources/{sourceID}/refresh` — trigger source refresh
- `GET /api/users` — list all users
- `POST /api/users` — create a new user

## Dependencies

- `pkg/auth` — authentication service
- `pkg/middleware` — JWT auth middleware
- `pkg/httputil` — JSON response/decode helpers
- `pkg/store` — stream, settings stores
- `pkg/channel` — channel store
- `pkg/epg` — EPG source store
- `pkg/session` — session manager
- `pkg/client` — client detector
- `pkg/output` — output plugin registry
- `pkg/source` — source registry
- `pkg/orchestrator` — playback, recording, refresh orchestration
- `pkg/recording` — recording store
- `pkg/strategy` — codec strategy resolution

## Testing

Tests use `httptest.NewServer` with real in-memory stores and JWT auth:

```bash
go test ./pkg/api/...
```
