# keyframe — Interfaces

## Public Functions

```go
func IsKeyframe(data []byte, codec string) bool
```
Returns true if the packet data contains a keyframe (IDR) NAL unit. Supported codecs: `"h264"`, `"hevc"`, `"h265"`. Returns false for unknown codecs or empty data.

```go
func FixDeltaUnit(data []byte, codec string) bool
```
Returns true if the packet is a delta (non-keyframe) unit. Inverse of IsKeyframe.

```go
func NewKeyframeTracker(isLive bool) *KeyframeTracker
```
Creates a tracker. Live mode never drops packets. VOD mode drops all packets until the first keyframe is seen.

## KeyframeTracker Methods

```go
func (t *KeyframeTracker) ShouldDrop(data []byte, codec string) bool
```
Returns true if the packet should be dropped. In VOD mode, drops everything before the first IDR. In live mode, always returns false.

```go
func (t *KeyframeTracker) Reset()
```
Resets the tracker state. After reset, VOD mode will again drop packets until the next keyframe. Call after seek operations.

## Internal Functions (unexported)

| Function | Purpose |
|----------|---------|
| `isKeyframeH264(nalus)` | Checks NAL type 5 (IDR slice) |
| `isKeyframeH265(nalus)` | Checks NAL types 16-21 (IRAP range) |
| `splitNALUnits(data)` | Annex B start code splitting (3 and 4 byte) |
| `trimTrailingZeros(data)` | Strips padding zeros between NAL units |
