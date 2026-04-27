# Encoder Interfaces

## Encoder

The `Encoder` struct wraps a libavcodec encoder context with optional hardware acceleration.

### Construction

- `NewVideoEncoder(EncodeOpts)` -- video encoder with HW frames context setup (VAAPI surface upload, QSV, NVENC, VideoToolbox)
- `NewAudioEncoder(AudioEncodeOpts)` -- audio encoder from codec name
- `NewAACEncoder(channels, sampleRate)` -- convenience AAC encoder

### Methods

- `Encode(frame) -> packets` -- encode one frame, returns zero or more compressed packets. Auto-uploads SW frames to HW surface when hwFramesCtx is set.
- `Flush() -> packets` -- drain buffered frames at end of stream
- `Extradata() -> bytes` -- codec-specific init data (SPS/PPS, AudioSpecificConfig)
- `FrameSize() -> int` -- audio encoder's required input frame size
- `Close()` -- free all resources (idempotent)

## AudioFIFO

Buffers variable-size decoded audio frames to the encoder's required frame size.

### Construction

- `NewAudioFIFOFromEncoder(encoder, channels, layout, rate)` -- derives frame size from encoder
- `NewAudioFIFO(encoder, frameSize, channels, sampleFmt, layout, rate)` -- explicit parameters

### Methods

- `Write(frame) -> packets` -- buffer input frame, encode complete frames, return packets. PTS interpolated from input frame timestamps.
- `Reset()` -- clear FIFO and PTS state (call on seek)
- `Close()` -- free FIFO buffer

## Resolution Functions

- `ResolveEncoderName(EncodeOpts) -> (string, error)` -- codec+hwaccel to ffmpeg encoder name
- `ResolveAudioEncoderName(codec) -> string` -- friendly codec name to ffmpeg encoder name
- `ProbeMaxBitDepth(hwaccel) -> int` -- probe HW device bit depth support (cached)
