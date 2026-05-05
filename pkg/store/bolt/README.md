# pkg/store/bolt — Bolt-backed persistent stores

## Purpose
Production persistence layer using bbolt (embedded key-value store). Replaces the in-memory stores for deployment. All data survives restarts.

## Status
All domain stores implemented. All stores use the keyenc prefix-key pattern for structured, scannable keys.

## Responsibilities
- Implement all domain store interfaces using bbolt buckets
- JSON serialization for complex values, plain strings for settings
- Thread-safe (bbolt handles this internally)
- Data directory configurable via bolt.Open(path)
- Auto-migrate old flat keys to prefixed keys on first access

## Does NOT
- Define interfaces — those live in pkg/store/interfaces.go

## Key Scheme (keyenc)

All stores use `pkg/store/bolt/keyenc/` for structured key encoding. Keys are colon-separated segments defined by struct tags.

| Store | Key Pattern | Index |
|-------|------------|-------|
| Streams | `streams:{sourceType}:{sourceID}:{vodType}:{streamID}` | `streamidx:{streamID}` -> full key, `tmdb:unresolved:{streamID}` -> source key |
| Channels | `channels:{groupID}:{channelID}` | `channelidx:{channelID}` -> full key |
| Groups | `groups:{groupID}` | |
| EPG Programs | `programs:{channelID}:{startUnix}` | |
| Recordings | `recordings:{userID}:{recordingID}` | `recordingidx:{recordingID}` -> full key |
| Favorites | `favorites:{userID}:{itemID}` | |
| Users | `users:{userID}` | |
| Clients | `clients:{clientID}` | |
| API Keys | `apikeys:{userID}:{keyID}` | `apikeyidx:{keyID}` -> full key |
| Invites | `invites:{token}` | |
| Source Configs | `sourceconfigs:{type}:{id}` | `srccfgidx:{id}` -> full key |
| Settings | `settings:{key}` | |
| Probe Cache | `probecache:{urlHash}` | |

Prefix scans (cursor Seek + HasPrefix) enable efficient filtered listing without full-bucket scans. Reverse index keys enable O(1) lookup by ID when the full key includes parent segments.

## Migration

Each store has a `migrateFromFlatKeys()` method called from its `DB` accessor in `bolt.go`. Migration detects old flat keys (bare IDs), rewrites them to prefixed keys, and deletes the old entries. Migration is idempotent — if prefixed keys already exist, it skips. All stores also have Get fallback for old flat keys (reads bare ID if prefixed key not found).

## Files
- `bolt.go` — DB struct, Open/Close, bucket creation, store accessors (with migration calls)
- `keyenc/` — Key encoding schema (NewSchema, Key, Prefix, Parse, Reverse)
- `streams.go` — StreamStore (prefix: `streams:`)
- `settings.go` — SettingsStore (prefix: `settings:`)
- `channels.go` — ChannelStore (prefix: `channels:`) + GroupStore (prefix: `groups:`)
- `epg.go` — EPGSourceStore + ProgramStore (prefix: `programs:`)
- `recordings.go` — RecordingStore (prefix: `recordings:`)
- `favorites.go` — FavoriteStore (prefix: `favorites:`)
- `users.go` — UserStore (prefix: `users:`)
- `clients.go` — ClientStore (prefix: `clients:`)
- `apikeys.go` — APIKeyStore (prefix: `apikeys:`, per-user prefix scan)
- `invites.go` — InviteStore (prefix: `invites:`)
- `source_configs.go` — SourceConfigStore (prefix: `sourceconfigs:`, per-type prefix scan)
- `probe_cache.go` — ProbeCacheStore (prefix: `probecache:`)
- `source_profiles.go` — SourceProfileStore (flat keys, no migration yet)
- `hdhr_devices.go` — HDHRDeviceStore (flat keys, no migration yet)
