# youtube

Resolves YouTube watch URLs to direct streamable URLs using [github.com/kkdai/youtube/v2](https://github.com/kkdai/youtube).

## Usage

```go
if youtube.IsYouTubeURL(streamURL) {
    resolved, err := youtube.ResolveStreamURL(ctx, streamURL)
    if err != nil {
        return err
    }
    streamURL = resolved
}
```

## Design

- **Not a source plugin** -- this is a utility used by the session pipeline layer.
- Source plugins (e.g. SpaceX) store canonical YouTube URLs. Resolution happens at pipeline open time, just before the demuxer.
- Prefers progressive mp4 formats (video + audio) at up to 1080p.
- Resolved URLs are temporary (YouTube signs them with expiry). This is fine for playback sessions; retries re-resolve.

## Supported URL formats

- `https://www.youtube.com/watch?v=VIDEO_ID`
- `https://youtu.be/VIDEO_ID`
- `https://www.youtube.com/shorts/VIDEO_ID`
- `https://www.youtube.com/live/VIDEO_ID`
- `m.youtube.com` and `youtube.com` (without www) variants
