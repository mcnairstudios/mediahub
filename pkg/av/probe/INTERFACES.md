# pkg/av/probe — Interfaces

## Public API

### Probe(url string, timeoutSec int) (*media.ProbeResult, error)
Opens a media URL, probes stream info, closes connection. Standalone use for metadata/library scanning.

### ExtractProbeResult(fc *astiav.FormatContext) *media.ProbeResult
Extracts stream info from an already-open FormatContext. Used by the demuxer to avoid double-opening.

## Dependencies

- `github.com/asticode/go-astiav` — CGO bindings to libavformat/libavcodec
- `pkg/media` — ProbeResult, VideoInfo, AudioTrack types
- `pkg/av/extradata` — SplitNALUnits, ParseH264SPS for interlace detection

## Output Type

Returns `*media.ProbeResult` with:
- `Video *media.VideoInfo` — codec, resolution, bit depth, interlace, frame rate, profile, pixel format, extradata
- `AudioTracks []media.AudioTrack` — per-track codec, channels, sample rate, language, AD flag, bitrate
- `DurationMs int64` — duration in milliseconds (0 or negative for live streams)
