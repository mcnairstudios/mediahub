# pkg/source — Source Plugin System

## Purpose
Defines the contract for input plugins that provide streams into the media cloud. Any video source (tuner, IPTV playlist, camera, streaming service) implements these interfaces.

## Responsibilities
- Define the `Source` interface that all source plugins must implement
- Provide optional capability interfaces (Discoverable, Retunable, VPNRoutable, etc.)
- Maintain a `Registry` of plugin factories for creating sources by type
- Define `SourceInfo` for unified source listing across all types
- Define `RefreshStatus` for progress reporting during scans/refreshes

## Does NOT
- Implement any specific source (M3U, HDHR, SAT>IP) — those are separate packages
- Persist anything — sources own their own storage
- Know about output delivery or clients
