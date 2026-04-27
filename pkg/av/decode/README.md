# pkg/av/decode

Video and audio decoder with hardware acceleration support and software fallback. Wraps libavcodec via go-astiav for in-process decoding with automatic HW-to-SW frame transfer.

## Features

- Video decoding with HW acceleration (VAAPI, QSV, VideoToolbox, CUDA/NVENC, D3D11VA, DXVA2, Vulkan)
- Automatic fallback to software decoding when HW init fails or codec open fails
- MaxBitDepth enforcement -- forces SW decode when source exceeds HW bit depth limit (e.g. Intel A380 8-bit BAR constraint)
- Audio decoding for any libavcodec-supported codec
- FlushBuffers via CGO (avcodec_flush_buffers) for seek support
- Automatic HW frame transfer to system memory on decode output
- Construction from CodecParameters (demuxer) or CodecID + extradata (manual)

## Functions

- **NewVideoDecoderFromParams** -- Create video decoder from CodecParameters with optional HW acceleration
- **NewVideoDecoder** -- Create video decoder from CodecID and extradata with optional HW acceleration
- **NewAudioDecoderFromParams** -- Create audio decoder from CodecParameters
- **NewAudioDecoder** -- Create audio decoder from CodecID and extradata
- **Decode** -- Send packet, receive decoded frames (handles HW frame transfer)
- **Flush** -- Drain buffered frames from decoder
- **FlushBuffers** -- Reset decoder state via avcodec_flush_buffers (for seek)
- **Close** -- Free decoder resources
- **HWAccelMap** -- Returns copy of supported HW acceleration name-to-type mapping
- **BitDepthFromPixelFormat** -- Extract bit depth from pixel format name
- **ExceedsMaxBitDepth** -- Check if pixel format exceeds a bit depth limit

## Build

Requires libavcodec dev headers (CGO):

```bash
CGO_ENABLED=1 go test ./pkg/av/decode/... -v
```
