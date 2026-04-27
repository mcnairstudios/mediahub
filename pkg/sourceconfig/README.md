# pkg/sourceconfig -- Source Configuration

## Purpose
Defines the data model and store interface for source configurations. A source config represents a configured input source (M3U account, SAT>IP server, HDHomeRun device, etc.) with type-specific key-value settings.

## Responsibilities
- Define the `SourceConfig` struct (ID, Type, Name, IsEnabled, Config map)
- Define the `Store` interface for CRUD + type-filtered listing
- Provide an in-memory `MemoryStore` implementation for testing

## Does NOT
- Perform any source-specific logic (refresh, parsing, connection) -- that lives in pkg/source/
- Persist data on disk -- bolt and other backends implement the Store interface externally
