# pkg/output/bridge — DecodeBridge (decode → process → encode)

## Purpose
Sits between the demuxer and the FanOut when transcoding is required. Decodes compressed packets, applies video processing (deinterlace, scale, bit-depth conversion), re-encodes, and pushes encoded packets to the downstream PacketSink (typically a FanOut).

## Responsibilities
- Own the video decoder, audio decoder, resampler, AudioFIFO, encoders
- Apply deinterlacer when source is interlaced
- Apply scaler when height reduction or bit-depth conversion needed
- Handle framerate correctly (50fps for deinterlaced 1080i/50)
- ResetForSeek: flush all decoders, resampler, AudioFIFO, encoders
- Pass through video in copy mode (no bridge needed)

## Does NOT
- Know about delivery formats (MSE, HLS, stream) — it just outputs encoded packets
- Serve HTTP — that's the output plugin's job
- Manage sessions — that's the session manager

## Key Design
```
Copy mode:       Demuxer → FanOut → [MSE, HLS, Recording, ...]
AudioOnly mode:  Demuxer → DecodeBridge (video passthrough, audio decode/resample/encode) → FanOut → [...]
Full transcode:  Demuxer → DecodeBridge → FanOut → [MSE, HLS, Recording, ...]
```

The DecodeBridge implements av.PacketSink so it slots into the same position as the FanOut in the demuxloop chain.

**AudioOnly mode**: When `AudioOnly: true`, video packets pass through untouched while audio goes through the full decode -> resample -> AudioFIFO -> encode chain. Used when the video codec can be copied but audio needs transcoding.

**Encoder extradata accessors**: `VideoEncoderExtradata()`, `VideoEncoderCodecID()`, `AudioEncoderExtradata()`, `AudioEncoderCodecID()` expose the encoder's extradata and codec IDs. Returns nil/0 when the respective encoder is not active (e.g. copy mode).

## Reference Implementation
Extracted from the decode/encode chains duplicated across tvproxy's 6 pipeline types in gopipeline.go. One implementation replaces six.
