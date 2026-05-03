# Playback Plan

## Problem

Playback is broken. The server decides codec/delivery and the frontend has to guess what it's getting. This leads to mismatches — H.265 sent to a browser expecting H.264, HLS sent when MSE was working, etc.

## Principle

The **frontend controls playback choices**. It knows what the browser supports. It requests the format it wants. The server delivers what's asked for.

## Architecture

### 1. Playback is pluggable

Each playback method is a plugin:
- **MSE** — fMP4 segments, browser SourceBuffer
- **HLS** — MPEG-TS segments, hls.js or native
- **DASH** — future, MPD manifest + segments
- **WebRTC** — future, ultra-low latency
- **Native** — direct video src= for simple formats

The frontend picks the plugin based on what the browser supports and what the user/client profile prefers.

### 2. Frontend controls the request

The frontend knows:
- What codecs the browser supports (MediaSource.isTypeSupported)
- What delivery modes work (MSE, HLS native, HLS.js)
- What the user's client profile says

So the frontend tells the server:
- `delivery=mse` or `delivery=hls` or `delivery=stream`
- `video_codec=h264` or `video_codec=h265` (what it can accept)
- `audio_codec=aac` (always for browser)
- `container=fmp4` or `container=mpegts`

The server's job is just to deliver what's requested. No guessing.

### 3. Client profile = player preset

Changing client profile in the UI changes how the player is created:
- Browser profile → MSE with fMP4, H.264/AAC
- Browser H.265 → MSE with fMP4, H.265/AAC (if browser supports)
- HLS profile → hls.js player, MPEG-TS segments
- etc.

The profile is a frontend concern that maps to request parameters.

## What was working

- MSE (fMP4) — was working for browser playback
- HLS — was working for Jellyfin/Apple TV

## What broke

- Agent modified `pkg/output/hls/hls.go` and `pkg/orchestrator/playback.go` in commit b6f8e65
- Changed extradata fallback, audio channel defaults, timebase handling, hasAudio detection
- HLS stopped initialising in browser
- Frontend started requesting HLS when MSE was the working path

## Immediate fix

1. Verify MSE still works (it was working before)
2. Verify HLS works for Jellyfin (not browser — Jellyfin uses its own player)
3. Don't mix them — MSE for browser, HLS for Jellyfin

## Implementation order

1. Fix current breakage — get MSE browser playback working again
2. Frontend codec detection — `MediaSource.isTypeSupported` probing on startup
3. Frontend delivery selection — request params based on what browser supports
4. Server respects request — no server-side override of what frontend asks for
5. DASH output plugin — new delivery mode
6. WebRTC output plugin — new delivery mode, signalling server
