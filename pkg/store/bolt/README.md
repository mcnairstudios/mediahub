# pkg/store/bolt — Bolt-backed persistent stores

## Purpose
Production persistence layer using bbolt (embedded key-value store). Replaces the in-memory stores for deployment. All data survives restarts.

## Status
StreamStore and SettingsStore implemented. Channel, EPG, and recording stores planned.

## Responsibilities
- Implement StreamStore, SettingsStore using bbolt buckets
- JSON serialization for complex values (streams), plain strings for settings
- Thread-safe (bbolt handles this internally)
- Data directory configurable via bolt.Open(path)

## Does NOT
- Define interfaces — those live in pkg/store/interfaces.go
- Handle migrations (data format is JSON in bolt, self-describing)

## Key Design
- Single bolt database file with multiple buckets ("streams", "settings")
- Stream values stored as JSON, settings values as plain strings
- Keys are string IDs
- Bulk operations use bolt transactions for atomicity
- BulkUpsert is a single transaction
- DeleteStaleBySource scans and deletes in one transaction
- Save() is a no-op (bolt persists on every write)

## Files
- `bolt.go` — DB struct, Open/Close, bucket creation, store accessors
- `streams.go` — StreamStore implementation (Get, List, ListBySource, BulkUpsert, DeleteBySource, DeleteStaleBySource, Save)
- `settings.go` — SettingsStore implementation (Get, Set, List)
- `bolt_test.go` — full test suite mirroring memory store tests + persistence test
