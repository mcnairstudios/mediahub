# pkg/store/bolt — Bolt-backed persistent stores

## Purpose
Production persistence layer using bbolt (embedded key-value store). Replaces the in-memory stores for deployment. All data survives restarts.

## Status
All domain stores implemented: Streams, Settings, Channels, Groups, EPG Sources, EPG Programs, Recordings, Users, Source Configs.

## Responsibilities
- Implement all domain store interfaces using bbolt buckets
- JSON serialization for complex values, plain strings for settings
- Thread-safe (bbolt handles this internally)
- Data directory configurable via bolt.Open(path)

## Does NOT
- Define interfaces — those live in pkg/store/interfaces.go
- Handle migrations (data format is JSON in bolt, self-describing)

## Key Design
- Single bolt database file with 9 buckets (streams, settings, channels, groups, epg_sources, epg_programs, recordings, users, source_configs)
- Stream values stored as JSON, settings values as plain strings
- Keys are string IDs
- Bulk operations use bolt transactions for atomicity
- BulkUpsert is a single transaction
- DeleteStaleBySource scans and deletes in one transaction
- Save() is a no-op (bolt persists on every write)

## Files
- `bolt.go` — DB struct, Open/Close, bucket creation, store accessors
- `streams.go` — StreamStore (Get, List, ListBySource, BulkUpsert, DeleteBySource, DeleteStaleBySource, Save)
- `settings.go` — SettingsStore (Get, Set, List)
- `channels.go` — ChannelStore + GroupStore
- `epg.go` — EPGSourceStore + ProgramStore (cursor-based range queries)
- `recordings.go` — RecordingStore (user-filtered List, status queries)
- `users.go` — UserStore (password hash stored alongside user)
- `source_configs.go` — SourceConfigStore (type-filtered listing)
- `bolt_test.go` — full test suite
