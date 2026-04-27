# pkg/av/mux

Muxing wrappers around go-astiav (libavformat) for three output modes:

## Muxers

### FragmentedMuxer
CMAF fragmented MP4 output for MSE browser playback. Separate video and audio tracks, each with their own ffmpeg muxer. Video fragments flush on keyframes, audio fragments flush at configurable intervals (default 2048ms). All segment writes are atomic (.tmp + rename).

Output layout:
```
init_video.mp4, init_audio.mp4   (ftyp + moov)
video_0001.m4s, audio_0001.m4s   (moof + mdat)
```

### StreamMuxer
Stream-copy remuxing to an io.Writer. Used for DLNA streaming and direct HTTP delivery where output goes to a response body. Supports any container format (mpegts, mp4, etc.).

### HLSMuxer
Native libavformat HLS muxer producing MPEG-TS segments with an m3u8 playlist. Uses `hls_time`, `hls_segment_filename`, `hls_list_size=0`, `hls_flags=append_list`. Includes packet duration fixing for video (frame rate based) and audio (frame size based), plus RescaleTs from input to output timebase.

## Key behaviours

- **WriteFrame not WriteInterleavedFrame**: FragmentedMuxer uses `WriteFrame` (single-stream tracks). `WriteInterleavedFrame` rejects duplicate DTS from B-frames.
- **Fragment flush**: `WriteFrame(nil)` for fMP4 fragment boundaries.
- **Monotonic DTS**: B-frame reordering can produce duplicate DTS; `ensureMonotonicDTS` adjusts to keep strictly increasing.
- **Seek Reset**: `Reset()` rebuilds track muxers to clear ffmpeg's internal cur_dts state. Segment numbering continues.
- **Duration fixing**: HLSMuxer fixes zero-duration packets using VideoFrameRate and AudioFrameSize from opts.
- **Codec string extraction**: `VideoCodecString()` parses the init segment for avc1/hev1/hvc1 codec strings.

## Testing

```bash
CGO_ENABLED=1 go test ./pkg/av/mux/... -v
```

Tests that require libx264 or aac encoders will skip if not available.
