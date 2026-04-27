# keyframe

NAL-level keyframe (IDR) detection for H.264 and H.265/HEVC video streams, plus a stateful tracker for VOD pre-IDR frame dropping.

## What it does

- **IsKeyframe** — parses Annex B NAL units from raw packet data, returns true if any NAL is an IDR frame (H.264 NAL type 5, H.265 NAL types 16-21)
- **FixDeltaUnit** — returns true if the packet is a delta (non-keyframe) unit
- **KeyframeTracker** — stateful tracker with two modes:
  - **Live mode** — never drops packets (streams start mid-GOP and that's fine)
  - **VOD mode** — drops all packets before the first keyframe (prevents decoder artifacts from partial GOPs after seek)

## NAL unit parsing

Handles both 3-byte (`0x000001`) and 4-byte (`0x00000001`) Annex B start codes. Splits the byte stream into individual NAL units and inspects the NAL type byte:

- H.264: `nalu[0] & 0x1F == 5` (IDR slice)
- H.265: `(nalu[0] >> 1) & 0x3F` in range 16-21 (BLA_W_LP through RSV_IRAP_VCL23)

## Usage

```go
tracker := keyframe.NewKeyframeTracker(false) // VOD mode

for pkt := range packets {
    if tracker.ShouldDrop(pkt.Data, "h264") {
        continue // pre-IDR frame, skip
    }
    // process packet
}

// After seek, reset to drop pre-IDR frames again
tracker.Reset()
```

## Pure Go

No CGO dependencies. Self-contained NAL unit splitting (no external extradata package required).
