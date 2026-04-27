# pkg/av/conv

Conversion layer between mediahub's `av.Packet` / `media.ProbeResult` types and go-astiav (libavformat/libavcodec) native types.

## Functions

- **ToAVPacket** — Converts `av.Packet` (nanosecond timestamps) to `astiav.Packet` (stream timebase). Uses `pkt.FromData()` for correct buffer ownership.
- **CodecIDFromString** — Maps codec name strings (e.g. "h264", "aac", "opus") to `astiav.CodecID`. Case-insensitive.
- **CodecParamsFromVideoProbe** — Creates `astiav.CodecParameters` from `media.VideoInfo` (codec, dimensions, extradata).
- **CodecParamsFromAudioProbe** — Creates `astiav.CodecParameters` from `media.AudioTrack` (codec, sample rate).

## Codec Map

Covers all codecs that probe/demux can return:

- Video: h264, hevc/h265, mpeg2video, mpeg4, vp8, vp9, av1, theora
- Audio: aac, aac_latm, ac3, eac3, dts, mp2, mp3, flac, vorbis, opus, truehd, pcm_s16le
- Subtitle: subrip, ass, webvtt

## Build

Requires CGO and ffmpeg dev libraries:

```bash
CGO_ENABLED=1 go test ./pkg/av/conv/...
```
