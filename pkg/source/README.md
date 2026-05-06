# pkg/source — Source Plugin System

## Purpose
Defines the contract for input plugins that provide streams into the media cloud. Any video source (tuner, IPTV playlist, camera, streaming service) implements these interfaces.

## Responsibilities
- Define the `Source` interface that all source plugins must implement
- Provide `BaseSource` embedding type with shared state (Info, SetRefreshResult, SetError, ClearState)
- Provide `HTTPClientFor` helper for WireGuard/default client selection
- Provide optional capability interfaces (Discoverable, Retunable, VPNRoutable, etc.)
- Maintain a `Registry` of plugin factories for creating sources by type
- Define `SourceInfo` for unified source listing across all types
- Define `RefreshStatus` for progress reporting during scans/refreshes

## Plugin System

The source package includes a plugin registration system that allows source types to declare their metadata, configuration fields, custom API routes, and frontend JavaScript.

- **[PLUGIN_GUIDE.md](PLUGIN_GUIDE.md)** — Full development guide with copy-paste examples for creating new source plugins
- **[INTERFACES.md](INTERFACES.md)** — Reference for all interfaces and structs including `PluginDescriptor`, `ConfigField`, `PluginRegistration`, `CustomRoute`, and `Registry` methods

Plugins self-register via `DefaultRegistry.RegisterPlugin()` in an `init()` function. The registry provides the `/api/source-types` endpoint with all available source types and their config fields, so the frontend can render add/edit forms dynamically.

## Does NOT
- Implement any specific source (M3U, HDHR, SAT>IP) — those are separate packages
- Persist anything — sources own their own storage
- Know about output delivery or clients
