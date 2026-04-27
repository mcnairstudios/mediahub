# pkg/output — Output Plugin System

## Purpose
Defines the contract for delivery mechanisms that transport media to clients. Each output plugin receives encoded packets and delivers them in a format-specific way (MSE segments, HLS playlists, raw streams, recordings).

## Responsibilities
- Define the `OutputPlugin` interface for all delivery modes
- Define `ServablePlugin` for HTTP-served delivery (MSE, HLS)
- Provide the `FanOut` distributor — one decode fans out to N output plugins simultaneously
- Maintain a `Registry` of plugin factories for creating outputs by delivery mode
- Error isolation — one plugin failing does not kill others in the FanOut

## Key Design
- The FanOut supports runtime Add/Remove — a recording can start mid-stream
- Plugins are independent — changing HLS cannot break MSE or recording
- Each plugin owns its own muxer and output lifecycle

## Does NOT
- Decode or encode video/audio — that's the DecodeBridge's job
- Know about source plugins or stream discovery
- Manage sessions — that's pkg/session
