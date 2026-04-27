# media -- Interfaces

No interfaces. This package defines shared types used across the system.

## Codec Types

```go
type VideoCodec string   // h264, h265, av1, mpeg2video, copy
type AudioCodec string   // aac, ac3, mp2, mp3, opus, copy
type Container  string   // mp4, mpegts, mkv
```

## Normalization Functions

```go
func NormalizeVideoCodec(s string) VideoCodec
func NormalizeAudioCodec(s string) AudioCodec
func BaseVideoCodec(vc VideoCodec) string
```

| Function | Purpose |
|----------|---------|
| `NormalizeVideoCodec` | Maps aliases to canonical form (hevc/hvc1/hev1 -> h265, avc/avc1 -> h264) |
| `NormalizeAudioCodec` | Maps aliases to canonical form (aac_latm -> aac, eac3 -> ac3) |
| `BaseVideoCodec` | Returns the base codec string without variant suffixes |

## Stream

Unified representation of any media stream (live or VOD).

```go
type Stream struct {
    ID, SourceType, SourceID, Name, URL, Group       string
    TvgID, TvgName, TvgLogo                          string
    IsActive                                          bool
    VideoCodec, AudioCodec                            string
    Width, Height, BitDepth                           int
    Interlaced                                        bool
    FramerateN, FramerateD                            int
    Duration                                          float64
}
```

## ProbeResult

Output of stream analysis.

```go
type ProbeResult struct {
    Video       *VideoInfo
    AudioTracks []AudioTrack
    SubTracks   []SubtitleTrack
    DurationMs  int64
}
```

### VideoInfo

```go
type VideoInfo struct {
    Index, Width, Height, BitDepth, FramerateN, FramerateD int
    Codec, Profile, PixFmt                                  string
    Interlaced                                              bool
    Extradata                                               []byte
}

func (vi *VideoInfo) FPS() float64
```

### AudioTrack

```go
type AudioTrack struct {
    Index, Channels, SampleRate, BitRate int
    Codec, Language                      string
    IsAD                                 bool
}
```

### SubtitleTrack

```go
type SubtitleTrack struct {
    Index    int
    Codec    string
    Language string
}
```
