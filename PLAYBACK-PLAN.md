# Playback Plan

## Current Problem

The server decides delivery mode and codec. The frontend gets whatever the server gives it and tries to play it. You end up with HLS segments being fed to an MSE player, or H.265 being sent to a browser that only handles H.264.

## How Playback Should Work

The server launches the frontend — so the backend still decides what's available (client profiles, source profiles, what outputs exist). But by the time the frontend hits a playback URL, it knows what it can play and the backend has no choice but to obey.

### The Flow

```
1. User clicks Play
2. Frontend checks: what can I play?
   - MediaSource.isTypeSupported('video/mp4; codecs="avc1.640028"') → true/false
   - MediaSource.isTypeSupported('video/mp4; codecs="hev1.1.6.L120"') → true/false
   - Can I do HLS natively? (Safari yes, Chrome via hls.js)
   - Can I do DASH? (via dash.js)
   - Can I do WebRTC? (RTCPeerConnection available)
3. Frontend picks the best option from what it supports
4. Frontend creates the player for that option
5. Frontend requests the matching URL from the server:
   - MSE:    GET /play/{id}?delivery=mse&video=h264&audio=aac&container=fmp4
   - HLS:    GET /play/{id}?delivery=hls&video=h264&audio=aac
   - DASH:   GET /play/{id}?delivery=dash&video=h264&audio=aac
   - WebRTC: POST /play/{id}/whep (WebRTC signalling)
6. Server creates a session matching exactly what was asked for
7. Player consumes the output
```

The frontend and the server are **always aligned** because the frontend chose both sides — it created the player AND told the server what to produce.

### Frontend Player Plugins

Each delivery mode has a player plugin in app.js:

| Plugin | Creates | Consumes | Container |
|--------|---------|----------|-----------|
| MSEPlayer | MediaSource + SourceBuffer | polls fMP4 segments | fMP4 |
| HLSPlayer | hls.js instance or native | m3u8 playlist + TS segments | MPEG-TS |
| DASHPlayer | dash.js instance | MPD manifest + segments | fMP4 |
| WebRTCPlayer | RTCPeerConnection | RTP via WHEP | RTP |
| DirectPlayer | video.src = url | raw stream | mp4/ts |

The frontend has ONE play function:

```
function play(streamID) {
    var caps = detectCapabilities();  // run once, cached
    var player = pickBestPlayer(caps); // MSEPlayer, HLSPlayer, etc.
    var params = player.serverParams(); // {delivery:'mse', video:'h264', ...}
    var url = '/play/' + streamID + '?' + encode(params);
    player.start(videoElement, url);
}
```

### Server Output Plugins (unchanged)

The server already has output plugins (MSE, HLS, Stream, Record). The only change is: the server uses the delivery/codec params from the request instead of deciding itself. The strategy layer resolves copy vs transcode based on what the frontend asked for vs what the source provides.

### Client Profiles Become Player Presets

A "client profile" in the UI is just a saved set of player preferences:

- **Browser** → MSEPlayer, H.264, AAC, fMP4
- **Browser H.265** → MSEPlayer, H.265, AAC, fMP4  
- **HLS** → HLSPlayer, H.264, AAC, MPEG-TS
- **Low Latency** → WebRTCPlayer (future)
- **Adaptive** → DASHPlayer (future)

Switching profile = switching which player plugin is used + what params are sent to the server. The profile dropdown is just a shortcut — advanced users could override individual settings.

### Capability Detection (run once)

```
var capabilities = {
    mse: !!window.MediaSource,
    mse_h264: MediaSource.isTypeSupported('video/mp4; codecs="avc1.640028"'),
    mse_h265: MediaSource.isTypeSupported('video/mp4; codecs="hev1.1.6.L120"'),
    mse_av1:  MediaSource.isTypeSupported('video/mp4; codecs="av01.0.08M.08"'),
    hls_native: videoElement.canPlayType('application/vnd.apple.mpegurl') !== '',
    hls_js: typeof Hls !== 'undefined' && Hls.isSupported(),
    webrtc: !!window.RTCPeerConnection,
};
```

This runs once on page load. The result determines which players are available.

### Why This Fixes Everything

- No mismatch: frontend creates the player THEN tells the server what to produce
- No guessing: server doesn't decide codec — it was told
- Adding DASH = add DASHPlayer plugin in frontend + DASH output plugin on server
- Adding WebRTC = add WebRTCPlayer plugin + WHEP signalling endpoint
- Existing MSE/HLS unchanged — they become player plugins with the same interface
- Jellyfin/Plex/DLNA are external clients that make their own requests — unaffected

## Implementation Order

1. **Refactor frontend play()** — extract current MSE player into MSEPlayer plugin
2. **Add capability detection** — probe browser on load, cache results
3. **Pass delivery params in request** — frontend sends what it wants
4. **Server reads params** — use request params instead of server-side strategy for delivery/codec
5. **Extract HLSPlayer** — hls.js based player plugin for browser HLS
6. **DASH output plugin** — server side MPD + segment production
7. **DASHPlayer** — dash.js based frontend plugin
8. **WebRTC output plugin** — WHEP/WHIP signalling + RTP
9. **WebRTCPlayer** — RTCPeerConnection frontend plugin

## Rules

- Frontend creates the player FIRST, then requests the matching stream
- Server never overrides what the frontend asked for
- One play() function, one interface, N player plugins
- Capability detection is cached, not per-play
- Pipeline code (pkg/session, pkg/output, pkg/lib/av) is NOT touched by agents
