# pkg/store — Persistence Interfaces

## Purpose
Defines the storage contracts for the media cloud. Interfaces allow swapping implementations (in-memory for testing, bolt for production, JSON for config).

## Responsibilities
- Define `StreamStore` interface for stream CRUD + source-based queries
- Define `SettingsStore` interface for key-value settings
- Provide `MemoryStreamStore` and `MemorySettingsStore` for testing and MVP
- All stores are thread-safe

## Key Design
- Streams are queried by `SourceType + SourceID` — one unified query pattern for all source types
- `DeleteStaleBySource` returns deleted IDs for downstream cleanup (channel mapping removal, etc.)

## Does NOT
- Implement production persistence (bolt, JSON files) — those are separate implementations
- Know about specific source types or delivery modes
