# pkg/connectivity — Connectivity Plugin System

## Purpose
Provides tunnel/proxy options for routing upstream traffic. The source doesn't know or care how the connection is made — it just gets a rewritten URL through a local proxy.

## Current: WireGuard
Runs a localhost HTTP proxy that routes through a WireGuard tunnel. Sources set `UseWireGuard=true` and the proxy rewrites their URL to `http://127.0.0.1:{port}/?url={original}`.

## Future Options
- Mesh networking (e.g. Nebula, ZeroTier)
- Tailscale
- Tor
- SSH tunnels
- Direct (no proxy — pass through)

## Key Design
- Each plugin runs a localhost HTTP proxy on a random port
- Sources just get a rewritten URL — they don't know about tunnels
- Testing connectivity = HTTP request to the localhost proxy
- The container only needs the tunnel interface — everything else is localhost

## Does NOT
- Know about sources, outputs, or media
- Manage sessions or playback
- Handle authentication (that's the upstream's job)
