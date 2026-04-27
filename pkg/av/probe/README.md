# pkg/av/probe

Probes a media file or URL to extract stream information using libavformat (via go-astiav). Returns codec, resolution, frame rate, bit depth, interlace status, audio tracks, and duration as a `*media.ProbeResult`.

## Usage

```go
result, err := probe.Probe("http://example.com/stream.ts", 10)
// result.Video.Codec, result.Video.Width, result.AudioTracks, etc.
```

For playback pipelines, prefer calling `demuxer.StreamInfo()` on an already-open demuxer instead of `Probe()` to avoid opening the URL twice.

`ExtractProbeResult(fc)` extracts info from an existing `*astiav.FormatContext` for use by the demuxer package.

## Build

Requires CGO and ffmpeg development libraries (libavformat, libavcodec, libavutil).

```bash
CGO_ENABLED=1 go test ./pkg/av/probe/... -v
```

Set `AVPROBE_TEST_FILE=/path/to/media.mkv` for integration tests with real media.
