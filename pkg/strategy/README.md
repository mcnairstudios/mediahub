# strategy

Decision engine that compares source probe data against client profile requirements and resolves the pipeline mode: copy, remux, or transcode.

## Usage

```go
d := strategy.Resolve(
    strategy.Input{
        VideoCodec: "h264",
        AudioCodec: "ac3",
        Width:      1920,
        Height:     1080,
        Interlaced: false,
        BitDepth:   8,
    },
    strategy.Output{
        VideoCodec:   "copy",
        AudioCodec:   "aac",
        Container:    "mpegts",
        HWAccel:      "vaapi",
        OutputHeight: 0,
        MaxBitDepth:  0,
    },
)
// d.NeedsTranscode == false (video copy)
// d.NeedsAudioTranscode == true (ac3 -> aac)
// d.VideoCodec == media.VideoCopy
// d.AudioCodec == media.AudioAAC
```

## Rules

- `"copy"` passes the source codec through untouched
- `"default"` means match source, not a global setting — resolves to copy when possible
- `OutputHeight` is a ceiling — only triggers transcode when source exceeds it
- `MaxBitDepth` forces transcode when source bit depth exceeds the limit
- Interlaced sources always trigger transcode (deinterlace required)
- Hardware acceleration is only set when video transcode is needed
- Audio decision is independent of video decision
