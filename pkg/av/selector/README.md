# pkg/av/selector

Audio track selection from a list of available tracks. Applies language preference, audio description (AD) filtering, codec priority, and channel count to pick the best track.

## Selection Priority

1. Filter out AD tracks (unless all tracks are AD)
2. Match preferred language (if set; if no match, keep all candidates)
3. Prefer higher codec priority: AAC > MP2 > AC3 > EAC3 > DTS > unknown
4. Among same codec, prefer higher channel count (5.1 > stereo)

## Usage

```go
idx := selector.SelectAudio(probeResult.AudioTracks, selector.AudioPrefs{Language: "en"})
```

Returns the selected track's `Index` field, or `-1` if no tracks are available.

## No CGO

Pure Go. Depends only on `pkg/media.AudioTrack`.
