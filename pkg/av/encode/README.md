# encode

Video and audio encoding via libavcodec (go-astiav bindings).

## Video Encoding

`NewVideoEncoder(opts)` creates a hardware-accelerated or software video encoder. The encoder table maps codec+hwaccel combinations to ffmpeg encoder names. Hardware acceleration platforms: VAAPI, QSV, NVENC, VideoToolbox. Automatic software fallback when HW init fails.

VAAPI surface upload: when the input frame is software (e.g. yuv420p) and the encoder has a HW frames context, `Encode()` automatically uploads via `TransferHardwareData` before encoding.

`EncoderName` on `EncodeOpts` overrides the table lookup entirely (for explicit encoder selection).

`Framerate` defaults to 25 if unset.

## Audio Encoding

`NewAudioEncoder(opts)` creates an audio encoder from codec name. The audio encoder map resolves friendly names to ffmpeg encoder names (opus -> libopus, mp3 -> libmp3lame, etc.).

`NewAACEncoder(channels, sampleRate)` is a convenience wrapper.

## AudioFIFO

Decoder output frame sizes vary by codec (DTS=512, FLAC=4096, AAC=1024). AudioFIFO buffers decoded audio to the encoder's required frame size. Never call the encoder directly for audio -- always go through AudioFIFO.

PTS interpolation uses a queue of input frame PTS+sample-offset pairs. Output frame PTS is computed by finding the most recent input entry at or before the output sample position, then adding the sample delta. This preserves accurate timestamps across seek boundaries and variable-size input frames.

## ProbeMaxBitDepth

Probes a hardware device to determine if it supports 10-bit pixel formats. Returns 8 if the device only supports 8-bit (no format names containing "10"), 0 if 10-bit is supported or the device can't be probed. Results are cached per hwaccel string.

## Testing

```bash
CGO_ENABLED=1 go test ./pkg/av/encode/... -v
```
