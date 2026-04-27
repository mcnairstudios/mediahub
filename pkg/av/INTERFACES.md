# AV Interfaces

## Demuxer

Opens a media source (URI or file) and reads compressed packets sequentially. Returns `io.EOF` at end of stream. Supports seeking (VOD) and audio track switching.

Implementations: libavformat demuxer (subpackage, uses go-astiav).

## Decoder

Takes compressed `Packet` values and produces decoded `Frame` values. May return zero frames (buffering, e.g. yadif deinterlacer needs two fields) or multiple frames per packet. `FlushBuffers()` resets internal state after seek.

Implementations: libavcodec decoder with optional HW acceleration (VideoToolbox, VAAPI, QSV, NVDEC).

## Encoder

Takes decoded `Frame` values and produces compressed `Packet` values. `Flush()` drains remaining buffered frames. `Extradata()` returns codec-specific init data (SPS/PPS for H.264, VPS/SPS/PPS for H.265). `FrameSize()` returns the required audio frame size (needed for AudioFIFO buffering).

Implementations: libavcodec encoder with optional HW acceleration (VAAPI, QSV, NVENC, VideoToolbox).

## Muxer / SegmentedMuxer

Writes compressed packets to an output container format (MPEG-TS, MP4, fMP4). `SegmentedMuxer` adds segment counting and reset (for HLS and MSE fMP4 delivery). `Reset()` recreates internal state after seek without restarting the output.

Implementations: libavformat muxer for MPEG-TS/MP4, custom Go fMP4 muxer, HLS segment muxer.

## Filter

Processes decoded frames. Used for deinterlacing (yadif), scaling (resolution reduction), and pixel format conversion. Returns nil frame when buffering (caller must continue, not error).

Implementations: libavfilter graph wrapper.

## PacketSink

The push interface that the demux loop targets. Output plugins implement this to receive compressed packets without knowing about the demuxer implementation. The `DecodeBridge` also implements this to feed packets into the decode/encode pipeline.

Methods use primitive types (byte slices, int64 timestamps) rather than `*Packet` to keep the boundary simple and avoid package coupling at the push site.
