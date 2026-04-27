# channel

Channels are user-curated collections of streams. They organise media for presentation to clients (Jellyfin, DLNA, HDHR emulation, browser).

A channel maps to EPG data via `TvgID` and can have multiple streams assigned for failover (first stream is primary).

## Boundaries

- References streams by ID string only — no dependency on `pkg/media/` or `pkg/store/`
- Store and GroupStore are interfaces — implementations live elsewhere
- stdlib only, zero external dependencies
