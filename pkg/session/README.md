# pkg/session — Session Manager

## Purpose
Manages active media sessions. One session per stream. Multiple users watching the same stream share one session (one decode, shared FanOut).

## Responsibilities
- Create and manage sessions keyed by streamID
- Attach/detach output plugins to a session's FanOut at runtime
- Track session lifecycle (create, seek, stop, cleanup)
- Preserve recorded sessions on cleanup, delete non-recorded ones

## Key Design
- Keyed by streamID (not channelID) — a channel is presentation, a stream is the source
- A new user joining an existing stream gets the same session — their output plugin is added to the FanOut
- The session owns a FanOut but does NOT own the decode/encode pipeline — that's a separate concern
- Recording flag controls whether files are preserved on session close

## Does NOT
- Demux, decode, or encode media — that's the AV pipeline layer
- Know about source plugins, client profiles, or HTTP routing
- Manage channels, EPG, or metadata
