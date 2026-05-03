# pkg/client

Client detection and profile resolution. Identifies who is requesting media (browser, Plex, Jellyfin, TV, Quest) from HTTP request metadata and maps them to output configuration.

## Design

Detection is based on two signals:

- **Port** — which listener the request arrived on (e.g. 8096 for Jellyfin clients)
- **Headers** — User-Agent and custom headers matched via rules

Each `Client` has a priority and a list of `MatchRule` entries. Rules use AND logic: all rules in a client must match for that client to be selected. The highest-priority matching client wins.

Match types: `contains`, `prefix`, `exact`, `regex`.

A `ListenPort` of 0 means the client can match on any port. A client with no `MatchRules` matches purely on port.

## Usage

```go
detector := client.NewDetector(clients)
matched := detector.Detect(8080, map[string]string{
    "User-Agent": "Mozilla/5.0 ...",
})
if matched != nil {
    // use matched.StreamProfileID to look up output profile
}
```

## Delivery Resolution

A client profile's delivery mode determines what the server produces:

- **Forced mode** (e.g. `mse`, `hls`, `dash`) — the pipeline produces exactly that format. The frontend creates the matching player.
- **`user`** (User Choice mode) — the frontend runs capability detection and presents a delivery dropdown showing only the modes the browser supports. The user picks, and the frontend sends `?delivery=mse` (or hls, dash, webrtc) on the playback request. The server creates a pipeline matching the user's selection.

User Choice mode lets admins give users freedom to experiment with delivery modes while forced modes lock clients to a known-good configuration.

## Dependencies

None. stdlib only. Does not depend on `net/http`, `pkg/output/`, or `pkg/source/`.
