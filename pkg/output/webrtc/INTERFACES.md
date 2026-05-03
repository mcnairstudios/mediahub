# WebRTC Plugin Interfaces

## Implemented Interfaces

### output.OutputPlugin
- `Mode() DeliveryMode` — returns `DeliveryWebRTC`
- `PushVideo(data []byte, pts, dts int64, keyframe bool) error` — packetizes H.264/HEVC NALUs as RTP and writes to video track
- `PushAudio(data []byte, pts, dts int64) error` — packetizes audio as RTP and writes to audio track
- `PushSubtitle(data []byte, pts int64, duration int64) error` — no-op (WebRTC does not carry subtitles)
- `EndOfStream()` — closes the PeerConnection
- `ResetForSeek()` — resets RTP sequence numbers, timestamps, PTS base; increments generation counter
- `Stop()` — closes PeerConnection and cleans up tracks
- `Status() PluginStatus` — reports connection health and bytes written

### output.ServablePlugin
- `ServeHTTP(w http.ResponseWriter, r *http.Request)` — WHEP endpoint (POST for offer/answer, DELETE for teardown)
- `Generation() int64` — returns current generation counter
- `WaitReady(ctx context.Context) error` — blocks until a peer connection is established

## Constructor

```go
func New(cfg output.PluginConfig) (output.OutputPlugin, error)
```

Takes the standard PluginConfig. Reads `Video.Codec` to select H.264 or HEVC packetization, and `Video.FPS()` for fallback frame rate. The PeerConnection is created lazily on the first WHEP POST request, not at construction time.

## HTTP Endpoints (via ServeHTTP)

| Method | Description | Response |
|--------|-------------|----------|
| POST | SDP offer → SDP answer | 201 Created, body = SDP answer |
| DELETE | Tear down connection | 204 No Content |
